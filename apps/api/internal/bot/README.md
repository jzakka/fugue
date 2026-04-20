# Bot Domain - Pioneer & Harvester

The bot domain implements a two-process crawling system for discovering and extracting content from external sites.

## Architecture

```
Pioneer (AI-powered, infrequent)
  ↓
Site Graph + Scripts (persisted)
  ↓
Harvester (rule-based, frequent)
  ↓
Content Pipeline → Pins
```

## Components

### Crawler Package (New)
**Purpose:** Decoupled BFS traversal with testable page fetching

**Key Features:**
- Fetcher interface separates BFS logic from HTTP
- FileFetcher for file-based testing
- HTTPFetcher for production HTTP requests
- Full BFS traversal with depth limits
- Same-domain validation
- URL normalization and deduplication

**Usage:**
```go
// Testing with file fixtures
fetcher := crawler.NewFileFetcher("testdata")
c := crawler.NewBFSCrawler(fetcher)
result, err := c.Crawl(ctx, "http://example.com/", 2)

// Production with HTTP
fetcher := crawler.NewHTTPFetcher(http.DefaultClient)
c := crawler.NewBFSCrawler(fetcher)
result, err := c.Crawl(ctx, "http://example.com/", 2)
```

### Pioneer (PioneerConsumer)
**Purpose:** Explore sites by consuming the `pioneer_frontier` queue, snapshot raw HTML, and fan out discovered links to both `pioneer_frontier` (new URLs) and `harvester_frontier` (original URL + snapshot key).

**Key Features:**
- `URLScheduler`-backed loop: `Dequeue → fetch → snapshot → extract → filter → Enqueue(pioneer) + EnqueueHarvester → SetStatus(fetched)`
- No in-memory queue, no visited map: all dedup/ordering owned by the scheduler (`FOR UPDATE SKIP LOCKED`)
- Multiple instances may run concurrently against the same scheduler
- Composable filter chain: `DomainFilter`, `ExtensionFilter`, `PathPatternFilter`, `RobotsFilter`, `CanonicalDedupFilter`

**Usage:**
```go
consumer := bot.NewPioneerConsumer(scheduler, snapshotStore, filterChain, fetcher)
err := consumer.Run(ctx)
```

### Harvester
**Purpose:** Execute scripts on stored graph, extract content

**Key Features:**
- **BFS graph traversal**: Follows edges from root node, processes level by level
- **Cycle detection**: Visited set prevents infinite loops in cyclic graphs
- **Priority sorting within levels**: listing/gallery nodes before detail nodes at same depth
- **Script execution** via ScriptExecutor
- **Integration** with existing Dedup → Download → Tag → Pin pipeline
- **Run statistics tracking**

**Traversal Algorithm:**
1. Find root URL node as starting point
2. Initialize visited set and BFS queue
3. For each level:
   - Sort nodes by type priority (listing > gallery > category > detail)
   - Process each node (fetch HTML, execute script, extract items)
   - Fetch children via edges, filter out visited nodes
   - Add unvisited children to next level
4. Continue until queue is empty

**Configuration:**
```go
HarvesterConfig{
    RateLimitMs:     2000,
    RetryFailedNodes: true,
    MaxRetries:      3,
}
```

**Usage:**
```go
harvester := bot.NewHarvester(siteRepo, graphRepo, scriptRepo, executor, pipeline, config)
stats, err := harvester.Run(ctx, siteID)
```

## Domain Types

### NodeType
- `listing`: Trending/popular pages (priority 100)
- `gallery`: Collection pages (priority 80)
- `category`: Tag/genre pages (priority 60)
- `detail`: Individual item pages (priority 10)
- `skip`: Login/signup/ad pages (priority 0)

### RunStatus
- `running`: In progress
- `completed`: Successfully finished
- `failed`: Encountered fatal error

## Interfaces

### AIClient
Abstracts AI model interaction for script generation.

```go
type AIClient interface {
    GenerateScript(ctx context.Context, req ScriptRequest) (ScriptResponse, error)
}
```

### ScriptExecutor
Abstracts script runtime (Node.js, Deno, etc).

```go
type ScriptExecutor interface {
    Execute(ctx context.Context, script string, html string, url string) ([]RawItem, error)
}
```

### DocumentPipeline
Persists a `PinDocument` as a single bot Pin, idempotently upserting on canonical URL.

```go
type DocumentPipeline interface {
    ProcessDocument(ctx context.Context, node db.BotGraphNode, doc PinDocument) (created bool, pinID uuid.UUID, err error)
    MarkSkipped(ctx context.Context, node db.BotGraphNode) error
}
```

`created=true` maps to the `PinsCreated` stat; `created=false` maps to `Deduped`. `MarkSkipped` records classifier-rejected nodes without creating a Pin. The `HarvestPipeline.MarkSkipped` is currently a no-op (frontier `harvested_at` writes are owned by the scheduler-consumer change); the Harvester still increments `Skipped` unconditionally so the stat is accurate before that wiring lands.

## PinDocument flow

The harvester converts one fetched page into one `PinDocument`:

```
fetch HTML
  ↓
PerSiteAdapter (if registered) ──→ on error ──→ GenericExtractor  (AdapterFallback++)
  ↓                                     ↓
  └────────────── PinDocument ──────────┘
                       ↓
              Classifier.Classify → pinnable=false ─→ MarkSkipped (Skipped++)
                       ↓ pinnable=true
              ProcessDocument → Upsert on canonical URL
                       ↓
              created=true → PinsCreated++   created=false → Deduped++
```

Stats categories are **mutually exclusive**: each node increments exactly one of `PinsCreated`, `Deduped`, `Skipped`, `Failed`. `AdapterFallback` is an **auxiliary** counter that may increment on the same node as any primary category.

### Extractors

- **GenericExtractor**: Walks HTML once and resolves `(title, body_text, canonical_url, thumbnail_url, media_candidates, lang, author, published_at)` via a fallback chain over OG meta → Twitter Card → JSON-LD (`schema.org/Article`, `CreativeWork`) → `<article>` → `<h1>`/`<title>` / `<link rel=canonical>` / `<html lang>` / `<time datetime>`. Media candidates are `<img>/<video>/<audio>/<source>` inside `<article>` (or `<body>` when no article), capped at 50 by default. Cross-domain canonical candidates are dropped inside the fallback chain; the returned canonical always points back to the fetched host.
- **PerSiteAdapter**: Site-specific extractors that take precedence over the generic extractor for their domain. Registered in an in-memory `AdapterRegistry` with exact and `*.example.com` wildcard matching (longest suffix wins; exact beats wildcard). On adapter error, the harvester falls back to `GenericExtractor` and increments `AdapterFallback`.
- **ScriptAdapter**: A `PerSiteAdapter` wrapping the existing `GojaExecutor`. Looks up the `(site_id, node_type)` script and collapses N `RawItem`s into 1 `PinDocument` — the first `RawItem` becomes the primary metadata (title/body/thumbnail), remaining items become `og_data.media_candidates`. Node type is passed via `context.Context` (`WithNodeType`/`NodeTypeFromContext`).

### Classifier

Single-pass rule-based gate with three reason codes (priority order):

1. **`listing`** — `links/words > threshold` (default 0.5) with division-by-zero guard
2. **`empty_body`** — body text byte length `< 200` bytes (default)
3. **`no_primary_media`** — no thumbnail and no media candidates (body length sufficient, else `empty_body` matches first)

The classifier depends only on `PinDocument` — never on `node_type` or external state. The verdict is persisted in `pins.og_data.classifier = {pinnable, reason?}`.

### Canonical-URL upsert

Bot Pins are keyed on canonical URL, enforced by a PostgreSQL **partial unique index**:

```sql
CREATE UNIQUE INDEX pins_url_bot_unique ON pins(url)
  WHERE creator_id = '00000000-0000-0000-0000-00000000f096';
```

The `UpsertBotPinByURL` sqlc query uses `ON CONFLICT (url) WHERE creator_id = '<literal UUID>'` — the literal is required because PostgreSQL only matches partial indexes when the predicate is IMMUTABLE. The UUID literal is duplicated across three locations (see `source.go` for the IMMUTABLE-sync policy). The query returns `(xmax = 0) AS inserted` which the harvester maps to `PinsCreated` (inserted=true) vs `Deduped` (inserted=false).

## Configuration (env)

| Env var                             | Default   | Description                                                                 |
| ----------------------------------- | --------- | --------------------------------------------------------------------------- |
| `HARVESTER_BODY_TEXT_MIN_BYTES`     | `200`     | Classifier `empty_body` threshold                                           |
| `HARVESTER_LINK_DENSITY_THRESHOLD`  | `0.5`     | Classifier `listing` threshold (links/words)                                |
| `HARVESTER_MEDIA_CANDIDATES_MAX`    | `50`      | Generic extractor `media_candidates` cap                                    |
| `FUGUE_BOT_CREATOR_ID`              | compiled  | Override bot UUID. **Must match** the migration predicate + upsert literal. |
| `HARVESTER_IMAGE_CACHE_MAX_BYTES`   | 20 MiB    | Primary image cache size threshold                                          |
| `HARVESTER_IMAGE_CACHE_TTL_DAYS`    | `90`      | Primary image cache age-based TTL                                           |

## Legacy Pipeline (RawItem)

```go
// HarvestPipeline.Process is retained for pre-existing tests. New code
// uses DocumentPipeline. RawItem flow is now confined to ScriptAdapter.
type Pipeline interface {
    Process(ctx context.Context, items []RawItem) (pinsCreated int, deduped int, failed int, err error)
}
```

## Database Schema

### bot_sites
Site definitions and crawl status

### bot_graph_nodes
URL nodes with depth, type, statistics

### bot_graph_edges
Link relationships between nodes

### bot_scripts
Parsing scripts per (site, node_type)

### bot_pioneer_runs
Pioneer execution history and AI costs

### bot_harvest_runs
Harvester execution history and extraction stats

## Testing

Mock implementations provided for all interfaces:
- `MockAIClient`: Returns dummy scripts
- `MockScriptExecutor`: Returns dummy items

Run tests:
```bash
go test ./internal/bot/...
```

## Image cache

Harvester caches the primary image of each new Pin to our object storage so Pin views are decoupled from upstream availability. Candidates are extracted from the item's page HTML in priority order — `<meta property="og:image">` → `<meta name|property="twitter:image">` → `<article>`/`<main>` 내 의미 있는 `<img>` (width·height 모두 ≥100 이거나 비어있지 않은 `alt`) → `<script type="application/ld+json">`의 `image` 필드 — 그리고 첫 번째 유효 후보(절대 URL, http/https, data: 아님, 1×1 추적 픽셀 아님)가 채택된다. 채택된 URL은 정규화(fragment 제거, scheme/host 소문자, path·query 보존) 후 `images/<sha256>/<unix_ts>.<ext>` 키로 저장되며, 확장자는 Content-Type → URL path → `.bin` fallback 순으로 결정된다. 성공 시 storage URL이, 실패(다운로드·업로드·20 MiB 임계 초과 중 어느 것이든) 시 원본 후보 URL이, 후보 없음 시 NULL이 단일 컬럼 `pin.og_image`에 기록된다. 이미지 캐시 실패는 Pin 생성을 차단하지 않는다.

## Future Work

- HTTP client implementation for fetchHTML
- Actual AI client integration (Claude/OpenAI)
- Node.js script executor
- CLI commands (fuguebot pioneer, fuguebot harvest)
- Kubernetes CronJob deployment
