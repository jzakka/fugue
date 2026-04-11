package crawler

import (
	"net/url"
	"path/filepath"
	"strings"
)

// normalizeURL normalizes a URL by:
// - Removing fragment (#anchor)
// - Normalizing trailing slashes
// - Converting to lowercase hostname
func normalizeURL(urlStr string) (string, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// Remove fragment
	parsed.Fragment = ""

	// Normalize trailing slash in path
	// /path and /path/ should be treated as the same
	if parsed.Path != "/" && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}

	// Lowercase hostname for consistent comparison
	parsed.Host = strings.ToLower(parsed.Host)

	return parsed.String(), nil
}

// isSameDomain checks if targetURL belongs to the same domain as baseURL.
// Subdomains are considered different domains.
// Protocol differences (http vs https) are ignored.
func isSameDomain(baseURL, targetURL string) bool {
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		return false
	}

	baseHost := strings.ToLower(strings.Split(base.Host, ":")[0])
	targetHost := strings.ToLower(strings.Split(target.Host, ":")[0])

	// Remove www. prefix for comparison
	baseHost = strings.TrimPrefix(baseHost, "www.")
	targetHost = strings.TrimPrefix(targetHost, "www.")

	return baseHost == targetHost
}

// makeAbsoluteURL converts a relative URL to absolute based on the base URL.
func makeAbsoluteURL(baseURL, relativeURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	rel, err := url.Parse(relativeURL)
	if err != nil {
		return "", err
	}

	return base.ResolveReference(rel).String(), nil
}

// shouldSkipURL returns true if the URL should be skipped based on file extension.
var skipExtensions = []string{
	".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg",
	".mp4", ".webm", ".avi", ".mov",
	".mp3", ".wav", ".ogg", ".flac",
	".pdf", ".doc", ".docx", ".xls", ".xlsx",
	".css", ".js", ".json", ".xml",
	".zip", ".tar", ".gz",
}

func shouldSkipURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return true
	}

	path := strings.ToLower(parsed.Path)
	ext := filepath.Ext(path)

	for _, skipExt := range skipExtensions {
		if ext == skipExt {
			return true
		}
	}

	return false
}

// isHTMLContent checks if the content type indicates HTML.
func isHTMLContent(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml")
}
