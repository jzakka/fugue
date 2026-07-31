-- name: SearchPinsBySimilarity :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url,
    (similarity(p.title, $1) +
      CASE WHEN EXISTS (
        SELECT 1 FROM pin_tags pt JOIN tags t ON t.id = pt.tag_id
        WHERE pt.pin_id = p.id AND t.name ILIKE '%' || $1 || '%'
      ) THEN 0.5 ELSE 0 END)::float4 AS score
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE similarity(p.title, $1) > 0.1
   OR EXISTS (
     SELECT 1 FROM pin_tags pt JOIN tags t ON t.id = pt.tag_id
     WHERE pt.pin_id = p.id AND t.name ILIKE '%' || $1 || '%'
   )
ORDER BY score DESC, p.created_at DESC, p.id DESC
LIMIT $2 OFFSET $3;

-- name: SearchPinsByILIKE :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE p.title ILIKE '%' || $1 || '%'
   OR EXISTS (
     SELECT 1 FROM pin_tags pt JOIN tags t ON t.id = pt.tag_id
     WHERE pt.pin_id = p.id AND t.name ILIKE '%' || $1 || '%'
   )
ORDER BY p.created_at DESC, p.id DESC
LIMIT $2 OFFSET $3;

-- name: SearchPinsWithTagFilter :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url,
    (similarity(p.title, $1) +
      CASE WHEN EXISTS (
        SELECT 1 FROM pin_tags pt JOIN tags t ON t.id = pt.tag_id
        WHERE pt.pin_id = p.id AND t.name ILIKE '%' || $1 || '%'
      ) THEN 0.5 ELSE 0 END)::float4 AS score
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE (similarity(p.title, $1) > 0.1
   OR EXISTS (
     SELECT 1 FROM pin_tags pt JOIN tags t ON t.id = pt.tag_id
     WHERE pt.pin_id = p.id AND t.name ILIKE '%' || $1 || '%'
   ))
  AND (SELECT COUNT(*) FROM pin_tags pt2
       WHERE pt2.pin_id = p.id AND pt2.tag_id = ANY($4::uuid[])) = array_length($4::uuid[], 1)
ORDER BY score DESC, p.created_at DESC, p.id DESC
LIMIT $2 OFFSET $3;

-- name: SearchPinsILIKEWithTagFilter :many
SELECT
    p.id, p.creator_id, p.media_url, p.media_type, p.url, p.title, p.description,
    p.og_image, p.og_data, p.created_at,
    c.id AS creator_id_ref,
    c.nickname AS creator_nickname,
    c.avatar_url AS creator_avatar_url
FROM pins p
JOIN creators c ON c.id = p.creator_id
WHERE (p.title ILIKE '%' || $1 || '%'
   OR EXISTS (
     SELECT 1 FROM pin_tags pt JOIN tags t ON t.id = pt.tag_id
     WHERE pt.pin_id = p.id AND t.name ILIKE '%' || $1 || '%'
   ))
  AND (SELECT COUNT(*) FROM pin_tags pt2
       WHERE pt2.pin_id = p.id AND pt2.tag_id = ANY($4::uuid[])) = array_length($4::uuid[], 1)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $2 OFFSET $3;

-- name: SearchCreatorsBySimilarity :many
SELECT id, nickname, avatar_url, created_at
FROM creators
WHERE similarity(nickname, $1) > 0.1
ORDER BY similarity(nickname, $1) DESC, created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: SearchCreatorsByILIKE :many
SELECT id, nickname, avatar_url, created_at
FROM creators
WHERE nickname ILIKE '%' || $1 || '%'
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: SearchBoardsBySimilarity :many
SELECT b.id, b.creator_id, b.name, b.description, b.is_public, b.created_at, b.updated_at,
       c.nickname AS creator_nickname
FROM boards b
JOIN creators c ON c.id = b.creator_id
WHERE b.is_public = true AND similarity(b.name, $1) > 0.1
ORDER BY similarity(b.name, $1) DESC, b.created_at DESC, b.id DESC
LIMIT $2 OFFSET $3;

-- name: SearchBoardsByILIKE :many
SELECT b.id, b.creator_id, b.name, b.description, b.is_public, b.created_at, b.updated_at,
       c.nickname AS creator_nickname
FROM boards b
JOIN creators c ON c.id = b.creator_id
WHERE b.is_public = true AND b.name ILIKE '%' || $1 || '%'
ORDER BY b.created_at DESC, b.id DESC
LIMIT $2 OFFSET $3;

-- name: SearchTopTags :many
SELECT t.id, t.name, t.slug, t.category, COUNT(*) AS count
FROM pin_tags pt
JOIN tags t ON t.id = pt.tag_id
WHERE pt.pin_id = ANY($1::uuid[])
GROUP BY t.id, t.name, t.slug, t.category
ORDER BY count DESC
LIMIT 10;
