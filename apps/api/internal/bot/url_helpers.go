package bot

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strings"
)

// urlPathContains checks if a URL path contains the given segment as a whole path component.
// Uses boundary-aware matching: /photos/ matches but "hot" inside "photos" does not match "hot".
func urlPathContains(urlPath string, segment string) bool {
	return strings.Contains(urlPath, "/"+segment+"/") ||
		strings.HasSuffix(urlPath, "/"+segment)
}

// hasExcludedExtension returns true for URLs ending in image/media/document/asset extensions
// that should not be crawled by Pioneer.
func hasExcludedExtension(urlStr string) bool {
	lower := strings.ToLower(urlStr)

	excluded := []string{
		// Images
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico",
		// Media
		".mp3", ".mp4", ".wav", ".webm", ".avi", ".mov",
		// Documents
		".pdf", ".zip", ".tar", ".gz", ".exe", ".dmg",
		// Static assets
		".css", ".js", ".json", ".xml", ".woff", ".woff2", ".ttf",
	}

	for _, ext := range excluded {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// templatePath normalizes a URL to a page template pattern for node deduplication.
// 1. Strips query parameters and fragment
// 2. Replaces pure-numeric path segments with {id}
func templatePath(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	segments := strings.Split(u.Path, "/")
	for i, seg := range segments {
		if seg != "" && isNumeric(seg) {
			segments[i] = "{id}"
		}
	}
	u.Path = strings.Join(segments, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// isNumeric returns true if s consists entirely of digits.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// hashURL hashes a URL for deduplication, using its template path so that
// equivalent pages collapse to the same key.
func hashURL(urlStr string) string {
	h := md5.Sum([]byte(templatePath(urlStr)))
	return fmt.Sprintf("%x", h)
}
