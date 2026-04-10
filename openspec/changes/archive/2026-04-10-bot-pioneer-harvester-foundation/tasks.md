## 1. Database Schema

- [x] 1.1 Create migration file for bot_sites table with status tracking fields
- [x] 1.2 Create migration file for bot_graph_nodes with url_hash, depth, node_type, statistics
- [x] 1.3 Create migration file for bot_graph_edges with link relationships
- [x] 1.4 Create migration file for bot_scripts with AI metadata and performance tracking
- [x] 1.5 Create migration file for bot_pioneer_runs with discovery and cost statistics
- [x] 1.6 Create migration file for bot_harvest_runs with extraction statistics
- [x] 1.7 Add indexes: idx_graph_nodes_site, idx_graph_nodes_hash, idx_graph_nodes_type
- [x] 1.8 Add indexes: idx_graph_edges_from, idx_scripts_site
- [x] 1.9 Add indexes: idx_pioneer_runs_site, idx_harvest_runs_site
- [x] 1.10 Run migration in dev environment and verify with \d+ commands

## 2. SQLC Queries

- [x] 2.1 Define Site CRUD queries (CreateSite, GetSite, UpdatePioneerStatus, UpdateLastHarvest)
- [x] 2.2 Define GraphNode CRUD queries (CreateNode, GetNodeByHash, UpdateNodeStats, ListNodesBySite)
- [x] 2.3 Define GraphEdge queries (CreateEdge, GetEdgesByNode)
- [x] 2.4 Define Script CRUD queries (CreateScript, GetScriptBySiteType, UpdateScriptStats)
- [x] 2.5 Define PioneerRun queries (CreateRun, UpdateRunStats, GetRunsBySite)
- [x] 2.6 Define HarvesterRun queries (CreateRun, UpdateRunStats, GetRunsBySite)
- [x] 2.7 Run sqlc generate and verify generated Go code compiles

## 3. Domain Types

- [x] 3.1 Define Site domain type with status enums (pending, in_progress, completed, failed)
- [x] 3.2 Define GraphNode domain type with NodeType enum (listing, gallery, detail, category, skip)
- [x] 3.3 Define GraphEdge domain type
- [x] 3.4 Define Script domain type with validation and execution statistics
- [x] 3.5 Define PioneerRun and HarvesterRun domain types
- [x] 3.6 Define RawItem domain type (title, description, media_url, source_url, media_type)

## 4. Repository Interfaces

- [x] 4.1 Define SiteRepository interface (Create, Get, UpdateStatus, List)
- [x] 4.2 Define GraphRepository interface (CreateNode, GetNode, CreateEdge, ListNodes, UpdateNodeStats)
- [x] 4.3 Define ScriptRepository interface (Create, Get, Update, UpdateStats)
- [x] 4.4 Define RunRepository interface (CreatePioneerRun, CreateHarvesterRun, UpdateRun, GetRuns)
- [x] 4.5 Implement repositories using sqlc-generated queries

## 5. Infrastructure Interfaces

- [x] 5.1 Define AIClient interface (GenerateScript method with ScriptRequest/Response types)
- [x] 5.2 Define ScriptExecutor interface (Execute method returning RawItem array)
- [x] 5.3 Add mock implementations for testing (MockAIClient, MockScriptExecutor)

## 6. Pioneer Domain Service

- [x] 6.1 Implement BFS crawler with priority queue (listing=100, gallery=80, category=60, detail=10)
- [x] 6.2 Implement URL classification (keyword matching for node types)
- [x] 6.3 Implement domain validation (strict same-domain check, no subdomains)
- [x] 6.4 Implement file extension filtering (images, media, documents, static assets)
- [x] 6.5 Implement depth limiting (max_depth from config)
- [x] 6.6 Implement script validation logic (estimate items, execute script, check 70% threshold)
- [x] 6.7 Implement AI script generation flow (prompt construction, cost tracking)
- [x] 6.8 Implement script reuse logic (check existing, validate, skip generation if pass)
- [x] 6.9 Implement graph persistence (nodes, edges with parent tracking)
- [x] 6.10 Implement run statistics tracking (nodes discovered/updated, scripts generated/reused, AI cost)
- [x] 6.11 Implement error handling for AI client failures (retry logic, cost tracking on failure)
- [x] 6.12 Implement timeout handling for HTTP requests during crawling

## 7. Harvester Domain Service

- [x] 7.1 Implement full graph traversal (query all nodes for site)
- [x] 7.2 Implement node sorting by type priority (listing first, detail last)
- [x] 7.3 Implement script execution flow (fetch HTML, load script, call ScriptExecutor)
- [x] 7.4 Implement node statistics update (success_count, fail_count, last_visited_at)
- [x] 7.5 Implement pipeline integration (pass RawItem[] to existing Dedup → Download → Tag → Pin)
- [x] 7.6 Implement run statistics tracking (nodes visited/succeeded/failed, items extracted, pins created)
- [x] 7.7 Implement error handling for missing scripts and execution failures

## 8. Testing

- [x] 8.1 Write unit tests for BFS crawler with mock graph
- [x] 8.2 Write unit tests for URL classification (all node types)
- [x] 8.3 Write unit tests for domain validation (same domain, subdomain, external)
- [x] 8.4 Write unit tests for script validation logic (70% threshold cases)
- [x] 8.5 Write unit tests for Harvester traversal with mock nodes
- [x] 8.6 Write integration tests for repositories using test database
- [x] 8.7 Write end-to-end test: Pioneer creates graph, Harvester extracts items (all mocked)

## 9. Documentation

- [x] 9.1 Document domain types and interfaces in godoc comments
- [x] 9.2 Add README to internal/bot/ explaining Pioneer vs Harvester
- [x] 9.3 Update CLAUDE.md to reference bot domain in project structure
