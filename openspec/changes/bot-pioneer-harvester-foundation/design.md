## Context

Fugue's current bot implementation processes sites without maintaining a persistent understanding of their structure. Every crawl session starts fresh, and AI-powered parsing happens on every run, leading to high costs and inability to scale to frequent updates.

The bot-architecture.md defines a two-process system:
- **Pioneer**: Expensive AI-powered exploration (daily/weekly) that maps site structure and generates parsing scripts
- **Harvester**: Cheap rule-based extraction (hourly) that uses Pioneer's artifacts

This design implements the foundation: database schema for graph storage and domain-layer abstractions that keep infrastructure concerns (AI models, script execution) behind interfaces.

**Current constraints:**
- PostgreSQL as primary data store
- Go backend (Chi router, sqlc for queries)
- AI interactions must be cost-tracked
- Must support future Kubernetes CronJob deployment

## Goals / Non-Goals

**Goals:**
- Define complete ERD for site graph, scripts, and run tracking
- Implement domain layer for Pioneer (graph exploration, script lifecycle)
- Implement domain layer for Harvester (graph traversal, content extraction)
- Abstract AI and script execution behind interfaces
- Enable monitoring through run statistics

**Non-Goals:**
- Concrete AI client implementation (interface only)
- Node.js script executor implementation (interface only)
- CLI commands (future phase)
- Kubernetes deployment manifests (future phase)
- Pioneer scheduling logic (future phase)
- Harvester scheduling logic (future phase)

## Decisions

### Decision 1: Separate tables for sites, nodes, edges, scripts, runs

**Rationale:** Graph structure (nodes/edges) is separate from execution artifacts (scripts) and operational history (runs). This allows:
- Graph queries independent of script performance
- Script reuse across multiple nodes of same type
- Historical analysis without denormalizing statistics into graph tables

**Alternatives considered:**
- Embed script code in nodes → rejected: same script used by multiple nodes, updates would require bulk edits
- Single runs table for both Pioneer and Harvester → rejected: different statistics make union queries awkward

### Decision 2: url_hash for efficient lookups

**Rationale:** URLs can be very long (2000+ chars). MD5 hash (32 chars) enables fast index lookups and UNIQUE constraints without hitting index size limits.

**Alternatives considered:**
- Use URL directly in UNIQUE constraint → rejected: PostgreSQL index size limits
- Use UUID from URL hash → rejected: adds complexity, MD5 hex is sufficient

### Decision 3: Node type classification (listing, gallery, detail, etc.)

**Rationale:** Different page types require different parsing strategies. Classification enables:
- Priority-based traversal (listing pages first for efficiency)
- Script specialization (one script per site + node_type)
- Skip patterns (low-priority detail pages)

**Alternatives considered:**
- Single generic script per site → rejected: one-size-fits-all scripts have poor extraction rates
- Per-URL scripts → rejected: graph of 500 nodes = 500 scripts, high AI cost

### Decision 4: Script validation statistics in bot_scripts table

**Rationale:** Scripts have a lifecycle: generate → validate → execute. Tracking success/fail counts enables:
- 70% threshold validation logic
- Detection of degraded scripts (site redesign)
- Performance monitoring (avg_execution_ms, avg_items_extracted)

### Decision 5: Abstract AI and script execution behind interfaces

**Rationale:** Domain layer should not depend on specific AI providers or runtime environments. Interfaces enable:
- Testing without real AI calls
- Swapping Claude for GPT or local models
- Script execution in Node.js, Deno, or future runtimes

**Interfaces:**
```go
type AIClient interface {
    GenerateScript(ctx context.Context, req ScriptRequest) (ScriptResponse, error)
}

type ScriptExecutor interface {
    Execute(ctx context.Context, script string, html string, url string) ([]RawItem, error)
}
```

### Decision 6: Separate pioneer_runs and harvest_runs tables

**Rationale:** Different operations track different metrics:
- Pioneer: nodes discovered, scripts generated/reused, AI cost
- Harvester: items extracted, pins created, dedup rate

Union table with nullable columns would be confusing and error-prone.

### Decision 7: BFS priority queue at domain layer, not in DB

**Rationale:** Priority queue logic (priority scores, visited set, depth tracking) is algorithmic, not persistent state. BFS state lives in-memory during a run; only the resulting graph is persisted.

**Alternatives considered:**
- Store BFS queue in DB → rejected: queue is transient, DB adds latency
- Store priority scores in nodes table → rejected: priorities are heuristic, not intrinsic to node

## Risks / Trade-offs

**[Risk]** Large graph size (500+ nodes per site, 10+ sites) → Query performance degradation  
**Mitigation:** Indexes on (site_id, url_hash), (site_id, node_type). Monitor query times in run statistics.

**[Risk]** Script validation false negatives (70% threshold too loose) → Bad scripts persist  
**Mitigation:** Track fail_count per script. Manual review UI for scripts with high fail rates (future).

**[Risk]** AI model changes (Claude API deprecation, price hikes) → Pioneer breaks  
**Mitigation:** AIClient interface isolates provider. Switching requires only adapter implementation.

**[Risk]** Graph evolution (deleted pages, new sections) → Stale nodes accumulate  
**Mitigation:** Pioneer marks visited nodes with last_visited_at. Prune nodes not visited in 30 days (future).

**[Trade-off]** Full graph re-traversal (no incremental crawl) → Redundant work  
**Benefit:** Simplicity. Incremental logic requires change detection, delta tracking, complex error recovery. Full traversal is stateless and easy to reason about.

**[Trade-off]** Synchronous script validation (blocks BFS) → Slower Pioneer runs  
**Benefit:** Immediate feedback. Async validation would complicate flow and require queue management.

## Migration Plan

**Phase 1: Schema creation (this change)**
1. Create migration file with all 6 tables
2. Run migration in dev environment
3. Generate sqlc queries for CRUD operations
4. Verify indexes with EXPLAIN ANALYZE on sample data

**Phase 2: Domain layer (this change)**
1. Implement domain types (Site, GraphNode, Script, Run)
2. Implement repository interfaces (SiteRepo, GraphRepo, ScriptRepo, RunRepo)
3. Implement Pioneer domain service (BFS, script lifecycle)
4. Implement Harvester domain service (traversal, extraction)
5. Unit tests with mock AI/executor

**Rollback:**
- If migration fails: DROP tables in reverse order (runs → scripts → edges → nodes → sites)
- If domain layer has bugs: No production impact yet (no API endpoints or cron jobs)

## Open Questions

- **Q:** Should script validation use a sandbox (timeout, memory limits)?  
  **A:** Deferred to implementation. Start with subprocess timeout, add resource limits if needed.

- **Q:** What happens when a site completely changes structure (redesign)?  
  **A:** Pioneer detects validation failures, regenerates scripts. Old graph nodes become stale but harmless. Future: prune by last_visited_at.

- **Q:** Should scripts be versioned (script_v1, script_v2)?  
  **A:** Not in this phase. UNIQUE(site_id, node_type) means updates replace old script. If versioning needed, add version column later.
