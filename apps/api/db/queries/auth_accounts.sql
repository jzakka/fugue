-- name: CreateAuthAccount :one
INSERT INTO auth_accounts (creator_id, provider, provider_id, email)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateAuthAccountWithProfile :one
INSERT INTO auth_accounts (creator_id, provider, provider_id, email, profile)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAuthAccountByProvider :one
SELECT * FROM auth_accounts
WHERE provider = $1 AND provider_id = $2;

-- name: GetAuthAccountByEmail :many
SELECT * FROM auth_accounts
WHERE email = $1;

-- name: GetAuthAccountByEmailForUpdate :many
SELECT * FROM auth_accounts
WHERE email = $1
FOR UPDATE;

-- name: ListAuthAccountsByCreator :many
SELECT * FROM auth_accounts
WHERE creator_id = $1;
