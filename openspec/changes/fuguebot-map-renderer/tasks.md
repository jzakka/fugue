## 0. Schema Investigation

- [x] 0.1 Verify if bot_graph_edges has edge_type column in the actual schema
- [x] 0.2 If no edge_type exists, update spec and design to remove edge type differentiation
- [x] 0.3 Confirm node_type column naming (not page_type) in bot_graph_nodes table

## 1. Database Query Layer

- [ ] 1.1 Create SQL query to fetch all sites with their domains
- [ ] 1.2 Create SQL query to fetch all nodes with site_id, node_type, url, visit stats
- [ ] 1.3 Create SQL query to fetch all edges with source/target node IDs (and edge_type if column exists)
- [ ] 1.4 Implement Go structs for graph data (Site, Node, Edge)
- [ ] 1.5 Implement repository functions in apps/api/internal/bot/repository/ for graph queries

## 2. Harvester Script Detection

- [ ] 2.1 Define constant for Harvester script path template: `apps/api/internal/bot/sources/{domain}/{node_type}.go`
- [ ] 2.2 Implement function to check script existence given (domain, node_type)
- [ ] 2.3 Implement function to calculate script coverage statistics (total vs covered nodes)
- [ ] 2.4 Add script coverage data to Node struct

## 3. Graph Data Serialization

- [ ] 3.1 Define JSON schema for graph data (nodes, edges, sites)
- [ ] 3.2 Implement function to serialize graph data to JSON
- [ ] 3.3 Add metadata (timestamp, total counts, coverage stats) to JSON output
- [ ] 3.4 Handle empty graph case (no nodes found)

## 4. HTML Visualization Template

- [ ] 4.1 Create D3.js force-directed graph template in apps/api/internal/bot/cmd/visualize/template.html
- [ ] 4.2 Implement site grouping with color coding
- [ ] 4.3 Add node rendering with node_type labels and script coverage visual indicators
- [ ] 4.4 Add edge rendering with directionality (arrows), style by type if edge_type exists
- [ ] 4.5 Implement interactive features (zoom, pan, drag nodes)
- [ ] 4.6 Add legend showing node colors, edge types (if applicable), and script coverage meaning
- [ ] 4.7 Add statistics panel (total nodes, edges, sites, coverage %)
- [ ] 4.8 Implement node tooltips showing URL, visit stats, and node_type

## 5. Graphviz Export (Optional)

- [ ] 5.1 Implement DOT format generator for Graphviz
- [ ] 5.2 Add site subgraph clustering in DOT output
- [ ] 5.3 Implement node styling based on script coverage
- [ ] 5.4 Add edge styling based on edge type (if edge_type column exists)
- [ ] 5.5 Implement Graphviz execution wrapper (check if installed)
- [ ] 5.6 Add PNG/SVG export via Graphviz with graceful fallback

## 6. CLI Command Implementation

- [ ] 6.1 Create command structure in apps/api/internal/bot/cmd/visualize/main.go
- [ ] 6.2 Add DB connection setup with config loading
- [ ] 6.3 Implement format flag for output type selection (html, png, svg)
- [ ] 6.4 Implement output flag for custom output path
- [ ] 6.5 Add filter-site flag for single-site visualization
- [ ] 6.6 Implement graph data fetching and processing pipeline
- [ ] 6.7 Implement HTML generation with embedded JSON data
- [ ] 6.8 Add error handling for empty DB and missing dependencies

## 7. Makefile Integration

- [ ] 7.1 Add `show-map` target to root Makefile
- [ ] 7.2 Configure target to run: `cd $(API_DIR) && go run internal/bot/cmd/visualize/main.go`
- [ ] 7.3 Configure default output path (e.g., ./graph.html)
- [ ] 7.4 Add help text for make show-map
- [ ] 7.5 Test make target with existing DB data

## 8. Documentation

- [ ] 8.1 Add usage example to README.md (how to run make show-map)
- [ ] 8.2 Document CLI flags and output formats
- [ ] 8.3 Add screenshot or example graph image to docs
- [ ] 8.4 Document Graphviz installation (optional dependency)
- [ ] 8.5 Update CLAUDE.md with script path convention for detection logic

## 9. Testing & Validation

- [ ] 9.1 Test with empty database (no nodes)
- [ ] 9.2 Test with single site, multiple nodes
- [ ] 9.3 Test with multiple sites and cross-site edges
- [ ] 9.4 Verify script detection logic with real bot/sources/ structure
- [ ] 9.5 Test HTML output opens correctly in browser
- [ ] 9.6 Test PNG/SVG export (if Graphviz available)
- [ ] 9.7 Test filtering by site
- [ ] 9.8 Validate coverage statistics accuracy
