package bot

import "testing"

func TestPickPrimaryImage(t *testing.T) {
	const pageURL = "https://example.com/posts/1"

	tests := []struct {
		name     string
		html     string
		pageURL  string
		expected string
	}{
		{
			name: "og:image takes priority over twitter and article img",
			html: `<html><head>
				<meta property="og:image" content="https://example.com/cover.jpg">
				<meta name="twitter:image" content="https://example.com/tw.jpg">
			</head><body><article><img src="https://example.com/in-article.jpg" width="600" height="400" alt="x"></article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/cover.jpg",
		},
		{
			name: "twitter:image via name attribute when og missing",
			html: `<html><head>
				<meta name="twitter:image" content="https://example.com/tw.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/tw.jpg",
		},
		{
			name: "twitter:image via property attribute when og missing",
			html: `<html><head>
				<meta property="twitter:image" content="https://example.com/tw2.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/tw2.jpg",
		},
		{
			name: "article img with width+height both >=100 is meaningful",
			html: `<html><body><article>
				<img src="https://example.com/big.jpg" width="600" height="400">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/big.jpg",
		},
		{
			name: "article img with non-empty alt is meaningful",
			html: `<html><body><article>
				<img src="https://example.com/alt.jpg" alt="hero">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/alt.jpg",
		},
		{
			name: "main tag also counts for article-img priority",
			html: `<html><body><main>
				<img src="https://example.com/main.jpg" alt="from main">
			</main></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/main.jpg",
		},
		{
			name: "article img failing both criteria is skipped, falls through to JSON-LD",
			html: `<html><head>
				<script type="application/ld+json">{"@type":"Article","image":"https://example.com/ld.jpg"}</script>
			</head><body><article>
				<img src="https://example.com/icon.png">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/ld.jpg",
		},
		{
			name: "JSON-LD image as string",
			html: `<html><head>
				<script type="application/ld+json">{"@type":"Article","image":"https://example.com/ld1.jpg"}</script>
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/ld1.jpg",
		},
		{
			name: "JSON-LD image as array (first element chosen)",
			html: `<html><head>
				<script type="application/ld+json">{"@type":"Article","image":["https://example.com/ld-a.jpg","https://example.com/ld-b.jpg"]}</script>
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/ld-a.jpg",
		},
		{
			name: "JSON-LD image as object with url field",
			html: `<html><head>
				<script type="application/ld+json">{"@type":"Article","image":{"url":"https://example.com/ld-obj.jpg","width":800}}</script>
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/ld-obj.jpg",
		},
		{
			name: "JSON-LD @graph array",
			html: `<html><head>
				<script type="application/ld+json">{"@graph":[{"@type":"WebPage"},{"@type":"Article","image":"https://example.com/graph.jpg"}]}</script>
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/graph.jpg",
		},
		{
			name: "relative URL resolved against page URL",
			html: `<html><head>
				<meta property="og:image" content="/static/cover.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/static/cover.jpg",
		},
		{
			name: "data: URI in og:image is rejected, falls to twitter",
			html: `<html><head>
				<meta property="og:image" content="data:image/png;base64,AAA">
				<meta name="twitter:image" content="https://example.com/tw.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/tw.jpg",
		},
		{
			name: "non-http(s) scheme is rejected",
			html: `<html><head>
				<meta property="og:image" content="ftp://example.com/a.jpg">
				<meta name="twitter:image" content="https://example.com/tw.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/tw.jpg",
		},
		{
			name: "tracking pixel filename is rejected (1x1)",
			html: `<html><head>
				<meta property="og:image" content="https://tracker.example.com/1x1.gif">
				<meta name="twitter:image" content="https://example.com/real.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/real.jpg",
		},
		{
			name: "tracking pixel filename 'pixel' keyword rejected",
			html: `<html><head>
				<meta property="og:image" content="https://tracker.example.com/pixel.gif">
				<meta name="twitter:image" content="https://example.com/real.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/real.jpg",
		},
		{
			name: "tracking pixel filename 'spacer' keyword rejected",
			html: `<html><head>
				<meta property="og:image" content="https://tracker.example.com/spacer.gif">
				<meta name="twitter:image" content="https://example.com/real.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/real.jpg",
		},
		{
			name: "img with width=1 height=1 is rejected (tracking pixel)",
			html: `<html><body><article>
				<img src="https://example.com/tiny.gif" width="1" height="1" alt="x">
				<img src="https://example.com/real.jpg" alt="real">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/real.jpg",
		},
		{
			name:     "no candidates returns empty string",
			html:     `<html><head><title>nothing</title></head><body><p>no imgs</p></body></html>`,
			pageURL:  pageURL,
			expected: "",
		},
		{
			name:     "empty html returns empty string",
			html:     "",
			pageURL:  pageURL,
			expected: "",
		},
		{
			name: "DOM order: first og:image wins when multiple",
			html: `<html><head>
				<meta property="og:image" content="https://example.com/first.jpg">
				<meta property="og:image" content="https://example.com/second.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/first.jpg",
		},
		{
			name: "width without height fails meaningful check (no alt)",
			html: `<html><head>
				<script type="application/ld+json">{"image":"https://example.com/ld.jpg"}</script>
			</head><body><article>
				<img src="https://example.com/onlywidth.jpg" width="600">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/ld.jpg",
		},
		{
			name: "width height both <100 without alt skipped",
			html: `<html><head>
				<script type="application/ld+json">{"image":"https://example.com/ld.jpg"}</script>
			</head><body><article>
				<img src="https://example.com/small.jpg" width="50" height="50">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/ld.jpg",
		},
		{
			name: "whitespace-only alt treated as empty",
			html: `<html><head>
				<script type="application/ld+json">{"image":"https://example.com/ld.jpg"}</script>
			</head><body><article>
				<img src="https://example.com/blankalt.jpg" alt="   ">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/ld.jpg",
		},
		{
			name: "px suffix on width/height parses correctly",
			html: `<html><body><article>
				<img src="https://example.com/px.jpg" width="600px" height="400px">
			</article></body></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/px.jpg",
		},
		{
			name: "empty content attribute on og:image is ignored",
			html: `<html><head>
				<meta property="og:image" content="">
				<meta name="twitter:image" content="https://example.com/tw.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/tw.jpg",
		},
		{
			name: "invalid JSON-LD is skipped gracefully",
			html: `<html><head>
				<script type="application/ld+json">{not valid json</script>
				<meta property="og:image" content="https://example.com/og.jpg">
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/og.jpg",
		},
		{
			name: "relative URL with empty pageURL cannot resolve",
			html: `<html><head>
				<meta property="og:image" content="/rel.jpg">
			</head></html>`,
			pageURL:  "",
			expected: "",
		},
		{
			name: "multiple JSON-LD blocks: first block invalid (data:), second block wins",
			html: `<html><head>
				<script type="application/ld+json">{"@type":"Article","image":"data:image/png;base64,AAA"}</script>
				<script type="application/ld+json">{"@type":"Article","image":"https://example.com/second.jpg"}</script>
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/second.jpg",
		},
		{
			name: "multiple JSON-LD blocks: DOM-order first block valid image wins over later",
			html: `<html><head>
				<script type="application/ld+json">{"@type":"Article","image":"https://example.com/first.jpg"}</script>
				<script type="application/ld+json">{"@type":"Article","image":"https://example.com/second.jpg"}</script>
			</head></html>`,
			pageURL:  pageURL,
			expected: "https://example.com/first.jpg",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := PickPrimaryImage([]byte(tc.html), tc.pageURL)
			if got != tc.expected {
				t.Errorf("PickPrimaryImage() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestPickPrimaryImage_CaseInsensitiveSchemeHost(t *testing.T) {
	// Sanity: URL normalization in keys is Decision 3's concern (image_cache),
	// but PickPrimaryImage itself should still return scheme/host as-provided
	// after go's url.Parse normalization (which lower-cases scheme/host).
	html := `<html><head><meta property="og:image" content="HTTPS://Example.com/a.jpg"></head></html>`
	got := PickPrimaryImage([]byte(html), "https://example.com/")
	if got == "" {
		t.Fatalf("expected non-empty URL, got empty")
	}
}
