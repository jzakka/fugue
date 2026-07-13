-- name: CreatePin :one
INSERT INTO pins (creator_id, media_url, media_type, url, title, description, og_image, og_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpsertBotPinByURL :one
-- Idempotent upsert keyed on canonical URL for the bot creator.
--
-- The ON CONFLICT predicate hard-codes the bot UUID literal because
-- PostgreSQL only matches partial unique indexes when the WHERE clause is
-- IMMUTABLE — parameter binding (`$N`) here would prevent arbiter inference.
-- The literal MUST stay in sync with `BotCreatorID` in
-- apps/api/internal/bot/source.go and the partial index predicate in
-- apps/api/db/migrations/000027_add_pins_url_bot_unique.up.sql.
-- The prev CTE has no such constraint, so it uses the $1 parameter.
--
-- The prev CTE is evaluated on the same snapshot as the INSERT, so
-- prev_og_image is the og_image value BEFORE this upsert overwrote it
-- (NULL on a fresh insert). The caller uses it to clean up the previously
-- referenced image-cache object when the reference is replaced.
WITH prev AS (
    SELECT og_image FROM pins
    WHERE url = $4 AND creator_id = $1
)
INSERT INTO pins (creator_id, media_url, media_type, url, title, description, og_image, og_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (url) WHERE creator_id = '00000000-0000-0000-0000-00000000f096'
DO UPDATE SET
    media_url = EXCLUDED.media_url,
    media_type = EXCLUDED.media_type,
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    og_image = EXCLUDED.og_image,
    og_data = EXCLUDED.og_data
RETURNING *, (xmax = 0) AS inserted,
    (SELECT og_image FROM prev) AS prev_og_image;

-- name: LinkPinTag :exec
INSERT INTO pin_tags (pin_id, tag_id) VALUES ($1, $2);

-- name: GetPin :one
SELECT * FROM pins
WHERE id = $1;

-- name: GetPinWithCreator :one
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.id = $1;

-- name: GetPinTags :many
SELECT t.id, t.name, t.slug, t.category
FROM pin_tags pt
JOIN tags t ON t.id = pt.tag_id
WHERE pt.pin_id = $1
ORDER BY t.category, t.display_order;

-- name: DeletePin :execrows
DELETE FROM pins
WHERE id = $1 AND creator_id = $2;

-- name: ListPinsWithCreator :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE ($1::varchar = '' OR p.media_type = $1)
  AND ($2::uuid[] IS NULL OR EXISTS (
    SELECT 1 FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($2::uuid[])
  ))
ORDER BY p.created_at DESC, p.id DESC
LIMIT $3 OFFSET $4;

-- name: ListLatestPinsExcludingRecommended :many
-- Latest ("보충") source for the personalized feed. Excludes the recommendation
-- population — pins by OTHER creators ($1) that match the requester's top tags
-- ($2) or top media types ($3) — so the latest source and the recommended
-- source (RecommendByTags / RecommendByMediaType) draw from disjoint pools. No
-- pin can then appear in both sources within a page or across pages, satisfying
-- the feed `페이지 간 작품 중복을 반환하지 않는다` SHALL. The requester's own pins are
-- never recommended (the recommend queries filter `creator_id != $caller`), so
-- they remain eligible here.
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE NOT (
    p.creator_id != $1
    AND (
        EXISTS (
            SELECT 1 FROM pin_tags pt
            WHERE pt.pin_id = p.id AND pt.tag_id = ANY($2::uuid[])
        )
        OR ($3::text[] IS NOT NULL AND p.media_type = ANY($3::text[]))
    )
)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4 OFFSET $5;

-- name: CountPins :one
SELECT count(*) FROM pins p
WHERE ($1::varchar = '' OR p.media_type = $1)
  AND ($2::uuid[] IS NULL OR EXISTS (
    SELECT 1 FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($2::uuid[])
  ));

-- name: ListPinsByCreator :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.creator_id = $1
  AND ($2::varchar = '' OR p.media_type = $2)
  AND ($3::uuid[] IS NULL OR EXISTS (
    SELECT 1 FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($3::uuid[])
  ))
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4 OFFSET $5;

-- name: CountPinsByCreatorFiltered :one
SELECT count(*) FROM pins p
WHERE p.creator_id = $1
  AND ($2::varchar = '' OR p.media_type = $2)
  AND ($3::uuid[] IS NULL OR EXISTS (
    SELECT 1 FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($3::uuid[])
  ));

-- name: RelatedPins :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.id != $1
  AND EXISTS (
    SELECT 1 FROM pin_tags pt
    WHERE pt.pin_id = p.id AND pt.tag_id = ANY($2::uuid[])
  )
ORDER BY
    CASE WHEN p.media_type = $3 THEN 0 ELSE 1 END,
    (SELECT count(*) FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($2::uuid[])) DESC,
    p.created_at DESC
LIMIT 10;

-- name: FallbackRelatedByMediaType :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.media_type = $1
  AND p.id != ALL($2::uuid[])
ORDER BY p.created_at DESC
LIMIT $3;

-- name: FallbackRelatedLatest :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.id != ALL($1::uuid[])
ORDER BY p.created_at DESC
LIMIT $2;

-- name: ListLatestPinsWithCreator :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE ($1::varchar = '' OR p.media_type = $1)
  AND ($2::uuid[] IS NULL OR EXISTS (
    SELECT 1 FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($2::uuid[])
  ))
  AND (p.created_at < $3::timestamptz OR (p.created_at = $3::timestamptz AND p.id < $4::uuid))
ORDER BY p.created_at DESC, p.id DESC
LIMIT $5;
