package bot

import (
	"bytes"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// PickPrimaryImage extracts a Pin's primary image candidate URL from the page HTML.
// The extraction follows a fixed priority:
//  1. og:image
//  2. twitter:image
//  3. meaningful <img> inside <article>/<main>
//  4. JSON-LD "image" field
//
// The returned URL is resolved to an absolute URL against pageURL. If no valid
// candidate is found, it returns an empty string.
func PickPrimaryImage(htmlBytes []byte, pageURL string) string {
	base, _ := url.Parse(pageURL)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return ""
	}

	// 1) og:image
	if v := pickOGImage(doc); v != "" {
		if abs := resolveAndValidate(v, base); abs != "" {
			return abs
		}
	}

	// 2) twitter:image
	if v := pickTwitterImage(doc); v != "" {
		if abs := resolveAndValidate(v, base); abs != "" {
			return abs
		}
	}

	// 3) article/main <img>
	if v := pickArticleImage(doc); v != "" {
		if abs := resolveAndValidate(v, base); abs != "" {
			return abs
		}
	}

	// 4) JSON-LD image
	for _, v := range pickJSONLDImages(doc) {
		if abs := resolveAndValidate(v, base); abs != "" {
			return abs
		}
	}

	return ""
}

func pickOGImage(doc *goquery.Document) string {
	var out string
	doc.Find(`meta[property="og:image"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if v, ok := s.Attr("content"); ok && strings.TrimSpace(v) != "" {
			out = strings.TrimSpace(v)
			return false
		}
		return true
	})
	return out
}

func pickTwitterImage(doc *goquery.Document) string {
	var out string
	// Support both name= and property= variants
	doc.Find(`meta[name="twitter:image"], meta[property="twitter:image"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if v, ok := s.Attr("content"); ok && strings.TrimSpace(v) != "" {
			out = strings.TrimSpace(v)
			return false
		}
		return true
	})
	return out
}

// pickArticleImage returns the src of the first meaningful <img> inside
// <article> or <main>. "Meaningful" means either width & height are both >= 100
// OR alt is a non-empty string.
func pickArticleImage(doc *goquery.Document) string {
	var out string
	doc.Find(`article img, main img`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src, ok := s.Attr("src")
		if !ok || strings.TrimSpace(src) == "" {
			return true
		}

		if !imgIsMeaningful(s) {
			return true
		}
		out = strings.TrimSpace(src)
		return false
	})
	return out
}

func imgIsMeaningful(s *goquery.Selection) bool {
	// Heuristic A: width/height both >= 100
	w := attrInt(s, "width")
	h := attrInt(s, "height")
	if w >= 100 && h >= 100 {
		return true
	}
	// Heuristic B: non-empty alt
	if alt, ok := s.Attr("alt"); ok && strings.TrimSpace(alt) != "" {
		return true
	}
	return false
}

func attrInt(s *goquery.Selection, name string) int {
	v, ok := s.Attr(name)
	if !ok {
		return 0
	}
	v = strings.TrimSpace(v)
	// Drop "px" or "%" suffixes
	v = strings.TrimSuffix(v, "px")
	if strings.HasSuffix(v, "%") {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// pickJSONLDImages returns flattened image URLs from all <script type="application/ld+json"> blocks.
func pickJSONLDImages(doc *goquery.Document) []string {
	var out []string
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		out = append(out, flattenJSONLDImage(raw)...)
	})
	return out
}

// flattenJSONLDImage parses a JSON-LD block and extracts "image" fields.
// It handles these shapes:
//
//	"image": "https://..."
//	"image": ["https://...", {"url": "..."}]
//	"image": {"url": "https://..."}
//
// Top-level may also be an array of objects.
func flattenJSONLDImage(raw string) []string {
	var top any
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return nil
	}
	var out []string
	walkJSONLD(top, &out)
	return out
}

func walkJSONLD(node any, out *[]string) {
	switch v := node.(type) {
	case map[string]any:
		// image field is collected first (priority), then nested objects are
		// recursed in a deterministic (alphabetical) key order so repeated
		// runs yield identical candidate ordering. Go map range is randomized
		// otherwise.
		if img, ok := v["image"]; ok {
			collectImageField(img, out)
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			if k == "image" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkJSONLD(v[k], out)
		}
	case []any:
		for _, child := range v {
			walkJSONLD(child, out)
		}
	}
}

func collectImageField(node any, out *[]string) {
	switch v := node.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			*out = append(*out, s)
		}
	case []any:
		for _, child := range v {
			collectImageField(child, out)
		}
	case map[string]any:
		if u, ok := v["url"].(string); ok && strings.TrimSpace(u) != "" {
			*out = append(*out, strings.TrimSpace(u))
		}
	}
}

// resolveAndValidate resolves href against base and enforces:
//   - http or https scheme only
//   - data: URI rejected
//   - tracking pixel patterns rejected ("pixel", "1x1", "spacer" substrings)
//
// Returns the absolute URL string on success, empty string on rejection.
func resolveAndValidate(href string, base *url.URL) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	// Reject data: URIs up front
	if strings.HasPrefix(strings.ToLower(href), "data:") {
		return ""
	}

	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base != nil && !u.IsAbs() {
		u = base.ResolveReference(u)
	}
	if !u.IsAbs() {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	if isTrackingPixel(u) {
		return ""
	}
	return u.String()
}

// isTrackingPixel checks if a URL contains suspicious tracking pixel patterns
// in its path or filename: "pixel", "1x1", "spacer".
func isTrackingPixel(u *url.URL) bool {
	lower := strings.ToLower(u.Path)
	if strings.Contains(lower, "pixel") || strings.Contains(lower, "1x1") || strings.Contains(lower, "spacer") {
		return true
	}
	return false
}
