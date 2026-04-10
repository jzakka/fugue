## Why

Fugue needs an efficient bot crawling system that balances cost with freshness. The current bot implementation lacks structured site graph management and relies on expensive AI calls for every crawl. By separating exploration (Pioneer) from harvesting (Harvester), we can use AI strategically for discovery while running free rule-based extraction at high frequency.

## What Changes

- Create database schema for site link graphs (nodes, edges, scripts)
- Implement Pioneer domain layer for AI-powered site exploration and script generation
- Implement Harvester domain layer for rule-based content extraction using stored scripts
- Abstract AI model interactions behind interfaces for flexibility
- Add run tracking and statistics for monitoring bot performance

## Capabilities

### New Capabilities

- `bot-graph-management`: Store and query site link graphs (nodes, edges) in PostgreSQL with efficient indexing
- `bot-pioneer-crawler`: Explore sites using BFS, classify page types, generate parsing scripts with AI, validate and reuse existing scripts
- `bot-harvester-crawler`: Traverse stored graphs, execute parsing scripts, extract content using existing pipeline
- `bot-script-lifecycle`: Generate, validate, store, and execute JavaScript parsing scripts with performance tracking
- `bot-run-tracking`: Record pioneer and harvester execution runs with statistics (nodes visited, items extracted, costs)

### Modified Capabilities

<!-- No existing bot capabilities are being modified at the spec level -->

## Impact

- **Database**: New tables in PostgreSQL for bot domain (6 new tables)
- **Backend**: New `internal/bot/` modules for graph, pioneer, harvester domains
- **Dependencies**: AI client interface (Claude API), Node.js runtime for script execution
- **Infrastructure**: Foundation for future CronJob deployments (Pioneer: daily, Harvester: hourly)
