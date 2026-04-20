package bot

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// MaxMediaCandidates is the default cap on PinDocument.MediaCandidates.
const MaxMediaCandidates = 50

// GenericExtractor turns raw HTML into a single PinDocument using a
// fallback chain over OG, Twitter Card, JSON-LD, <article>, and standard
// HTML tags. It never returns a nil document — failure is signalled by
// returning the zero-value document with an error.
type GenericExtractor struct {
	// MediaCandidatesLimit caps the number of MediaCandidate entries the
	// extractor will return. Zero means use MaxMediaCandidates.
	MediaCandidatesLimit int
}

// NewGenericExtractor returns an extractor configured with defaults.
func NewGenericExtractor() *GenericExtractor {
	return &GenericExtractor{MediaCandidatesLimit: MaxMediaCandidates}
}

// Extract parses the given HTML and returns a PinDocument. fetchURL is the
// URL the Harvester actually fetched and is used to (a) resolve relative
// URLs and (b) reject cross-domain canonical candidates inside the
// canonical_url fallback chain.
//
// Contract: the returned PinDocument is always a usable value; if parsing
// fails entirely a minimal document is returned (CanonicalURL = fetchURL,
// Source = fetchURL) along with the parse error so the Harvester can record
// a Failed stat without panicking.
func (e *GenericExtractor) Extract(htmlBytes []byte, fetchURL string) (PinDocument, error) {
	limit := e.MediaCandidatesLimit
	if limit <= 0 {
		limit = MaxMediaCandidates
	}

	doc := PinDocument{
		CanonicalURL: fetchURL,
		OGData: OGData{
			Source:    fetchURL,
			Extractor: "generic",
		},
	}

	base, _ := url.Parse(fetchURL)

	if len(htmlBytes) == 0 {
		return doc, nil
	}

	root, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return doc, err
	}

	scan := newExtractScan(base)
	scan.walk(root, false)

	// Resolve fields with priority chains.
	doc.Title = pickFirstNonEmpty(scan.ogTitle, scan.twTitle, scan.jsonLDTitle, scan.h1, scan.htmlTitle)
	doc.BodyText = pickFirstNonEmpty(scan.jsonLDBody, scan.articleText, scan.bodyText)
	doc.ThumbnailURL = pickFirstAbsoluteURL(base, scan.ogImage, scan.twImage, scan.jsonLDImage, scan.firstArticleImage)
	doc.Lang = pickFirstNonEmpty(scan.ogLocale, scan.htmlLang)
	doc.Author = pickFirstNonEmpty(scan.ogAuthor, scan.jsonLDAuthor)

	if t := pickFirstTime(scan.ogPublished, scan.timeDatetime, scan.jsonLDPublished); t != nil {
		doc.PublishedAt = t
		doc.OGData.PublishedAt = t
	}
	doc.OGData.Lang = doc.Lang
	doc.OGData.Author = doc.Author

	doc.CanonicalURL = pickCanonicalWithCrossDomainGuard(base, scan.linkCanonical, scan.ogURL, fetchURL)

	mediaCandidates := buildMediaCandidates(base, scan, limit)
	doc.MediaCandidates = mediaCandidates
	doc.OGData.MediaCandidates = mediaCandidates

	return doc, nil
}

// extractScan accumulates raw values discovered while walking the DOM.
// Resolution into final fields happens after the walk completes.
type extractScan struct {
	base *url.URL

	ogTitle       string
	ogImage       string
	ogURL         string
	ogLocale      string
	ogPublished   *time.Time
	ogAuthor      string
	twTitle       string
	twImage       string
	jsonLDTitle   string
	jsonLDBody    string
	jsonLDImage   string
	jsonLDAuthor  string
	jsonLDPublished *time.Time
	htmlTitle     string
	htmlLang      string
	h1            string
	linkCanonical string
	timeDatetime  *time.Time

	articleText       string
	bodyText          string
	firstArticleImage string

	mediaImages []MediaCandidate
	mediaVideos []MediaCandidate
	mediaAudios []MediaCandidate

	// State for body-text and density tracking.
	inArticle        bool
	sawArticle       bool
	articleTextBuf   strings.Builder
	bodyTextBuf      strings.Builder
}

func newExtractScan(base *url.URL) *extractScan {
	return &extractScan{base: base}
}

func (s *extractScan) walk(n *html.Node, inArticle bool) {
	if n == nil {
		return
	}

	if n.Type == html.ElementNode {
		switch strings.ToLower(n.Data) {
		case "html":
			if v := getAttr(n, "lang"); v != "" && s.htmlLang == "" {
				s.htmlLang = v
			}
		case "title":
			if s.htmlTitle == "" {
				s.htmlTitle = strings.TrimSpace(textContent(n))
			}
		case "meta":
			s.handleMeta(n)
		case "link":
			if rel := getAttr(n, "rel"); strings.EqualFold(rel, "canonical") {
				if href := getAttr(n, "href"); href != "" && s.linkCanonical == "" {
					s.linkCanonical = href
				}
			}
		case "h1":
			if s.h1 == "" {
				if txt := strings.TrimSpace(textContent(n)); txt != "" {
					s.h1 = txt
				}
			}
		case "time":
			if dt := getAttr(n, "datetime"); dt != "" && s.timeDatetime == nil {
				if t := parseTime(dt); t != nil {
					s.timeDatetime = t
				}
			}
		case "article":
			s.sawArticle = true
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				s.walk(c, true)
			}
			if s.articleText == "" {
				s.articleText = strings.TrimSpace(s.articleTextBuf.String())
			}
			return
		case "img":
			if src := getAttr(n, "src"); src != "" {
				cand := MediaCandidate{
					Type:   "image",
					URL:    src,
					Width:  parseIntAttr(getAttr(n, "width")),
					Height: parseIntAttr(getAttr(n, "height")),
				}
				s.mediaImages = append(s.mediaImages, cand)
				if inArticle && s.firstArticleImage == "" {
					s.firstArticleImage = src
				}
			}
		case "video":
			if src := getAttr(n, "src"); src != "" {
				s.mediaVideos = append(s.mediaVideos, MediaCandidate{
					Type:   "video",
					URL:    src,
					Width:  parseIntAttr(getAttr(n, "width")),
					Height: parseIntAttr(getAttr(n, "height")),
				})
			}
		case "audio":
			if src := getAttr(n, "src"); src != "" {
				s.mediaAudios = append(s.mediaAudios, MediaCandidate{
					Type: "audio",
					URL:  src,
				})
			}
		case "source":
			s.handleSource(n)
		case "script":
			if strings.EqualFold(getAttr(n, "type"), "application/ld+json") {
				s.handleJSONLD(textContent(n))
			}
			return
		case "style", "noscript":
			return
		}
	}

	if n.Type == html.TextNode {
		if inArticle {
			s.articleTextBuf.WriteString(n.Data)
			s.articleTextBuf.WriteString(" ")
		} else {
			s.bodyTextBuf.WriteString(n.Data)
			s.bodyTextBuf.WriteString(" ")
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s.walk(c, inArticle)
	}

	if n.Type == html.ElementNode && strings.EqualFold(n.Data, "body") {
		if s.bodyText == "" {
			s.bodyText = strings.TrimSpace(s.bodyTextBuf.String())
		}
	}
}

func (s *extractScan) handleMeta(n *html.Node) {
	property := getAttr(n, "property")
	name := getAttr(n, "name")
	content := strings.TrimSpace(getAttr(n, "content"))
	if content == "" {
		return
	}
	switch strings.ToLower(property) {
	case "og:title":
		if s.ogTitle == "" {
			s.ogTitle = content
		}
	case "og:image":
		if s.ogImage == "" {
			s.ogImage = content
		}
	case "og:url":
		if s.ogURL == "" {
			s.ogURL = content
		}
	case "og:locale":
		if s.ogLocale == "" {
			s.ogLocale = normalizeLang(content)
		}
	case "og:description":
		if s.bodyText == "" {
			s.bodyText = content
		}
	case "article:published_time":
		if s.ogPublished == nil {
			s.ogPublished = parseTime(content)
		}
	case "article:author":
		if s.ogAuthor == "" {
			s.ogAuthor = content
		}
	}
	switch strings.ToLower(name) {
	case "twitter:title":
		if s.twTitle == "" {
			s.twTitle = content
		}
	case "twitter:image":
		if s.twImage == "" {
			s.twImage = content
		}
	case "description":
		if s.bodyText == "" {
			s.bodyText = content
		}
	}
}

func (s *extractScan) handleSource(n *html.Node) {
	src := getAttr(n, "src")
	if src == "" {
		src = getAttr(n, "srcset")
	}
	if src == "" {
		return
	}
	mediaType := mediaTypeFromMIME(getAttr(n, "type"))
	if mediaType == "" {
		return
	}
	cand := MediaCandidate{Type: mediaType, URL: src}
	switch mediaType {
	case "image":
		s.mediaImages = append(s.mediaImages, cand)
	case "video":
		s.mediaVideos = append(s.mediaVideos, cand)
	case "audio":
		s.mediaAudios = append(s.mediaAudios, cand)
	}
}

func (s *extractScan) handleJSONLD(body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	var any interface{}
	if err := json.Unmarshal([]byte(body), &any); err != nil {
		return
	}
	collectJSONLD(any, s)
}

func collectJSONLD(v interface{}, s *extractScan) {
	switch t := v.(type) {
	case map[string]interface{}:
		if str, ok := t["headline"].(string); ok && s.jsonLDTitle == "" {
			s.jsonLDTitle = strings.TrimSpace(str)
		}
		if str, ok := t["name"].(string); ok && s.jsonLDTitle == "" {
			s.jsonLDTitle = strings.TrimSpace(str)
		}
		if str, ok := t["articleBody"].(string); ok && s.jsonLDBody == "" {
			s.jsonLDBody = strings.TrimSpace(str)
		}
		if str, ok := t["description"].(string); ok && s.jsonLDBody == "" {
			s.jsonLDBody = strings.TrimSpace(str)
		}
		if img, ok := t["image"]; ok && s.jsonLDImage == "" {
			s.jsonLDImage = firstStringFromImageField(img)
		}
		if str, ok := t["datePublished"].(string); ok && s.jsonLDPublished == nil {
			s.jsonLDPublished = parseTime(str)
		}
		if author, ok := t["author"]; ok && s.jsonLDAuthor == "" {
			s.jsonLDAuthor = firstStringFromAuthorField(author)
		}
		if g, ok := t["@graph"]; ok {
			collectJSONLD(g, s)
		}
	case []interface{}:
		for _, elem := range t {
			collectJSONLD(elem, s)
		}
	}
}

func firstStringFromImageField(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		for _, elem := range t {
			if s := firstStringFromImageField(elem); s != "" {
				return s
			}
		}
	case map[string]interface{}:
		if u, ok := t["url"].(string); ok {
			return u
		}
	}
	return ""
}

func firstStringFromAuthorField(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		for _, elem := range t {
			if s := firstStringFromAuthorField(elem); s != "" {
				return s
			}
		}
	case map[string]interface{}:
		if n, ok := t["name"].(string); ok {
			return n
		}
	}
	return ""
}

func pickFirstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func pickFirstAbsoluteURL(base *url.URL, values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if abs, ok := absolutize(base, v); ok {
			return abs
		}
	}
	return ""
}

func pickFirstTime(values ...*time.Time) *time.Time {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// pickCanonicalWithCrossDomainGuard applies the canonical fallback chain and
// drops any candidate whose host differs from fetchURL's host. This prevents
// page-level canonical hijacking from collapsing two distinct fetches into
// the same pins.url. The final fallback is fetchURL itself.
func pickCanonicalWithCrossDomainGuard(base *url.URL, candidates ...string) string {
	fetchHost := ""
	if base != nil {
		fetchHost = strings.ToLower(base.Hostname())
	}
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		abs, ok := absolutize(base, raw)
		if !ok {
			continue
		}
		u, err := url.Parse(abs)
		if err != nil {
			continue
		}
		if fetchHost != "" && strings.ToLower(u.Hostname()) != fetchHost {
			// Cross-domain canonical: skip and continue fallback chain.
			continue
		}
		return abs
	}
	if base != nil {
		return base.String()
	}
	return ""
}

func absolutize(base *url.URL, raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
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
	return u.String(), true
}

func buildMediaCandidates(base *url.URL, s *extractScan, limit int) []MediaCandidate {
	all := make([]MediaCandidate, 0, len(s.mediaImages)+len(s.mediaVideos)+len(s.mediaAudios))
	all = append(all, s.mediaImages...)
	all = append(all, s.mediaVideos...)
	all = append(all, s.mediaAudios...)

	out := make([]MediaCandidate, 0, len(all))
	seen := make(map[string]struct{}, len(all))
	for _, c := range all {
		abs, ok := absolutize(base, c.URL)
		if !ok {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}
		c.URL = abs
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mediaTypeFromMIME(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if mime == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	}
	return ""
}

// normalizeLang trims an OG locale value like "en_US" → "en-US" so it lines
// up with the BCP-47-ish convention used by html[lang].
func normalizeLang(v string) string {
	return strings.ReplaceAll(strings.TrimSpace(v), "_", "-")
}

func parseTime(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return &t
		}
	}
	return nil
}
