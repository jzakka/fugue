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

## Primary Image Cache

Harvester는 Pin 생성 시점에 page HTML에서 primary 이미지를 추출하여 object storage에 캐시한다. 추출 우선순위는 `og:image` → `twitter:image` → article/main 내 의미있는 `<img>` → JSON-LD `image`이며, 첫 번째 유효 후보만 채택한다. 저장 키는 `images/<sha256(normalizedURL)>/<unix_ts>.<ext>` 형식이고, 확장자는 응답 Content-Type → URL path → `.bin` 순서로 결정된다. 다운로드/업로드 실패, 크기 초과(기본 20 MiB, `HARVESTER_IMAGE_CACHE_MAX_BYTES`로 조정) 등 **모든 실패는 단일 fallback 경로**로 처리되어 Pin의 `og_image` 컬럼에 원본 후보 URL이 그대로 기록되고 Pin 생성은 계속된다. 후보가 없으면 `og_image`는 NULL. 본 스펙상 `og_image`와 `thumbnail_url`은 동일 의미이지만 현재 스키마에는 `og_image`만 존재하므로 그 컬럼만 사용한다.

## Future Work

- HTTP client implementation for fetchHTML
- Actual AI client integration (Claude/OpenAI)
- Node.js script executor
- CLI commands (fuguebot pioneer, fuguebot harvest)
- Kubernetes CronJob deployment
