package httpclient

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
		name string
	}{
		{"127.0.0.1", true, "loopback IPv4"},
		{"::1", true, "loopback IPv6"},
		{"169.254.169.254", true, "AWS IMDS link-local"},
		{"169.254.0.1", true, "link-local IPv4"},
		{"fe80::1", true, "link-local IPv6"},
		{"0.0.0.0", true, "unspecified IPv4"},
		{"::", true, "unspecified IPv6"},
		{"10.0.0.1", true, "private 10/8"},
		{"172.16.0.1", true, "private 172.16/12"},
		{"172.31.255.254", true, "private 172.16/12 upper"},
		{"192.168.1.1", true, "private 192.168/16"},
		{"100.64.0.1", true, "carrier-grade NAT"},
		{"198.18.0.1", true, "benchmarking"},
		{"192.0.2.1", true, "TEST-NET-1"},
		{"198.51.100.1", true, "TEST-NET-2"},
		{"203.0.113.1", true, "TEST-NET-3"},
		{"fc00::1", true, "IPv6 ULA"},
		{"fd00::1", true, "IPv6 ULA upper half"},
		{"8.8.8.8", false, "public Google DNS"},
		{"1.1.1.1", false, "public Cloudflare DNS"},
		{"172.32.0.1", false, "outside 172.16/12"},
		{"2606:4700:4700::1111", false, "public Cloudflare IPv6"},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("%s: parse failed for %q", c.name, c.ip)
		}
		got := IsPrivateIP(ip)
		if got != c.want {
			t.Errorf("%s: IsPrivateIP(%s) = %v, want %v", c.name, c.ip, got, c.want)
		}
	}
}

// TestNewSSRFSafeClient_BlocksLoopback verifies the dialer refuses to connect
// to a httptest server bound to 127.0.0.1. The error message must indicate
// the IP block (not a timeout or generic dial failure) so callers can
// distinguish SSRF rejection from transient errors.
func TestNewSSRFSafeClient_BlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server handler ran — SSRF block did not fire")
	}))
	defer srv.Close()

	c := NewSSRFSafeClient(Options{
		ConnectTimeout: 2 * time.Second,
		TotalTimeout:   2 * time.Second,
		MaxRedirects:   3,
	})

	_, err := c.Get(srv.URL)
	if err == nil {
		t.Fatal("expected error when fetching loopback URL, got nil")
	}
	if !strings.Contains(err.Error(), "blocked private/reserved IP") {
		t.Errorf("error missing SSRF block marker: %v", err)
	}
}

// TestNewSSRFSafeClient_BlocksRedirectToPrivate verifies CheckRedirect
// refuses to follow a 302 whose Location resolves to a private IP, even
// when the initial host is public. We simulate this with a httptest server
// (loopback IP itself triggers the same path).
func TestNewSSRFSafeClient_BlocksRedirectToPrivate(t *testing.T) {
	// First server returns 302 → second server (also loopback). The first
	// dial itself is blocked because httptest binds to 127.0.0.1, so for
	// this case we verify the CheckRedirect path with a public-IP indirect
	// stub: skipped if env not available. Instead, we exercise CheckRedirect
	// directly via a constructed request.
	c := NewSSRFSafeClient(Options{
		ConnectTimeout: 2 * time.Second,
		TotalTimeout:   2 * time.Second,
		MaxRedirects:   3,
	})

	// CheckRedirect signature: func(req *http.Request, via []*http.Request) error
	req, _ := http.NewRequest("GET", "http://127.0.0.1/x", nil)
	err := c.CheckRedirect(req, nil)
	if err == nil {
		t.Fatal("expected CheckRedirect to reject 127.0.0.1, got nil")
	}
	if !strings.Contains(err.Error(), "blocked redirect to private IP") {
		t.Errorf("CheckRedirect error missing block marker: %v", err)
	}
}

func TestNewSSRFSafeClient_BlocksNonHTTPRedirect(t *testing.T) {
	c := NewSSRFSafeClient(Options{
		ConnectTimeout: 2 * time.Second,
		TotalTimeout:   2 * time.Second,
		MaxRedirects:   3,
	})

	req, _ := http.NewRequest("GET", "file:///etc/passwd", nil)
	err := c.CheckRedirect(req, nil)
	if err == nil {
		t.Fatal("expected CheckRedirect to reject file:// redirect, got nil")
	}
	if !strings.Contains(err.Error(), "non-http scheme") {
		t.Errorf("CheckRedirect error missing scheme marker: %v", err)
	}
}

func TestNewSSRFSafeClient_TooManyRedirects(t *testing.T) {
	c := NewSSRFSafeClient(Options{
		ConnectTimeout: 2 * time.Second,
		TotalTimeout:   2 * time.Second,
		MaxRedirects:   2,
	})

	via := []*http.Request{{}, {}}
	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	err := c.CheckRedirect(req, via)
	if err == nil {
		t.Fatal("expected CheckRedirect to enforce maxRedirects, got nil")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("CheckRedirect error missing too-many marker: %v", err)
	}
}

// TestNewSSRFSafeClient_TotalTimeout uses a slow server bound to a public
// hostname via a custom resolver substitute is complex; instead we test the
// Timeout field is set on the returned client so its enforcement is left to
// net/http itself (well-tested).
func TestNewSSRFSafeClient_TimeoutConfigured(t *testing.T) {
	c := NewSSRFSafeClient(Options{
		ConnectTimeout: 1 * time.Second,
		TotalTimeout:   3 * time.Second,
		MaxRedirects:   5,
	})
	if c.Timeout != 3*time.Second {
		t.Errorf("client.Timeout = %v, want 3s", c.Timeout)
	}
}

// TestNewSSRFSafeClient_Defaults verifies zero-value Options falls back to
// the package defaults rather than panicking or producing a non-functional
// client.
func TestNewSSRFSafeClient_Defaults(t *testing.T) {
	c := NewSSRFSafeClient(Options{})
	if c.Timeout == 0 {
		t.Error("default Timeout must be non-zero")
	}
	if c.Transport == nil {
		t.Error("Transport must be set")
	}
	if c.CheckRedirect == nil {
		t.Error("CheckRedirect must be set")
	}
}

// TestNewSSRFSafeClient_BlocksLinkLocalDial verifies the dialer explicitly
// rejects 169.254.169.254 (AWS IMDS). We can't actually dial it in a unit
// test, but we can verify the rejection by calling the Transport's
// DialContext directly through a manual setup; instead we use a httptest
// server on loopback (already covered) plus the IsPrivateIP table test
// asserts 169.254.169.254 is in the blocked set. Together these prove the
// IMDS-blocking property: the dialer calls IsPrivateIP on every resolved IP
// and 169.254.169.254 returns true.
func TestNewSSRFSafeClient_LinkLocalCoverage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Manual DialContext invocation on a literal link-local address.
	c := NewSSRFSafeClient(Options{ConnectTimeout: 1 * time.Second, TotalTimeout: 2 * time.Second, MaxRedirects: 3})
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.Transport)
	}
	_, err := tr.DialContext(ctx, "tcp", "169.254.169.254:80")
	if err == nil {
		t.Fatal("expected dial to 169.254.169.254 to be blocked, got nil error")
	}
	if !strings.Contains(err.Error(), "blocked private/reserved IP") {
		t.Errorf("DialContext error missing block marker: %v", err)
	}
}
