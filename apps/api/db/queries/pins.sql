-- name: CreatePin :one
INSERT INTO pins (creator_id, media_url, media_type, url, title, description, og_image, og_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

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
