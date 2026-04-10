package bot

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Pioneer config
type PioneerConfig struct {
	MaxDepth         int
	MaxNodesPerSite  int
	RateLimitMs      int
	SuccessThreshold float64 // 0.7 = 70%
}

// Pioneer explores sites and generates parsing scripts
type Pioneer struct {
	siteRepo   SiteRepository
	graphRepo  GraphRepository
	scriptRepo ScriptRepository
	runRepo    RunRepository
	aiClient   AIClient
	executor   ScriptExecutor
	config     PioneerConfig
}

// NewPioneer creates a new Pioneer service
func NewPioneer(
	siteRepo SiteRepository,
	graphRepo GraphRepository,
	scriptRepo ScriptRepository,
	runRepo RunRepository,
	aiClient AIClient,
	executor ScriptExecutor,
	config PioneerConfig,
) *Pioneer {
	return &Pioneer{
		siteRepo:   siteRepo,
		graphRepo:  graphRepo,
		scriptRepo: scriptRepo,
		runRepo:    runRepo,
		aiClient:   aiClient,
		executor:   executor,
		config:     config,
	}
}

// Run executes a full pioneer crawl for a site
func (p *Pioneer) Run(ctx context.Context, siteID uuid.UUID) error {
	// Create run record
	run, err := p.runRepo.CreatePioneerRun(ctx, db.CreatePioneerRunParams{
		SiteID: siteID,
		Status: string(RunStatusRunning),
	})
	if err != nil {
		return fmt.Errorf("create pioneer run: %w", err)
	}

	// Update site status to in_progress
	now := time.Now()
	err = p.siteRepo.UpdatePioneerStatus(ctx, db.UpdatePioneerStatusParams{
		ID:                 siteID,
		PioneerStatus:      sql.NullString{String: string(SiteStatusInProgress), Valid: true},
		PioneerStartedAt:   sql.NullTime{Time: now, Valid: true},
		PioneerCompletedAt: sql.NullTime{Valid: false},
	})
	if err != nil {
		return fmt.Errorf("update site status: %w", err)
	}

	// Get site details
	site, err := p.siteRepo.Get(ctx, siteID)
	if err != nil {
		return fmt.Errorf("get site: %w", err)
	}

	// Execute crawl
	stats, crawlErr := p.crawl(ctx, site)

	// Update run statistics
	completedAt := time.Now()
	status := string(RunStatusCompleted)
	var errorMsg *string
	if crawlErr != nil {
		status = string(RunStatusFailed)
		msg := crawlErr.Error()
		errorMsg = &msg
	}

	err = p.runRepo.UpdatePioneerRunStats(ctx, db.UpdatePioneerRunStatsParams{
		ID:               run.ID,
		CompletedAt:      sql.NullTime{Time: completedAt, Valid: true},
		Status:           status,
		NodesDiscovered:  sql.NullInt32{Int32: int32(stats.NodesDiscovered), Valid: true},
		NodesUpdated:     sql.NullInt32{Int32: int32(stats.NodesUpdated), Valid: true},
		ScriptsGenerated: sql.NullInt32{Int32: int32(stats.ScriptsGenerated), Valid: true},
		ScriptsReused:    sql.NullInt32{Int32: int32(stats.ScriptsReused), Valid: true},
		AiApiCalls:       sql.NullInt32{Int32: int32(stats.AIAPICalls), Valid: true},
		AiCostUsd:        sql.NullString{String: fmt.Sprintf("%.6f", stats.AICostUSD), Valid: true},
		ErrorMessage:     sql.NullString{String: stringOrEmpty(errorMsg), Valid: errorMsg != nil},
	})
	if err != nil {
		return fmt.Errorf("update run stats: %w", err)
	}

	// Update site status
	siteStatus := string(SiteStatusCompleted)
	if crawlErr != nil {
		siteStatus = string(SiteStatusFailed)
	}
	err = p.siteRepo.UpdatePioneerStatus(ctx, db.UpdatePioneerStatusParams{
		ID:                 siteID,
		PioneerStatus:      sql.NullString{String: siteStatus, Valid: true},
		PioneerStartedAt:   sql.NullTime{Time: now, Valid: true},
		PioneerCompletedAt: sql.NullTime{Time: completedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update final site status: %w", err)
	}

	return crawlErr
}

type crawlStats struct {
	NodesDiscovered  int
	NodesUpdated     int
	ScriptsGenerated int
	ScriptsReused    int
	AIAPICalls       int
	AICostUSD        float64
}

// crawl performs the BFS crawl and script generation
func (p *Pioneer) crawl(ctx context.Context, site db.BotSite) (*crawlStats, error) {
	stats := &crawlStats{}

	// Parse root domain
	rootDomain, err := extractDomain(site.RootUrl)
	if err != nil {
		return stats, fmt.Errorf("parse root domain: %w", err)
	}

	// Initialize BFS queue with root URL
	queue := NewPriorityQueue()
	visited := make(map[string]bool)

	// Add root node
	rootHash := hashURL(site.RootUrl)
	queue.Push(&QueueItem{
		URL:       site.RootUrl,
		URLHash:   rootHash,
		Depth:     0,
		ParentURL: nil,
		Priority:  100, // High priority for root
	})
	visited[rootHash] = true

	// BFS traversal
	for !queue.IsEmpty() && stats.NodesDiscovered < p.config.MaxNodesPerSite {
		item := queue.Pop()

		// Check depth limit
		if item.Depth > p.config.MaxDepth {
			continue
		}

		// Rate limiting
		time.Sleep(time.Duration(p.config.RateLimitMs) * time.Millisecond)

		// Fetch HTML (with timeout)
		html, fetchErr := p.fetchHTML(ctx, item.URL)
		if fetchErr != nil {
			// Log error but continue
			continue
		}

		// Classify node type
		nodeType := classifyURL(item.URL)
		if nodeType == NodeTypeSkip {
			continue
		}

		// Create or get existing node
		node, err := p.graphRepo.GetNodeByHash(ctx, db.GetNodeByHashParams{
			SiteID:  site.ID,
			UrlHash: item.URLHash,
		})

		nodeExists := err == nil
		if !nodeExists {
			// Create new node
			node, err = p.graphRepo.CreateNode(ctx, db.CreateNodeParams{
				SiteID:    site.ID,
				Url:       item.URL,
				UrlHash:   item.URLHash,
				Depth:     int32(item.Depth),
				NodeType:  sql.NullString{String: string(nodeType), Valid: true},
				ParentUrl: sql.NullString{String: stringOrEmpty(item.ParentURL), Valid: item.ParentURL != nil},
				ScriptID:  uuid.NullUUID{Valid: false},
			})
			if err != nil {
				continue // Skip if can't create node
			}
			stats.NodesDiscovered++
		} else {
			stats.NodesUpdated++
		}

		// Handle script for this node
		script, scriptErr := p.handleScript(ctx, site.ID, nodeType, html, item.URL, stats)
		if scriptErr == nil && script != nil {
			// Link script to node (best effort)
			_ = p.graphRepo.UpdateNodeScript(ctx, db.UpdateNodeScriptParams{
				ID:       node.ID,
				ScriptID: uuid.NullUUID{UUID: script.ID, Valid: true},
			})
		}

		// Extract links from HTML and add to queue
		links := extractLinks(html, item.URL)
		for _, link := range links {
			// Validate domain
			if !isSameDomain(link, rootDomain) {
				continue
			}

			// Check file extensions
			if hasExcludedExtension(link) {
				continue
			}

			linkHash := hashURL(link)
			if visited[linkHash] {
				continue
			}

			linkNodeType := classifyURL(link)
			priority := NodeTypePriority(linkNodeType)

			queue.Push(&QueueItem{
				URL:       link,
				URLHash:   linkHash,
				Depth:     item.Depth + 1,
				ParentURL: &item.URL,
				Priority:  priority,
			})
			visited[linkHash] = true

			// Note: Edges will be created when the target node is actually visited
			// For now, we just track the relationship via ParentURL in the node
		}
	}

	return stats, nil
}

// handleScript manages script generation and validation
func (p *Pioneer) handleScript(ctx context.Context, siteID uuid.UUID, nodeType NodeType, html, url string, stats *crawlStats) (*db.BotScript, error) {
	// Try to get existing script
	script, err := p.scriptRepo.GetBySiteType(ctx, db.GetScriptBySiteTypeParams{
		SiteID:   siteID,
		NodeType: string(nodeType),
	})

	if err == nil {
		// Script exists - validate it
		valid, _ := p.validateScript(ctx, script.ScriptCode, html, url)
		if valid {
			// Reuse existing script
			stats.ScriptsReused++
			return &script, nil
		}
	}

	// Generate new script
	resp, err := p.aiClient.GenerateScript(ctx, ScriptRequest{
		URL:      url,
		HTML:     html,
		NodeType: nodeType,
	})
	if err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}

	stats.AIAPICalls++
	stats.AICostUSD += resp.CostUSD

	// Save script
	costStr := fmt.Sprintf("%.6f", resp.CostUSD)
	script, err = p.scriptRepo.Create(ctx, db.CreateScriptParams{
		SiteID:            siteID,
		NodeType:          string(nodeType),
		ScriptLang:        sql.NullString{String: "js", Valid: true},
		ScriptCode:        resp.ScriptCode,
		AiModel:           sql.NullString{String: resp.Model, Valid: true},
		GenerationCostUsd: sql.NullString{String: costStr, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("save script: %w", err)
	}

	stats.ScriptsGenerated++
	return &script, nil
}

// validateScript checks if a script meets the 70% threshold
func (p *Pioneer) validateScript(ctx context.Context, scriptCode, html, url string) (bool, error) {
	// Estimate expected items from HTML
	expectedCount := estimateItemCount(html)
	if expectedCount == 0 {
		return false, nil
	}

	// Execute script
	items, err := p.executor.Execute(ctx, scriptCode, html, url)
	if err != nil {
		return false, err
	}

	// Calculate success rate
	successRate := float64(len(items)) / float64(expectedCount)
	return successRate >= p.config.SuccessThreshold, nil
}

// fetchHTML fetches HTML content with timeout
func (p *Pioneer) fetchHTML(ctx context.Context, urlStr string) (string, error) {
	// TODO: Implement actual HTTP fetching with timeout
	// For now, return empty to allow compilation
	return "", fmt.Errorf("not implemented")
}

// URL Classification (Task 6.2)
func classifyURL(urlStr string) NodeType {
	lower := strings.ToLower(urlStr)

	// Skip patterns
	skipPatterns := []string{"ad", "popup", "login", "signup", "cart", "checkout"}
	for _, pattern := range skipPatterns {
		if strings.Contains(lower, pattern) {
			return NodeTypeSkip
		}
	}

	// Listing patterns (priority 100)
	listingPatterns := []string{"trending", "popular", "hot", "featured", "recent", "explore"}
	for _, pattern := range listingPatterns {
		if strings.Contains(lower, pattern) {
			return NodeTypeListing
		}
	}

	// Gallery patterns (priority 80)
	galleryPatterns := []string{"gallery", "collection", "album", "showcase"}
	for _, pattern := range galleryPatterns {
		if strings.Contains(lower, pattern) {
			return NodeTypeGallery
		}
	}

	// Category patterns (priority 60)
	categoryPatterns := []string{"category", "tag", "genre", "style"}
	for _, pattern := range categoryPatterns {
		if strings.Contains(lower, pattern) {
			return NodeTypeCategory
		}
	}

	// Detail page: has numeric ID pattern (check AFTER keyword patterns to avoid false positives)
	// "shots/12345" should be detail, but "shots/recent" is listing
	if regexp.MustCompile(`/\d{4,}(?:/|$)`).MatchString(urlStr) {
		return NodeTypeDetail
	}

	return NodeTypeListing // Default
}

// Domain validation (Task 6.3)
func extractDomain(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// Normalize: remove www. prefix
	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	return host, nil
}

func isSameDomain(urlStr, rootDomain string) bool {
	domain, err := extractDomain(urlStr)
	if err != nil {
		return false
	}

	// Normalize rootDomain too
	normalizedRoot := strings.TrimPrefix(strings.ToLower(rootDomain), "www.")

	return domain == normalizedRoot
}

// File extension filtering (Task 6.4)
func hasExcludedExtension(urlStr string) bool {
	lower := strings.ToLower(urlStr)

	excluded := []string{
		// Images
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico",
		// Media
		".mp3", ".mp4", ".wav", ".webm", ".avi", ".mov",
		// Documents
		".pdf", ".zip", ".tar", ".gz", ".exe", ".dmg",
		// Static assets
		".css", ".js", ".json", ".xml", ".woff", ".woff2", ".ttf",
	}

	for _, ext := range excluded {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// Helper: hash URL for deduplication
func hashURL(urlStr string) string {
	h := md5.Sum([]byte(urlStr))
	return fmt.Sprintf("%x", h)
}

// Helper: estimate item count from HTML (Task 6.6)
func estimateItemCount(html string) int {
	// Count common item indicators
	imgCount := strings.Count(html, "<img")
	cardCount := strings.Count(html, `class="card"`) + strings.Count(html, `class="item"`)
	articleCount := strings.Count(html, "<article")

	// Return max of these heuristics
	max := imgCount
	if cardCount > max {
		max = cardCount
	}
	if articleCount > max {
		max = articleCount
	}
	return max
}

// Helper: extract links from HTML
func extractLinks(html, baseURL string) []string {
	// Simplified link extraction
	// TODO: Implement proper HTML parsing
	var links []string

	// Basic regex for href attributes
	re := regexp.MustCompile(`href="([^"]+)"`)
	matches := re.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]
			// Convert relative to absolute
			absoluteURL := makeAbsolute(link, baseURL)
			links = append(links, absoluteURL)
		}
	}

	return links
}

func makeAbsolute(link, baseURL string) string {
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return link
	}

	rel, err := url.Parse(link)
	if err != nil {
		return link
	}

	return base.ResolveReference(rel).String()
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
