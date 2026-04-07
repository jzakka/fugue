package og

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// OGResult holds the parsed OpenGraph metadata from a URL.
type OGResult struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Image         string `json:"image"`
	SiteName      string `json:"site_name"`
	URL           string `json:"url"`
	DetectedField string `json:"detected_field"`
}

// domainFieldMap maps known domains to creative fields.
var domainFieldMap = map[string]string{
	"soundcloud.com": "음악",
	"spotify.com":    "음악",
	"bandcamp.com":   "음악",
	"pixiv.net":      "미술",
	"artstation.com": "미술",
	"deviantart.com": "미술",
	"youtube.com":    "영상편집",
	"vimeo.com":      "영상편집",
	"github.com":     "프로그래밍",
	"codepen.io":     "프로그래밍",
	"medium.com":     "글",
	"brunch.co.kr":   "글",
}

const (
	maxResponseBytes = 1 << 20 // 1 MB
	maxRedirects     = 5
	connectTimeout   = 3 * time.Second
	totalTimeout     = 5 * time.Second
)

// Service fetches and parses OpenGraph metadata from URLs with SSRF protection.
type Service struct {
	client *http.Client
}

// NewService creates a new OG fetching service with SSRF-safe HTTP client.
func NewService() *Service {
	dialer := &net.Dialer{
		Timeout: connectTimeout,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("og: invalid address %q: %w", addr, err)
			}

			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("og: DNS lookup failed for %q: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("og: no IP addresses found for %q", host)
			}

			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("og: blocked private/reserved IP %s for host %q", ip.IP, host)
				}
			}

			// Connect to the first resolved IP.
			target := net.JoinHostPort(ips[0].IP.String(), port)
			return dialer.DialContext(ctx, network, target)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   totalTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("og: too many redirects (max %d)", maxRedirects)
			}

			// Validate scheme on every redirect hop.
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("og: blocked redirect to non-http scheme %q", req.URL.Scheme)
			}

			// Re-resolve and verify IP on each redirect hop.
			host := req.URL.Hostname()
			ips, err := net.DefaultResolver.LookupIPAddr(req.Context(), host)
			if err != nil {
				return fmt.Errorf("og: DNS lookup failed on redirect to %q: %w", host, err)
			}
			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return fmt.Errorf("og: blocked redirect to private IP %s for host %q", ip.IP, host)
				}
			}
			return nil
		},
	}

	return &Service{client: client}
}

// Fetch retrieves and parses OG metadata from the given URL.
func (s *Service) Fetch(ctx context.Context, rawURL string) (*OGResult, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("og: invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("og: only http/https schemes allowed, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("og: missing host in URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("og: failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "FugueBot/1.0 (+https://fugue.app)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("og: fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("og: upstream returned HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxResponseBytes)

	result := parseHTML(limited)
	result.URL = rawURL
	result.DetectedField = detectField(parsed.Hostname())

	return result, nil
}

// parseHTML extracts OG, Twitter, and fallback HTML metadata from the response body.
func parseHTML(r io.Reader) *OGResult {
	result := &OGResult{}
	tokenizer := html.NewTokenizer(r)

	var htmlTitle string
	var inTitle bool

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			// EOF or read error — finalize and return.
			applyFallbacks(result, htmlTitle)
			return result

		case html.StartTagToken, html.SelfClosingTagToken:
			tn, hasAttr := tokenizer.TagName()
			tagName := string(tn)

			if tagName == "title" && tt == html.StartTagToken {
				inTitle = true
				continue
			}

			// Stop parsing at <body> — OG tags live in <head>.
			if tagName == "body" {
				applyFallbacks(result, htmlTitle)
				return result
			}

			if tagName == "meta" && hasAttr {
				parseMeta(tokenizer, result)
			}

		case html.TextToken:
			if inTitle {
				htmlTitle += string(tokenizer.Text())
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			if string(tn) == "title" {
				inTitle = false
			}
			if string(tn) == "head" {
				applyFallbacks(result, htmlTitle)
				return result
			}
		}
	}
}

// parseMeta extracts relevant attributes from a <meta> tag.
func parseMeta(tokenizer *html.Tokenizer, result *OGResult) {
	var property, name, content string

	for {
		key, val, more := tokenizer.TagAttr()
		k := string(key)
		v := string(val)

		switch k {
		case "property":
			property = v
		case "name":
			name = v
		case "content":
			content = v
		}
		if !more {
			break
		}
	}

	if content == "" {
		return
	}

	// OG tags (highest priority).
	switch property {
	case "og:title":
		if result.Title == "" {
			result.Title = content
		}
	case "og:description":
		if result.Description == "" {
			result.Description = content
		}
	case "og:image":
		if result.Image == "" {
			result.Image = content
		}
	case "og:site_name":
		if result.SiteName == "" {
			result.SiteName = content
		}
	}

	// Twitter cards (fallback if OG is missing).
	switch name {
	case "twitter:title":
		if result.Title == "" {
			result.Title = content
		}
	case "twitter:description":
		if result.Description == "" {
			result.Description = content
		}
	case "twitter:image":
		if result.Image == "" {
			result.Image = content
		}
	}

	// Standard HTML meta description (lowest priority fallback).
	if name == "description" && result.Description == "" {
		result.Description = content
	}
}

// applyFallbacks fills in missing fields from the HTML <title>.
func applyFallbacks(result *OGResult, htmlTitle string) {
	title := strings.TrimSpace(htmlTitle)
	if result.Title == "" && title != "" {
		result.Title = title
	}
}

// detectField returns a creative field name based on the URL's domain.
func detectField(hostname string) string {
	lower := strings.ToLower(hostname)

	for domain, field := range domainFieldMap {
		if lower == domain || strings.HasSuffix(lower, "."+domain) {
			return field
		}
	}
	return ""
}

// isPrivateIP returns true if the IP is in a private, loopback, or link-local range.
func isPrivateIP(ip net.IP) bool {
	// Loopback: 127.0.0.0/8 and ::1
	if ip.IsLoopback() {
		return true
	}
	// Link-local: 169.254.0.0/16 and fe80::/10
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Unspecified: 0.0.0.0 and ::
	if ip.IsUnspecified() {
		return true
	}

	// Private IPv4 ranges.
	privateRanges := []struct {
		network string
	}{
		{"10.0.0.0/8"},
		{"172.16.0.0/12"},
		{"192.168.0.0/16"},
		{"100.64.0.0/10"},   // Carrier-grade NAT
		{"198.18.0.0/15"},   // Benchmarking
		{"192.0.0.0/24"},    // IETF Protocol Assignments
		{"192.0.2.0/24"},    // Documentation (TEST-NET-1)
		{"198.51.100.0/24"}, // Documentation (TEST-NET-2)
		{"203.0.113.0/24"},  // Documentation (TEST-NET-3)
		{"fc00::/7"},        // IPv6 unique local
	}

	for _, r := range privateRanges {
		_, cidr, err := net.ParseCIDR(r.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}
