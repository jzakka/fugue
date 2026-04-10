package bot

import (
	"context"
	"crypto/md5"
	"database/sql"
	"errors"
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
	MaxNodesPerSite  int
	RateLimitMs      int
	SuccessThreshold float64 // 0.7 = 70%
}

// Pioneer explores sites and generates parsing scripts
type Pioneer struct {
	siteRepo   SiteRepository
	graphRepo  GraphRepository
	scriptRepo ScriptRepository
	aiClient   AIClient
	executor   ScriptExecutor
	config     PioneerConfig
}

// NewPioneer creates a new Pioneer service
func NewPioneer(
	siteRepo SiteRepository,
	graphRepo GraphRepository,
	scriptRepo ScriptRepository,
	aiClient AIClient,
	executor ScriptExecutor,
	config PioneerConfig,
) *Pioneer {
	return &Pioneer{
		siteRepo:   siteRepo,
		graphRepo:  graphRepo,
		scriptRepo: scriptRepo,
		aiClient:   aiClient,
		executor:   executor,
		config:     config,
	}
}

// Run executes a full pioneer crawl for a site
func (p *Pioneer) Run(ctx context.Context, siteID uuid.UUID) error {
	// Get site details
	site, err := p.siteRepo.Get(ctx, siteID)
	if err != nil {
		return fmt.Errorf("get site: %w", err)
	}

	// Execute crawl
	return p.crawl(ctx, site)
}

// crawl performs the BFS crawl and script generation
// crawl performs the BFS crawl and script generation
func (p *Pioneer) crawl(ctx context.Context, site db.BotSite) error {
	// Parse root domain
	rootDomain, err := extractDomain(site.RootUrl)
	if err != nil {
		return fmt.Errorf("parse root domain: %w", err)
	}

	// Initialize BFS queue with root URL
	queue := NewPriorityQueue()
	visited := make(map[string]bool)

	// Add root node
	rootHash := hashURL(site.RootUrl)
	queue.Push(&QueueItem{
		URL:      site.RootUrl,
		URLHash:  rootHash,
		Priority: 100, // High priority for root
	})
	visited[rootHash] = true

	nodesProcessed := 0

	// BFS traversal
	for !queue.IsEmpty() && nodesProcessed < p.config.MaxNodesPerSite {
		item := queue.Pop()

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
		_, err := p.graphRepo.GetNodeByHash(ctx, db.GetNodeByHashParams{
			SiteID:  site.ID,
			UrlHash: item.URLHash,
		})

		// Distinguish "not found" from DB errors
		nodeExists := false
		if err == nil {
			nodeExists = true
		} else if !errors.Is(err, sql.ErrNoRows) {
			// Real DB error - fail loudly
			return fmt.Errorf("failed to check node existence for %s: %w", item.URL, err)
		}

		if !nodeExists {
			// Create new node
			_, err = p.graphRepo.CreateNode(ctx, db.CreateNodeParams{
				SiteID:   site.ID,
				Url:      item.URL,
				UrlHash:  item.URLHash,
				NodeType: sql.NullString{String: string(nodeType), Valid: true},
				ScriptID: uuid.NullUUID{Valid: false},
			})
			if err != nil {
				// Check if it's a unique constraint violation (concurrent insert)
				if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
					continue // Another worker created it, skip
				}
				// Real error - fail
				return fmt.Errorf("failed to create node for %s: %w", item.URL, err)
			}
		}

		nodesProcessed++

		// Handle script for this node type
		_, scriptErr := p.handleScript(ctx, site.ID, nodeType, html, item.URL)
		if scriptErr != nil {
			// Log error but continue
			continue
		}

		// Parse links and add to queue
		links := parseLinks(html, item.URL)
		for _, link := range links {
			// Domain validation
			if !isSameDomain(link, rootDomain) {
				continue
			}

			// File extension filtering
			if hasExcludedExtension(link) {
				continue
			}

			linkHash := hashURL(link)
			if visited[linkHash] {
				continue
			}
			visited[linkHash] = true

			// Classify and calculate priority
			linkType := classifyURL(link)
			priority := NodeTypePriority(linkType)

			queue.Push(&QueueItem{
				URL:      link,
				URLHash:  linkHash,
				Priority: priority,
			})
		}
	}

	return nil
}
func (p *Pioneer) handleScript(ctx context.Context, siteID uuid.UUID, nodeType NodeType, html, url string) (*db.BotScript, error) {
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
			return &script, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		// Real DB error - don't fall through to regeneration
		return nil, fmt.Errorf("failed to check script existence: %w", err)
	}

	// Generate new script (only if not found or validation failed)
	resp, err := p.aiClient.GenerateScript(ctx, ScriptRequest{
		URL:      url,
		HTML:     html,
		NodeType: nodeType,
	})
	if err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}

	// Save script
	script, err = p.scriptRepo.Create(ctx, db.CreateScriptParams{
		SiteID:     siteID,
		NodeType:   string(nodeType),
		ScriptLang: sql.NullString{String: "js", Valid: true},
		ScriptCode: resp.ScriptCode,
		AiModel:    sql.NullString{String: resp.Model, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("save script: %w", err)
	}

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
