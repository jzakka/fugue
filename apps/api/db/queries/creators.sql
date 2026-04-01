-- name: CreateCreator :one
INSERT INTO creators (nickname, roles, contacts)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateCreatorFromOAuth :one
INSERT INTO creators (nickname, bio, roles, contacts, avatar_url, email)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CreateCreatorFromOAuthOnConflict :one
INSERT INTO creators (nickname, bio, roles, contacts, avatar_url, email)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (email) DO NOTHING
RETURNING *;

-- name: GetCreator :one
SELECT * FROM creators
WHERE id = $1;

-- name: GetCreatorByEmail :one
SELECT * FROM creators
WHERE email = $1;

-- name: GetCreatorByEmailForUpdate :one
SELECT * FROM creators
WHERE email = $1
FOR UPDATE;

-- name: UpdateCreator :one
UPDATE creators
SET nickname = $2,
    bio = $3,
    roles = $4,
    contacts = $5,
    avatar_url = $6,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListCreatorsByRoles :many
SELECT * FROM creators
WHERE roles && $1::text[]
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWorksByCreator :one
SELECT count(*) FROM works
WHERE creator_id = $1;
