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

-- name: ListWorksWithCreator :many
SELECT
    w.id, w.creator_id, w.url, w.title, w.description,
    w.field, w.tags, w.og_image, w.og_data, w.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM works w
JOIN creators c ON c.id = w.creator_id
WHERE ($1::varchar = '' OR w.field = $1)
  AND ($2::text[] IS NULL OR w.tags && $2::text[])
ORDER BY w.created_at DESC, w.id DESC
LIMIT $3 OFFSET $4;

-- name: CountWorks :one
SELECT count(*) FROM works
WHERE ($1::varchar = '' OR field = $1)
  AND ($2::text[] IS NULL OR tags && $2::text[]);

-- name: ListWorksByCreator :many
SELECT
    w.id, w.creator_id, w.url, w.title, w.description,
    w.field, w.tags, w.og_image, w.og_data, w.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM works w
JOIN creators c ON c.id = w.creator_id
WHERE w.creator_id = $1
  AND ($2::varchar = '' OR w.field = $2)
  AND ($3::text[] IS NULL OR w.tags && $3::text[])
ORDER BY w.created_at DESC, w.id DESC
LIMIT $4 OFFSET $5;

-- name: CountWorksByCreatorFiltered :one
SELECT count(*) FROM works
WHERE creator_id = $1
  AND ($2::varchar = '' OR field = $2)
  AND ($3::text[] IS NULL OR tags && $3::text[]);
