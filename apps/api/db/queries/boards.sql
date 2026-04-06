-- name: CreateBoard :one
INSERT INTO boards (creator_id, name, description, is_public)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetBoard :one
SELECT * FROM boards
WHERE id = $1;

-- name: UpdateBoard :one
UPDATE boards
SET name = $3,
    description = $4,
    is_public = $5,
    updated_at = now()
WHERE id = $1 AND creator_id = $2
RETURNING *;

-- name: DeleteBoard :execrows
DELETE FROM boards
WHERE id = $1 AND creator_id = $2;

-- name: ListBoardsByCreator :many
SELECT * FROM boards
WHERE creator_id = $1
ORDER BY updated_at DESC;

-- name: ListPublicBoardsByCreator :many
SELECT * FROM boards
WHERE creator_id = $1 AND is_public = true
ORDER BY updated_at DESC;

-- name: AddPinToBoard :exec
INSERT INTO board_pins (board_id, pin_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemovePinFromBoard :execrows
DELETE FROM board_pins
WHERE board_id = $1 AND pin_id = $2;

-- name: ListBoardPins :many
SELECT
    p.id, p.creator_id, p.url, p.title, p.description,
    p.field, p.tags, p.og_image, p.og_data, p.pin_count, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM board_pins bp
JOIN pins p ON p.id = bp.pin_id
JOIN creators c ON c.id = p.creator_id
WHERE bp.board_id = $1
ORDER BY bp.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountBoardPins :one
SELECT count(*) FROM board_pins
WHERE board_id = $1;

-- name: ListBoardPinImages :many
SELECT p.og_image FROM board_pins bp
JOIN pins p ON p.id = bp.pin_id
WHERE bp.board_id = $1
  AND p.og_image IS NOT NULL
ORDER BY bp.created_at DESC
LIMIT 4;
