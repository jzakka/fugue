package bot

// RawItem represents a single crawled item before dedup/tagging.
type RawItem struct {
	Title       string
	Description string
	MediaURL    string // Direct download URL for the media
	SourceURL   string // Original page URL (stored as pin.url)
	MediaType   string // "image", "audio", "video"
}
