package bot

import (
	"net/url"
	"strings"
)

const (
	DefaultSimThreshold   = 0.5
	DefaultMergeThreshold = 5
	drainWildcard         = "{param}"
)

// DrainCluster holds a group of URLs that share the same template pattern.
type DrainCluster struct {
	template []string
	urls     [][]string
}

// Template returns the cluster's current template as a joined path string.
func (c *DrainCluster) Template() string {
	return "/" + strings.Join(c.template, "/")
}

// DrainTree implements a simplified Drain algorithm for URL path pattern detection.
// Groups URLs by segment count, then clusters by token-level similarity.
// Unlike the original Drain paper's depth-2 tree (first-segment routing), this uses
// depth-1 (segment-count only) to avoid missing depth-2 leaf explosion patterns.
type DrainTree struct {
	groups       map[int][]*DrainCluster // segmentCount → clusters
	simThreshold float64
}

// NewDrainTree creates a DrainTree with the given similarity threshold.
func NewDrainTree(simThreshold float64) *DrainTree {
	return &DrainTree{
		groups:       make(map[int][]*DrainCluster),
		simThreshold: simThreshold,
	}
}

// Insert adds a URL into the DrainTree, either joining an existing cluster
// or creating a new one based on similarity.
func (dt *DrainTree) Insert(rawURL string) {
	segments := extractPathSegments(rawURL)
	if len(segments) == 0 {
		return
	}

	depth := len(segments)
	clusters := dt.groups[depth]

	bestIdx := -1
	bestSim := 0.0
	bestHasWild := false
	for i, c := range clusters {
		sim, hasWild := drainSimilarity(c.template, segments)
		if sim > bestSim {
			bestSim = sim
			bestIdx = i
			bestHasWild = hasWild
		}
	}

	// Asymmetric threshold: strict > for templates with wildcards, >= otherwise.
	// This allows depth-2 leaf explosion (sim=0.5 with no wildcards) to cluster,
	// while preventing wildcard creep (sim=0.5 with wildcards → rejected).
	joined := false
	if bestIdx >= 0 {
		if bestHasWild {
			joined = bestSim > dt.simThreshold
		} else {
			joined = bestSim >= dt.simThreshold
		}
	}

	if joined {
		c := clusters[bestIdx]
		c.urls = append(c.urls, segments)
		for i := range c.template {
			if c.template[i] != drainWildcard && c.template[i] != segments[i] {
				c.template[i] = drainWildcard
			}
		}
	} else {
		tmpl := make([]string, len(segments))
		copy(tmpl, segments)
		dt.groups[depth] = append(clusters, &DrainCluster{
			template: tmpl,
			urls:     [][]string{segments},
		})
	}
}

// drainSimilarity calculates similarity between a template and segments.
// Wildcard positions in the template are excluded from the calculation to prevent creep.
// Returns the similarity ratio and whether the template contains wildcards.
func drainSimilarity(template, segments []string) (float64, bool) {
	if len(template) != len(segments) {
		return 0, false
	}

	hasWildcard := false
	matches := 0
	total := 0

	for i := range template {
		if template[i] == drainWildcard {
			hasWildcard = true
			continue
		}
		total++
		if template[i] == segments[i] {
			matches++
		}
	}

	if total == 0 {
		return 1.0, hasWildcard
	}
	return float64(matches) / float64(total), hasWildcard
}

// MergeTarget represents a group of URLs to be merged into a single template node.
type MergeTarget struct {
	Prefix      string   // e.g. "/howto/search" or "/tags"
	ParamValues []string // e.g. ["AIart", "8bit"] or ["TAG1", "TAG2"]
	Suffix      string   // e.g. "" (leaf explosion) or "/artwork" (mid-path)
}

// MergeTargets returns merge targets from clusters exceeding the count threshold.
// Mid-path wildcards (not at last position) require static segment verification.
func (dt *DrainTree) MergeTargets(countThreshold int) []MergeTarget {
	var results []MergeTarget

	for _, clusters := range dt.groups {
		for _, c := range clusters {
			if len(c.urls) <= countThreshold {
				continue
			}

			// Find the first wildcard position
			paramIdx := -1
			wildcardCount := 0
			for i, t := range c.template {
				if t == drainWildcard {
					wildcardCount++
					if paramIdx < 0 {
						paramIdx = i
					}
				}
			}
			if paramIdx < 0 {
				continue
			}
			// Skip multi-wildcard clusters (too ambiguous)
			if wildcardCount > 1 {
				continue
			}

			isLastPos := paramIdx == len(c.template)-1
			if !isLastPos && !hasStaticAfterWildcard(c.template, paramIdx) {
				continue
			}

			prefix := "/" + strings.Join(c.template[:paramIdx], "/")
			suffix := ""
			if !isLastPos {
				suffix = "/" + strings.Join(c.template[paramIdx+1:], "/")
			}

			var paramValues []string
			for _, u := range c.urls {
				if paramIdx < len(u) {
					paramValues = append(paramValues, u[paramIdx])
				}
			}

			results = append(results, MergeTarget{
				Prefix:      prefix,
				ParamValues: paramValues,
				Suffix:      suffix,
			})
		}
	}

	return results
}

// hasStaticAfterWildcard checks if any token after the wildcard position
// is a static segment (not wrapped in {}).
func hasStaticAfterWildcard(template []string, wildcardIdx int) bool {
	for i := wildcardIdx + 1; i < len(template); i++ {
		if !isParameterized(template[i]) {
			return true
		}
	}
	return false
}

// isParameterized returns true if a segment looks like a parameter ({...}).
func isParameterized(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

// extractPathSegments parses a URL string and returns non-empty path segments.
func extractPathSegments(rawURL string) []string {
	path := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		path = u.Path
	}
	var segments []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	return segments
}
