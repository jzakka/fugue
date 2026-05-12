package bot

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// PickPrimaryImage extracts the primary image URL from HTML bytes using a 4-level
// priority: og:image → twitter:image → article/main <img> → JSON-LD image.
// Returns the first valid absolute URL, or "" if none is found.
// The pageURL is used to resolve relative URLs.
func PickPrimaryImage(htmlBytes []byte, pageURL string) string {
	if len(htmlBytes) == 0 {
		return ""
	}

	base, err := url.Parse(pageURL)
	if err != nil {
		base = nil
	}

	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return ""
	}

	// Collect candidates in DOM order per priority bucket.
	var ogImages []string
	var twitterImages []string
	var articleImgs []articleImgCandidate
	var jsonLDImages []string

	var walk func(n *html.Node, inArticle bool)
	walk = func(n *html.Node, inArticle bool) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "meta":
				property := getAttr(n, "property")
				name := getAttr(n, "name")
				content := getAttr(n, "content")
				if content == "" {
					break
				}
				if strings.EqualFold(property, "og:image") {
					ogImages = append(ogImages, content)
				}
				if strings.EqualFold(name, "twitter:image") || strings.EqualFold(property, "twitter:image") {
					twitterImages = append(twitterImages, content)
				}
			case "article", "main":
				// Recurse with inArticle=true.
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c, true)
				}
				return
			case "img":
				if inArticle {
					if cand, ok := buildArticleImgCandidate(n); ok {
						articleImgs = append(articleImgs, cand)
					}
				}
			case "script":
				if strings.EqualFold(getAttr(n, "type"), "application/ld+json") {
					txt := textContent(n)
					jsonLDImages = append(jsonLDImages, extractJSONLDImages(txt)...)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inArticle)
		}
	}
	walk(doc, false)

	// Priority 1: og:image
	for _, raw := range ogImages {
		if valid, ok := validateCandidate(raw, base, false, 0, 0); ok {
			return valid
		}
	}
	// Priority 2: twitter:image
	for _, raw := range twitterImages {
		if valid, ok := validateCandidate(raw, base, false, 0, 0); ok {
			return valid
		}
	}
	// Priority 3: article/main <img>
	for _, c := range articleImgs {
		if valid, ok := validateCandidate(c.src, base, true, c.width, c.height); ok {
			return valid
		}
	}
	// Priority 4: JSON-LD image
	for _, raw := range jsonLDImages {
		if valid, ok := validateCandidate(raw, base, false, 0, 0); ok {
			return valid
		}
	}
	return ""
}

type articleImgCandidate struct {
	src    string
	width  int
	height int
	alt    string
}

// buildArticleImgCandidate constructs a candidate from an <img> node, returning
// ok=false if it fails the "meaningful img" criteria (Decision 11): width and
// height both ≥ 100, OR non-empty alt.
func buildArticleImgCandidate(n *html.Node) (articleImgCandidate, bool) {
	src := getAttr(n, "src")
	if src == "" {
		return articleImgCandidate{}, false
	}
	w := parseIntAttr(getAttr(n, "width"))
	h := parseIntAttr(getAttr(n, "height"))
	alt := strings.TrimSpace(getAttr(n, "alt"))

	meaningful := (w >= 100 && h >= 100) || alt != ""
	if !meaningful {
		return articleImgCandidate{}, false
	}
	return articleImgCandidate{src: src, width: w, height: h, alt: alt}, true
}

// validateCandidate applies the validity rules:
//   - Resolves to absolute URL (via base)
//   - Scheme must be http or https
//   - Rejects data: URIs
//   - Rejects 1×1 tracking pixel patterns (filename contains pixel/1x1/spacer)
//   - For <img>-sourced candidates (isImgTag=true), also rejects when both
//     width and height are ≤ 1
//
// Returns (absoluteURL, true) if valid, ("", false) otherwise.
func validateCandidate(raw string, base *url.URL, isImgTag bool, width, height int) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if !u.IsAbs() {
		if base == nil {
			return "", false
		}
		u = base.ResolveReference(u)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	// 1×1 tracking pixel heuristic (Decision 10): filename keyword match.
	lastSeg := lastPathSegment(u.Path)
	lowerSeg := strings.ToLower(lastSeg)
	for _, kw := range []string{"pixel", "1x1", "spacer"} {
		if strings.Contains(lowerSeg, kw) {
			return "", false
		}
	}
	// 1×1 tracking pixel heuristic: width/height both ≤ 1 for <img> candidates.
	if isImgTag && width > 0 && height > 0 && width <= 1 && height <= 1 {
		return "", false
	}
	return u.String(), true
}

func lastPathSegment(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// parseIntAttr extracts a non-negative integer from an HTML size attribute
// value such as "600", "600px", or " 600 ". Leading whitespace is trimmed,
// then the longest leading run of ASCII digits is parsed. Returns 0 for
// empty, leading-non-digit, or over-large values. Negative signs and
// multi-byte characters cause the function to return 0 (they cannot appear
// in a valid HTML width/height attribute).
func parseIntAttr(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0
	}
	return n
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		if x == nil {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return b.String()
}

// extractJSONLDImages parses a JSON-LD script body and returns any image URLs
// found on schema.org-style "image" fields. Handles:
//   - "image": "https://..."
//   - "image": ["https://...", ...]
//   - "image": {"url": "https://...", ...}
//   - top-level array of objects
//   - @graph array of objects
func extractJSONLDImages(body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var any interface{}
	if err := json.Unmarshal([]byte(body), &any); err != nil {
		return nil
	}
	var out []string
	collectJSONLDImages(any, &out)
	return out
}

func collectJSONLDImages(v interface{}, out *[]string) {
	switch t := v.(type) {
	case map[string]interface{}:
		if img, ok := t["image"]; ok {
			appendJSONLDImageField(img, out)
		}
		if g, ok := t["@graph"]; ok {
			collectJSONLDImages(g, out)
		}
	case []interface{}:
		for _, elem := range t {
			collectJSONLDImages(elem, out)
		}
	}
}

func appendJSONLDImageField(v interface{}, out *[]string) {
	switch t := v.(type) {
	case string:
		if t != "" {
			*out = append(*out, t)
		}
	case []interface{}:
		for _, elem := range t {
			appendJSONLDImageField(elem, out)
		}
	case map[string]interface{}:
		if u, ok := t["url"].(string); ok && u != "" {
			*out = append(*out, u)
		}
	}
}
