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
- Define ERD for site graph and parsing scripts
- Implement domain layer for Pioneer (BFS exploration, script lifecycle)
- Implement domain layer for Harvester (graph traversal, content extraction)
- Abstract AI and script execution behind interfaces

**Non-Goals:**
- Concrete AI client implementation (interface only)
- Node.js script executor implementation (interface only)
- Run statistics tracking (use logging/metrics instead)
- CLI commands (future phase)
- Kubernetes deployment manifests (future phase)
- Scheduling logic (future phase)

## Decisions

### Decision 1: Separate tables for sites, nodes, edges, scripts

**Rationale:** Graph structure (nodes/edges) is separate from execution artifacts (scripts). This allows:
- Graph queries independent of script performance
- Script reuse across multiple nodes of same type
- Clean OLTP schema without mixing operational statistics

**Alternatives considered:**
- Embed script code in nodes → rejected: same script used by multiple nodes, updates would require bulk edits
- Include run statistics in OLTP tables → rejected: OLAP data doesn't belong in transactional DB

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

### Decision 4: Script validation [UPDATED]

**Original plan:** 통계 필드(validation counts, execution times)로 스크립트 품질 추적.  
**Current:** 통계 필드 제거됨 (Decision 9). 검증 로직은 유지하되 성공/실패는 로깅으로 수집.

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

### Decision 6: No run statistics in OLTP database

**Rationale:** Operational metrics (AI cost, items extracted, success rates) are OLAP data. Mixing them with transactional graph data:
- Bloats production tables with append-only logs
- Complicates queries (graph structure vs execution history)
- Better served by structured logging or metrics systems

**Alternatives:**
- Use application logging for debugging
- Use metrics (Prometheus) for monitoring
- Use existing event pipeline (Kinesis → S3) for analytics

### Decision 7: BFS priority queue at domain layer, not in DB

**Rationale:** Priority queue logic (priority scores, visited set) is algorithmic, not persistent state. BFS state lives in-memory during a run; only the resulting graph is persisted.

**Alternatives considered:**
- Store BFS queue in DB → rejected: queue is transient, DB adds latency
- Store priority scores in nodes table → rejected: priorities are heuristic, not intrinsic to node

### Decision 8: Minimal node metadata (no depth, parent_url, visit stats)

Graph nodes should store only classification state, not traversal history.

- **Removed:** depth, parent_url, success_count, visit_count, fail_count, last_visited_at
- **Rationale:** Traversal metadata belongs in logs/metrics, not OLTP schema. Nodes represent URL classification (hub/list/detail), not execution history.
- **Result:** Nodes contain only: id, site_id, url, url_hash, node_type, script_id, timestamps

### Decision 9: 통계 필드 제거

bot_scripts, bot_sites, bot_sources 테이블에서 운영 통계를 모두 제거.

- **bot_scripts:** validation/execution 카운터, 평균 실행시간, 생성 비용 제거
- **bot_sites:** pioneer 상태, harvest 타임스탬프 제거  
- **bot_sources:** platform, stats, last_crawled_at 제거
- **Rationale:** 통계는 로깅/메트릭 시스템으로 수집. OLTP ≠ OLAP.

### Decision 10: Source 플러그인 재설계 필요

platform 필드 제거로 인해 기존 Source 매핑 로직이 작동하지 않음. 향후 재설계 필요.

- **Current:** engine.go에서 Source 플러그인 매핑 비활성화 (TODO 주석)
- **Future:** bot_sources 테이블의 역할 재정의 또는 Source 플러그인 식별 방법 변경

## Risks / Trade-offs

**[Risk]** Large graph size (500+ nodes per site, 10+ sites) → Query performance degradation  
**Mitigation:** Indexes on (site_id, url_hash), (site_id, node_type). Monitor via application logging.

**[Risk]** Script validation false negatives (70% threshold too loose) → Bad scripts persist  
**Mitigation:** Track fail_count per script. Manual review UI for scripts with high fail rates (future).

**[Risk]** AI model changes (Claude API deprecation, price hikes) → Pioneer breaks  
**Mitigation:** AIClient interface isolates provider. Switching requires only adapter implementation.

**[Risk]** Graph evolution (deleted pages, new sections) → Stale nodes accumulate  
**Mitigation:** Periodic graph cleanup (delete nodes not referenced in recent crawls - future).

**[Trade-off]** Full graph re-traversal (no incremental crawl) → Redundant work  
**Benefit:** Simplicity. Incremental logic requires change detection, delta tracking, complex error recovery. Full traversal is stateless and easy to reason about.

**[Trade-off]** Synchronous script validation (blocks BFS) → Slower Pioneer runs  
**Benefit:** Immediate feedback. Async validation would complicate flow and require queue management.

**[Trade-off]** No run statistics in DB → Limited operational visibility  
**Benefit:** Clean schema separation. Logging/metrics provide better tooling for ops data than SQL queries.

## Migration Plan

**Phase 1: Schema creation (this change)**
1. Create migration files for 4 tables (sites, nodes, edges, scripts)
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
