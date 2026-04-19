package bot

import "github.com/google/uuid"

// BotCreatorID is the fixed UUID for the fuguebot system account (from seed.sql).
var BotCreatorID = uuid.MustParse("00000000-0000-0000-0000-00000000f096")

// RawItem represents a single crawled item before dedup/tagging.
type RawItem struct {
	Title       string
	Description string
	MediaURL    string // Direct download URL for the media
	SourceURL   string // Original page URL (stored as pin.url)
	MediaType   string // "image", "audio", "video"
	// PageHTML is optional raw HTML bytes of the source page, used by the
	// pipeline to extract a primary image candidate (og:image / twitter:image
	// / article img / JSON-LD). If nil/empty, primary image caching is
	// skipped for this item.
	PageHTML []byte
}
