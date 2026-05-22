package bot

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
)

// TestRobotsFilter_SSRFWiring_RejectsLoopback verifies that the production
// httpClient wired by NewRobotsFilter (httpclient.NewSSRFSafeClient) refuses
// to dial a loopback IP supplied as a caller-untrusted link host. The filter
// must record a fail-open cache entry (filter MUST NOT block the link based
// on a fetch it could not perform) and MUST NOT invoke SetHostRate.
//
// This pins the security contract: an external page planting
// `<a href="http://127.0.0.1:1/...">` cannot use RobotsFilter as an SSRF
// pivot, even though the resulting fail-open semantic still passes the
// link through to subsequent filters.
func TestRobotsFilter_SSRFWiring_RejectsLoopback(t *testing.T) {
	setter := &fakeRateSetter{}
	f := NewRobotsFilter(setter)
	links := []crawler.Link{{URL: "http://127.0.0.1:1/scan-target"}}

	out := f.Filter(links)

	if len(out) != 1 {
		t.Fatalf("loopback host should fail-open (1 link through), got %d", len(out))
	}
	entry, ok := f.cache["127.0.0.1"]
	if !ok {
		t.Fatal("expected cache entry for 127.0.0.1 to be recorded by SSRF reject path")
	}
	if !entry.failOpen {
		t.Errorf("expected failOpen=true on SSRF-reject cache entry")
	}
	if setter.callCount() != 0 {
		t.Errorf("SetHostRate must not be invoked for an SSRF-blocked host; got %d calls", setter.callCount())
	}
}

// TestRobotsFilter_SSRFWiring_RejectsIMDS verifies the same protection for
// AWS EC2 IMDS (169.254.169.254). Link-local block at httpclient layer.
func TestRobotsFilter_SSRFWiring_RejectsIMDS(t *testing.T) {
	setter := &fakeRateSetter{}
	f := NewRobotsFilter(setter)
	links := []crawler.Link{{URL: "http://169.254.169.254/latest/meta-data/iam/security-credentials/foo"}}

	out := f.Filter(links)

	if len(out) != 1 {
		t.Fatalf("IMDS host should fail-open (1 link through), got %d", len(out))
	}
	entry, ok := f.cache["169.254.169.254"]
	if !ok {
		t.Fatal("expected cache entry for 169.254.169.254 to be recorded by SSRF reject path")
	}
	if !entry.failOpen {
		t.Errorf("expected failOpen=true on SSRF-reject cache entry")
	}
	if setter.callCount() != 0 {
		t.Errorf("SetHostRate must not be invoked for an SSRF-blocked host; got %d calls", setter.callCount())
	}
}

// TestRobotsFilter_SSRFWiring_RejectsRFC1918 verifies the same protection
// for an RFC 1918 address (10.x.x.x).
func TestRobotsFilter_SSRFWiring_RejectsRFC1918(t *testing.T) {
	f := NewRobotsFilter(nil)
	links := []crawler.Link{{URL: "http://10.0.0.1/scan"}}

	out := f.Filter(links)

	if len(out) != 1 {
		t.Fatalf("RFC 1918 host should fail-open (1 link through), got %d", len(out))
	}
	entry, ok := f.cache["10.0.0.1"]
	if !ok {
		t.Fatal("expected cache entry for 10.0.0.1 to be recorded by SSRF reject path")
	}
	if !entry.failOpen {
		t.Errorf("expected failOpen=true on SSRF-reject cache entry")
	}
}

// TestRobotsFilter_NewRobotsFilter_WiresSSRFSafeTransport pins the
// constructor-level wiring: NewRobotsFilter MUST install a non-nil custom
// Transport on httpClient (the SSRF-safe Transport installed by
// httpclient.NewSSRFSafeClient). Bare http.Client would have Transport==nil
// at construction (the default Transport is lazy-initialized by stdlib).
// This guards against accidental reversion to `&http.Client{Timeout: ...}`.
func TestRobotsFilter_NewRobotsFilter_WiresSSRFSafeTransport(t *testing.T) {
	f := NewRobotsFilter(nil)
	if f.httpClient == nil {
		t.Fatal("httpClient not wired")
	}
	if f.httpClient.Transport == nil {
		t.Fatal("SSRF-safe client must install a custom Transport (DialContext guard); got nil — likely regressed to bare http.Client")
	}
	if f.httpClient.CheckRedirect == nil {
		t.Fatal("SSRF-safe client must install CheckRedirect to re-run guard on each hop; got nil")
	}
	if f.httpClient.Timeout != robotsFetchTimeout {
		t.Errorf("httpClient.Timeout = %v, want %v", f.httpClient.Timeout, robotsFetchTimeout)
	}
}

// TestRobotsFilter_BodyLimit_TruncatesAtCap verifies that
// io.LimitReader(resp.Body, robotsBodyMaxBytes) silently truncates beyond
// the cap so an oversized robots.txt cannot OOM the crawler. The semantic
// proof: a Disallow rule placed BEFORE the cap participates in matching,
// while one placed AFTER the cap does not.
func TestRobotsFilter_BodyLimit_TruncatesAtCap(t *testing.T) {
	// Build robots.txt with:
	//   - "/before/" Disallow before the cap (definitely parsed)
	//   - >robotsBodyMaxBytes bytes of padding comments
	//   - "/after/" Disallow past the cap (must NOT be parsed)
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Disallow: /before/\n")
	// Padding: each line is "#" + many bytes + "\n" so it gets dropped at parse.
	padLine := "# " + strings.Repeat("x", 1024) + "\n"
	written := b.Len()
	for written < robotsBodyMaxBytes+8*1024 {
		b.WriteString(padLine)
		written += len(padLine)
	}
	b.WriteString("Disallow: /after/\n")
	body := b.String()

	ts := newRobotsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		_, _ = w.Write([]byte(body))
	})
	defer ts.Close()

	f := filterForTestServer(ts, nil)
	links := []crawler.Link{
		{URL: "https://example.com/before/x"}, // must be filtered (parsed)
		{URL: "https://example.com/after/y"},  // must pass (truncated)
		{URL: "https://example.com/unrelated"},
	}
	got := f.Filter(links)

	beforeBlocked := true
	afterBlocked := false
	unrelatedThrough := false
	for _, l := range got {
		switch l.URL {
		case "https://example.com/before/x":
			beforeBlocked = false
		case "https://example.com/after/y":
			afterBlocked = true
		case "https://example.com/unrelated":
			unrelatedThrough = true
		}
	}
	if !beforeBlocked {
		t.Errorf("/before/ Disallow should be parsed (before cap) and block the link")
	}
	if !afterBlocked {
		t.Errorf("/after/ Disallow should be truncated (past cap) and NOT block the link — body limit not enforced?")
	}
	if !unrelatedThrough {
		t.Errorf("unrelated link should pass; got %+v", got)
	}
}

// fakeRateSetter, newRobotsTestServer, filterForTestServer reused from
// robots_filter_test.go (same package).
