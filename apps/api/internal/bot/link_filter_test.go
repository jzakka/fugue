package bot

import (
	"testing"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
)

// makeLink is a test helper to create a crawler.Link with optional selectors.
func makeLink(url string, selectors ...crawler.Selector) crawler.Link {
	return crawler.Link{URL: url, Selectors: selectors}
}

func sel(tag string) crawler.Selector {
	return crawler.Selector{TagName: tag}
}

// --- 6.2 TestCanonicalURL ---

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "remove utm_source, keep other params",
			in:   "https://example.com/page?utm_source=twitter&id=123",
			want: "https://example.com/page?id=123",
		},
		{
			name: "remove all tracking params",
			in:   "https://example.com/page?utm_source=a&utm_medium=b&utm_campaign=c&fbclid=d&gclid=e",
			want: "https://example.com/page",
		},
		{
			name: "remove www prefix",
			in:   "https://www.example.com/page",
			want: "https://example.com/page",
		},
		{
			name: "remove trailing slash",
			in:   "https://example.com/page/",
			want: "https://example.com/page",
		},
		{
			name: "keep root slash",
			in:   "https://example.com/",
			want: "https://example.com/",
		},
		{
			name: "combined: www + tracking + trailing slash",
			in:   "https://www.example.com/gallery/?utm_source=ig&ref=home",
			want: "https://example.com/gallery",
		},
		{
			name: "no changes needed",
			in:   "https://example.com/page?id=42",
			want: "https://example.com/page?id=42",
		},
		{
			name: "scheme and host lowercased, path case preserved",
			in:   "HTTPS://Example.COM/Page",
			want: "https://example.com/Page",
		},
		{
			name: "http default port 80 removed",
			in:   "http://example.com:80/path",
			want: "http://example.com/path",
		},
		{
			name: "https default port 443 removed",
			in:   "https://example.com:443/path",
			want: "https://example.com/path",
		},
		{
			name: "non-default port 8080 preserved",
			in:   "http://example.com:8080/path",
			want: "http://example.com:8080/path",
		},
		{
			name: "query params sorted by key",
			in:   "https://example.com/page?b=2&a=1&c=3",
			want: "https://example.com/page?a=1&b=2&c=3",
		},
		{
			name: "tracking removed, remaining params sorted",
			in:   "https://example.com/page?utm_source=twitter&id=123&a=z",
			want: "https://example.com/page?a=z&id=123",
		},
		{
			name: "combined canonical case",
			in:   "http://Example.com:80/path/?b=2&a=1#frag",
			want: "http://example.com/path?a=1&b=2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalURL(tt.in)
			if got != tt.want {
				t.Errorf("canonicalURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// --- 6.3 TestSemanticPriorityModifier ---

func TestSemanticPriorityModifier(t *testing.T) {
	tests := []struct {
		name string
		link crawler.Link
		want int
	}{
		{"footer link", makeLink("https://x.com", sel("body"), sel("footer"), sel("a")), -50},
		{"aside link", makeLink("https://x.com", sel("aside")), -50},
		{"nav link", makeLink("https://x.com", sel("nav"), sel("a")), -20},
		{"header link", makeLink("https://x.com", sel("header"), sel("a")), -20},
		{"main content", makeLink("https://x.com", sel("main"), sel("article"), sel("a")), 0},
		{"no selectors", makeLink("https://x.com"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := semanticPriorityModifier(tt.link)
			if got != tt.want {
				t.Errorf("semanticPriorityModifier() = %d, want %d", got, tt.want)
			}
		})
	}
}

// --- 6.4 TestDomainFilter ---

func TestDomainFilter(t *testing.T) {
	urls := func(links []crawler.Link) []string {
		out := make([]string, len(links))
		for i, l := range links {
			out[i] = l.URL
		}
		return out
	}

	t.Run("Allow empty - cross-site default allow", func(t *testing.T) {
		f := &DomainFilter{}
		links := []crawler.Link{
			makeLink("https://seed.com/a"),
			makeLink("https://other.net/b"),
			makeLink("https://sub.third.org/c"),
		}
		if got := f.Filter(links); len(got) != 3 {
			t.Errorf("expected all 3 to pass, got %v", urls(got))
		}
	})

	t.Run("Deny keyword blocks matched host", func(t *testing.T) {
		f := &DomainFilter{DenyKeywords: []string{"adnetwork"}}
		links := []crawler.Link{
			makeLink("https://example.com/page"),
			makeLink("https://tracker.adnetwork.com/beacon"),
		}
		got := f.Filter(links)
		if len(got) != 1 || got[0].URL != "https://example.com/page" {
			t.Errorf("deny failed; got %v", urls(got))
		}
	})

	t.Run("Allow list - whitelist mode", func(t *testing.T) {
		f := &DomainFilter{AllowKeywords: []string{"music"}}
		links := []crawler.Link{
			makeLink("https://music.io/track/1"),
			makeLink("https://other.net/page"),
		}
		got := f.Filter(links)
		if len(got) != 1 || got[0].URL != "https://music.io/track/1" {
			t.Errorf("whitelist failed; got %v", urls(got))
		}
	})

	t.Run("Deny wins over Allow", func(t *testing.T) {
		f := &DomainFilter{
			AllowKeywords: []string{"example.com"},
			DenyKeywords:  []string{"tracker"},
		}
		links := []crawler.Link{
			makeLink("https://tracker.example.com/beacon"),
			makeLink("https://docs.example.com/page"),
		}
		got := f.Filter(links)
		if len(got) != 1 || got[0].URL != "https://docs.example.com/page" {
			t.Errorf("deny-wins failed; got %v", urls(got))
		}
	})

	t.Run("www prefix is normalized away", func(t *testing.T) {
		f := &DomainFilter{AllowKeywords: []string{"example.com"}}
		links := []crawler.Link{makeLink("https://www.example.com/page")}
		got := f.Filter(links)
		if len(got) != 1 {
			t.Errorf("www normalization failed; got %v", urls(got))
		}
	})

	t.Run("case insensitive host matching", func(t *testing.T) {
		f := &DomainFilter{AllowKeywords: []string{"example.com"}}
		links := []crawler.Link{makeLink("https://WWW.Example.COM/page")}
		got := f.Filter(links)
		if len(got) != 1 {
			t.Errorf("case-insensitive matching failed; got %v", urls(got))
		}
	})

	t.Run("unparseable or hostless URL is dropped", func(t *testing.T) {
		// Satisfies spec scenario "파싱 불가능한 URL은 큐에 포함되지 않는다":
		// the very first filter in the chain drops items the rest of the
		// pipeline could not reason about anyway.
		f := &DomainFilter{}
		links := []crawler.Link{
			makeLink(""),                            // empty URL → no host
			makeLink("://bad-url"),                  // unparseable
			makeLink("https://ok.example.com/page"), // valid
		}
		got := f.Filter(links)
		if len(got) != 1 || got[0].URL != "https://ok.example.com/page" {
			t.Errorf("expected only the valid URL to pass; got %v", urls(got))
		}
	})

	t.Run("uppercase deny keyword normalized", func(t *testing.T) {
		// Deny list entries are lowercased internally; callers may supply
		// mixed-case keywords without changing matching behavior.
		f := &DomainFilter{DenyKeywords: []string{"TRACKER"}}
		links := []crawler.Link{makeLink("https://tracker.example.com/beacon")}
		got := f.Filter(links)
		if len(got) != 0 {
			t.Errorf("uppercase deny keyword failed to match; got %v", urls(got))
		}
	})
}

// --- 6.5 TestExtensionFilter ---

func TestExtensionFilter(t *testing.T) {
	f := &ExtensionFilter{}
	links := []crawler.Link{
		makeLink("https://example.com/photo.jpg"),
		makeLink("https://example.com/style.css"),
		makeLink("https://example.com/gallery/artwork-123"),
		makeLink("https://example.com/page.html"),
	}
	got := f.Filter(links)
	if len(got) != 2 {
		t.Fatalf("expected 2 links, got %d", len(got))
	}
	if got[0].URL != "https://example.com/gallery/artwork-123" {
		t.Errorf("unexpected link: %s", got[0].URL)
	}
}

// --- 6.6 TestPathPatternFilter ---

func TestPathPatternFilter(t *testing.T) {
	f := &PathPatternFilter{} // uses defaultExcludePatterns

	t.Run("excludes ad and popup segments", func(t *testing.T) {
		links := []crawler.Link{
			makeLink("https://example.com/ad/banner"),
			makeLink("https://example.com/popup/subscribe"),
			makeLink("https://example.com/gallery/popular"),
		}
		got := f.Filter(links)
		if len(got) != 1 {
			t.Fatalf("expected 1 link, got %d", len(got))
		}
		if got[0].URL != "https://example.com/gallery/popular" {
			t.Errorf("unexpected link: %s", got[0].URL)
		}
	})

	t.Run("boundary-aware: loading does not match login", func(t *testing.T) {
		links := []crawler.Link{
			makeLink("https://example.com/loading/page"),
			makeLink("https://example.com/login/page"),
		}
		got := f.Filter(links)
		if len(got) != 1 {
			t.Fatalf("expected 1 link, got %d", len(got))
		}
		if got[0].URL != "https://example.com/loading/page" {
			t.Errorf("expected loading to pass, got %s", got[0].URL)
		}
	})

	t.Run("custom patterns", func(t *testing.T) {
		custom := &PathPatternFilter{ExcludePatterns: []string{"private"}}
		links := []crawler.Link{
			makeLink("https://example.com/private/page"),
			makeLink("https://example.com/public/page"),
			makeLink("https://example.com/login/page"), // not excluded with custom patterns
		}
		got := custom.Filter(links)
		if len(got) != 2 {
			t.Fatalf("expected 2 links, got %d", len(got))
		}
	})
}

// --- 6.7 TestCanonicalDedupFilter ---

func TestCanonicalDedupFilter(t *testing.T) {
	existingNodeID := uuid.New()
	visited := map[string]uuid.UUID{
		hashURL("https://example.com/visited"): existingNodeID,
	}
	f := NewCanonicalDedupFilter(visited)

	links := []crawler.Link{
		makeLink("https://example.com/page1"),
		makeLink("https://example.com/page1"),                    // exact duplicate
		makeLink("https://example.com/page2?utm_source=twitter"), // canonical dup of next
		makeLink("https://www.example.com/page2"),                // canonical dup of previous
		makeLink("https://example.com/visited"),                  // already visited
		makeLink("https://example.com/page3"),                    // unique
	}

	got := f.Filter(links)

	// Should keep: page1 (first), page2?utm_source=twitter (first canonical), page3
	if len(got) != 3 {
		t.Fatalf("expected 3 links, got %d: %v", len(got), got)
	}
	if got[0].URL != "https://example.com/page1" {
		t.Errorf("first link should be page1, got %s", got[0].URL)
	}
	if got[1].URL != "https://example.com/page2?utm_source=twitter" {
		t.Errorf("second link should be page2 with utm, got %s", got[1].URL)
	}
	if got[2].URL != "https://example.com/page3" {
		t.Errorf("third link should be page3, got %s", got[2].URL)
	}

	// LastVisited should record the visited URL
	if len(f.LastVisited) != 1 {
		t.Fatalf("expected 1 LastVisited, got %d", len(f.LastVisited))
	}
	if f.LastVisited[0].NodeID != existingNodeID {
		t.Errorf("LastVisited NodeID = %v, want %v", f.LastVisited[0].NodeID, existingNodeID)
	}
}

// Pins pioneer/spec.md "Pioneer는 인메모리 크롤 상태를 보유하지 않는다": dedup
// must be batch-local. If the seen set survived across Filter calls, a page
// re-filtered after a transient Enqueue failure (10-min lease retry) would
// lose all of its links while the page itself gets marked fetched for 365
// days. Cross-batch dedup belongs to the scheduler (ON CONFLICT DO NOTHING).
func TestCanonicalDedupFilterIsStatelessAcrossCalls(t *testing.T) {
	f := NewCanonicalDedupFilter(nil)
	links := []crawler.Link{makeLink("https://example.com/retry-me")}

	if got := f.Filter(links); len(got) != 1 {
		t.Fatalf("first call: expected 1 link, got %d", len(got))
	}
	if got := f.Filter(links); len(got) != 1 {
		t.Fatalf("second call: expected 1 link (batch-local dedup only), got %d", len(got))
	}
}

// --- 6.8 TestFilterChain ---

func TestFilterChain(t *testing.T) {
	t.Run("chain applies filters in order", func(t *testing.T) {
		chain := NewFilterChain(
			&DomainFilter{DenyKeywords: []string{"other.com"}},
			&ExtensionFilter{},
			&PathPatternFilter{},
		)
		links := []crawler.Link{
			makeLink("https://example.com/gallery/art"),
			makeLink("https://other.com/page"),         // removed by DomainFilter (deny)
			makeLink("https://example.com/photo.jpg"),  // removed by ExtensionFilter
			makeLink("https://example.com/login/form"), // removed by PathPatternFilter
		}
		got := chain.Apply(links)
		if len(got) != 1 {
			t.Fatalf("expected 1 link, got %d", len(got))
		}
		if got[0].URL != "https://example.com/gallery/art" {
			t.Errorf("expected gallery/art, got %s", got[0].URL)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		chain := NewFilterChain(&DomainFilter{})
		got := chain.Apply([]crawler.Link{})
		if len(got) != 0 {
			t.Errorf("expected empty, got %d", len(got))
		}
	})

	t.Run("nil list", func(t *testing.T) {
		chain := NewFilterChain(&DomainFilter{})
		got := chain.Apply(nil)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}
