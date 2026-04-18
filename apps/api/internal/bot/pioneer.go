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

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Package-level compiled regex patterns for URL classification
var numericIDPattern = regexp.MustCompile(`^\d+$`)
var pathNumericIDPattern = regexp.MustCompile(`/\d{4,}(?:/|$)`)

// Pioneer config
type PioneerConfig struct {
	MaxNodesPerSite  int
	RateLimitMs      int
	SuccessThreshold float64 // 0.7 = 70%

	// SnapshotEnabled gates the raw-HTML snapshot upload step
	// (pioneer-snapshot-storage spec). When false, no PUTs occur and the
	// crawl loop behaves identically to the pre-feature implementation.
	SnapshotEnabled bool
}

// Pioneer explores sites and generates parsing scripts
type Pioneer struct {
	siteRepo   SiteRepository
	graphRepo  GraphRepository
	scriptRepo ScriptRepository
	aiClient   AIClient
	executor   ScriptExecutor
	config     PioneerConfig

	// snapshotStore is best-effort raw-HTML storage for downstream
	// reuse by Harvester. Nil means snapshots are disabled regardless
	// of config.SnapshotEnabled (defensive double-gate).
	snapshotStore   snapshot.SnapshotStore
	snapshotMetrics *snapshot.Metrics
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

// WithSnapshotStore injects a raw-HTML snapshot store. The Pioneer will
// only call it when config.SnapshotEnabled is true (per spec
// "Scenario: 비활성화 시 업로드 스킵"). metrics may be nil; the
// snapshot package's *Metrics methods are nil-safe.
func (p *Pioneer) WithSnapshotStore(store snapshot.SnapshotStore, metrics *snapshot.Metrics) *Pioneer {
	p.snapshotStore = store
	p.snapshotMetrics = metrics
	return p
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
func (p *Pioneer) crawl(ctx context.Context, site db.BotSite) error {
	fmt.Printf("🚀 Starting crawl for %s (root: %s)\n", site.Domain, site.RootUrl)

	// Parse root domain
	rootDomain, err := extractDomain(site.RootUrl)
	if err != nil {
		return fmt.Errorf("parse root domain: %w", err)
	}
	fmt.Printf("📍 Root domain: %s\n", rootDomain)

	// Initialize BFS queue with root URL
	queue := NewPriorityQueue()
	visited := make(map[string]uuid.UUID) // hash → node ID

	// Initialize FilterChain
	dedupFilter := NewCanonicalDedupFilter(visited)
	filterChain := NewFilterChain(
		&DomainFilter{RootDomain: rootDomain},
		&ExtensionFilter{},
		&PathPatternFilter{},
		dedupFilter,
	)

	// Load existing edges for stale edge detection
	type edgeKey struct{ from, to uuid.UUID }
	existingEdges := make(map[edgeKey]uuid.UUID) // edgeKey → edge ID
	confirmedEdges := make(map[edgeKey]bool)
	siteEdges, edgeListErr := p.graphRepo.ListEdgesBySiteNodes(ctx, site.ID)
	if edgeListErr != nil {
		fmt.Printf("⚠️  Could not load existing edges: %v\n", edgeListErr)
	} else {
		for _, e := range siteEdges {
			existingEdges[edgeKey{e.FromNodeID, e.ToNodeID}] = e.ID
		}
	}

	// Add root node
	rootHash := hashURL(site.RootUrl)
	queue.Push(&QueueItem{
		URL:      site.RootUrl,
		URLHash:  rootHash,
		Priority: 100, // High priority for root
	})
	visited[rootHash] = uuid.Nil // ID will be set when processed
	fmt.Printf("🌱 Added root node to queue\n")

	nodesProcessed := 0

	// BFS traversal
	fmt.Printf("🔄 Starting BFS traversal (max nodes: %d)...\n", p.config.MaxNodesPerSite)
	for !queue.IsEmpty() && nodesProcessed < p.config.MaxNodesPerSite {
		item := queue.Pop()
		fmt.Printf("\n📥 Processing: %s\n", item.URL)

		// Rate limiting
		time.Sleep(time.Duration(p.config.RateLimitMs) * time.Millisecond)

		// Fetch HTML (with timeout)
		html, finalURL, fetchErr := p.fetchHTML(ctx, item.URL)
		if fetchErr != nil {
			// Log error but continue
			fmt.Printf("Error fetching %s: %v\n", item.URL, fetchErr)
			continue
		}
		fmt.Printf("✅ Fetched %d bytes (final: %s)\n", len(html), finalURL)

		// Snapshot the raw response for downstream Harvester reuse.
		//
		// Per the pioneer-snapshot-storage spec this is best-effort and
		// fail-open: any error is logged + counted but never breaks the
		// crawl. fetchHTMLShared has already enforced "2xx + body length > 0"
		// (see helpers.go:46-58), so reaching this point means the spec's
		// "fetch success" precondition holds.
		//
		// We pass templatePath(finalURL) — the normalized URL — to honor
		// design Decision 1a's contract that the key is sha256 of the
		// *normalized* URL. Pioneer and Harvester must share the same
		// normalization so the harvester change can reconstruct the key
		// from the URL alone. templatePath is the same normalizer used
		// by hashURL/CreateNode above, so a single fetched page
		// produces one stable snapshot key regardless of redirect-time
		// query-string variation.
		//
		// Done here, before any state-changing operation, so the upload
		// outcome cannot influence subsequent scheduler/graph state — the
		// inverse of the deprecated `fuguebot_pseudo.go` pattern that
		// gated SetStatus on SaveRawContent success.
		p.saveSnapshot(ctx, templatePath(finalURL), []byte(html))

		// Classify node type using the final URL after redirects
		nodeType := classifyURL(finalURL)
		fmt.Printf("🏷️  Node type: %s\n", nodeType)

		// Recompute hash from finalURL to handle redirects correctly
		canonical := templatePath(finalURL)
		finalHash := hashURL(finalURL)
		var currentNodeID uuid.UUID
		existingNode, err := p.graphRepo.GetNodeByHash(ctx, db.GetNodeByHashParams{
			SiteID:  site.ID,
			UrlHash: finalHash,
		})

		if err == nil {
			currentNodeID = existingNode.ID
		} else if !errors.Is(err, sql.ErrNoRows) {
			// Real DB error - fail loudly
			return fmt.Errorf("failed to check node existence for %s: %w", finalURL, err)
		} else {
			// Create new node
			newNode, createErr := p.graphRepo.CreateNode(ctx, db.CreateNodeParams{
				SiteID:    site.ID,
				Url:       canonical,
				UrlHash:   finalHash,
				NodeType:  sql.NullString{String: string(nodeType), Valid: true},
				ScriptID:  uuid.NullUUID{Valid: false},
				SampleUrl: sql.NullString{String: finalURL, Valid: true},
			})
			if createErr != nil {
				// Check if it's a unique constraint violation (concurrent insert)
				if strings.Contains(createErr.Error(), "duplicate key") || strings.Contains(createErr.Error(), "unique constraint") {
					continue // Another worker created it, skip
				}
				// Real error - fail
				return fmt.Errorf("failed to create node for %s: %w", finalURL, createErr)
			}
			currentNodeID = newNode.ID
			nodesProcessed++
		}

		// Mark both original and final URL hashes as visited
		visited[item.URLHash] = currentNodeID
		visited[finalHash] = currentNodeID

		fmt.Printf("📊 Nodes processed: %d/%d (new)\n", nodesProcessed, p.config.MaxNodesPerSite)

		// Handle script for this node type
		_, scriptErr := p.handleScript(ctx, site.ID, nodeType, html, finalURL)
		if scriptErr != nil {
			// Log error but continue to parse links
			fmt.Printf("⚠️  Script error (will continue): %v\n", scriptErr)
			// Don't return or continue - keep going to parse links
		}

		// Extract links using DOM-based parser
		crawlerLinks, extractErr := crawler.ExtractLinksWithSelectors(strings.NewReader(html), finalURL)
		if extractErr != nil {
			fmt.Printf("⚠️  Link extraction error (will continue): %v\n", extractErr)
			crawlerLinks = nil
		}
		fmt.Printf("📊 Found %d raw links from %s\n", len(crawlerLinks), finalURL)

		// Apply FilterChain: domain, extension, path pattern, dedup
		filteredLinks := filterChain.Apply(crawlerLinks)
		fmt.Printf("📊 %d links after filtering\n", len(filteredLinks))

		// Create edges for already-visited links
		for _, vl := range dedupFilter.LastVisited {
			if vl.NodeID != uuid.Nil {
				edgeErr := p.graphRepo.CreateEdge(ctx, db.CreateEdgeParams{
					FromNodeID: currentNodeID,
					ToNodeID:   vl.NodeID,
				})
				if edgeErr != nil {
					fmt.Printf("⚠️  Edge error (visited, will continue): %v\n", edgeErr)
				} else {
					confirmedEdges[edgeKey{currentNodeID, vl.NodeID}] = true
				}
			}
		}

		// Process new links
		addedCount := 0
		for _, link := range filteredLinks {
			if nodesProcessed >= p.config.MaxNodesPerSite {
				break
			}

			linkHash := hashURL(link.URL)
			linkType := classifyURL(link.URL)
			priority := NodeTypePriority(linkType) + semanticPriorityModifier(link)

			linkCanonical := templatePath(link.URL)
			childNode, createErr := p.graphRepo.CreateNode(ctx, db.CreateNodeParams{
				SiteID:    site.ID,
				Url:       linkCanonical,
				UrlHash:   linkHash,
				NodeType:  sql.NullString{String: string(linkType), Valid: true},
				ScriptID:  uuid.NullUUID{Valid: false},
				SampleUrl: sql.NullString{String: link.URL, Valid: true},
			})
			if createErr != nil {
				// Duplicate key defense: concurrent insert recovery
				if strings.Contains(createErr.Error(), "duplicate key") || strings.Contains(createErr.Error(), "unique constraint") {
					existingChild, getErr := p.graphRepo.GetNodeByHash(ctx, db.GetNodeByHashParams{
						SiteID:  site.ID,
						UrlHash: linkHash,
					})
					if getErr == nil {
						visited[linkHash] = existingChild.ID
						edgeErr := p.graphRepo.CreateEdge(ctx, db.CreateEdgeParams{
							FromNodeID: currentNodeID,
							ToNodeID:   existingChild.ID,
						})
						if edgeErr != nil {
							fmt.Printf("⚠️  Edge error (will continue): %v\n", edgeErr)
						} else {
							confirmedEdges[edgeKey{currentNodeID, existingChild.ID}] = true
						}
						queue.Push(&QueueItem{
							URL:      link.URL,
							URLHash:  linkHash,
							Priority: priority,
						})
						addedCount++
					}
					continue
				}
				fmt.Printf("⚠️  Node creation error (will continue): %v\n", createErr)
				continue
			}

			visited[linkHash] = childNode.ID
			nodesProcessed++

			// Create edge: parent → child
			edgeErr := p.graphRepo.CreateEdge(ctx, db.CreateEdgeParams{
				FromNodeID: currentNodeID,
				ToNodeID:   childNode.ID,
			})
			if edgeErr != nil {
				fmt.Printf("⚠️  Edge error (will continue): %v\n", edgeErr)
			} else {
				confirmedEdges[edgeKey{currentNodeID, childNode.ID}] = true
			}

			queue.Push(&QueueItem{
				URL:      link.URL,
				URLHash:  linkHash,
				Priority: priority,
			})
			addedCount++
		}
		fmt.Printf("✅ Added %d links to queue (queue size: %d)\n", addedCount, queue.Len())
	}

	// Cleanup stale edges: delete edges from visited nodes that were not re-confirmed
	visitedNodeIDs := make(map[uuid.UUID]bool)
	for _, nodeID := range visited {
		if nodeID != uuid.Nil {
			visitedNodeIDs[nodeID] = true
		}
	}
	var staleEdgeIDs []uuid.UUID
	for ek, edgeID := range existingEdges {
		if visitedNodeIDs[ek.from] && !confirmedEdges[ek] {
			staleEdgeIDs = append(staleEdgeIDs, edgeID)
		}
	}
	if len(staleEdgeIDs) > 0 {
		delErr := p.graphRepo.DeleteEdgesByIDs(ctx, staleEdgeIDs)
		if delErr != nil {
			fmt.Printf("⚠️  Failed to delete %d stale edges: %v\n", len(staleEdgeIDs), delErr)
		} else {
			fmt.Printf("🧹 Deleted %d stale edges\n", len(staleEdgeIDs))
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

// fetchHTML fetches HTML content using the shared fetch function.
// Returns (html, finalURL, error) where finalURL is the URL after any redirects.
func (p *Pioneer) fetchHTML(ctx context.Context, urlStr string) (string, string, error) {
	return fetchHTMLShared(ctx, urlStr)
}

// urlPathContains checks if a URL path contains the given segment as a whole path component.
// Uses boundary-aware matching: /photos/ matches but "hot" inside "photos" does not match "hot".
func urlPathContains(urlPath string, segment string) bool {
	// Check as path segment: /segment/ or /segment? or /segment at end
	return strings.Contains(urlPath, "/"+segment+"/") ||
		strings.HasSuffix(urlPath, "/"+segment)
}

// URL Classification
func classifyURL(urlStr string) NodeType {
	u, parseErr := url.Parse(strings.ToLower(urlStr))
	if parseErr != nil {
		return NodeTypeList
	}
	path := u.Path

	// Detail page: content singular path segments (check BEFORE listing to avoid "hot" in "photos")
	detailPathPatterns := []string{"artworks", "photos", "works", "illust", "artwork", "photo"}
	for _, pattern := range detailPathPatterns {
		if urlPathContains(path, pattern) {
			return NodeTypeDetail
		}
	}

	// Detail page: query parameter with explicit ID keys
	q := u.Query()
	idKeys := []string{"id", "illust_id", "artwork_id", "photo_id"}
	for _, key := range idKeys {
		if val := q.Get(key); val != "" {
			if numericIDPattern.MatchString(val) {
				return NodeTypeDetail
			}
		}
	}

	// Listing patterns (path segment match)
	listingPatterns := []string{"trending", "popular", "hot", "featured", "recent", "explore"}
	for _, pattern := range listingPatterns {
		if urlPathContains(path, pattern) {
			return NodeTypeList
		}
	}

	// Gallery patterns
	galleryPatterns := []string{"gallery", "galleries", "collection", "collections", "album", "albums", "showcase"}
	for _, pattern := range galleryPatterns {
		if urlPathContains(path, pattern) {
			return NodeTypeList
		}
	}

	// Category patterns
	categoryPatterns := []string{"category", "categories", "tag", "tags", "genre", "genres", "style", "styles", "contest", "contests", "event", "events"}
	for _, pattern := range categoryPatterns {
		if urlPathContains(path, pattern) {
			return NodeTypeList
		}
	}

	// Detail page: has numeric ID pattern in path
	if pathNumericIDPattern.MatchString(path) {
		return NodeTypeDetail
	}

	return NodeTypeList // Default
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

// templatePath normalizes a URL to a page template pattern for node deduplication.
// 1. Strips query parameters and fragment
// 2. Replaces pure-numeric path segments with {id}
func templatePath(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	// Keep scheme + host + path only
	segments := strings.Split(u.Path, "/")
	for i, seg := range segments {
		if seg != "" && isNumeric(seg) {
			segments[i] = "{id}"
		}
	}
	u.Path = strings.Join(segments, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// isNumeric returns true if s consists entirely of digits.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// Helper: hash URL for deduplication (uses template path for node pattern matching)
func hashURL(urlStr string) string {
	h := md5.Sum([]byte(templatePath(urlStr)))
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

// saveSnapshot uploads the raw fetched body for a URL to the snapshot
// store. It is fail-open: feature-flag off, missing store, empty body, or
// upload failure all return without affecting the caller's control flow.
//
// The hook is called only after fetchHTMLShared confirmed HTTP 2xx and a
// non-empty body — see helpers.go. fetchHTMLShared returns an error for
// 4xx/5xx, timeouts, and zero-length bodies, so this method is naturally
// not invoked for those failure modes (spec: "Scenario: HTTP 404 응답",
// "Scenario: 네트워크 타임아웃", "Scenario: 본문이 비어 있는 성공 응답").
func (p *Pioneer) saveSnapshot(ctx context.Context, normalizedURL string, body []byte) {
	if !p.config.SnapshotEnabled || p.snapshotStore == nil {
		return
	}

	start := time.Now()
	err := p.snapshotStore.Put(ctx, normalizedURL, body)
	elapsed := time.Since(start)

	if err != nil {
		p.snapshotMetrics.IncFailure()
		// Structured-ish log: URL, hash, error, and elapsed so operators
		// can trace which key failed and whether it was a fast reject
		// (auth/4xx) vs a slow timeout, without re-running the hash.
		fmt.Printf("⚠️  snapshot upload failed url=%q hash=%s elapsed=%s err=%v\n",
			normalizedURL, snapshot.HashNormalizedURL(normalizedURL), elapsed, err)
		return
	}
	// Observe duration only on success so the histogram represents
	// the cost of the steady-state happy path; failure latencies (which
	// can be dominated by client-side timeouts) get their own signal
	// via the failure counter and the elapsed= field on the log line.
	p.snapshotMetrics.ObserveDuration(elapsed)
	p.snapshotMetrics.IncSuccess()
}

// Helper: extract links from HTML
