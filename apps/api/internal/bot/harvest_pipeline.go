package bot

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	// Decoders registered in media_validator.go cover GIF/PNG/JPEG; the
	// blank import here mirrors that set so this file's image.DecodeConfig
	// call resolves without a build dependency on the validator file's
	// internal init order.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/httpclient"
)

// maxMediaStreamBytes caps the streamed body size for non-image media in
// downloadAndUpload. storage.Upload only validates the declared `size` arg;
// without this cap a server that lies about Content-Length could stream
// unbounded bytes. 200 MiB is 2x the storage.maxBytes[MediaVideo] ceiling so
// legitimate uploads have headroom while malicious responses are cut off.
const maxMediaStreamBytes int64 = 200 << 20

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

// errImageInvalidMedia is returned by cacheImage when the downloaded bytes
// fail the in-process validity check (decoding or minimum dimensions).
// Triggers the same fallback path so the canonical key never receives an
// invalid file (harvester-media-validation design.md D3).
var errImageInvalidMedia = errors.New("image fails validity check")

// BotDB abstracts the database queries needed by HarvestPipeline.
type BotDB interface {
	BotPinExistsByURL(ctx context.Context, arg db.BotPinExistsByURLParams) (bool, error)
	CreatePin(ctx context.Context, arg db.CreatePinParams) (db.Pin, error)
	UpsertBotPinByURL(ctx context.Context, arg db.UpsertBotPinByURLParams) (db.UpsertBotPinByURLRow, error)
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
		db:      db,
		storage: storage,
		client: httpclient.NewSSRFSafeClient(httpclient.Options{
			ConnectTimeout: 5 * time.Second,
			TotalTimeout:   60 * time.Second,
			MaxRedirects:   5,
		}),
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

// ProcessDocument upserts a PinDocument as a single bot Pin keyed on the
// canonical URL. Returns (created, pinID, err) where created=true means a
// new row was inserted (PinsCreated stat) and created=false means an
// existing bot Pin was updated (Deduped stat).
//
// ScriptAdapter / GenericExtractor have already collapsed N RawItems into
// one PinDocument by this point — there is no N→1 reduction here.
func (p *HarvestPipeline) ProcessDocument(ctx context.Context, _ db.BotGraphNode, doc PinDocument) (bool, uuid.UUID, error) {
	if doc.CanonicalURL == "" {
		return false, uuid.Nil, fmt.Errorf("pin document missing canonical_url")
	}

	mediaURL, mediaType := pickMediaForPin(doc)
	if mediaURL == "" {
		return false, uuid.Nil, fmt.Errorf("pin document missing media_url (no thumbnail or media candidate)")
	}

	// Serialize og_data BEFORE the cache upload: once a cache object is
	// stored, the only failure left before the upsert is the upsert itself,
	// so the compensating delete below covers the whole window (no
	// uncompensated orphan can be created by a marshal failure).
	ogJSON, jsonErr := MarshalOGData(doc.OGData)
	if jsonErr != nil {
		return false, uuid.Nil, fmt.Errorf("marshal og_data: %w", jsonErr)
	}

	// Optionally download+cache the primary image so we serve the Pin's
	// og_image from our own storage. On any failure we fall back to the
	// extractor-provided URL so we still have something to render.
	ogImage := sql.NullString{}
	cacheStored := false
	if p.imageCacheEnabled && doc.ThumbnailURL != "" {
		cached, cacheErr := p.cacheImage(ctx, doc.ThumbnailURL)
		if cacheErr != nil {
			log.Printf("harvest: doc image cache fallback (canonical=%s thumb=%s): %v", doc.CanonicalURL, doc.ThumbnailURL, cacheErr)
		} else {
			cacheStored = true
		}
		ogImage = sql.NullString{String: cached, Valid: true}
	}

	row, err := p.db.UpsertBotPinByURL(ctx, db.UpsertBotPinByURLParams{
		CreatorID:   BotCreatorID,
		MediaUrl:    mediaURL,
		MediaType:   mediaType,
		Url:         sql.NullString{String: doc.CanonicalURL, Valid: true},
		Title:       doc.Title,
		Description: sql.NullString{String: doc.BodyText, Valid: doc.BodyText != ""},
		OgImage:     ogImage,
		OgData:      pqtype.NullRawMessage{RawMessage: ogJSON, Valid: len(ogJSON) > 0},
	})
	if err != nil {
		// Compensating delete: the cache object stored this attempt is now
		// unreferenced. Best-effort — the upsert error is returned unchanged
		// either way (spec: 미참조가 된 이미지 캐시 객체는 처리 경로에서 정리된다).
		if cacheStored {
			if delErr := p.storage.DeleteByURL(ctx, ogImage.String); delErr != nil {
				log.Printf("harvest: image cache compensating delete failed (url=%s): %v", ogImage.String, delErr)
			}
		}
		return false, uuid.Nil, fmt.Errorf("upsert bot pin: %w", err)
	}

	// Replacement cleanup: the row no longer references its previous
	// og_image, so delete the old cache object. Ownership checks (our URL?
	// image cache namespace?) live in the adapter; here we only compare
	// values. Best-effort — failures never affect the pipeline result.
	if row.PrevOgImage.Valid && (!ogImage.Valid || row.PrevOgImage.String != ogImage.String) {
		if delErr := p.storage.DeleteByURL(ctx, row.PrevOgImage.String); delErr != nil {
			log.Printf("harvest: image cache replaced-object delete failed (url=%s): %v", row.PrevOgImage.String, delErr)
		}
	}
	return row.Inserted, row.ID, nil
}

// MarkSkipped is the hook the Harvester calls when classifier returns
// pinnable=false. Frontier `harvested_at` writes are owned by the
// scheduler-consumer change; until that is wired this is a no-op so the
// Harvester can still record the Skipped stat without blocking on a table
// that may not exist yet.
func (p *HarvestPipeline) MarkSkipped(_ context.Context, _ db.BotGraphNode) error {
	return nil
}

// pickMediaForPin chooses (mediaURL, mediaType) for a Pin row.
// Preference: ThumbnailURL → first MediaCandidate URL.
// MediaType defaults to "image" when only a thumbnail is available.
func pickMediaForPin(doc PinDocument) (string, string) {
	if doc.ThumbnailURL != "" {
		mediaType := "image"
		for _, c := range doc.MediaCandidates {
			if c.URL == doc.ThumbnailURL && c.Type != "" {
				mediaType = c.Type
				break
			}
		}
		return doc.ThumbnailURL, mediaType
	}
	for _, c := range doc.MediaCandidates {
		if c.URL != "" {
			t := c.Type
			if t == "" {
				t = "image"
			}
			return c.URL, t
		}
	}
	return "", ""
}

// downloadAndUpload downloads media from the source URL and uploads it via Storage.
//
// Per harvester-media-validation design.md D3, the canonical key MUST NOT
// receive an invalid image. This path performs in-process validation
// (header decode + minimum dims + minimum bytes) for image media before
// upload, mirroring cacheImage()'s temp-buffer-then-upload contract. Video
// and audio paths still upload streaming (no probe here) because ffprobe
// is wired into the candidate-stage validator, not this storage path; the
// candidate-stage validator is the architectural home for type-aware probing
// (design.md D2).
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

	// For image media: buffer the body, run the same minimum-bytes /
	// header-decode checks cacheImage() uses, and only upload to the
	// canonical key when the bytes pass. Non-image types stream straight
	// through (their integrity is checked at the candidate stage).
	//
	// Enforce an explicit stream-size cap (harvester/spec.md L749 SHALL)
	// using the same cacheImage() pattern: Content-Length precheck +
	// LimitReader(cap+1) + post-read overshoot check. Without this guard
	// an attacker-controlled image-MIME response could stream unbounded
	// bytes into the harvester process before the in-process min-bytes /
	// DecodeConfig validation fires — OOM-killing the worker and dropping
	// every in-flight batch. cacheImage() (L494) and the non-image branch
	// below (L408) both already enforce caps; this branch was the lone
	// unbounded fetcher in HarvestPipeline. The cap reuses
	// p.imageCacheMaxBytes (default 20 MiB) so all image-data fetches in
	// HarvestPipeline share a single source-of-truth threshold.
	if mediaType == "image" {
		if resp.ContentLength > 0 && resp.ContentLength > p.imageCacheMaxBytes {
			return "", fmt.Errorf("%w: content-length %d > %d", errImageOversize, resp.ContentLength, p.imageCacheMaxBytes)
		}
		limited := io.LimitReader(resp.Body, p.imageCacheMaxBytes+1)
		body, readErr := io.ReadAll(limited)
		if readErr != nil {
			return "", fmt.Errorf("read media body: %w", readErr)
		}
		if int64(len(body)) > p.imageCacheMaxBytes {
			return "", fmt.Errorf("%w: read %d bytes", errImageOversize, len(body))
		}
		if int64(len(body)) < DefaultImageMinBytes {
			return "", fmt.Errorf("%w: bytes=%d below min=%d", errImageInvalidMedia, len(body), DefaultImageMinBytes)
		}
		cfg, _, decErr := image.DecodeConfig(bytes.NewReader(body))
		if decErr != nil {
			return "", fmt.Errorf("%w: decode failed: %v", errImageInvalidMedia, decErr)
		}
		if cfg.Width < DefaultImageMinWidth || cfg.Height < DefaultImageMinHeight {
			return "", fmt.Errorf("%w: dims=%dx%d below min=%dx%d", errImageInvalidMedia, cfg.Width, cfg.Height, DefaultImageMinWidth, DefaultImageMinHeight)
		}
		ext := extensionFromURL(mediaURL)
		filename := fmt.Sprintf("bot/%s%s", uuid.New().String(), ext)
		uploadedURL, upErr := p.storage.Upload(ctx, filename, contentType, int64(len(body)), bytes.NewReader(body))
		if upErr != nil {
			return "", fmt.Errorf("upload media: %w", upErr)
		}
		return uploadedURL, nil
	}

	// Generate a unique filename
	ext := extensionFromURL(mediaURL)
	filename := fmt.Sprintf("bot/%s%s", uuid.New().String(), ext)

	// Upload; pass Content-Length if available, -1 otherwise. Wrap the body
	// in a LimitReader so a server that lies about Content-Length cannot
	// stream unbounded bytes into storage (storage.Upload only checks the
	// declared `size` arg).
	size := resp.ContentLength
	body := io.LimitReader(resp.Body, maxMediaStreamBytes)
	uploadedURL, err := p.storage.Upload(ctx, filename, contentType, size, body)
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
	// Strip fragment and query string; either would leak into the object
	// key (and thus the public URL) since uploads respect the caller's key.
	u := rawURL
	if idx := strings.Index(u, "#"); idx != -1 {
		u = u[:idx]
	}
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

	// In-process image validation: reject undecodable bytes or sub-threshold
	// dimensions BEFORE the canonical key receives them. This is the
	// last-mile safety net for the temp-buffer → canonical-key contract
	// (harvester-media-validation design.md D3). The body is in memory only;
	// rejection simply discards it without an Upload call so MinIO/S3 never
	// sees the placeholder.
	if int64(len(body)) < DefaultImageMinBytes {
		return candidateURL, fmt.Errorf("%w: bytes=%d below threshold", errImageInvalidMedia, len(body))
	}
	if cfg, _, decErr := image.DecodeConfig(bytes.NewReader(body)); decErr != nil {
		return candidateURL, fmt.Errorf("%w: decode: %v", errImageInvalidMedia, decErr)
	} else if cfg.Width < DefaultImageMinWidth || cfg.Height < DefaultImageMinHeight {
		return candidateURL, fmt.Errorf("%w: %dx%d below %dx%d threshold",
			errImageInvalidMedia, cfg.Width, cfg.Height, DefaultImageMinWidth, DefaultImageMinHeight)
	}

	key := buildImageCacheKey(normalized, contentType, p.nowUnix())

	storageURL, err := p.storage.Upload(ctx, key, contentType, int64(len(body)), bytes.NewReader(body))
	if err != nil {
		return candidateURL, fmt.Errorf("upload: %w", err)
	}
	log.Printf("harvest: image cache success (candidate=%s key=%s)", candidateURL, key)
	return storageURL, nil
}
