-- name: CreateInteraction :exec
INSERT INTO interactions (user_id, pin_id, type)
VALUES ($1, $2, $3);

-- name: GetUserTagFrequency :many
SELECT unnest(p.tags) AS tag, COUNT(*) AS freq
FROM pins p
WHERE p.creator_id = $1
GROUP BY tag
ORDER BY freq DESC
LIMIT $2;

-- name: CountUserPins :one
SELECT count(*) FROM pins
WHERE creator_id = $1;

-- name: RecommendByTags :many
SELECT
    p.id, p.creator_id, p.url, p.title, p.description,
    p.field, p.tags, p.og_image, p.og_data, p.pin_count, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.tags && $1::text[]
  AND p.creator_id != $2
ORDER BY array_length(p.tags & $1::text[], 1) DESC NULLS LAST, p.created_at DESC
LIMIT $3;
