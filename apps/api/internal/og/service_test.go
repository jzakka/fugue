package og

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewService_SSRFWiring is the integration-level smoke test that the OG
// fetcher picks up the SSRF-safe transport via httpclient.NewSSRFSafeClient.
// The dialer-level coverage (every blocked CIDR, redirect re-check, timeout
// configuration) lives in apps/api/internal/httpclient/ssrf_test.go — this
// test only verifies the constructor wires the safe client, mirroring
// apps/api/internal/bot/pioneer_consumer_ssrf_test.go.
func TestNewService_SSRFWiring_RejectsIMDS(t *testing.T) {
	s := NewService()
	_, err := s.Fetch(context.Background(),
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/test-role")
	if err == nil {
		t.Fatal("expected error fetching AWS IMDS, got nil")
	}
	if !strings.Contains(err.Error(), "blocked private/reserved IP") {
		t.Errorf("error missing SSRF block marker: %v", err)
	}
}

func TestNewService_SSRFWiring_RejectsLoopback(t *testing.T) {
	s := NewService()
	_, err := s.Fetch(context.Background(), "http://127.0.0.1:1/")
	if err == nil {
		t.Fatal("expected error fetching loopback, got nil")
	}
	if !strings.Contains(err.Error(), "blocked private/reserved IP") {
		t.Errorf("error missing SSRF block marker: %v", err)
	}
}

func TestNewService_SSRFWiring_RejectsPrivateIPv4(t *testing.T) {
	s := NewService()
	_, err := s.Fetch(context.Background(), "http://10.0.0.1/index.html")
	if err == nil {
		t.Fatal("expected error fetching private IPv4, got nil")
	}
	if !strings.Contains(err.Error(), "blocked private/reserved IP") {
		t.Errorf("error missing SSRF block marker: %v", err)
	}
}

// TestNewService_WiresSSRFSafeClient asserts the constructor installs a
// non-nil custom Transport (the SSRF-safe one). This is the constructor-level
// guard that any future refactor of NewService must keep delegating to
// httpclient.NewSSRFSafeClient — symmetric to pioneer_consumer_ssrf_test.go's
// TestNewDefaultConsumerFetcher_WiresSSRFSafeClient.
func TestNewService_WiresSSRFSafeClient(t *testing.T) {
	s := NewService()
	if s == nil {
		t.Fatal("NewService returned nil")
	}
	if s.client == nil {
		t.Fatal("client field not wired")
	}
	if s.client.Transport == nil {
		t.Fatal("SSRF-safe client must install a custom Transport with the IP-checking DialContext; got nil")
	}
}

// TestService_FetchPublicHappy verifies the OG parser still extracts metadata
// when the SSRF guard is bypassed via a hand-built client (httptest binds to
// loopback, which NewService's safe client would refuse). This is the
// regression guard against breaking the OG parse path while swapping the
// transport.
func TestService_FetchPublicHappy(t *testing.T) {
	const body = `<!doctype html><html><head>
<meta property="og:title" content="Hello Title">
<meta property="og:description" content="Hello Desc">
<meta property="og:image" content="https://cdn.example.com/img.jpg">
<meta property="og:site_name" content="ExampleSite">
</head><body>ignored</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := &Service{client: srv.Client()}
	got, err := s.Fetch(context.Background(), srv.URL+"/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Hello Title" {
		t.Errorf("Title=%q, want %q", got.Title, "Hello Title")
	}
	if got.Description != "Hello Desc" {
		t.Errorf("Description=%q, want %q", got.Description, "Hello Desc")
	}
	if got.Image != "https://cdn.example.com/img.jpg" {
		t.Errorf("Image=%q, want %q", got.Image, "https://cdn.example.com/img.jpg")
	}
	if got.SiteName != "ExampleSite" {
		t.Errorf("SiteName=%q, want %q", got.SiteName, "ExampleSite")
	}
	if got.URL != srv.URL+"/page" {
		t.Errorf("URL=%q, want %q", got.URL, srv.URL+"/page")
	}
}
