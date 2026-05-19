// Package httpclient provides an SSRF-safe *http.Client factory shared by
// any code that fetches caller-untrusted URLs (e.g. URLs extracted from
// crawled HTML). The dialer resolves every IP, rejects private/reserved
// ranges before connecting, and CheckRedirect re-runs the same check on
// every redirect hop. See openspec/specs/harvester/spec.md for the contract
// this implementation satisfies for the bot pipeline.
package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Options configures NewSSRFSafeClient.
type Options struct {
	ConnectTimeout time.Duration
	TotalTimeout   time.Duration
	MaxRedirects   int
}

// NewSSRFSafeClient returns an *http.Client whose dialer rejects connections
// to private, loopback, link-local, and reserved IP ranges. CheckRedirect
// re-resolves and re-checks each redirect hop, and non-http(s) schemes are
// rejected.
func NewSSRFSafeClient(opts Options) *http.Client {
	connectTimeout := opts.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = 5 * time.Second
	}
	totalTimeout := opts.TotalTimeout
	if totalTimeout <= 0 {
		totalTimeout = 30 * time.Second
	}
	maxRedirects := opts.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 5
	}

	dialer := &net.Dialer{Timeout: connectTimeout}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("httpclient: invalid address %q: %w", addr, err)
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("httpclient: DNS lookup failed for %q: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("httpclient: no IP addresses found for %q", host)
			}
			for _, ip := range ips {
				if IsPrivateIP(ip.IP) {
					return nil, fmt.Errorf("httpclient: blocked private/reserved IP %s for host %q", ip.IP, host)
				}
			}
			target := net.JoinHostPort(ips[0].IP.String(), port)
			return dialer.DialContext(ctx, network, target)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   totalTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("httpclient: too many redirects (max %d)", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("httpclient: blocked redirect to non-http scheme %q", req.URL.Scheme)
			}
			host := req.URL.Hostname()
			ips, err := net.DefaultResolver.LookupIPAddr(req.Context(), host)
			if err != nil {
				return fmt.Errorf("httpclient: DNS lookup failed on redirect to %q: %w", host, err)
			}
			for _, ip := range ips {
				if IsPrivateIP(ip.IP) {
					return fmt.Errorf("httpclient: blocked redirect to private IP %s for host %q", ip.IP, host)
				}
			}
			return nil
		},
	}
}

// IsPrivateIP reports whether ip falls inside loopback, link-local,
// IPv4/IPv6 private ranges, or the reserved ranges enumerated below.
// Callers that build their own dialer can reuse this single source of
// truth so the SSRF policy stays consistent across packages.
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsUnspecified() {
		return true
	}

	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",   // Carrier-grade NAT
		"198.18.0.0/15",   // Benchmarking
		"192.0.0.0/24",    // IETF Protocol Assignments
		"192.0.2.0/24",    // Documentation (TEST-NET-1)
		"198.51.100.0/24", // Documentation (TEST-NET-2)
		"203.0.113.0/24",  // Documentation (TEST-NET-3)
		"fc00::/7",        // IPv6 unique local
	}
	for _, c := range privateCIDRs {
		_, cidr, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
