package bot

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// SiteRepo implements SiteRepository using sqlc-generated queries
type SiteRepo struct {
	q *db.Queries
}

func NewSiteRepo(database *sql.DB) *SiteRepo {
	return &SiteRepo{q: db.New(database)}
}

func (r *SiteRepo) Create(ctx context.Context, arg db.CreateSiteParams) (db.BotSite, error) {
	return r.q.CreateSite(ctx, arg)
}

func (r *SiteRepo) Get(ctx context.Context, id uuid.UUID) (db.BotSite, error) {
	return r.q.GetSite(ctx, id)
}

func (r *SiteRepo) GetByDomain(ctx context.Context, domain string) (db.BotSite, error) {
	return r.q.GetSiteByDomain(ctx, domain)
}

func (r *SiteRepo) ListActive(ctx context.Context) ([]db.BotSite, error) {
	return r.q.ListActiveSites(ctx)
}

// GraphRepo implements GraphRepository using sqlc-generated queries
type GraphRepo struct {
	q *db.Queries
}

func NewGraphRepo(database *sql.DB) *GraphRepo {
	return &GraphRepo{q: db.New(database)}
}

func (r *GraphRepo) CreateNode(ctx context.Context, arg db.CreateNodeParams) (db.BotGraphNode, error) {
	return r.q.CreateNode(ctx, arg)
}

func (r *GraphRepo) GetNodeByHash(ctx context.Context, arg db.GetNodeByHashParams) (db.BotGraphNode, error) {
	return r.q.GetNodeByHash(ctx, arg)
}

func (r *GraphRepo) UpdateNodeScript(ctx context.Context, arg db.UpdateNodeScriptParams) error {
	return r.q.UpdateNodeScript(ctx, arg)
}

func (r *GraphRepo) ListNodesBySite(ctx context.Context, siteID uuid.UUID) ([]db.BotGraphNode, error) {
	return r.q.ListNodesBySite(ctx, siteID)
}

func (r *GraphRepo) ListNodesByType(ctx context.Context, arg db.ListNodesByTypeParams) ([]db.BotGraphNode, error) {
	return r.q.ListNodesByType(ctx, arg)
}

func (r *GraphRepo) CreateEdge(ctx context.Context, arg db.CreateEdgeParams) error {
	return r.q.CreateEdge(ctx, arg)
}

func (r *GraphRepo) GetEdgesByNode(ctx context.Context, fromNodeID uuid.UUID) ([]db.BotGraphEdge, error) {
	return r.q.GetEdgesByNode(ctx, fromNodeID)
}

// ScriptRepo implements ScriptRepository using sqlc-generated queries
type ScriptRepo struct {
	q *db.Queries
}

func NewScriptRepo(database *sql.DB) *ScriptRepo {
	return &ScriptRepo{q: db.New(database)}
}

func (r *ScriptRepo) Create(ctx context.Context, arg db.CreateScriptParams) (db.BotScript, error) {
	return r.q.CreateScript(ctx, arg)
}

func (r *ScriptRepo) GetBySiteType(ctx context.Context, arg db.GetScriptBySiteTypeParams) (db.BotScript, error) {
	return r.q.GetScriptBySiteType(ctx, arg)
}
