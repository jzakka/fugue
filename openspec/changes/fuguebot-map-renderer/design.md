## Context

Pioneer currently builds a comprehensive graph of discovered content nodes and their relationships in PostgreSQL. This graph contains:
- Sites (domains being crawled)
- Nodes (pages discovered, with node_type classification)
- Edges (links between nodes)
- Visit statistics (Harvester interaction tracking)

However, developers have no way to visualize this graph structure. When debugging Pioneer logic, planning Harvester script development, or understanding site topology, developers must manually write SQL queries to explore the graph.

The Bot system consists of two components:
- **Pioneer**: Graph-based crawler that discovers nodes and edges
- **Harvester**: Extracts structured content from discovered nodes using page_type-specific parsers

Harvester parsers are implemented in `apps/api/internal/bot/sources/<domain>/` with one file per node_type. Not all discovered node_types have parsers yet.

## Goals / Non-Goals

**Goals:**
- Provide instant visual feedback on Pioneer's crawl progress and coverage
- Enable developers to identify which node_types need Harvester parsers
- Visualize site structure and link topology for debugging
- Make graph data accessible without writing SQL queries
- Support both quick exploration (browser) and documentation (image export)

**Non-Goals:**
- Real-time live updates (graph is static snapshot)
- Graph editing or manipulation through the UI
- Performance metrics or crawl speed visualization
- Harvester execution logs (only script existence check)
- Graph analytics or complex queries (focus is visualization)

## Decisions

### Decision 1: Make target over standalone CLI tool
**Choice:** Implement as `make show-map` that wraps a Go command  
**Alternatives:**
- Standalone binary: More flexible but harder to discover
- Web service endpoint: Over-engineered for dev tooling

**Rationale:** Makefile is the primary developer interface for the project. A make target provides discoverability and consistency with existing workflows (`make test`, `make run`).

### Decision 2: HTML with embedded D3.js for default output
**Choice:** Generate standalone HTML file with D3.js force-directed graph  
**Alternatives:**
- Graphviz DOT: Static but requires external tools
- Terminal-based (termgraph): Limited visual richness
- Dedicated web UI: Over-engineered for occasional use

**Rationale:** D3.js provides interactivity (zoom, pan, drag) without requiring a running server. Self-contained HTML files are easy to share and archive. Force-directed layout automatically handles complex topologies.

### Decision 3: Check Harvester script existence by file system
**Choice:** Look for files in `apps/api/internal/bot/sources/<domain>/<node_type>.go`  
**Alternatives:**
- Database flag: Requires manual maintenance
- Code parsing: Over-complex and fragile

**Rationale:** File existence is ground truth. Convention-based paths (`sources/<domain>/<node_type>.go`) make this reliable and zero-maintenance.

### Decision 4: Optional image export with Graphviz
**Choice:** Support `--format=png|svg` via Graphviz DOT  
**Alternatives:**
- Headless Chrome screenshots: Heavy dependency
- Server-side rendering: Complex infrastructure

**Rationale:** Graphviz is industry-standard for graph export. DOT format is simple to generate. Optional dependency doesn't block primary use case (HTML).

### Decision 5: Site grouping with visual clustering
**Choice:** Use D3.js force simulation with explicit site grouping force  
**Alternatives:**
- Subgraphs: Cluttered for many nodes
- Separate graphs per site: Loses cross-site edge visibility

**Rationale:** Force simulation naturally clusters related nodes while preserving cross-site edge visibility. Color-coding by site provides clear visual distinction.

## Risks / Trade-offs

**[Risk] Large graphs (1000+ nodes) may be slow or unreadable**  
→ Mitigation: Add filtering by site/node_type. Limit initial render to N nodes with pagination.

**[Risk] D3.js requires JavaScript knowledge for maintenance**  
→ Mitigation: Use well-documented template. Most changes are server-side Go code.

**[Risk] Graphviz installation is optional but creates UX inconsistency**  
→ Mitigation: Graceful fallback with clear error message pointing to installation instructions.

**[Risk] File path convention for script detection could become outdated**  
→ Mitigation: Document the convention in CLAUDE.md. Single constant for path template.

**[Trade-off] Static snapshot vs. live updates**  
Trade-off: Simpler implementation but requires re-running command to see changes. Acceptable for debugging workflow.

**[Trade-off] No graph editing capabilities**  
Trade-off: Read-only visualization is simpler and safer (no accidental graph corruption). Editing is not a core use case.

## Migration Plan

1. Implement Go command in `apps/api/internal/bot/cmd/visualize/`
2. Add `make show-map` target to root Makefile
3. Test with existing DB data (Pioneer must have run at least once)
4. Document in README with example screenshot
5. No rollback needed (pure addition, no breaking changes)

## Open Questions

- Should we include node/edge creation timestamps in the visualization?
- Filter UI: Checkboxes in HTML or CLI flags only?
- Color scheme: Use DESIGN.md brand colors or generic graph colors?
- Should we show "orphan" nodes (no edges) separately?
