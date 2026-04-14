package visualize

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/db"
)

const (
	// ScriptPathTemplate defines where Harvester scripts are located
	ScriptPathTemplate = "apps/api/internal/bot/sources/%s/%s.go"
)

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

	// Create domain lookup map
	domainMap := make(map[uuid.UUID]string)
	for _, site := range sites {
		domainMap[site.ID] = site.Domain
	}

	nodes := make([]Node, len(nodeRows))
	for i, row := range nodeRows {
		domain := domainMap[row.SiteID]
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
			HasScript: CheckScriptExists(domain, nodeType),
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

// CheckScriptExists checks if a Harvester script file exists for the given domain and node type
func CheckScriptExists(domain, nodeType string) bool {
	if domain == "" || nodeType == "" {
		return false
	}

	scriptPath := fmt.Sprintf(ScriptPathTemplate, domain, nodeType)
	_, err := os.Stat(scriptPath)
	return err == nil
}

