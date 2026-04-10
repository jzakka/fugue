package bot

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Harvester config
type HarvesterConfig struct {
	RateLimitMs      int
	RetryFailedNodes bool
	MaxRetries       int
}

// Pipeline processes extracted items
type Pipeline interface {
	Process(ctx context.Context, items []RawItem) (pinsCreated int, deduped int, error error)
}

// Harvester executes scripts and extracts content
type Harvester struct {
	siteRepo   SiteRepository
	graphRepo  GraphRepository
	scriptRepo ScriptRepository
	runRepo    RunRepository
	executor   ScriptExecutor
	pipeline   Pipeline
	config     HarvesterConfig
}

// NewHarvester creates a new Harvester service
func NewHarvester(
	siteRepo SiteRepository,
	graphRepo GraphRepository,
	scriptRepo ScriptRepository,
	runRepo RunRepository,
	executor ScriptExecutor,
	pipeline Pipeline,
	config HarvesterConfig,
) *Harvester {
	return &Harvester{
		siteRepo:   siteRepo,
		graphRepo:  graphRepo,
		scriptRepo: scriptRepo,
		runRepo:    runRepo,
		executor:   executor,
		pipeline:   pipeline,
		config:     config,
	}
}

// Run executes a full harvest for a site
func (h *Harvester) Run(ctx context.Context, siteID uuid.UUID) error {
	// Create run record
	run, err := h.runRepo.CreateHarvesterRun(ctx, db.CreateHarvesterRunParams{
		SiteID: siteID,
		Status: string(RunStatusRunning),
	})
	if err != nil {
		return fmt.Errorf("create harvester run: %w", err)
	}

	// Execute harvest
	stats, harvestErr := h.harvest(ctx, siteID)

	// Update run statistics
	completedAt := time.Now()
	status := string(RunStatusCompleted)
	var errorMsg *string
	if harvestErr != nil {
		status = string(RunStatusFailed)
		msg := harvestErr.Error()
		errorMsg = &msg
	}

	err = h.runRepo.UpdateHarvesterRunStats(ctx, db.UpdateHarvesterRunStatsParams{
		ID:                run.ID,
		CompletedAt:       sql.NullTime{Time: completedAt, Valid: true},
		Status:            status,
		NodesVisited:      sql.NullInt32{Int32: int32(stats.NodesVisited), Valid: true},
		NodesSucceeded:    sql.NullInt32{Int32: int32(stats.NodesSucceeded), Valid: true},
		NodesFailed:       sql.NullInt32{Int32: int32(stats.NodesFailed), Valid: true},
		ItemsExtracted:    sql.NullInt32{Int32: int32(stats.ItemsExtracted), Valid: true},
		ItemsDeduplicated: sql.NullInt32{Int32: int32(stats.ItemsDeduplicated), Valid: true},
		PinsCreated:       sql.NullInt32{Int32: int32(stats.PinsCreated), Valid: true},
		ErrorMessage:      sql.NullString{String: stringOrEmpty(errorMsg), Valid: errorMsg != nil},
	})
	if err != nil {
		return fmt.Errorf("update run stats: %w", err)
	}

	// Update site last harvest time (best effort)
	if harvestErr == nil {
		_ = h.siteRepo.UpdateLastHarvest(ctx, db.UpdateLastHarvestParams{
			LastHarvestAt: sql.NullTime{Time: completedAt, Valid: true},
			ID:            siteID,
		})
	}

	return harvestErr
}

type harvestStats struct {
	NodesVisited      int
	NodesSucceeded    int
	NodesFailed       int
	ItemsExtracted    int
	ItemsDeduplicated int
	PinsCreated       int
}

// harvest performs the graph traversal and content extraction
func (h *Harvester) harvest(ctx context.Context, siteID uuid.UUID) (*harvestStats, error) {
	stats := &harvestStats{}

	// Task 7.1: Get all nodes for this site
	nodes, err := h.graphRepo.ListNodesBySite(ctx, siteID)
	if err != nil {
		return stats, fmt.Errorf("list nodes: %w", err)
	}

	// Task 7.2: Sort nodes by type priority (listing first, detail last)
	h.sortNodesByPriority(nodes)

	// Process each node
	for _, node := range nodes {
		stats.NodesVisited++

		// Rate limiting
		time.Sleep(time.Duration(h.config.RateLimitMs) * time.Millisecond)

		// Task 7.3: Execute script and extract items
		items, execErr := h.executeNode(ctx, node)

		if execErr != nil {
			// Task 7.7: Handle execution errors
			stats.NodesFailed++

			// Task 7.4: Update node failure statistics
			h.updateNodeFailure(ctx, node)

			// Continue with next node
			continue
		}

		stats.NodesSucceeded++
		stats.ItemsExtracted += len(items)

		// Task 7.4: Update node success statistics
		h.updateNodeSuccess(ctx, node)

		// Task 7.5: Send items to pipeline
		if len(items) > 0 {
			pinsCreated, deduped, pipeErr := h.pipeline.Process(ctx, items)
			if pipeErr != nil {
				// Log error but continue
				continue
			}
			stats.PinsCreated += pinsCreated
			stats.ItemsDeduplicated += deduped
		}
	}

	return stats, nil
}

// sortNodesByPriority sorts nodes by type priority (listing first)
func (h *Harvester) sortNodesByPriority(nodes []db.BotGraphNode) {
	sort.Slice(nodes, func(i, j int) bool {
		// Get priority for each node type
		priI := 0
		priJ := 0

		if nodes[i].NodeType.Valid {
			priI = NodeTypePriority(NodeType(nodes[i].NodeType.String))
		}
		if nodes[j].NodeType.Valid {
			priJ = NodeTypePriority(NodeType(nodes[j].NodeType.String))
		}

		// Higher priority first
		return priI > priJ
	})
}

// executeNode fetches HTML, loads script, and executes it
func (h *Harvester) executeNode(ctx context.Context, node db.BotGraphNode) ([]RawItem, error) {
	// Task 7.7: Check if node has a script
	if !node.ScriptID.Valid {
		return nil, fmt.Errorf("node has no script")
	}

	// Get the script
	var script db.BotScript
	var err error

	if node.NodeType.Valid {
		script, err = h.scriptRepo.GetBySiteType(ctx, db.GetScriptBySiteTypeParams{
			SiteID:   node.SiteID,
			NodeType: node.NodeType.String,
		})
		if err != nil {
			return nil, fmt.Errorf("get script: %w", err)
		}
	} else {
		return nil, fmt.Errorf("node type not set")
	}

	// Fetch HTML
	html, err := h.fetchHTML(ctx, node.Url)
	if err != nil {
		return nil, fmt.Errorf("fetch HTML: %w", err)
	}

	// Execute script
	items, err := h.executor.Execute(ctx, script.ScriptCode, html, node.Url)
	if err != nil {
		return nil, fmt.Errorf("execute script: %w", err)
	}

	return items, nil
}

// updateNodeSuccess increments success count and updates last visited time
func (h *Harvester) updateNodeSuccess(ctx context.Context, node db.BotGraphNode) {
	now := time.Now()

	visitCount := int32(0)
	if node.VisitCount.Valid {
		visitCount = node.VisitCount.Int32
	}

	successCount := int32(0)
	if node.SuccessCount.Valid {
		successCount = node.SuccessCount.Int32
	}

	failCount := int32(0)
	if node.FailCount.Valid {
		failCount = node.FailCount.Int32
	}

	// Update node stats (best effort)
	_ = h.graphRepo.UpdateNodeStats(ctx, db.UpdateNodeStatsParams{
		ID:            node.ID,
		VisitCount:    sql.NullInt32{Int32: visitCount + 1, Valid: true},
		SuccessCount:  sql.NullInt32{Int32: successCount + 1, Valid: true},
		FailCount:     sql.NullInt32{Int32: failCount, Valid: true},
		LastVisitedAt: sql.NullTime{Time: now, Valid: true},
	})
}

// updateNodeFailure increments fail count and updates last visited time
func (h *Harvester) updateNodeFailure(ctx context.Context, node db.BotGraphNode) {
	now := time.Now()

	visitCount := int32(0)
	if node.VisitCount.Valid {
		visitCount = node.VisitCount.Int32
	}

	successCount := int32(0)
	if node.SuccessCount.Valid {
		successCount = node.SuccessCount.Int32
	}

	failCount := int32(0)
	if node.FailCount.Valid {
		failCount = node.FailCount.Int32
	}

	// Update node stats (best effort)
	_ = h.graphRepo.UpdateNodeStats(ctx, db.UpdateNodeStatsParams{
		ID:            node.ID,
		VisitCount:    sql.NullInt32{Int32: visitCount + 1, Valid: true},
		SuccessCount:  sql.NullInt32{Int32: successCount, Valid: true},
		FailCount:     sql.NullInt32{Int32: failCount + 1, Valid: true},
		LastVisitedAt: sql.NullTime{Time: now, Valid: true},
	})
}

// fetchHTML fetches HTML content with timeout
func (h *Harvester) fetchHTML(ctx context.Context, urlStr string) (string, error) {
	// TODO: Implement actual HTTP fetching with timeout
	// For now, return empty to allow compilation
	return "", fmt.Errorf("not implemented")
}
