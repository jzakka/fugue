package bot

import (
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

// defaultImageCacheMaxBytes is the default per-image download ceiling (20 MiB).
const defaultImageCacheMaxBytes int64 = 20 * 1024 * 1024

// BotDB abstracts the database queries needed by HarvestPipeline.
type BotDB interface {
	BotPinExistsByURL(ctx context.Context, arg db.BotPinExistsByURLParams) (bool, error)
	CreatePin(ctx context.Context, arg db.CreatePinParams) (db.Pin, error)
}

// HarvestPipeline implements Pipeline by deduping, downloading media, and creating Pins.
type HarvestPipeline struct {
	db      BotDB
	storage Storage
	client  *http.Client

	// imageCacheMaxBytes caps per-image download size. Exceeding triggers fallback.
	imageCacheMaxBytes int64
	// imageCacheEnabled toggles primary-image caching entirely.
	imageCacheEnabled bool
}

// HarvestPipelineOption configures optional HarvestPipeline behavior.
type HarvestPipelineOption func(*HarvestPipeline)

// WithImageCacheMaxBytes overrides the per-image download size cap (bytes).
func WithImageCacheMaxBytes(n int64) HarvestPipelineOption {
	return func(p *HarvestPipeline) {
		if n > 0 {
			p.imageCacheMaxBytes = n
		}
	}
}

// WithImageCacheEnabled toggles primary-image caching.
func WithImageCacheEnabled(v bool) HarvestPipelineOption {
	return func(p *HarvestPipeline) { p.imageCacheEnabled = v }
}

// NewHarvestPipeline creates a new HarvestPipeline.
// Env overrides:
//
//	HARVESTER_IMAGE_CACHE_MAX_BYTES — integer byte cap (default 20 MiB)
//	HARVESTER_IMAGE_CACHE_ENABLED   — "0"/"false" disables caching (default enabled)
func NewHarvestPipeline(db BotDB, storage Storage, opts ...HarvestPipelineOption) *HarvestPipeline {
	p := &HarvestPipeline{
		db:                 db,
		storage:            storage,
		client:             &http.Client{},
		imageCacheMaxBytes: defaultImageCacheMaxBytes,
		imageCacheEnabled:  true,
	}

	// Env-level configuration
	if v := strings.TrimSpace(os.Getenv("HARVESTER_IMAGE_CACHE_MAX_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			p.imageCacheMaxBytes = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("HARVESTER_IMAGE_CACHE_ENABLED")); v != "" {
		switch strings.ToLower(v) {
		case "0", "false", "no", "off":
			p.imageCacheEnabled = false
		}
	}

	for _, opt := range opts {
		opt(p)
	}
	return p
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

		// Pick primary image candidate and cache to object storage.
		// Two columns (og_image / thumbnail_url) are meant to hold the same value;
		// only og_image is backed by current schema.
		ogImage := sql.NullString{}
		if p.imageCacheEnabled {
			if candidate := PickPrimaryImage([]byte(item.PageHTML), item.SourceURL); candidate != "" {
				cached, cacheErr := p.cacheImage(ctx, candidate)
				if cacheErr != nil {
					log.Printf("harvest: image cache fallback for pin=%s candidate=%s: %v",
						item.SourceURL, candidate, cacheErr)
				} else {
					log.Printf("harvest: image cache ok pin=%s candidate=%s stored=%s",
						item.SourceURL, candidate, cached)
				}
				ogImage = sql.NullString{String: cached, Valid: true}
			} else {
				log.Printf("harvest: image cache no-candidate pin=%s", item.SourceURL)
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

// cacheImage downloads a primary image and uploads it to object storage under
// the "images/<sha256(url)>/<unix_ts>.<ext>" key.
//
// Return contract:
//   - Success: returns (storage URL, nil).
//   - Failure (download / upload / oversize): returns (original URL, error).
//
// Callers MUST write the returned string into the Pin column regardless of error.
// The error is for logging/metrics only.
func (p *HarvestPipeline) cacheImage(ctx context.Context, rawURL string) (string, error) {
	maxBytes := p.imageCacheMaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultImageCacheMaxBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return rawURL, fmt.Errorf("image cache: new request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return rawURL, fmt.Errorf("image cache: download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return rawURL, fmt.Errorf("image cache: download status %d", resp.StatusCode)
	}

	// Pre-check Content-Length against cap.
	if resp.ContentLength > 0 && resp.ContentLength > maxBytes {
		return rawURL, fmt.Errorf("image cache: oversize content-length %d > %d", resp.ContentLength, maxBytes)
	}

	// Buffered read with hard cap. Read up to maxBytes+1 to detect overflow.
	limited := io.LimitReader(resp.Body, maxBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return rawURL, fmt.Errorf("image cache: read body: %w", err)
	}
	if int64(len(buf)) > maxBytes {
		return rawURL, fmt.Errorf("image cache: oversize read %d > %d", len(buf), maxBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	ext := extensionForImage(contentType, rawURL)
	normalized := normalizeImageURL(rawURL, "")
	hash := sha256Hex(normalized)
	key := imageCacheKey(hash, time.Now().Unix(), ext)

	uploadCT := contentType
	if uploadCT == "" {
		uploadCT = "application/octet-stream"
	}

	uploadedURL, upErr := p.storage.Upload(ctx, key, uploadCT, int64(len(buf)), strings.NewReader(string(buf)))
	if upErr != nil {
		return rawURL, fmt.Errorf("image cache: upload: %w", upErr)
	}
	if !strings.HasPrefix(key, "images/") {
		// Defensive: our key builder always uses images/, but guard anyway.
		return rawURL, errors.New("image cache: invariant: key missing images/ prefix")
	}
	return uploadedURL, nil
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
