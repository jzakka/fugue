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
	// PageHTML is the raw HTML bytes of SourceURL, used by the pipeline to
	// extract a primary image candidate (og:image / twitter:image / article
	// img / JSON-LD image). Optional; when empty, no caching is attempted.
	PageHTML string
}
