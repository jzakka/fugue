-- name: CreatePin :one
INSERT INTO pins (creator_id, url, title, description, field, tags, og_image, og_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPin :one
SELECT * FROM pins
WHERE id = $1;

-- name: GetPinWithCreator :one
SELECT
    p.id, p.creator_id, p.url, p.title, p.description,
    p.field, p.tags, p.og_image, p.og_data, p.pin_count, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.id = $1;

-- name: DeletePin :execrows
DELETE FROM pins
WHERE id = $1 AND creator_id = $2;

-- name: UpdatePinCountByURL :exec
UPDATE pins SET pin_count = (
    SELECT COUNT(DISTINCT p2.creator_id) FROM pins p2 WHERE p2.url = pins.url
) WHERE pins.url = $1;

-- name: GetPinURL :one
SELECT url FROM pins WHERE id = $1;

-- name: ListPinsWithCreator :many
SELECT
    p.id, p.creator_id, p.url, p.title, p.description,
    p.field, p.tags, p.og_image, p.og_data, p.pin_count, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE ($1::varchar = '' OR p.field = $1)
  AND ($2::text[] IS NULL OR p.tags && $2::text[])
ORDER BY p.created_at DESC, p.id DESC
LIMIT $3 OFFSET $4;

-- name: CountPins :one
SELECT count(*) FROM pins
WHERE ($1::varchar = '' OR field = $1)
  AND ($2::text[] IS NULL OR tags && $2::text[]);

-- name: ListPinsByCreator :many
SELECT
    p.id, p.creator_id, p.url, p.title, p.description,
    p.field, p.tags, p.og_image, p.og_data, p.pin_count, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.creator_id = $1
  AND ($2::varchar = '' OR p.field = $2)
  AND ($3::text[] IS NULL OR p.tags && $3::text[])
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4 OFFSET $5;

-- name: CountPinsByCreatorFiltered :one
SELECT count(*) FROM pins
WHERE creator_id = $1
  AND ($2::varchar = '' OR field = $2)
  AND ($3::text[] IS NULL OR tags && $3::text[]);

-- name: RelatedPins :many
SELECT
    p.id, p.creator_id, p.url, p.title, p.description,
    p.field, p.tags, p.og_image, p.og_data, p.pin_count, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.id != $1
  AND p.tags && $2::text[]
ORDER BY
    CASE WHEN p.field = $3 THEN 0 ELSE 1 END,
    array_length(p.tags & $2::text[], 1) DESC NULLS LAST,
    p.created_at DESC
LIMIT 10;

-- name: ListLatestPinsWithCreator :many
SELECT
    p.id, p.creator_id, p.url, p.title, p.description,
    p.field, p.tags, p.og_image, p.og_data, p.pin_count, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE ($1::varchar = '' OR p.field = $1)
  AND ($2::text[] IS NULL OR p.tags && $2::text[])
  AND (p.created_at < $3::timestamptz OR (p.created_at = $3::timestamptz AND p.id < $4::uuid))
ORDER BY p.created_at DESC, p.id DESC
LIMIT $5;
