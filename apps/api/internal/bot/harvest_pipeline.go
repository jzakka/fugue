package bot

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// imageCacheMaxBytesEnv is the environment variable name used to override
// the default primary image cache size threshold (Decision 5).
const imageCacheMaxBytesEnv = "HARVESTER_IMAGE_CACHE_MAX_BYTES"

// DefaultImageCacheMaxBytes is the default maximum number of bytes the
// primary image cache will download before aborting and falling back to the
// original URL (Decision 5). 20 MiB.
const DefaultImageCacheMaxBytes int64 = 20 * 1024 * 1024

// imageCacheTTLDaysEnv is the environment variable name used to override
// the default TTL (in days) applied to cached primary image objects in the
// images/ storage namespace (harvester-image-cache-ttl Decision D3).
const imageCacheTTLDaysEnv = "HARVESTER_IMAGE_CACHE_TTL_DAYS"

// DefaultImageCacheTTLDays is the default age-based TTL (in days) for
// objects stored under the primary image cache namespace
// (harvester-image-cache-ttl Decision D2). Object removal itself is
// performed asynchronously by the storage bucket's lifecycle rule; this
// application-side value is the single source of truth the lifecycle
// configuration is derived from.
const DefaultImageCacheTTLDays int = 90

// errImageOversize is returned by cacheImage when the candidate exceeds the
// configured size threshold. It triggers the single fallback path along with
// download and upload failures.
var errImageOversize = errors.New("image exceeds size threshold")

// BotDB abstracts the database queries needed by HarvestPipeline.
type BotDB interface {
	BotPinExistsByURL(ctx context.Context, arg db.BotPinExistsByURLParams) (bool, error)
	CreatePin(ctx context.Context, arg db.CreatePinParams) (db.Pin, error)
}

// HarvestPipeline implements Pipeline by deduping, downloading media, and creating Pins.
type HarvestPipeline struct {
	db                 BotDB
	storage            Storage
	client             *http.Client
	imageCacheMaxBytes int64
	imageCacheTTLDays  int
	imageCacheEnabled  bool
	nowUnix            func() int64
}

// HarvestPipelineOption configures a HarvestPipeline.
type HarvestPipelineOption func(*HarvestPipeline)

// WithImageCacheMaxBytes overrides the default primary image size threshold.
func WithImageCacheMaxBytes(n int64) HarvestPipelineOption {
	return func(p *HarvestPipeline) {
		if n > 0 {
			p.imageCacheMaxBytes = n
		}
	}
}

// WithImageCacheTTLDays overrides the default age-based TTL (in days)
// applied to cached primary image objects. Non-positive values are ignored
// (preserving the previously resolved value, env-derived or default), in
// keeping with the WithImageCacheMaxBytes convention.
func WithImageCacheTTLDays(n int) HarvestPipelineOption {
	return func(p *HarvestPipeline) {
		if n > 0 {
			p.imageCacheTTLDays = n
		}
	}
}

// WithImageCacheEnabled toggles primary image caching on or off.
func WithImageCacheEnabled(enabled bool) HarvestPipelineOption {
	return func(p *HarvestPipeline) {
		p.imageCacheEnabled = enabled
	}
}

// NewHarvestPipeline creates a new HarvestPipeline. The default primary
// image cache threshold is DefaultImageCacheMaxBytes (20 MiB); it can be
// overridden via the HARVESTER_IMAGE_CACHE_MAX_BYTES environment variable,
// and then further overridden by the WithImageCacheMaxBytes option.
//
// The default age-based TTL for cached image objects is
// DefaultImageCacheTTLDays (90 days); it can be overridden via the
// HARVESTER_IMAGE_CACHE_TTL_DAYS environment variable. This value is not
// consumed on the Pin-creation hot path — it is held as runtime metadata so
// external configuration (bucket lifecycle rule) can be sourced from it
// (harvester-image-cache-ttl Decision D3/D4).
func NewHarvestPipeline(db BotDB, storage Storage, opts ...HarvestPipelineOption) *HarvestPipeline {
	maxBytes := DefaultImageCacheMaxBytes
	if v := strings.TrimSpace(os.Getenv(imageCacheMaxBytesEnv)); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			maxBytes = parsed
		} else {
			log.Printf("harvest: invalid %s=%q, falling back to default %d bytes", imageCacheMaxBytesEnv, v, maxBytes)
		}
	}
	ttlDays := DefaultImageCacheTTLDays
	if v := strings.TrimSpace(os.Getenv(imageCacheTTLDaysEnv)); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			ttlDays = parsed
		} else {
			log.Printf("harvest: invalid %s=%q, falling back to default %d days", imageCacheTTLDaysEnv, v, ttlDays)
		}
	}
	p := &HarvestPipeline{
		db:                 db,
		storage:            storage,
		client:             &http.Client{},
		imageCacheMaxBytes: maxBytes,
		imageCacheTTLDays:  ttlDays,
		imageCacheEnabled:  true,
		nowUnix:            func() int64 { return time.Now().Unix() },
	}
	for _, opt := range opts {
		opt(p)
	}
	log.Printf("harvest: image cache configured: max_bytes=%d ttl_days=%d enabled=%t", p.imageCacheMaxBytes, p.imageCacheTTLDays, p.imageCacheEnabled)
	return p
}

// ImageCacheTTLDays returns the configured age-based TTL (in days) applied
// to objects in the primary image cache namespace. The value is advisory —
// actual object removal is performed by the storage bucket's lifecycle rule
// configured from this same value (harvester-image-cache-ttl Decision D3).
func (p *HarvestPipeline) ImageCacheTTLDays() int {
	return p.imageCacheTTLDays
}

// Process deduplicates items, downloads and uploads media, and creates Pins.
func (p *HarvestPipeline) Process(ctx context.Context, items []RawItem) (pinsCreated int, deduped int, failed int, err error) {
	// Batch dedup: track sourceURLs within this batch
	seen := make(map[string]bool)

	for _, item := range items {
		// Batch-level dedup
		if seen[item.SourceURL] {
			deduped++
			continue
		}
		seen[item.SourceURL] = true

		// DB-level dedup: check if bot already created a pin for this sourceURL
		exists, dbErr := p.db.BotPinExistsByURL(ctx, db.BotPinExistsByURLParams{
			Url:       sql.NullString{String: item.SourceURL, Valid: true},
			CreatorID: BotCreatorID,
		})
		if dbErr != nil {
			log.Printf("harvest: dedup check failed for %s: %v", item.SourceURL, dbErr)
			failed++
			continue
		}
		if exists {
			deduped++
			continue
		}

		// Download media
		mediaURL, dlErr := p.downloadAndUpload(ctx, item.MediaURL, item.MediaType)
		if dlErr != nil {
			log.Printf("harvest: download/upload failed for %s: %v", item.MediaURL, dlErr)
			failed++
			continue
		}

		// Primary image caching: extract candidate from page HTML and cache to
		// our storage. cacheImage always returns a string to record: success →
		// storage URL, any failure → original candidate URL. If no candidate
		// is found, the column stays NULL.
		var ogImage sql.NullString
		if p.imageCacheEnabled && len(item.PageHTML) > 0 {
			candidate := PickPrimaryImage(item.PageHTML, item.SourceURL)
			if candidate != "" {
				cached, cacheErr := p.cacheImage(ctx, candidate)
				if cacheErr != nil {
					log.Printf("harvest: image cache fallback (source=%s candidate=%s): %v", item.SourceURL, candidate, cacheErr)
				}
				ogImage = sql.NullString{String: cached, Valid: true}
			}
		}

		// Create Pin
		_, createErr := p.db.CreatePin(ctx, db.CreatePinParams{
			CreatorID:   BotCreatorID,
			MediaUrl:    mediaURL,
			MediaType:   item.MediaType,
			Url:         sql.NullString{String: item.SourceURL, Valid: true},
			Title:       item.Title,
			Description: sql.NullString{String: item.Description, Valid: item.Description != ""},
			OgImage:     ogImage,
			OgData:      pqtype.NullRawMessage{},
		})
		if createErr != nil {
			log.Printf("harvest: pin creation failed for %s: %v", item.SourceURL, createErr)
			failed++
			continue
		}

		pinsCreated++
	}

	return pinsCreated, deduped, failed, nil
}

// downloadAndUpload downloads media from the source URL and uploads it via Storage.
func (p *HarvestPipeline) downloadAndUpload(ctx context.Context, mediaURL string, mediaType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download media: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download media: status %d", resp.StatusCode)
	}

	// Determine content type from response or infer from mediaType
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = inferContentType(mediaType, mediaURL)
	}

	// Generate a unique filename
	ext := extensionFromURL(mediaURL)
	filename := fmt.Sprintf("bot/%s%s", uuid.New().String(), ext)

	// Upload; pass Content-Length if available, -1 otherwise
	size := resp.ContentLength
	uploadedURL, err := p.storage.Upload(ctx, filename, contentType, size, resp.Body)
	if err != nil {
		return "", fmt.Errorf("upload media: %w", err)
	}

	return uploadedURL, nil
}

// inferContentType returns a content type string based on media type and URL.
func inferContentType(mediaType string, mediaURL string) string {
	switch mediaType {
	case "image":
		ext := strings.ToLower(path.Ext(mediaURL))
		switch ext {
		case ".png":
			return "image/png"
		case ".gif":
			return "image/gif"
		case ".webp":
			return "image/webp"
		default:
			return "image/jpeg"
		}
	case "audio":
		return "audio/mpeg"
	case "video":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

// extensionFromURL extracts the file extension from a URL path.
func extensionFromURL(rawURL string) string {
	// Strip query string
	u := rawURL
	if idx := strings.Index(u, "?"); idx != -1 {
		u = u[:idx]
	}
	ext := path.Ext(u)
	if ext == "" {
		return ".bin"
	}
	return ext
}

// cacheImage downloads the given candidate image URL and uploads it to
// object storage under the images/<hash>/<unix_ts>.<ext> key. Returns the
// storage URL on success. On any failure (download, upload, or size
// threshold exceeded), returns the original candidate URL unchanged together
// with a non-nil error describing the reason. Callers always record the
// returned string in the Pin's primary image URL column.
//
// candidateURL MUST be an absolute URL. Callers should obtain it from
// PickPrimaryImage, which resolves relative candidates against the page URL
// before returning. Passing a relative URL causes normalization to fail and
// the function returns the original string via the fallback path.
func (p *HarvestPipeline) cacheImage(ctx context.Context, candidateURL string) (string, error) {
	normalized, err := normalizeImageURL(candidateURL, "")
	if err != nil {
		return candidateURL, fmt.Errorf("normalize: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return candidateURL, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return candidateURL, fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return candidateURL, fmt.Errorf("download: status %d", resp.StatusCode)
	}

	// Pre-check Content-Length against threshold (Decision 5).
	if resp.ContentLength > 0 && resp.ContentLength > p.imageCacheMaxBytes {
		return candidateURL, fmt.Errorf("%w: content-length %d > %d", errImageOversize, resp.ContentLength, p.imageCacheMaxBytes)
	}

	// Read with a hard cap of threshold+1 bytes so we can detect overshoot on
	// servers that lie about / omit Content-Length.
	limited := io.LimitReader(resp.Body, p.imageCacheMaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return candidateURL, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > p.imageCacheMaxBytes {
		return candidateURL, fmt.Errorf("%w: read %d bytes", errImageOversize, len(body))
	}

	contentType := resp.Header.Get("Content-Type")
	key := buildImageCacheKey(normalized, contentType, p.nowUnix())

	storageURL, err := p.storage.Upload(ctx, key, contentType, int64(len(body)), bytes.NewReader(body))
	if err != nil {
		return candidateURL, fmt.Errorf("upload: %w", err)
	}
	log.Printf("harvest: image cache success (candidate=%s key=%s)", candidateURL, key)
	return storageURL, nil
}
