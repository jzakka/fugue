-- name: CreateCreatorFromOAuth :one
INSERT INTO creators (nickname, avatar_url, email)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateCreatorFromOAuthOnConflict :one
INSERT INTO creators (nickname, avatar_url, email)
VALUES ($1, $2, $3)
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
    avatar_url = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountPinsByCreator :one
SELECT count(*) FROM pins
WHERE creator_id = $1;
