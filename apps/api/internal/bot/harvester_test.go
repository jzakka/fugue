package bot

import (
	"database/sql"
	"testing"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Test Harvester node sorting by priority
func TestHarvesterSortNodesByPriority(t *testing.T) {
	h := &Harvester{}

	nodes := []db.BotGraphNode{
		{NodeType: sql.NullString{String: "detail", Valid: true}},   // priority 10
		{NodeType: sql.NullString{String: "listing", Valid: true}},  // priority 100
		{NodeType: sql.NullString{String: "gallery", Valid: true}},  // priority 80
		{NodeType: sql.NullString{String: "category", Valid: true}}, // priority 60
		{NodeType: sql.NullString{String: "skip", Valid: true}},     // priority 0
	}

	h.sortNodesByPriority(nodes)

	// After sorting: listing (100), gallery (80), category (60), detail (10), skip (0)
	expected := []string{"listing", "gallery", "category", "detail", "skip"}
	for i, node := range nodes {
		if node.NodeType.String != expected[i] {
			t.Errorf("Position %d: got %s, want %s", i, node.NodeType.String, expected[i])
		}
	}
}

func TestHarvesterSortWithNullTypes(t *testing.T) {
	h := &Harvester{}

	nodes := []db.BotGraphNode{
		{NodeType: sql.NullString{Valid: false}},                   // no type
		{NodeType: sql.NullString{String: "listing", Valid: true}}, // priority 100
		{NodeType: sql.NullString{Valid: false}},                   // no type
	}

	// Should not panic
	h.sortNodesByPriority(nodes)

	// Listing should be first
	if nodes[0].NodeType.Valid && nodes[0].NodeType.String != "listing" {
		t.Error("Expected listing node to be first")
	}
}
