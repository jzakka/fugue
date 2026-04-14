-- name: ListActiveBotSources :many
SELECT id, name, seed_urls, interval_hours, enabled, created_at
FROM bot_sources
WHERE enabled = true
ORDER BY created_at;

-- name: ListAllBotSources :many
SELECT id, name, seed_urls, interval_hours, enabled, created_at
FROM bot_sources
ORDER BY created_at;

-- name: GetBotSource :one
SELECT id, name, seed_urls, interval_hours, enabled, created_at
FROM bot_sources
WHERE id = $1;

-- name: CreateBotSource :one
INSERT INTO bot_sources (name, seed_urls, interval_hours, enabled)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ToggleBotSource :one
UPDATE bot_sources
SET enabled = $2
WHERE id = $1
RETURNING *;

-- name: DeleteBotSource :execrows
DELETE FROM bot_sources
WHERE id = $1;

-- name: PinURLExists :one
SELECT EXISTS(SELECT 1 FROM pins WHERE url = $1) AS url_exists;

-- name: ListAllTags :many
SELECT id, name FROM tags;

-- Bot Sites queries
-- name: CreateSite :one
INSERT INTO bot_sites (domain, root_url)
VALUES ($1, $2)
RETURNING *;

-- name: GetSite :one
SELECT * FROM bot_sites
WHERE id = $1;

-- name: GetSiteByDomain :one
SELECT * FROM bot_sites
WHERE domain = $1;

-- name: ListActiveSites :many
SELECT * FROM bot_sites
WHERE active = true
ORDER BY created_at;

-- Bot Graph Nodes queries
-- name: CreateNode :one
INSERT INTO bot_graph_nodes (site_id, url, url_hash, node_type, script_id, sample_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetNodeByHash :one
SELECT * FROM bot_graph_nodes
WHERE site_id = $1 AND url_hash = $2;

-- name: GetNodeByURL :one
SELECT * FROM bot_graph_nodes
WHERE site_id = $1 AND url = $2;

-- name: UpdateNodeScript :exec
UPDATE bot_graph_nodes
SET script_id = $2, updated_at = now()
WHERE id = $1;

-- name: ListNodesBySite :many
SELECT * FROM bot_graph_nodes
WHERE site_id = $1
ORDER BY created_at;

-- name: ListNodesByType :many
SELECT * FROM bot_graph_nodes
WHERE site_id = $1 AND node_type = $2
ORDER BY created_at;

-- Bot Graph Edges queries
-- name: CreateEdge :exec
INSERT INTO bot_graph_edges (from_node_id, to_node_id)
VALUES ($1, $2)
ON CONFLICT (from_node_id, to_node_id) DO NOTHING;

-- name: GetEdgesByNode :many
SELECT * FROM bot_graph_edges
WHERE from_node_id = $1;

-- name: ListEdgesBySiteNodes :many
SELECT e.id, e.from_node_id, e.to_node_id
FROM bot_graph_edges e
JOIN bot_graph_nodes n ON e.from_node_id = n.id
WHERE n.site_id = $1;

-- name: DeleteEdgesByIDs :exec
DELETE FROM bot_graph_edges WHERE id = ANY($1::uuid[]);

-- Bot Scripts queries
-- name: CreateScript :one
INSERT INTO bot_scripts (site_id, node_type, script_lang, script_code, ai_model)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (site_id, node_type) DO UPDATE
SET script_code = EXCLUDED.script_code,
    ai_model = EXCLUDED.ai_model,
    updated_at = now()
RETURNING *;

-- name: GetScriptBySiteType :one
SELECT * FROM bot_scripts
WHERE site_id = $1 AND node_type = $2;

-- Graph Visualization queries
-- name: ListAllSitesForGraph :many
SELECT id, domain, root_url, active, created_at
FROM bot_sites
ORDER BY domain;

-- name: ListAllNodesForGraph :many
SELECT id, site_id, url, node_type, sample_url, created_at
FROM bot_graph_nodes
ORDER BY site_id, created_at;

-- name: ListAllEdgesForGraph :many
SELECT id, from_node_id, to_node_id, created_at
FROM bot_graph_edges
ORDER BY created_at;

