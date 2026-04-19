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

### Pioneer
**Purpose:** Explore sites using BFS, generate parsing scripts with AI

**Key Features:**
- Priority-based BFS traversal (listing pages first)
- URL classification (listing, gallery, category, detail, skip)
- Strict domain validation (no subdomains)
- AI-powered script generation
- Script validation (70% threshold)
- Script reuse to minimize AI costs

**Configuration:**
```go
PioneerConfig{
    MaxDepth:        5,
    MaxNodesPerSite: 500,
    RateLimitMs:     2000,
    SuccessThreshold: 0.7, // 70%
}
```

**Usage:**
```go
pioneer := bot.NewPioneer(siteRepo, graphRepo, scriptRepo, runRepo, aiClient, executor, config)
err := pioneer.Run(ctx, siteID)
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

### Pipeline
Processes extracted items through existing bot pipeline.

```go
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
