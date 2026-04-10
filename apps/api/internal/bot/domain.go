package bot

import (
	"time"

	"github.com/google/uuid"
)

// Site represents a crawlable site with its configuration and status
type Site struct {
	ID                 uuid.UUID
	Domain             string
	RootURL            string
	PioneerStatus      SiteStatus
	PioneerStartedAt   *time.Time
	PioneerCompletedAt *time.Time
	LastHarvestAt      *time.Time
	Active             bool
	Metadata           map[string]interface{}
	CreatedAt          time.Time
}

// SiteStatus represents the current state of a site's pioneer crawl
type SiteStatus string

const (
	SiteStatusPending    SiteStatus = "pending"
	SiteStatusInProgress SiteStatus = "in_progress"
	SiteStatusCompleted  SiteStatus = "completed"
	SiteStatusFailed     SiteStatus = "failed"
)

// GraphNode represents a URL node in the site graph
type GraphNode struct {
	ID            uuid.UUID
	SiteID        uuid.UUID
	URL           string
	URLHash       string
	Depth         int
	NodeType      NodeType
	ParentURL     *string
	ScriptID      *uuid.UUID
	VisitCount    int
	SuccessCount  int
	FailCount     int
	LastVisitedAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NodeType represents the type of page in the site graph
type NodeType string

const (
	NodeTypeListing  NodeType = "listing"
	NodeTypeGallery  NodeType = "gallery"
	NodeTypeDetail   NodeType = "detail"
	NodeTypeCategory NodeType = "category"
	NodeTypeSkip     NodeType = "skip"
)

// NodeTypePriority returns the priority value for BFS traversal
func NodeTypePriority(nt NodeType) int {
	switch nt {
	case NodeTypeListing:
		return 100
	case NodeTypeGallery:
		return 80
	case NodeTypeCategory:
		return 60
	case NodeTypeDetail:
		return 10
	case NodeTypeSkip:
		return 0
	default:
		return 0
	}
}

// GraphEdge represents a link relationship between two nodes
type GraphEdge struct {
	ID         uuid.UUID
	SiteID     uuid.UUID
	FromNodeID uuid.UUID
	ToNodeID   uuid.UUID
	CreatedAt  time.Time
}

// Script represents a parsing script for a specific site and node type
type Script struct {
	ID                     uuid.UUID
	SiteID                 uuid.UUID
	NodeType               string
	ScriptLang             string
	ScriptCode             string
	AIModel                string
	GenerationCostUSD      *float64
	ValidationSuccessCount int
	ValidationFailCount    int
	LastValidatedAt        *time.Time
	SuccessCount           int
	FailCount              int
	AvgExecutionMs         *int
	AvgItemsExtracted      *float64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
