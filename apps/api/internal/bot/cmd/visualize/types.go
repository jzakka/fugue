package visualize

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// GraphData represents the complete graph structure for visualization
type GraphData struct {
	Sites    []Site   `json:"sites"`
	Nodes    []Node   `json:"nodes"`
	Edges    []Edge   `json:"edges"`
	Metadata Metadata `json:"metadata"`
}

// Site represents a crawled domain
type Site struct {
	ID        uuid.UUID `json:"id"`
	Domain    string    `json:"domain"`
	RootURL   string    `json:"root_url"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Node represents a discovered page
type Node struct {
	ID        uuid.UUID `json:"id"`
	SiteID    uuid.UUID `json:"site_id"`
	URL       string    `json:"url"`
	SampleURL string    `json:"sample_url,omitempty"`
	NodeType  string    `json:"node_type"`
	HasScript bool      `json:"has_script"`
	CreatedAt time.Time `json:"created_at"`
}

// Edge represents a link between nodes
type Edge struct {
	ID         uuid.UUID `json:"id"`
	FromNodeID uuid.UUID `json:"from_node_id"`
	ToNodeID   uuid.UUID `json:"to_node_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// MarshalJSON adds source/target fields for D3 forceLink compatibility
func (e Edge) MarshalJSON() ([]byte, error) {
	type edgeJSON struct {
		ID         uuid.UUID `json:"id"`
		FromNodeID uuid.UUID `json:"from_node_id"`
		ToNodeID   uuid.UUID `json:"to_node_id"`
		Source     uuid.UUID `json:"source"`
		Target     uuid.UUID `json:"target"`
		CreatedAt  time.Time `json:"created_at"`
	}
	return json.Marshal(edgeJSON{
		ID:         e.ID,
		FromNodeID: e.FromNodeID,
		ToNodeID:   e.ToNodeID,
		Source:     e.FromNodeID,
		Target:     e.ToNodeID,
		CreatedAt:  e.CreatedAt,
	})
}

// Metadata contains summary statistics
type Metadata struct {
	GeneratedAt    time.Time     `json:"generated_at"`
	TotalSites     int           `json:"total_sites"`
	TotalNodes     int           `json:"total_nodes"`
	TotalEdges     int           `json:"total_edges"`
	ScriptCoverage CoverageStats `json:"script_coverage"`
}

// CoverageStats tracks script implementation coverage
type CoverageStats struct {
	TotalNodes      int     `json:"total_nodes"`
	CoveredNodes    int     `json:"covered_nodes"`
	CoveragePercent float64 `json:"coverage_percent"`
}
