package bot

import (
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
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

// trackingParams lists URL query parameters to strip during canonicalization.
var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"ref":          true,
	"fbclid":       true,
	"gclid":        true,
}

// canonicalURL normalizes a URL by removing tracking parameters,
// stripping the www. prefix, and removing trailing slashes.
func canonicalURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	// Remove www. prefix
	u.Host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")

	// Remove tracking parameters
	q := u.Query()
	for param := range trackingParams {
		q.Del(param)
	}
	u.RawQuery = q.Encode()

	// Remove trailing slash (but keep root "/")
	if u.Path != "/" && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}

	// Remove fragment
	u.Fragment = ""

	return u.String()
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

// DomainFilter keeps only links whose domain matches RootDomain.
type DomainFilter struct {
	RootDomain string
}

func (f *DomainFilter) Filter(links []crawler.Link) []crawler.Link {
	var out []crawler.Link
	for _, l := range links {
		if isSameDomain(l.URL, f.RootDomain) {
			out = append(out, l)
		}
	}
	return out
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
type CanonicalDedupFilter struct {
	visited     map[string]uuid.UUID // shared with crawl loop: hash(url) → node ID
	seen        map[string]bool      // internal: hash(canonicalURL(url)) → seen
	LastVisited []VisitedLink
}

// NewCanonicalDedupFilter creates a dedup filter sharing the given visited map.
func NewCanonicalDedupFilter(visited map[string]uuid.UUID) *CanonicalDedupFilter {
	return &CanonicalDedupFilter{
		visited: visited,
		seen:    make(map[string]bool),
	}
}

func (f *CanonicalDedupFilter) Filter(links []crawler.Link) []crawler.Link {
	f.LastVisited = nil // reset per call
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
		if f.seen[ch] {
			continue
		}
		f.seen[ch] = true
		out = append(out, l)
	}
	return out
}
