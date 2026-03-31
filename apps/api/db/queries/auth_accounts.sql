-- name: CreateAuthAccount :one
INSERT INTO auth_accounts (creator_id, provider, provider_id, email)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAuthAccountByProvider :one
SELECT * FROM auth_accounts
WHERE provider = $1 AND provider_id = $2;

-- name: GetAuthAccountByEmail :many
SELECT * FROM auth_accounts
WHERE email = $1;
