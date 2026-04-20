package bot

import "github.com/google/uuid"

// BotCreatorID is the fixed UUID for the fuguebot system account (from seed.sql).
//
// IMMUTABLE-sync policy:
//
// PostgreSQL partial unique indexes require an IMMUTABLE predicate, so the
// bot UUID cannot be parameter-bound at query time. The same UUID literal
// MUST appear verbatim in three places:
//
//  1. This constant (used at runtime by Go code that inserts/queries bot Pins).
//  2. The partial index predicate in
//     apps/api/db/migrations/000027_add_pins_url_bot_unique.up.sql
//     (`WHERE creator_id = '00000000-0000-0000-0000-00000000f096'`).
//  3. The `ON CONFLICT ... WHERE` clause of `UpsertBotPinByURL` in
//     apps/api/db/queries/pins.sql (so arbiter inference matches the index).
//
// Changing the bot UUID therefore requires a NEW migration that drops and
// recreates the index plus a coordinated update of all three locations. Do
// not attempt to make this value env-configurable in a way that diverges
// from the literal compiled into the index predicate.
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
