package bot

import (
	"context"
	_ "database/sql"
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
	executor   ScriptExecutor
	pipeline   Pipeline
	config     HarvesterConfig
}

// NewHarvester creates a new Harvester service
func NewHarvester(
	siteRepo SiteRepository,
	graphRepo GraphRepository,
	scriptRepo ScriptRepository,
	executor ScriptExecutor,
	pipeline Pipeline,
	config HarvesterConfig,
) *Harvester {
	return &Harvester{
		siteRepo:   siteRepo,
		graphRepo:  graphRepo,
		scriptRepo: scriptRepo,
		executor:   executor,
		pipeline:   pipeline,
		config:     config,
	}
}

func (h *Harvester) Run(ctx context.Context, siteID uuid.UUID) error {
	// Execute harvest
	return h.harvest(ctx, siteID)
}

func (h *Harvester) harvest(ctx context.Context, siteID uuid.UUID) error {
	// Get all nodes for this site
	nodes, err := h.graphRepo.ListNodesBySite(ctx, siteID)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	// Sort nodes by type priority (listing first, detail last)
	h.sortNodesByPriority(nodes)

	// Process each node
	for _, node := range nodes {
		// Rate limiting
		time.Sleep(time.Duration(h.config.RateLimitMs) * time.Millisecond)

		// Execute script and extract items
		items, execErr := h.executeNode(ctx, node)

		if execErr != nil {
			// Continue with next node on error
			continue
		}

		// Send items to pipeline
		if len(items) > 0 {
			_, _, pipeErr := h.pipeline.Process(ctx, items)
			if pipeErr != nil {
				// Log error but continue
				continue
			}
		}
	}

	return nil
}
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
func (h *Harvester) fetchHTML(ctx context.Context, urlStr string) (string, error) {
	// TODO: Implement actual HTTP fetching with timeout
	// For now, return empty to allow compilation
	return "", fmt.Errorf("not implemented")
}
