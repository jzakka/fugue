-- name: ListActiveBotSources :many
SELECT id, name, platform, seed_urls, interval_hours, enabled, last_crawled_at, stats, created_at
FROM bot_sources
WHERE enabled = true
ORDER BY created_at;

-- name: ListAllBotSources :many
SELECT id, name, platform, seed_urls, interval_hours, enabled, last_crawled_at, stats, created_at
FROM bot_sources
ORDER BY created_at;

-- name: GetBotSource :one
SELECT id, name, platform, seed_urls, interval_hours, enabled, last_crawled_at, stats, created_at
FROM bot_sources
WHERE id = $1;

-- name: CreateBotSource :one
INSERT INTO bot_sources (name, platform, seed_urls, interval_hours, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateBotSourceStats :exec
UPDATE bot_sources
SET last_crawled_at = now(), stats = $2
WHERE id = $1;

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
INSERT INTO bot_sites (domain, root_url, metadata)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSite :one
SELECT * FROM bot_sites
WHERE id = $1;

-- name: GetSiteByDomain :one
SELECT * FROM bot_sites
WHERE domain = $1;

-- name: UpdatePioneerStatus :exec
UPDATE bot_sites
SET pioneer_status = $2, pioneer_started_at = $3, pioneer_completed_at = $4
WHERE id = $1;

-- name: UpdateLastHarvest :exec
UPDATE bot_sites
SET last_harvest_at = $1
WHERE id = $2;

-- name: ListActiveSites :many
SELECT * FROM bot_sites
WHERE active = true
ORDER BY created_at;

-- Bot Graph Nodes queries
-- name: CreateNode :one
INSERT INTO bot_graph_nodes (site_id, url, url_hash, depth, node_type, parent_url, script_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetNodeByHash :one
SELECT * FROM bot_graph_nodes
WHERE site_id = $1 AND url_hash = $2;

-- name: UpdateNodeStats :exec
UPDATE bot_graph_nodes
SET visit_count = $2, success_count = $3, fail_count = $4, last_visited_at = $5, updated_at = now()
WHERE id = $1;

-- name: UpdateNodeScript :exec
UPDATE bot_graph_nodes
SET script_id = $2, updated_at = now()
WHERE id = $1;

-- name: ListNodesBySite :many
SELECT * FROM bot_graph_nodes
WHERE site_id = $1
ORDER BY depth, node_type;

-- name: ListNodesByType :many
SELECT * FROM bot_graph_nodes
WHERE site_id = $1 AND node_type = $2
ORDER BY depth;

-- Bot Graph Edges queries
-- name: CreateEdge :exec
INSERT INTO bot_graph_edges (site_id, from_node_id, to_node_id, link_text)
VALUES ($1, $2, $3, $4)
ON CONFLICT (from_node_id, to_node_id) DO NOTHING;

-- name: GetEdgesByNode :many
SELECT * FROM bot_graph_edges
WHERE from_node_id = $1;

-- Bot Scripts queries
-- name: CreateScript :one
INSERT INTO bot_scripts (site_id, node_type, script_lang, script_code, ai_model, generation_cost_usd)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (site_id, node_type) DO UPDATE
SET script_code = EXCLUDED.script_code,
    ai_model = EXCLUDED.ai_model,
    generation_cost_usd = EXCLUDED.generation_cost_usd,
    updated_at = now()
RETURNING *;

-- name: GetScriptBySiteType :one
SELECT * FROM bot_scripts
WHERE site_id = $1 AND node_type = $2;

-- name: UpdateScriptValidationStats :exec
UPDATE bot_scripts
SET validation_success_count = $2,
    validation_fail_count = $3,
    last_validated_at = $4,
    updated_at = now()
WHERE id = $1;

-- name: UpdateScriptExecutionStats :exec
UPDATE bot_scripts
SET success_count = $2,
    fail_count = $3,
    avg_execution_ms = $4,
    avg_items_extracted = $5,
    updated_at = now()
WHERE id = $1;

-- Bot Pioneer Runs queries
-- name: CreatePioneerRun :one
INSERT INTO bot_pioneer_runs (site_id, status)
VALUES ($1, $2)
RETURNING *;

-- name: UpdatePioneerRunStats :exec
UPDATE bot_pioneer_runs
SET completed_at = $2,
    status = $3,
    nodes_discovered = $4,
    nodes_updated = $5,
    scripts_generated = $6,
    scripts_reused = $7,
    ai_api_calls = $8,
    ai_cost_usd = $9,
    error_message = $10
WHERE id = $1;

-- name: GetPioneerRunsBySite :many
SELECT * FROM bot_pioneer_runs
WHERE site_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- Bot Harvester Runs queries
-- name: CreateHarvesterRun :one
INSERT INTO bot_harvest_runs (site_id, status)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateHarvesterRunStats :exec
UPDATE bot_harvest_runs
SET completed_at = $2,
    status = $3,
    nodes_visited = $4,
    nodes_succeeded = $5,
    nodes_failed = $6,
    items_extracted = $7,
    items_deduplicated = $8,
    pins_created = $9,
    error_message = $10
WHERE id = $1;

-- name: GetHarvesterRunsBySite :many
SELECT * FROM bot_harvest_runs
WHERE site_id = $1
ORDER BY started_at DESC
LIMIT $2;
