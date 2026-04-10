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
	UpdatePioneerStatus(ctx context.Context, arg db.UpdatePioneerStatusParams) error
	UpdateLastHarvest(ctx context.Context, arg db.UpdateLastHarvestParams) error
	ListActive(ctx context.Context) ([]db.BotSite, error)
}

// GraphRepository provides access to graph node and edge data
type GraphRepository interface {
	CreateNode(ctx context.Context, arg db.CreateNodeParams) (db.BotGraphNode, error)
	GetNodeByHash(ctx context.Context, arg db.GetNodeByHashParams) (db.BotGraphNode, error)
	UpdateNodeStats(ctx context.Context, arg db.UpdateNodeStatsParams) error
	UpdateNodeScript(ctx context.Context, arg db.UpdateNodeScriptParams) error
	ListNodesBySite(ctx context.Context, siteID uuid.UUID) ([]db.BotGraphNode, error)
	ListNodesByType(ctx context.Context, arg db.ListNodesByTypeParams) ([]db.BotGraphNode, error)

	CreateEdge(ctx context.Context, arg db.CreateEdgeParams) error
	GetEdgesByNode(ctx context.Context, fromNodeID uuid.UUID) ([]db.BotGraphEdge, error)
}

// ScriptRepository provides access to script data
type ScriptRepository interface {
	Create(ctx context.Context, arg db.CreateScriptParams) (db.BotScript, error)
	GetBySiteType(ctx context.Context, arg db.GetScriptBySiteTypeParams) (db.BotScript, error)
	UpdateValidationStats(ctx context.Context, arg db.UpdateScriptValidationStatsParams) error
	UpdateExecutionStats(ctx context.Context, arg db.UpdateScriptExecutionStatsParams) error
}

// RunRepository provides access to run history data
type RunRepository interface {
	CreatePioneerRun(ctx context.Context, arg db.CreatePioneerRunParams) (db.BotPioneerRun, error)
	UpdatePioneerRunStats(ctx context.Context, arg db.UpdatePioneerRunStatsParams) error
	GetPioneerRunsBySite(ctx context.Context, arg db.GetPioneerRunsBySiteParams) ([]db.BotPioneerRun, error)

	CreateHarvesterRun(ctx context.Context, arg db.CreateHarvesterRunParams) (db.BotHarvestRun, error)
	UpdateHarvesterRunStats(ctx context.Context, arg db.UpdateHarvesterRunStatsParams) error
	GetHarvesterRunsBySite(ctx context.Context, arg db.GetHarvesterRunsBySiteParams) ([]db.BotHarvestRun, error)
}
