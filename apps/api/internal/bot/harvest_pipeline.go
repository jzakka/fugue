package bot

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

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
}

// NewHarvestPipeline creates a new HarvestPipeline.
func NewHarvestPipeline(db BotDB, storage Storage) *HarvestPipeline {
	return &HarvestPipeline{
		db:      db,
		storage: storage,
		client:  &http.Client{},
	}
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

		// Create Pin
		_, createErr := p.db.CreatePin(ctx, db.CreatePinParams{
			CreatorID:   BotCreatorID,
			MediaUrl:    mediaURL,
			MediaType:   item.MediaType,
			Url:         sql.NullString{String: item.SourceURL, Valid: true},
			Title:       item.Title,
			Description: sql.NullString{String: item.Description, Valid: item.Description != ""},
			OgImage:     sql.NullString{},
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
