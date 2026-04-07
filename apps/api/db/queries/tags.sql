-- name: ListTags :many
SELECT id, name, slug, category, display_order
FROM tags
ORDER BY category, display_order;

-- name: ListTagsByCategory :many
SELECT id, name, slug, category, display_order
FROM tags
WHERE category = $1
ORDER BY display_order;

-- name: SearchTags :many
SELECT id, name, slug, category, display_order
FROM tags
WHERE name ILIKE '%' || $1 || '%'
ORDER BY category, display_order
LIMIT 50;

-- name: GetTagsByIDs :many
SELECT id, name, slug, category, display_order
FROM tags
WHERE id = ANY($1::uuid[]);

-- name: GetTagsForPins :many
SELECT pt.pin_id, t.id, t.name, t.slug, t.category
FROM pin_tags pt
JOIN tags t ON t.id = pt.tag_id
WHERE pt.pin_id = ANY($1::uuid[])
ORDER BY t.category, t.display_order;
