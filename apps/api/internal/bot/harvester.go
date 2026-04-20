package bot

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// HarvestStats are the per-node aggregated counters returned by Run.
//
// PinsCreated/Deduped/Skipped/Failed are the four PRIMARY categories: each
// processed node increments exactly one of them. AdapterFallback is an
// AUXILIARY counter that tracks how many nodes had to fall back to the
// generic extractor because their PerSiteAdapter returned an error — it is
// independent of the primary categories and may increment at the same time.
type HarvestStats struct {
	NodesProcessed  int
	PinsCreated     int
	Deduped         int
	Skipped         int
	Failed          int
	AdapterFallback int
}

// HarvesterConfig is process-level harvester tuning.
type HarvesterConfig struct {
	RateLimitMs      int
	RetryFailedNodes bool
	MaxRetries       int
}

// DocumentPipeline persists a PinDocument as a Pin and reports whether a
// new row was inserted (created=true) or an existing row was updated
// (created=false). Returning created=false maps to the Deduped stat.
type DocumentPipeline interface {
	ProcessDocument(ctx context.Context, node db.BotGraphNode, doc PinDocument) (created bool, pinID uuid.UUID, err error)
	MarkSkipped(ctx context.Context, node db.BotGraphNode) error
}

// FrontierMarker marks a frontier row's harvested_at without creating a Pin
// (used when classifier returns pinnable=false). Implemented as part of the
// scheduler-consumer change; the Harvester only needs the small interface
// here.

// Harvester harvests pages by walking the graph and converting each node's
// HTML into a PinDocument via either a PerSiteAdapter or the generic
// extractor.
type Harvester struct {
	siteRepo    SiteRepository
	graphRepo   GraphRepository
	scriptRepo  ScriptRepository
	executor    ScriptExecutor
	pipeline    DocumentPipeline
	registry    AdapterRegistry
	extractor   *GenericExtractor
	classifier  *Classifier
	config      HarvesterConfig
}

// NewHarvester wires the harvester. registry/extractor/classifier may be nil
// in legacy callers; defaults are constructed lazily.
func NewHarvester(
	siteRepo SiteRepository,
	graphRepo GraphRepository,
	scriptRepo ScriptRepository,
	executor ScriptExecutor,
	pipeline DocumentPipeline,
	registry AdapterRegistry,
	extractor *GenericExtractor,
	classifier *Classifier,
	config HarvesterConfig,
) *Harvester {
	if registry == nil {
		registry = NewInMemoryAdapterRegistry()
	}
	if extractor == nil {
		extractor = NewGenericExtractor()
	}
	if classifier == nil {
		classifier = NewClassifier()
	}
	return &Harvester{
		siteRepo:   siteRepo,
		graphRepo:  graphRepo,
		scriptRepo: scriptRepo,
		executor:   executor,
		pipeline:   pipeline,
		registry:   registry,
		extractor:  extractor,
		classifier: classifier,
		config:     config,
	}
}

// Run executes the harvester for the given site.
func (h *Harvester) Run(ctx context.Context, siteID uuid.UUID) (HarvestStats, error) {
	return h.harvestBFS(ctx, siteID)
}

func (h *Harvester) harvestBFS(ctx context.Context, siteID uuid.UUID) (HarvestStats, error) {
	var stats HarvestStats

	site, err := h.siteRepo.Get(ctx, siteID)
	if err != nil {
		return stats, fmt.Errorf("get site: %w", err)
	}

	rootNode, err := h.findRootNode(ctx, site)
	if err != nil {
		return stats, fmt.Errorf("find root node: %w", err)
	}

	allNodes, err := h.graphRepo.ListNodesBySite(ctx, siteID)
	if err != nil {
		return stats, fmt.Errorf("list nodes: %w", err)
	}

	nodeMap := make(map[uuid.UUID]db.BotGraphNode, len(allNodes))
	for _, node := range allNodes {
		nodeMap[node.ID] = node
	}

	visited := make(map[uuid.UUID]bool)
	queue := NewBFSQueue()
	queue.AddLevel([]db.BotGraphNode{rootNode})
	visited[rootNode.ID] = true

	for !queue.IsEmpty() {
		levelNodes := queue.PopLevel()
		h.sortNodesByPriority(levelNodes)

		var nextLevel []db.BotGraphNode
		for _, node := range levelNodes {
			time.Sleep(time.Duration(h.config.RateLimitMs) * time.Millisecond)
			stats.NodesProcessed++

			h.processNode(ctx, node, &stats)

			edges, edgesErr := h.graphRepo.GetEdgesByNode(ctx, node.ID)
			if edgesErr != nil {
				log.Printf("harvester: get edges for node %s: %v", node.ID, edgesErr)
				continue
			}
			for _, edge := range edges {
				childID := edge.ToNodeID
				if visited[childID] {
					continue
				}
				visited[childID] = true
				if childNode, exists := nodeMap[childID]; exists {
					nextLevel = append(nextLevel, childNode)
				}
			}
		}
		if len(nextLevel) > 0 {
			queue.AddLevel(nextLevel)
		}
	}

	return stats, nil
}

// processNode is the per-node fetch → extract → classify → persist pipeline.
// It mutates stats in place so the caller sees exactly one primary-category
// increment plus, optionally, an AdapterFallback increment.
func (h *Harvester) processNode(ctx context.Context, node db.BotGraphNode, stats *HarvestStats) {
	fetchURL := node.Url
	if node.SampleUrl.Valid && node.SampleUrl.String != "" {
		fetchURL = node.SampleUrl.String
	}

	htmlStr, _, err := fetchHTMLShared(ctx, fetchURL)
	if err != nil {
		log.Printf("harvester: fetch %s: %v", fetchURL, err)
		stats.Failed++
		return
	}
	htmlBytes := []byte(htmlStr)

	doc, fellBack, err := h.extractDocument(ctx, htmlBytes, fetchURL, node)
	if fellBack {
		stats.AdapterFallback++
	}
	if err != nil {
		log.Printf("harvester: extract %s: %v", fetchURL, err)
		stats.Failed++
		return
	}

	// Extractors (generic + adapters) are responsible for setting
	// og_data.source to the fetch URL; we do not override here.

	linkStats := ComputeLinkStats(htmlBytes)
	pinnable, reason := h.classifier.Classify(doc, linkStats)
	doc.OGData.Classifier = &ClassifierVerdict{Pinnable: pinnable, Reason: reason}

	if !pinnable {
		if err := h.pipeline.MarkSkipped(ctx, node); err != nil {
			log.Printf("harvester: mark skipped %s: %v", fetchURL, err)
			stats.Failed++
			return
		}
		stats.Skipped++
		return
	}

	// Rune-safe truncation of body_text to fit pins.description (500 runes).
	doc.BodyText = truncateRunes(doc.BodyText, 500)
	// og_data MUST NOT carry body_text — it lives only in pins.description.

	created, _, err := h.pipeline.ProcessDocument(ctx, node, doc)
	if err != nil {
		log.Printf("harvester: persist %s: %v", fetchURL, err)
		stats.Failed++
		return
	}
	if created {
		stats.PinsCreated++
	} else {
		stats.Deduped++
	}
}

// extractDocument resolves the right extractor for the node's domain, runs
// it, and falls back to the generic extractor on adapter error. Returns
// (doc, fellBack, err) where fellBack=true iff an adapter was attempted and
// failed.
func (h *Harvester) extractDocument(ctx context.Context, htmlBytes []byte, fetchURL string, node db.BotGraphNode) (PinDocument, bool, error) {
	domain := hostnameOf(fetchURL)
	adapter, hasAdapter := h.registry.Resolve(domain)

	if hasAdapter {
		nodeType := ""
		if node.NodeType.Valid {
			nodeType = node.NodeType.String
		}
		adapterCtx := WithNodeType(ctx, nodeType)
		doc, err := adapter.Extract(adapterCtx, htmlBytes, fetchURL)
		if err == nil {
			FillExtractorIdentity(&doc, adapter.Name())
			return doc, false, nil
		}
		log.Printf("harvester: adapter %s failed (%v); falling back to generic", adapter.Name(), err)
		// fall through to generic
		genericDoc, gErr := h.extractor.Extract(htmlBytes, fetchURL)
		if gErr != nil {
			return genericDoc, true, gErr
		}
		FillExtractorIdentity(&genericDoc, "generic")
		return genericDoc, true, nil
	}

	doc, err := h.extractor.Extract(htmlBytes, fetchURL)
	if err != nil {
		return doc, false, err
	}
	FillExtractorIdentity(&doc, "generic")
	return doc, false, nil
}

func (h *Harvester) findRootNode(ctx context.Context, site db.BotSite) (db.BotGraphNode, error) {
	rootHash := hashURL(site.RootUrl)
	node, err := h.graphRepo.GetNodeByHash(ctx, db.GetNodeByHashParams{
		SiteID:  site.ID,
		UrlHash: rootHash,
	})
	if err != nil {
		return db.BotGraphNode{}, fmt.Errorf("root node not found for URL %s (hash: %s): %w (suggest running Pioneer first)", site.RootUrl, rootHash, err)
	}
	return node, nil
}

func (h *Harvester) sortNodesByPriority(nodes []db.BotGraphNode) {
	sort.Slice(nodes, func(i, j int) bool {
		priI := 0
		priJ := 0
		if nodes[i].NodeType.Valid {
			priI = NodeTypePriority(NodeType(nodes[i].NodeType.String))
		}
		if nodes[j].NodeType.Valid {
			priJ = NodeTypePriority(NodeType(nodes[j].NodeType.String))
		}
		return priI > priJ
	})
}

// hostnameOf returns the lowercase hostname of rawURL, or "" if invalid.
func hostnameOf(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// truncateRunes returns the first n runes of s. Cuts on rune boundaries so
// multi-byte characters are never split.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
