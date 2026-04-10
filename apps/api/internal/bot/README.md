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
- Full graph traversal every run
- Node sorting by type priority
- Script execution via ScriptExecutor
- Integration with existing Dedup → Download → Tag → Pin pipeline
- Run statistics tracking

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
harvester := bot.NewHarvester(siteRepo, graphRepo, scriptRepo, runRepo, executor, pipeline, config)
err := harvester.Run(ctx, siteID)
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
    Process(ctx context.Context, items []RawItem) (pinsCreated int, deduped int, error error)
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

## Future Work

- HTTP client implementation for fetchHTML
- Actual AI client integration (Claude/OpenAI)
- Node.js script executor
- CLI commands (fuguebot pioneer, fuguebot harvest)
- Kubernetes CronJob deployment
