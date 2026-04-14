package bot

import (
	"context"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// SiteRepository provides access to site data
type SiteRepository interface {
	Create(ctx context.Context, arg db.CreateSiteParams) (db.BotSite, error)
	Get(ctx context.Context, id uuid.UUID) (db.BotSite, error)
	GetByDomain(ctx context.Context, domain string) (db.BotSite, error)
	ListActive(ctx context.Context) ([]db.BotSite, error)
}

// GraphRepository provides access to graph node and edge data
type GraphRepository interface {
	CreateNode(ctx context.Context, arg db.CreateNodeParams) (db.BotGraphNode, error)
	GetNodeByHash(ctx context.Context, arg db.GetNodeByHashParams) (db.BotGraphNode, error)
	// GetNodeByURL retrieves a node by site ID and exact URL match
	// Used by Harvester to find the root node for BFS traversal
	GetNodeByURL(ctx context.Context, arg db.GetNodeByURLParams) (db.BotGraphNode, error)
	UpdateNodeScript(ctx context.Context, arg db.UpdateNodeScriptParams) error
	ListNodesBySite(ctx context.Context, siteID uuid.UUID) ([]db.BotGraphNode, error)
	ListNodesByType(ctx context.Context, arg db.ListNodesByTypeParams) ([]db.BotGraphNode, error)

	CreateEdge(ctx context.Context, arg db.CreateEdgeParams) error
	GetEdgesByNode(ctx context.Context, fromNodeID uuid.UUID) ([]db.BotGraphEdge, error)
	ListEdgesBySiteNodes(ctx context.Context, siteID uuid.UUID) ([]db.ListEdgesBySiteNodesRow, error)
	DeleteEdgesByIDs(ctx context.Context, ids []uuid.UUID) error
}

// ScriptRepository provides access to script data
type ScriptRepository interface {
	Create(ctx context.Context, arg db.CreateScriptParams) (db.BotScript, error)
	GetBySiteType(ctx context.Context, arg db.GetScriptBySiteTypeParams) (db.BotScript, error)
}
