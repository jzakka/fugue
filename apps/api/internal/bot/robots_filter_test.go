package bot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
)

// newRobotsTestServer builds a minimal HTTP server that routes /robots.txt
// requests to a single handler. Tests drive the handler to simulate the
// variety of robots.txt responses the filter must handle.
func newRobotsTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", handler)
	return httptest.NewTLSServer(mux)
}

// filterForTestServer returns a RobotsFilter configured to dial the given
// TLS test server instead of the real internet. The server's certificate is
// trusted via the test client the server publishes.
func filterForTestServer(ts *httptest.Server, setter HostRateSetter) *RobotsFilter {
	f := NewRobotsFilter(setter)
	f.httpClient = ts.Client()
	// Rewrite requests for https://<host>/robots.txt to the test server URL.
	// The transport below forces host→server mapping so the filter's
	// "https://<host>/robots.txt" URL is redirected to the test server.
	base := ts.URL // e.g. https://127.0.0.1:PORT
	f.httpClient.Transport = &rewriteTransport{base: base, inner: ts.Client().Transport}
	return f
}

// rewriteTransport redirects every outbound request to the test server's
// base URL, preserving the request path. Avoids the need to stand up a
// real DNS + TLS setup for the test host.
type rewriteTransport struct {
	base  string
	inner http.RoundTripper
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := parseBase(r.base)
	if err != nil {
		return nil, err
	}
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = target.scheme
	req2.URL.Host = target.host
	req2.Host = target.host
	return r.inner.RoundTrip(req2)
}

type parsedBase struct {
	scheme, host string
}

func parseBase(base string) (parsedBase, error) {
	// httptest.NewTLSServer yields "https://host:port". Hand-parse to avoid
	// pulling another url import just for the two fields we need.
	const prefix = "https://"
	if !strings.HasPrefix(base, prefix) {
		return parsedBase{}, http.ErrUseLastResponse
	}
	return parsedBase{scheme: "https", host: strings.TrimPrefix(base, prefix)}, nil
}

// fakeRateSetter records SetHostRate calls for assertion.
type fakeRateSetter struct {
	mu    sync.Mutex
	calls []fakeRateCall
}

type fakeRateCall struct {
	host  string
	rate  float64
	burst int
}

func (f *fakeRateSetter) SetHostRate(host string, rate float64, burst int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeRateCall{host, rate, burst})
}

func (f *fakeRateSetter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// TestRobotsFilter_DisallowBlocks verifies that paths matching a Disallow
// rule in robots.txt are removed from the link set.
func TestRobotsFilter_DisallowBlocks(t *testing.T) {
	body := "User-agent: *\nDisallow: /private/\n"
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{
		{URL: "https://example.com/public/page"},
		{URL: "https://example.com/private/secret"},
	}
	got := f.Filter(links)
	if len(got) != 1 || got[0].URL != "https://example.com/public/page" {
		t.Fatalf("expected only /public/page to pass, got %+v", got)
	}
}

// TestRobotsFilter_NoRulesAllowsAll checks that when robots.txt has no
// rules, every link passes.
func TestRobotsFilter_NoRulesAllowsAll(t *testing.T) {
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\n"))
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{{URL: "https://example.com/x"}, {URL: "https://example.com/y"}}
	if got := f.Filter(links); len(got) != 2 {
		t.Fatalf("expected both links to pass; got %d", len(got))
	}
}

// TestRobotsFilter_FugueBotPreferredOverWildcard ensures the FugueBot block
// takes precedence and the wildcard block is not merged.
func TestRobotsFilter_FugueBotPreferredOverWildcard(t *testing.T) {
	body := "User-agent: *\nDisallow: /\n\nUser-agent: FugueBot\nDisallow: /private/\n"
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{
		{URL: "https://example.com/public/a"},  // allowed for FugueBot
		{URL: "https://example.com/private/b"}, // disallowed for FugueBot
	}
	got := f.Filter(links)
	if len(got) != 1 || got[0].URL != "https://example.com/public/a" {
		t.Fatalf("FugueBot block should dominate; got %+v", got)
	}
}

// TestRobotsFilter_WildcardFallback verifies the `*` block is used when no
// FugueBot block is present.
func TestRobotsFilter_WildcardFallback(t *testing.T) {
	body := "User-agent: *\nDisallow: /blocked/\n"
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{
		{URL: "https://example.com/ok"},
		{URL: "https://example.com/blocked/x"},
	}
	got := f.Filter(links)
	if len(got) != 1 || got[0].URL != "https://example.com/ok" {
		t.Fatalf("wildcard fallback failed; got %+v", got)
	}
}

// TestRobotsFilter_404AllowsAll treats "no robots.txt" as "everything
// allowed" with an explicit rule set (not fail-open).
func TestRobotsFilter_404AllowsAll(t *testing.T) {
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{{URL: "https://example.com/any"}}
	if got := f.Filter(links); len(got) != 1 {
		t.Fatalf("404 should pass all; got %d", len(got))
	}
}

// TestRobotsFilter_5xxFailOpen: on server errors, the filter must not block.
func TestRobotsFilter_5xxFailOpen(t *testing.T) {
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{{URL: "https://example.com/any"}}
	if got := f.Filter(links); len(got) != 1 {
		t.Fatalf("5xx should fail-open; got %d", len(got))
	}
}

// TestRobotsFilter_TTLRefetch verifies that once 24h has elapsed the filter
// re-fetches robots.txt. The test injects a clock and counts handler hits.
func TestRobotsFilter_TTLRefetch(t *testing.T) {
	var hits int32
	var mu sync.Mutex
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /x/\n"))
	})
	defer ts.Close()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := filterForTestServer(ts, nil)
	f.now = func() time.Time { return now }

	links := []crawler.Link{{URL: "https://example.com/ok"}}
	_ = f.Filter(links) // first fetch
	_ = f.Filter(links) // within TTL: cache hit
	now = now.Add(25 * time.Hour)
	_ = f.Filter(links) // TTL expired: re-fetch

	mu.Lock()
	defer mu.Unlock()
	if hits != 2 {
		t.Fatalf("expected 2 fetches (initial + TTL expiry); got %d", hits)
	}
}

// TestRobotsFilter_CrawlDelayInvokesRateSetter ensures Crawl-delay: N is
// translated to SetHostRate(host, 1/N, 1) and not repeated while the cache
// entry is still valid.
func TestRobotsFilter_CrawlDelayInvokesRateSetter(t *testing.T) {
	body := "User-agent: *\nCrawl-delay: 5\n"
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	setter := &fakeRateSetter{}
	f := filterForTestServer(ts, setter)

	links := []crawler.Link{{URL: "https://example.com/a"}, {URL: "https://example.com/b"}}
	_ = f.Filter(links) // triggers fetch + SetHostRate
	_ = f.Filter(links) // cache hit: must not invoke SetHostRate again

	if setter.callCount() != 1 {
		t.Fatalf("expected SetHostRate once; got %d calls", setter.callCount())
	}
	call := setter.calls[0]
	if call.burst != 1 {
		t.Errorf("burst = %d, want 1", call.burst)
	}
	if diff := call.rate - 0.2; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("rate = %v, want 0.2", call.rate)
	}
}

// TestRobotsFilter_PrefixSemantics documents that Disallow uses plain string
// prefix matching per the spec's simplified RFC 9309 subset. Side effect: a
// short Disallow prefix may match longer unrelated paths with the same
// prefix (e.g. `Disallow: /a` blocks `/about`). Operators should publish
// trailing slashes on directory rules to avoid surprise matches.
func TestRobotsFilter_PrefixSemantics(t *testing.T) {
	body := "User-agent: *\nDisallow: /a\n"
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{
		{URL: "https://example.com/about"},    // blocked: shares the `/a` prefix
		{URL: "https://example.com/articles"}, // blocked: shares the `/a` prefix
		{URL: "https://example.com/b"},        // passes
	}
	got := f.Filter(links)
	if len(got) != 1 || got[0].URL != "https://example.com/b" {
		t.Fatalf("prefix semantics: expected only /b to pass; got %+v", got)
	}
}

// TestRobotsFilter_MultiUAGroup verifies that consecutive User-agent lines
// form a single group per RFC 9309 — Disallow lines under the group apply to
// both preferred (FugueBot) and wildcard (*) blocks. Preferred still wins when
// resolving rules for FugueBot.
func TestRobotsFilter_MultiUAGroup(t *testing.T) {
	body := "User-agent: FugueBot\nUser-agent: *\nDisallow: /shared/\n"
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{
		{URL: "https://example.com/shared/x"},
		{URL: "https://example.com/ok"},
	}
	got := f.Filter(links)
	if len(got) != 1 || got[0].URL != "https://example.com/ok" {
		t.Fatalf("multi-UA group: expected /ok to pass; got %+v", got)
	}
}

// TestRobotsFilter_TTLBoundary pins the inclusive-boundary semantics: exactly
// 24h after the fetch, the cache entry is still considered fresh; at 24h+1ns,
// a re-fetch is required.
func TestRobotsFilter_TTLBoundary(t *testing.T) {
	var hits int
	var mu sync.Mutex
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		_, _ = w.Write([]byte("User-agent: *\n"))
	})
	defer ts.Close()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := filterForTestServer(ts, nil)
	f.now = func() time.Time { return now }

	links := []crawler.Link{{URL: "https://example.com/x"}}
	_ = f.Filter(links) // first fetch

	now = now.Add(24 * time.Hour) // exactly at TTL: still fresh
	_ = f.Filter(links)
	mu.Lock()
	if hits != 1 {
		mu.Unlock()
		t.Fatalf("at exactly 24h TTL the entry must still be fresh; hits=%d", hits)
	}
	mu.Unlock()

	now = now.Add(1 * time.Nanosecond) // 24h + 1ns: must re-fetch
	_ = f.Filter(links)
	mu.Lock()
	defer mu.Unlock()
	if hits != 2 {
		t.Fatalf("past TTL boundary must trigger re-fetch; hits=%d", hits)
	}
}

// TestRobotsFilter_InvalidCrawlDelayIgnored: non-numeric Crawl-delay values
// are dropped and never reach the scheduler.
func TestRobotsFilter_InvalidCrawlDelayIgnored(t *testing.T) {
	body := "User-agent: *\nCrawl-delay: soon\n"
	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	setter := &fakeRateSetter{}
	f := filterForTestServer(ts, setter)
	_ = f.Filter([]crawler.Link{{URL: "https://example.com/a"}})
	if setter.callCount() != 0 {
		t.Fatalf("invalid Crawl-delay must not trigger SetHostRate; got %d calls", setter.callCount())
	}
}
