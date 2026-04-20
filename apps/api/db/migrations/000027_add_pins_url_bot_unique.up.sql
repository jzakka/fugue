-- Partial unique index so the bot has at most one Pin per canonical URL.
-- User-owned Pins are untouched (URL duplication is still allowed for users).
--
-- NOTE: `CONCURRENTLY` cannot run inside a transaction — migration tooling
-- must apply this file outside of `BEGIN/COMMIT`.
--
-- NOTE: The `creator_id` literal below MUST stay in sync with the
-- `BotCreatorID` constant in `apps/api/internal/bot/source.go`. Postgres
-- partial indexes require an IMMUTABLE predicate, so parameter binding is
-- not possible; changing the bot UUID requires a new migration that drops
-- and recreates the index.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS pins_url_bot_unique
ON pins(url)
WHERE creator_id = '00000000-0000-0000-0000-00000000f096';
