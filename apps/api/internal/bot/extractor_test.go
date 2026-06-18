package bot

import (
	"strings"
	"testing"
)

func extract(t *testing.T, html, fetchURL string) PinDocument {
	t.Helper()
	doc, err := NewGenericExtractor().Extract([]byte(html), fetchURL)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	return doc
}

func TestExtractor_OGOnly(t *testing.T) {
	page := `<html><head>
<meta property="og:title" content="OG Title">
<meta property="og:description" content="OG body description">
<meta property="og:image" content="https://cdn.example.com/og.jpg">
<meta property="og:locale" content="en_US">
<meta property="og:url" content="https://example.com/article">
<meta property="article:published_time" content="2026-01-15T10:00:00Z">
<meta property="article:author" content="OG Author">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/article")

	if doc.Title != "OG Title" {
		t.Errorf("title = %q, want OG Title", doc.Title)
	}
	if doc.BodyText != "OG body description" {
		t.Errorf("body = %q", doc.BodyText)
	}
	if doc.ThumbnailURL != "https://cdn.example.com/og.jpg" {
		t.Errorf("thumb = %q", doc.ThumbnailURL)
	}
	if doc.Lang != "en-US" {
		t.Errorf("lang = %q, want en-US", doc.Lang)
	}
	if doc.Author != "OG Author" {
		t.Errorf("author = %q", doc.Author)
	}
	if doc.PublishedAt == nil || doc.PublishedAt.Year() != 2026 {
		t.Errorf("published = %v", doc.PublishedAt)
	}
	if doc.CanonicalURL != "https://example.com/article" {
		t.Errorf("canonical = %q", doc.CanonicalURL)
	}
	if doc.OGData.Source != "https://example.com/article" {
		t.Errorf("source = %q", doc.OGData.Source)
	}
	if doc.OGData.Extractor != "generic" {
		t.Errorf("extractor = %q", doc.OGData.Extractor)
	}
}

func TestExtractor_JSONLDOnly(t *testing.T) {
	page := `<html><head>
<script type="application/ld+json">{
  "@context":"https://schema.org",
  "@type":"Article",
  "headline":"JSONLD Headline",
  "articleBody":"This is the body.",
  "image":"https://cdn.example.com/jsonld.jpg",
  "datePublished":"2026-02-01T08:00:00Z",
  "author":{"@type":"Person","name":"Jane Doe"}
}</script>
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/x")

	if doc.Title != "JSONLD Headline" {
		t.Errorf("title = %q", doc.Title)
	}
	if doc.BodyText != "This is the body." {
		t.Errorf("body = %q", doc.BodyText)
	}
	if doc.ThumbnailURL != "https://cdn.example.com/jsonld.jpg" {
		t.Errorf("thumb = %q", doc.ThumbnailURL)
	}
	if doc.Author != "Jane Doe" {
		t.Errorf("author = %q", doc.Author)
	}
	if doc.PublishedAt == nil || doc.PublishedAt.Month().String() != "February" {
		t.Errorf("published = %v", doc.PublishedAt)
	}
}

func TestExtractor_ArticleTagOnly(t *testing.T) {
	page := `<html lang="ko"><head><title>HTML Title</title></head>
<body>
<article>
  <h1>Article H1</h1>
  <p>The article body text goes here.</p>
  <img src="/img/hero.jpg" width="600" height="400" alt="hero">
</article>
</body></html>`

	doc := extract(t, page, "https://example.com/post/1")

	if doc.Title != "Article H1" {
		t.Errorf("title = %q", doc.Title)
	}
	if !strings.Contains(doc.BodyText, "article body text") {
		t.Errorf("body = %q", doc.BodyText)
	}
	if doc.Lang != "ko" {
		t.Errorf("lang = %q", doc.Lang)
	}
	if doc.ThumbnailURL != "https://example.com/img/hero.jpg" {
		t.Errorf("thumb = %q", doc.ThumbnailURL)
	}
}

func TestExtractor_BodyTextArticleWinsOverJSONLD(t *testing.T) {
	// harvester/spec.md body_text chain: <article> textContent is position 1,
	// JSON-LD articleBody position 2. When both are present, <article> wins.
	page := `<html><head>
<script type="application/ld+json">{
  "@context":"https://schema.org",
  "@type":"Article",
  "articleBody":"JSON-LD article body."
}</script>
</head><body>
<article><p>The real article element text.</p></article>
</body></html>`

	doc := extract(t, page, "https://example.com/post/1")
	if !strings.Contains(doc.BodyText, "real article element text") {
		t.Errorf("expected <article> textContent to win over JSON-LD articleBody, got %q", doc.BodyText)
	}
}

func TestExtractor_BodyTextDensityBlockWinsOverOGDescription(t *testing.T) {
	// harvester/spec.md body_text chain: body text block is position 3,
	// og:description position 4. When both are present (and no <article>/JSON-LD),
	// the body text block wins.
	page := `<html><head>
<meta property="og:description" content="OG fallback description">
</head><body>
<p>This is the main body paragraph with the actual content.</p>
</body></html>`

	doc := extract(t, page, "https://example.com/post/2")
	if !strings.Contains(doc.BodyText, "main body paragraph") {
		t.Errorf("expected body text block to win over og:description, got %q", doc.BodyText)
	}
}

func TestExtractor_BodyTextMetaDescriptionFallback(t *testing.T) {
	// With no <article>, no JSON-LD, no body text block, and no og:description,
	// meta[name=description] (position 5) is used.
	page := `<html><head>
<meta name="description" content="Meta description fallback">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/post/3")
	if doc.BodyText != "Meta description fallback" {
		t.Errorf("expected meta description fallback, got %q", doc.BodyText)
	}
}

func TestExtractor_BodyTextOGDescriptionWinsOverMetaDescription(t *testing.T) {
	// og:description (position 4) precedes meta[name=description] (position 5).
	page := `<html><head>
<meta property="og:description" content="OG wins">
<meta name="description" content="Meta loses">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/post/4")
	if doc.BodyText != "OG wins" {
		t.Errorf("expected og:description to win over meta description, got %q", doc.BodyText)
	}
}

func TestExtractor_NothingPresent(t *testing.T) {
	page := `<html><body><p>Just some text</p></body></html>`
	doc := extract(t, page, "https://example.com/empty")

	if doc.Title != "" {
		t.Errorf("title should be empty, got %q", doc.Title)
	}
	if doc.CanonicalURL != "https://example.com/empty" {
		t.Errorf("canonical should fall back to fetch URL, got %q", doc.CanonicalURL)
	}
	if doc.OGData.Source != "https://example.com/empty" {
		t.Errorf("source should be fetch URL, got %q", doc.OGData.Source)
	}
}

func TestExtractor_CrossDomainCanonicalIgnored(t *testing.T) {
	page := `<html><head>
<link rel="canonical" href="https://other.example.org/promoted">
<meta property="og:url" content="https://example.com/article">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/article?ref=tw")

	// Cross-domain canonical must be ignored; og:url is same host so it wins.
	if doc.CanonicalURL != "https://example.com/article" {
		t.Errorf("expected og:url to win after cross-domain canonical was dropped, got %q", doc.CanonicalURL)
	}
}

func TestExtractor_CrossDomainCanonicalAllSkippedFallsBackToFetch(t *testing.T) {
	page := `<html><head>
<link rel="canonical" href="https://other.example.org/promoted">
<meta property="og:url" content="https://yet-another.example.net/x">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/article")

	if doc.CanonicalURL != "https://example.com/article" {
		t.Errorf("expected fallback to fetch URL, got %q", doc.CanonicalURL)
	}
}

func TestExtractor_ReturnsNonNilOnEmptyInput(t *testing.T) {
	doc := extract(t, "", "https://example.com/foo")
	if doc.CanonicalURL != "https://example.com/foo" {
		t.Errorf("expected canonical = fetch URL, got %q", doc.CanonicalURL)
	}
	if doc.OGData.Extractor != "generic" {
		t.Errorf("extractor = %q", doc.OGData.Extractor)
	}
}

func TestExtractor_MediaCandidatesCollectsTypes(t *testing.T) {
	page := `<html><body>
<img src="https://cdn.example.com/a.jpg" alt="a">
<video src="https://cdn.example.com/b.mp4"></video>
<audio src="https://cdn.example.com/c.mp3"></audio>
</body></html>`

	doc := extract(t, page, "https://example.com/x")
	types := map[string]bool{}
	for _, c := range doc.MediaCandidates {
		types[c.Type] = true
	}
	if !types["image"] || !types["video"] || !types["audio"] {
		t.Errorf("expected image+video+audio, got %v", types)
	}
}

func TestExtractor_MediaCandidatesLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("<html><body>")
	for i := 0; i < 60; i++ {
		b.WriteString(`<img src="https://cdn.example.com/`)
		b.WriteString(strings.Repeat("a", i+1))
		b.WriteString(`.jpg" alt="x">`)
	}
	b.WriteString("</body></html>")

	doc := extract(t, b.String(), "https://example.com/x")
	if len(doc.MediaCandidates) > MaxMediaCandidates {
		t.Errorf("expected ≤ %d candidates, got %d", MaxMediaCandidates, len(doc.MediaCandidates))
	}
}

func TestExtractor_SourceSrcsetFirstURLOnly(t *testing.T) {
	// harvester/spec.md "source의 srcset 다중 후보에서 첫 URL만 수집":
	// a <source srcset="..."> with descriptor-bearing comma-separated
	// candidates must collect only the first candidate's URL, absolutized,
	// with no descriptor or comma leaking into the URL.
	page := `<html><body><article>
<picture>
<source srcset="a.webp 1x, b.webp 2x" type="image/webp">
<img src="fallback.jpg" alt="x">
</picture>
</article></body></html>`

	doc := extract(t, page, "https://example.com/article/page")
	var got string
	for _, c := range doc.MediaCandidates {
		if c.Type == "image" && strings.Contains(c.URL, "a.webp") {
			got = c.URL
		}
		if strings.ContainsAny(c.URL, " ,") || strings.Contains(c.URL, "%20") || strings.Contains(c.URL, "%2C") {
			t.Errorf("media candidate URL contains descriptor/comma artifact: %q", c.URL)
		}
	}
	if got != "https://example.com/article/a.webp" {
		t.Errorf("first srcset URL = %q, want https://example.com/article/a.webp (candidates: %v)", got, doc.MediaCandidates)
	}
}

func TestExtractor_SourceSrcsetSingleURLNoDescriptor(t *testing.T) {
	// Regression: a single-URL srcset without descriptor must still be
	// collected unchanged.
	page := `<html><body><article>
<picture>
<source srcset="https://cdn.example.com/only.webp" type="image/webp">
</picture>
</article></body></html>`

	doc := extract(t, page, "https://example.com/x")
	found := false
	for _, c := range doc.MediaCandidates {
		if c.Type == "image" && c.URL == "https://cdn.example.com/only.webp" {
			found = true
		}
	}
	if !found {
		t.Errorf("single-URL srcset not collected: %v", doc.MediaCandidates)
	}
}

func TestExtractor_OGTitleWinsOverHTMLTitle(t *testing.T) {
	page := `<html><head>
<title>HTML Title</title>
<meta property="og:title" content="OG Title">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/x")
	if doc.Title != "OG Title" {
		t.Errorf("title = %q, want OG Title", doc.Title)
	}
}

func TestExtractor_TwitterFallbackForTitle(t *testing.T) {
	page := `<html><head>
<title>HTML Title</title>
<meta name="twitter:title" content="TW Title">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/x")
	if doc.Title != "TW Title" {
		t.Errorf("title = %q, want TW Title", doc.Title)
	}
}

func TestExtractor_MetaNameAuthorExtracted(t *testing.T) {
	// harvester/spec.md "lang/author/published_at 추출" lists meta[name=author]
	// as one of the three author sources. When it is the only author source,
	// it must populate the author field.
	page := `<html><head>
<meta name="author" content="Meta Only Author">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/x")
	if doc.Author != "Meta Only Author" {
		t.Errorf("author = %q, want Meta Only Author", doc.Author)
	}
}

func TestExtractor_OGArticleAuthorWinsOverMetaAuthor(t *testing.T) {
	// og:article:author precedence is preserved over meta[name=author].
	page := `<html><head>
<meta property="article:author" content="OG Article Author">
<meta name="author" content="Meta Author">
</head><body></body></html>`

	doc := extract(t, page, "https://example.com/x")
	if doc.Author != "OG Article Author" {
		t.Errorf("author = %q, want OG Article Author", doc.Author)
	}
}

func TestExtractor_RelativeImageResolved(t *testing.T) {
	page := `<html><head><meta property="og:image" content="/img/og.jpg"></head><body></body></html>`
	doc := extract(t, page, "https://example.com/articles/1")
	if doc.ThumbnailURL != "https://example.com/img/og.jpg" {
		t.Errorf("thumb = %q", doc.ThumbnailURL)
	}
}
