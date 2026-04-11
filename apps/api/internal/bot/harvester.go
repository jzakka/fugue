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
	// Execute harvest using BFS
	return h.harvestBFS(ctx, siteID)
}

// harvestBFS traverses the graph using BFS and processes nodes level by level
func (h *Harvester) harvestBFS(ctx context.Context, siteID uuid.UUID) error {
	// Get the site to find root URL
	site, err := h.siteRepo.Get(ctx, siteID)
	if err != nil {
		return fmt.Errorf("get site: %w", err)
	}

	// Find root node
	rootNode, err := h.findRootNode(ctx, site)
	if err != nil {
		return fmt.Errorf("find root node: %w", err)
	}

	// Fetch all nodes once and create a lookup map for efficiency
	allNodes, err := h.graphRepo.ListNodesBySite(ctx, siteID)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	nodeMap := make(map[uuid.UUID]db.BotGraphNode)
	for _, node := range allNodes {
		nodeMap[node.ID] = node
	}

	// Initialize visited set for cycle detection
	visited := make(map[uuid.UUID]bool)

	// Initialize BFS queue with root node
	queue := NewBFSQueue()
	queue.AddLevel([]db.BotGraphNode{rootNode})
	visited[rootNode.ID] = true

	// BFS traversal: process nodes level by level
	for !queue.IsEmpty() {
		// Get all nodes at current level
		levelNodes := queue.PopLevel()

		// Sort nodes by type priority within the level
		h.sortNodesByPriority(levelNodes)

		// Collect next level nodes
		var nextLevel []db.BotGraphNode

		// Process each node in current level
		for _, node := range levelNodes {
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

			// Get edges to find child nodes
			edges, edgesErr := h.graphRepo.GetEdgesByNode(ctx, node.ID)
			if edgesErr != nil {
				// Log error but continue
				continue
			}

			// Extract child node IDs and add unvisited children to next level
			for _, edge := range edges {
				childID := edge.ToNodeID

				// Skip if already visited (cycle detection)
				if visited[childID] {
					continue
				}
				visited[childID] = true

				// Look up child node from map
				if childNode, exists := nodeMap[childID]; exists {
					nextLevel = append(nextLevel, childNode)
				}
			}
		}

		// Add next level to queue if not empty
		if len(nextLevel) > 0 {
			queue.AddLevel(nextLevel)
		}
	}

	return nil
}

// findRootNode locates the root URL node for the site
func (h *Harvester) findRootNode(ctx context.Context, site db.BotSite) (db.BotGraphNode, error) {
	node, err := h.graphRepo.GetNodeByURL(ctx, db.GetNodeByURLParams{
		SiteID: site.ID,
		Url:    site.RootUrl,
	})
	if err != nil {
		return db.BotGraphNode{}, fmt.Errorf("root node not found for URL %s: %w (suggest running Pioneer first)", site.RootUrl, err)
	}
	return node, nil
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
