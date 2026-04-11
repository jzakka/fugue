## Why

Pioneer crawls content sources and builds a graph of nodes (pages, artists, works) and edges (connections between them). Currently, this graph data exists only in the database with no way to visualize what Pioneer has discovered. Developers need visibility into the graph structure to debug crawl logic, verify script coverage, and understand site topology before building Harvester parsers.

## What Changes

- Add a visualization command (`make show-map` or `make visualize-graph`) that renders the current Pioneer graph
- Display nodes grouped by source site (domain)
- Show connections between nodes with edge types
- Indicate which nodes have Harvester parsing scripts implemented vs. still needed
- Provide an interactive or static graph view (e.g., browser-based, SVG, or terminal output)

## Capabilities

### New Capabilities
<!-- None -->

### Modified Capabilities
- `tooling`: Add graph visualization capability to render Pioneer's discovered node graph with site grouping, node connections, and script coverage indicators

## Impact

- **Bot System**: New visualization tooling for the Pioneer graph stored in PostgreSQL
- **Developer Experience**: Adds visibility into crawl progress and script coverage
- **Build System**: New `make` target for graph rendering
- **Dependencies**: May require a graph visualization library (e.g., Graphviz, D3.js, or Go-based renderer)
