-- name: ListActiveBotSources :many
SELECT id, name, platform, seed_urls, interval_hours, enabled, last_crawled_at, stats, created_at
FROM bot_sources
WHERE enabled = true
ORDER BY created_at;

-- name: ListAllBotSources :many
SELECT id, name, platform, seed_urls, interval_hours, enabled, last_crawled_at, stats, created_at
FROM bot_sources
ORDER BY created_at;

-- name: GetBotSource :one
SELECT id, name, platform, seed_urls, interval_hours, enabled, last_crawled_at, stats, created_at
FROM bot_sources
WHERE id = $1;

-- name: CreateBotSource :one
INSERT INTO bot_sources (name, platform, seed_urls, interval_hours, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateBotSourceStats :exec
UPDATE bot_sources
SET last_crawled_at = now(), stats = $2
WHERE id = $1;

-- name: ToggleBotSource :one
UPDATE bot_sources
SET enabled = $2
WHERE id = $1
RETURNING *;

-- name: DeleteBotSource :execrows
DELETE FROM bot_sources
WHERE id = $1;

-- name: PinURLExists :one
SELECT EXISTS(SELECT 1 FROM pins WHERE url = $1) AS url_exists;

-- name: ListAllTags :many
SELECT id, name FROM tags;
