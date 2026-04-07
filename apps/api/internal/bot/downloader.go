package bot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

// downloadTimeout returns the HTTP download timeout based on media type.
func downloadTimeout(mediaType string) time.Duration {
	switch mediaType {
	case "audio":
		return 120 * time.Second
	case "video":
		return 300 * time.Second
	default: // image
		return 30 * time.Second
	}
}

// Downloader handles downloading media from external URLs and uploading to S3.
type Downloader struct {
	store      *storage.Client
	httpClient *http.Client
}

// NewDownloader creates a new Downloader with the given storage client.
func NewDownloader(store *storage.Client) *Downloader {
	return &Downloader{
		store: store,
		// httpClient timeout is set per-request via context
		httpClient: &http.Client{},
	}
}

// DownloadResult holds info about a successfully downloaded and uploaded media file.
type DownloadResult struct {
	MediaURL  string // S3 public URL
	MediaType string // "image", "audio", "video"
}

// DownloadAndUpload fetches media from the given URL, buffers it in memory,
// then uploads it to S3 via the storage client.
func (d *Downloader) DownloadAndUpload(ctx context.Context, mediaURL string, expectedMediaType string) (*DownloadResult, error) {
	timeout := downloadTimeout(expectedMediaType)
	dlCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("downloader: create request: %w", err)
	}
	req.Header.Set("User-Agent", "FugueBot/1.0 (+https://fugue.app/bot)")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloader: fetch %s: %w", mediaURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloader: fetch %s: HTTP %d", mediaURL, resp.StatusCode)
	}

	// Buffer the entire response body to get the size.
	// Content-Length may be absent or incorrect for some servers.
	var buf bytes.Buffer
	n, err := io.Copy(&buf, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("downloader: read body from %s: %w", mediaURL, err)
	}

	contentType := resp.Header.Get("Content-Type")
	log.Printf("downloader: fetched %s (%d bytes, Content-Type: %s)", mediaURL, n, contentType)

	// Upload to S3 via storage client.
	// The storage client validates MIME type and file size.
	result, err := d.store.Upload(ctx, filenameFromURL(mediaURL), contentType, n, &buf)
	if err != nil {
		return nil, fmt.Errorf("downloader: upload %s: %w", mediaURL, err)
	}

	return &DownloadResult{
		MediaURL:  result.URL,
		MediaType: string(result.MediaType),
	}, nil
}

// filenameFromURL extracts a filename from a URL for storage key generation.
func filenameFromURL(rawURL string) string {
	// Just use a simple approach - the storage client generates its own UUID key anyway
	return "bot-media"
}
