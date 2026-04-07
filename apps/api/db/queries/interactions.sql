-- name: CreateInteraction :exec
INSERT INTO interactions (user_id, pin_id, type)
VALUES ($1, $2, $3);

-- name: GetUserTagFrequency :many
SELECT pt.tag_id AS tag_id, COUNT(*) AS freq
FROM pins p
JOIN pin_tags pt ON pt.pin_id = p.id
WHERE p.creator_id = $1
GROUP BY pt.tag_id
ORDER BY freq DESC
LIMIT $2;

-- name: CountUserPins :one
SELECT count(*) FROM pins
WHERE creator_id = $1;

-- name: GetUserMediaTypeFrequency :many
SELECT p.media_type, COUNT(*) AS freq
FROM pins p
WHERE p.creator_id = $1
GROUP BY p.media_type
ORDER BY freq DESC
LIMIT $2;

-- name: RecommendByTags :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE EXISTS (
    SELECT 1 FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($1::uuid[])
)
  AND p.creator_id != $2
ORDER BY
    (SELECT count(*) FROM pin_tags pt WHERE pt.pin_id = p.id AND pt.tag_id = ANY($1::uuid[])) DESC,
    p.created_at DESC
LIMIT $3;

-- name: RecommendByMediaType :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.media_type = ANY($1::text[])
  AND p.creator_id != $2
ORDER BY p.created_at DESC
LIMIT $3;
