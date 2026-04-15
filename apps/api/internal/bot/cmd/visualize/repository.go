package visualize

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// scriptKey is used as a map key for (site_id, node_type) pairs
type scriptKey struct {
	SiteID   uuid.UUID
	NodeType string
}

// GraphRepository fetches graph data from the database
type GraphRepository struct {
	queries *db.Queries
}

// NewGraphRepository creates a new repository instance
func NewGraphRepository(queries *db.Queries) *GraphRepository {
	return &GraphRepository{queries: queries}
}

// FetchGraphData retrieves all graph data from the database
func (r *GraphRepository) FetchGraphData(ctx context.Context) (*GraphData, error) {
	// Fetch sites
	siteRows, err := r.queries.ListAllSitesForGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sites: %w", err)
	}

	sites := make([]Site, len(siteRows))
	for i, row := range siteRows {
		sites[i] = Site{
			ID:        row.ID,
			Domain:    row.Domain,
			RootURL:   row.RootUrl,
			Active:    row.Active,
			CreatedAt: row.CreatedAt,
		}
	}

	// Fetch nodes
	nodeRows, err := r.queries.ListAllNodesForGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}

	// Build script existence lookup from DB
	scriptRows, err := r.queries.ListScriptKeysForGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scripts: %w", err)
	}
	scriptSet := make(map[scriptKey]bool, len(scriptRows))
	for _, row := range scriptRows {
		scriptSet[scriptKey{SiteID: row.SiteID, NodeType: row.NodeType}] = true
	}

	nodes := make([]Node, len(nodeRows))
	for i, row := range nodeRows {
		nodeType := ""
		if row.NodeType.Valid {
			nodeType = row.NodeType.String
		}

		sampleURL := ""
		if row.SampleUrl.Valid {
			sampleURL = row.SampleUrl.String
		}

		nodes[i] = Node{
			ID:        row.ID,
			SiteID:    row.SiteID,
			URL:       row.Url,
			SampleURL: sampleURL,
			NodeType:  nodeType,
			HasScript: scriptSet[scriptKey{SiteID: row.SiteID, NodeType: nodeType}],
			CreatedAt: row.CreatedAt,
		}
	}

	// Fetch edges
	edgeRows, err := r.queries.ListAllEdgesForGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch edges: %w", err)
	}

	edges := make([]Edge, len(edgeRows))
	for i, row := range edgeRows {
		edges[i] = Edge{
			ID:         row.ID,
			FromNodeID: row.FromNodeID,
			ToNodeID:   row.ToNodeID,
			CreatedAt:  row.CreatedAt,
		}
	}

	return &GraphData{
		Sites: sites,
		Nodes: nodes,
		Edges: edges,
		Metadata: Metadata{
			GeneratedAt: time.Now(),
			TotalSites:  len(sites),
			TotalNodes:  len(nodes),
			TotalEdges:  len(edges),
		},
	}, nil
}
