-- name: CreateWork :one
INSERT INTO works (creator_id, url, title, description, field, tags, og_image, og_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetWork :one
SELECT * FROM works
WHERE id = $1;

-- name: DeleteWork :exec
DELETE FROM works
WHERE id = $1 AND creator_id = $2;

-- name: ListWorks :many
SELECT * FROM works
WHERE ($1::varchar = '' OR field = $1)
  AND ($2::text[] IS NULL OR tags && $2::text[])
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: RecommendWorks :many
SELECT * FROM works
WHERE field = ANY($1::text[])
  AND tags && $2::text[]
ORDER BY array_length(tags & $2::text[], 1) DESC NULLS LAST, created_at DESC
LIMIT $3 OFFSET $4;
