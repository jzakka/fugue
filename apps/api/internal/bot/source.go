package bot

import "context"

// Source defines the interface for a crawl plugin.
// Each external platform implements this interface.
type Source interface {
	// Name returns the platform identifier (e.g. "unsplash", "fma").
	Name() string
	// Crawl fetches raw items from the external platform.
	Crawl(ctx context.Context) ([]RawItem, error)
}

// RawItem represents a single crawled item before dedup/tagging.
type RawItem struct {
	Title       string
	Description string
	MediaURL    string // Direct download URL for the media
	SourceURL   string // Original page URL (stored as pin.url)
	MediaType   string // "image", "audio", "video"
}
