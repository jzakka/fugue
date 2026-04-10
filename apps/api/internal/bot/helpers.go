package bot

import (
	"database/sql"
	"net/url"
	"regexp"
	"strings"
)

// toNullString converts a plain string to sql.NullString.
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// parseLinks extracts all links from HTML
func parseLinks(html, baseURL string) []string {
	// Simple regex-based link extraction
	// In production, use a proper HTML parser
	re := regexp.MustCompile(`href=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(html, -1)

	var links []string
	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]
			// Convert relative to absolute URLs
			if strings.HasPrefix(link, "/") {
				baseU, err := url.Parse(baseURL)
				if err == nil {
					link = baseU.Scheme + "://" + baseU.Host + link
				}
			}
			links = append(links, link)
		}
	}
	return links
}
