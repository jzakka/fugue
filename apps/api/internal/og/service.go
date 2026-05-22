package og

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/chungsanghwa/fugue/apps/api/internal/httpclient"
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

// NewService creates a new OG fetching service backed by the shared SSRF-safe
// HTTP client (`apps/api/internal/httpclient`). The shared client is the
// single source of truth for the SSRF policy (CIDR allowlist, DialContext
// IP check, CheckRedirect re-resolve, scheme allowlist) so that any future
// reserved-range additions stay consistent across packages — see the
// `IsPrivateIP` docstring "Callers that build their own dialer can reuse
// this single source of truth so the SSRF policy stays consistent across
// packages." og keeps its tighter domain timeouts (3s connect / 5s total /
// 5 redirects) by passing them as Options; OG pages are short-lived so the
// httpclient default (5s/30s/5) would be too loose for this caller.
func NewService() *Service {
	return &Service{
		client: httpclient.NewSSRFSafeClient(httpclient.Options{
			ConnectTimeout: connectTimeout,
			TotalTimeout:   totalTimeout,
			MaxRedirects:   maxRedirects,
		}),
	}
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
