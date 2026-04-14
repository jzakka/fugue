package bot

import (
	"sort"
	"testing"
)

func TestDrainInsertAndTemplate(t *testing.T) {
	dt := NewDrainTree(0.5)
	dt.Insert("/howto/search/AIart")
	dt.Insert("/howto/search/8bit")

	clusters := dt.groups[3]
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Template() != "/howto/search/{param}" {
		t.Errorf("expected template '/howto/search/{param}', got %q", clusters[0].Template())
	}
}

func TestDrainSimilarityNoWildcard(t *testing.T) {
	sim, hasWild := drainSimilarity(
		[]string{"tags", "TAG1", "artwork"},
		[]string{"tags", "TAG2", "artwork"},
	)
	if hasWild {
		t.Error("expected no wildcard")
	}
	if sim < 0.66 || sim > 0.67 {
		t.Errorf("expected sim ~0.67, got %f", sim)
	}
}

func TestDrainSimilarityWithWildcard(t *testing.T) {
	sim, hasWild := drainSimilarity(
		[]string{"tags", drainWildcard, "artwork"},
		[]string{"tags", "TAG3", "illustrations"},
	)
	if !hasWild {
		t.Error("expected wildcard")
	}
	// Wildcard excluded: only "tags" matches, "artwork" != "illustrations"
	if sim != 0.5 {
		t.Errorf("expected sim 0.5, got %f", sim)
	}
}

// --- 5.2 Leaf explosion ---

func TestLeafExplosionDepth3(t *testing.T) {
	dt := NewDrainTree(0.5)
	for _, kw := range []string{"a", "b", "c", "d", "e", "f"} {
		dt.Insert("/howto/search/" + kw)
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Prefix != "/howto/search" {
		t.Errorf("expected prefix '/howto/search', got %q", targets[0].Prefix)
	}
	if targets[0].Suffix != "" {
		t.Errorf("expected empty suffix, got %q", targets[0].Suffix)
	}
	if len(targets[0].ParamValues) != 6 {
		t.Errorf("expected 6 param values, got %d", len(targets[0].ParamValues))
	}
}

func TestLeafExplosionDepth2(t *testing.T) {
	dt := NewDrainTree(0.5)
	for _, page := range []string{"a", "b", "c", "d", "e", "f"} {
		dt.Insert("/howto/" + page)
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Prefix != "/howto" {
		t.Errorf("expected prefix '/howto', got %q", targets[0].Prefix)
	}
}

func TestLeafExplosionExactThreshold(t *testing.T) {
	dt := NewDrainTree(0.5)
	for _, kw := range []string{"a", "b", "c", "d", "e"} {
		dt.Insert("/howto/search/" + kw)
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (5 == threshold), got %d", len(targets))
	}
}

func TestLeafExplosionBelowThreshold(t *testing.T) {
	dt := NewDrainTree(0.5)
	for _, kw := range []string{"a", "b", "c"} {
		dt.Insert("/howto/search/" + kw)
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}
}

// --- 5.3 Mid-path ---

func TestMidPathBasic(t *testing.T) {
	dt := NewDrainTree(0.5)
	for i := 0; i < 6; i++ {
		dt.Insert("/tags/TAG" + string(rune('A'+i)) + "/artwork")
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Prefix != "/tags" {
		t.Errorf("expected prefix '/tags', got %q", targets[0].Prefix)
	}
	if targets[0].Suffix != "/artwork" {
		t.Errorf("expected suffix '/artwork', got %q", targets[0].Suffix)
	}
}

func TestMidPathMultiSuffix(t *testing.T) {
	dt := NewDrainTree(0.5)
	// Insert by suffix batch (not interleaved by tag) — Drain is insertion-order
	// sensitive; batching by suffix lets each suffix form its own cluster.
	for i := 0; i < 6; i++ {
		dt.Insert("/tags/TAG" + string(rune('A'+i)) + "/artwork")
	}
	for i := 0; i < 6; i++ {
		dt.Insert("/tags/TAG" + string(rune('A'+i)) + "/illustrations")
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	suffixes := []string{targets[0].Suffix, targets[1].Suffix}
	sort.Strings(suffixes)
	if suffixes[0] != "/artwork" || suffixes[1] != "/illustrations" {
		t.Errorf("unexpected suffixes: %v", suffixes)
	}
}

func TestMidPathDeepNesting(t *testing.T) {
	dt := NewDrainTree(0.5)
	for i := 0; i < 6; i++ {
		dt.Insert("/users/USER" + string(rune('A'+i)) + "/posts/recent")
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Prefix != "/users" {
		t.Errorf("expected prefix '/users', got %q", targets[0].Prefix)
	}
	if targets[0].Suffix != "/posts/recent" {
		t.Errorf("expected suffix '/posts/recent', got %q", targets[0].Suffix)
	}
}

// --- 5.4 False positive prevention ---

func TestFalsePositiveAPIRoutes(t *testing.T) {
	dt := NewDrainTree(0.5)
	routes := []string{"users", "posts", "comments", "boards", "feeds", "tags"}
	for _, r := range routes {
		dt.Insert("/api/" + r + "/{id}")
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (API routes should not merge), got %d", len(targets))
	}
}

func TestStaticIntermediateAllowed(t *testing.T) {
	dt := NewDrainTree(0.5)
	for i := 0; i < 6; i++ {
		dt.Insert("/categories/CAT" + string(rune('A'+i)) + "/items/{id}")
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Suffix != "/items/{id}" {
		t.Errorf("expected suffix '/items/{id}', got %q", targets[0].Suffix)
	}
}

// --- 5.5 Depth separation ---

func TestDepthSeparation(t *testing.T) {
	dt := NewDrainTree(0.5)
	// depth=2 URLs
	for i := 0; i < 6; i++ {
		dt.Insert("/tags/" + string(rune('a'+i)))
	}
	// depth=3 URLs (same first segment but different depth)
	for i := 0; i < 6; i++ {
		dt.Insert("/tags/" + string(rune('A'+i)) + "/artwork")
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (one per depth), got %d", len(targets))
	}

	// Sort by suffix to make assertions deterministic
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Suffix < targets[j].Suffix
	})

	// First: leaf explosion at depth 2
	if targets[0].Suffix != "" {
		t.Errorf("expected empty suffix for depth-2, got %q", targets[0].Suffix)
	}
	// Second: mid-path at depth 3
	if targets[1].Suffix != "/artwork" {
		t.Errorf("expected suffix '/artwork' for depth-3, got %q", targets[1].Suffix)
	}
}

// --- 5.6 Idempotency ---

func TestIdempotency(t *testing.T) {
	dt := NewDrainTree(0.5)
	// After a merge, only the template node remains: /tags/{param}/artwork
	dt.Insert("/tags/{param}/artwork")

	targets := dt.MergeTargets(5)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (single template node, count=1), got %d", len(targets))
	}
}

func TestRootLevelNotMerged(t *testing.T) {
	dt := NewDrainTree(0.5)
	// Depth-1 URLs: each has a unique first segment → can't cluster
	pages := []string{"/signup", "/login", "/info", "/about", "/terms", "/privacy"}
	for _, p := range pages {
		dt.Insert(p)
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (root-level pages), got %d", len(targets))
	}
}

func TestMultipleTargets(t *testing.T) {
	dt := NewDrainTree(0.5)
	for i := 0; i < 10; i++ {
		dt.Insert("/howto/search/" + string(rune('a'+i)))
	}
	for i := 0; i < 8; i++ {
		dt.Insert("/tags/TAG" + string(rune('A'+i)) + "/artwork")
	}

	targets := dt.MergeTargets(5)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
}

func TestExtractPathSegments(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"https://www.pixiv.net/howto/search/AIart", []string{"howto", "search", "AIart"}},
		{"/artworks/{id}", []string{"artworks", "{id}"}},
		{"/tags/photo/artwork", []string{"tags", "photo", "artwork"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := extractPathSegments(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("extractPathSegments(%q): got %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("extractPathSegments(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
