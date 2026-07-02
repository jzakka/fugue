package bot

import (
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
	"github.com/chungsanghwa/fugue/apps/api/internal/urlcanon"
)

// LinkFilter defines the contract for filtering crawled links.
// Implementations receive a slice of links and return only those that pass the filter criteria.
type LinkFilter interface {
	Filter(links []crawler.Link) []crawler.Link
}

// FilterChain applies multiple LinkFilter instances in sequence.
// Each filter's output becomes the next filter's input.
type FilterChain struct {
	filters []LinkFilter
}

// NewFilterChain creates a FilterChain with the given filters applied in order.
func NewFilterChain(filters ...LinkFilter) *FilterChain {
	return &FilterChain{filters: filters}
}

// Apply runs all registered filters sequentially on the input links.
// Returns nil for nil input. Returns the input unchanged if no filters are registered.
func (c *FilterChain) Apply(links []crawler.Link) []crawler.Link {
	if links == nil {
		return nil
	}
	for _, f := range c.filters {
		if f == nil {
			continue
		}
		links = f.Filter(links)
	}
	return links
}

// --- Helpers ---

// canonicalURL is a thin wrapper around urlcanon.Canonical. It exists so
// existing bot callers don't have to change their import paths. The single
// source of truth for canonicalization rules is urlcanon (shared with the
// scheduler so url_hash and snapshot_key stay in lockstep).
func canonicalURL(urlStr string) string {
	return urlcanon.Canonical(urlStr)
}

// stripWWW removes a leading "www." from a host string. Input must already
// be lowercase. Kept as a local helper because DomainFilter's host-keyword
// matching wants just the www-stripped hostname, not full canonicalization.
func stripWWW(host string) string {
	return strings.TrimPrefix(host, "www.")
}

// VisitedLink records a link that was already visited, pairing the original
// Link with the existing node's UUID for edge creation.
type VisitedLink struct {
	Link   crawler.Link
	NodeID uuid.UUID
}

// semanticPriorityModifier returns a priority adjustment based on where
// the link appears in the page DOM (footer/aside: -50, nav/header: -20, else: 0).
func semanticPriorityModifier(link crawler.Link) int {
	for _, sel := range link.Selectors {
		tag := strings.ToLower(sel.TagName)
		switch tag {
		case "footer", "aside":
			return -50
		case "nav", "header":
			return -20
		}
	}
	return 0
}

// --- Filter Implementations ---

// DomainFilter selects links by substring-matching the link host against
// AllowKeywords / DenyKeywords. Hosts are normalized (lowercased, "www."
// stripped) before matching.
//
// Deny always wins: a host matched by any DenyKeywords entry is rejected.
// If AllowKeywords is empty (the default), all non-denied hosts pass —
// this is the cross-site default-allow mode. If AllowKeywords is
// non-empty, a host must match at least one Allow entry to pass.
type DomainFilter struct {
	AllowKeywords []string
	DenyKeywords  []string
}

func (f *DomainFilter) Filter(links []crawler.Link) []crawler.Link {
	var out []crawler.Link
	for _, l := range links {
		host := normalizedHost(l.URL)
		if host == "" {
			// Unparseable or hostless URL: drop.
			continue
		}
		if matchesAnyKeyword(host, f.DenyKeywords) {
			continue
		}
		if len(f.AllowKeywords) == 0 || matchesAnyKeyword(host, f.AllowKeywords) {
			out = append(out, l)
		}
	}
	return out
}

// normalizedHost returns the link host lowercased with "www." stripped.
// Returns "" for URLs that cannot be parsed or have no host.
func normalizedHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	h := strings.ToLower(u.Hostname())
	if h == "" {
		return ""
	}
	return stripWWW(h)
}

// matchesAnyKeyword returns true if host contains any of keywords as a
// case-insensitive substring. Empty keywords are skipped.
func matchesAnyKeyword(host string, keywords []string) bool {
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(host, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// ExtensionFilter removes links with excluded file extensions.
type ExtensionFilter struct{}

func (f *ExtensionFilter) Filter(links []crawler.Link) []crawler.Link {
	var out []crawler.Link
	for _, l := range links {
		if !hasExcludedExtension(l.URL) {
			out = append(out, l)
		}
	}
	return out
}

// defaultExcludePatterns are path segments to skip during crawling.
var defaultExcludePatterns = []string{"ad", "popup", "login", "signup", "cart", "checkout"}

// PathPatternFilter removes links whose URL path contains excluded segments.
// Uses boundary-aware matching via urlPathContains.
type PathPatternFilter struct {
	ExcludePatterns []string
}

func (f *PathPatternFilter) Filter(links []crawler.Link) []crawler.Link {
	patterns := f.ExcludePatterns
	if patterns == nil {
		patterns = defaultExcludePatterns
	}
	var out []crawler.Link
	for _, l := range links {
		u, err := url.Parse(strings.ToLower(l.URL))
		if err != nil {
			continue
		}
		excluded := false
		for _, p := range patterns {
			if urlPathContains(u.Path, p) {
				excluded = true
				break
			}
		}
		if !excluded {
			out = append(out, l)
		}
	}
	return out
}

// CanonicalDedupFilter removes duplicate links using URL canonicalization.
// Already-visited URLs (found in the visited map) are recorded in LastVisited.
//
// Dedup is strictly batch-local: the seen set lives inside a single Filter
// call. A process-lifetime seen map would be in-memory crawl state, which
// pioneer/spec.md ("Pioneer는 인메모리 크롤 상태를 보유하지 않는다") prohibits —
// and it silently drops links forever when an Enqueue fails and the lease
// retry re-filters the same page. Cross-batch dedup is the scheduler's job
// (EnqueuePioneer ON CONFLICT (url_hash) DO NOTHING).
type CanonicalDedupFilter struct {
	visited     map[string]uuid.UUID // shared with crawl loop: hash(url) → node ID
	LastVisited []VisitedLink
}

// NewCanonicalDedupFilter creates a dedup filter sharing the given visited map.
func NewCanonicalDedupFilter(visited map[string]uuid.UUID) *CanonicalDedupFilter {
	return &CanonicalDedupFilter{
		visited: visited,
	}
}

func (f *CanonicalDedupFilter) Filter(links []crawler.Link) []crawler.Link {
	f.LastVisited = nil // reset per call
	seen := make(map[string]bool, len(links))
	var out []crawler.Link
	for _, l := range links {
		// Check visited map (exact URL hash)
		h := hashURL(l.URL)
		if nodeID, ok := f.visited[h]; ok {
			f.LastVisited = append(f.LastVisited, VisitedLink{Link: l, NodeID: nodeID})
			continue
		}

		// Check canonical dedup (normalized URL hash)
		ch := hashURL(canonicalURL(l.URL))
		if seen[ch] {
			continue
		}
		seen[ch] = true
		out = append(out, l)
	}
	return out
}
