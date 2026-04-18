package bot

import "testing"

func TestPickPrimaryImage(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		pageURL string
		want    string
	}{
		{
			name:    "og:image wins priority",
			pageURL: "https://example.com/post",
			html: `<html><head>
				<meta property="og:image" content="https://example.com/og.jpg">
				<meta name="twitter:image" content="https://example.com/tw.jpg">
				</head><body><article><img src="https://example.com/a.jpg" width="400" height="400" alt="x"></article></body></html>`,
			want: "https://example.com/og.jpg",
		},
		{
			name:    "twitter:image when no og",
			pageURL: "https://example.com/post",
			html: `<html><head>
				<meta name="twitter:image" content="https://example.com/tw.jpg">
				</head><body></body></html>`,
			want: "https://example.com/tw.jpg",
		},
		{
			name:    "twitter:image property variant",
			pageURL: "https://example.com/post",
			html: `<html><head>
				<meta property="twitter:image" content="https://example.com/tw2.jpg">
				</head></html>`,
			want: "https://example.com/tw2.jpg",
		},
		{
			name:    "article img by size",
			pageURL: "https://example.com/post",
			html: `<html><body><article>
				<img src="https://example.com/a.jpg" width="400" height="300">
				</article></body></html>`,
			want: "https://example.com/a.jpg",
		},
		{
			name:    "article img by alt",
			pageURL: "https://example.com/post",
			html: `<html><body><main>
				<img src="https://example.com/m.jpg" alt="a nice picture">
				</main></body></html>`,
			want: "https://example.com/m.jpg",
		},
		{
			name:    "article img too small and no alt → skipped",
			pageURL: "https://example.com/post",
			html: `<html><body><article>
				<img src="https://example.com/tiny.jpg" width="50" height="50">
				</article></body></html>`,
			want: "",
		},
		{
			name:    "JSON-LD string image",
			pageURL: "https://example.com/post",
			html: `<html><head><script type="application/ld+json">
				{"@type":"Article","image":"https://example.com/ld.jpg"}
				</script></head></html>`,
			want: "https://example.com/ld.jpg",
		},
		{
			name:    "JSON-LD array image",
			pageURL: "https://example.com/post",
			html: `<html><head><script type="application/ld+json">
				{"@type":"Article","image":["https://example.com/ld1.jpg","https://example.com/ld2.jpg"]}
				</script></head></html>`,
			want: "https://example.com/ld1.jpg",
		},
		{
			name:    "JSON-LD object image with url",
			pageURL: "https://example.com/post",
			html: `<html><head><script type="application/ld+json">
				{"@type":"Article","image":{"url":"https://example.com/ldobj.jpg"}}
				</script></head></html>`,
			want: "https://example.com/ldobj.jpg",
		},
		{
			name:    "relative url resolved against page",
			pageURL: "https://example.com/a/b/",
			html:    `<html><head><meta property="og:image" content="/static/cover.jpg"></head></html>`,
			want:    "https://example.com/static/cover.jpg",
		},
		{
			name:    "data URI rejected, falls back",
			pageURL: "https://example.com/post",
			html: `<html><head>
				<meta property="og:image" content="data:image/png;base64,AAA">
				<meta name="twitter:image" content="https://example.com/tw.jpg">
				</head></html>`,
			want: "https://example.com/tw.jpg",
		},
		{
			name:    "tracking pixel rejected",
			pageURL: "https://example.com/post",
			html: `<html><head>
				<meta property="og:image" content="https://tracker.example.com/pixel.gif">
				<meta name="twitter:image" content="https://example.com/tw.jpg">
				</head></html>`,
			want: "https://example.com/tw.jpg",
		},
		{
			name:    "1x1 pattern rejected",
			pageURL: "https://example.com/post",
			html: `<html><head>
				<meta property="og:image" content="https://example.com/1x1.png">
				</head></html>`,
			want: "",
		},
		{
			name:    "ftp scheme rejected",
			pageURL: "https://example.com/post",
			html: `<html><head>
				<meta property="og:image" content="ftp://example.com/foo.png">
				</head></html>`,
			want: "",
		},
		{
			name:    "no candidate",
			pageURL: "https://example.com/post",
			html:    `<html><body><p>nothing</p></body></html>`,
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PickPrimaryImage([]byte(tc.html), tc.pageURL)
			if got != tc.want {
				t.Errorf("PickPrimaryImage() = %q, want %q", got, tc.want)
			}
		})
	}
}
