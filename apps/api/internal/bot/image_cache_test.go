package bot

import (
	"strings"
	"testing"
)

func TestNormalizeImageURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		pageURL string
		want    string
		wantErr bool
	}{
		{
			name:    "absolute URL passes through with lower-cased scheme/host",
			raw:     "HTTPS://Example.com/path/Img.JPG?w=800",
			pageURL: "https://example.com/",
			want:    "https://example.com/path/Img.JPG?w=800",
		},
		{
			name:    "fragment is stripped",
			raw:     "https://example.com/a.jpg#foo",
			pageURL: "https://example.com/",
			want:    "https://example.com/a.jpg",
		},
		{
			name:    "relative URL resolved against page URL",
			raw:     "/static/a.jpg",
			pageURL: "https://example.com/posts/1",
			want:    "https://example.com/static/a.jpg",
		},
		{
			name:    "path case preserved",
			raw:     "https://example.com/Foo/Bar.PNG",
			pageURL: "https://example.com/",
			want:    "https://example.com/Foo/Bar.PNG",
		},
		{
			name:    "query preserved as-is including ordering",
			raw:     "https://cdn.example.com/a.jpg?w=800&h=600&sig=abc",
			pageURL: "https://example.com/",
			want:    "https://cdn.example.com/a.jpg?w=800&h=600&sig=abc",
		},
		{
			name:    "scheme/host lower-case only differences produce same normalized URL",
			raw:     "HTTP://EXAMPLE.COM/a.jpg",
			pageURL: "https://example.com/",
			want:    "http://example.com/a.jpg",
		},
		{
			name:    "empty URL errors",
			raw:     "",
			pageURL: "https://example.com/",
			wantErr: true,
		},
		{
			name:    "relative with empty page URL errors",
			raw:     "/rel.jpg",
			pageURL: "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeImageURL(tc.raw, tc.pageURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeImageURL(%q, %q) = %q, want error", tc.raw, tc.pageURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeImageURL(%q, %q) unexpected error: %v", tc.raw, tc.pageURL, err)
			}
			if got != tc.want {
				t.Errorf("normalizeImageURL(%q, %q) = %q, want %q", tc.raw, tc.pageURL, got, tc.want)
			}
		})
	}
}

func TestHashImageURL(t *testing.T) {
	// Same input → same hash; different input → different hash.
	a := hashImageURL("https://example.com/a.jpg")
	b := hashImageURL("https://example.com/a.jpg")
	c := hashImageURL("https://example.com/b.jpg")

	if a != b {
		t.Errorf("expected identical hashes for same input, got %q and %q", a, b)
	}
	if a == c {
		t.Errorf("expected different hashes for different inputs, got %q for both", a)
	}
	if len(a) != 64 {
		t.Errorf("expected 64-char hex, got length %d", len(a))
	}
	// Must be lowercase hex.
	for _, r := range a {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			t.Errorf("hash must be lower-case hex, got rune %q in %q", r, a)
			break
		}
	}
}

func TestContentTypeToExt(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/gif", ".gif"},
		{"IMAGE/JPEG", ".jpg"},
		{"image/jpeg; charset=binary", ".jpg"},
		{" image/png ", ".png"},
		{"image/svg+xml", ""},
		{"application/octet-stream", ""},
		{"", ""},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := contentTypeToExt(tc.in)
			if got != tc.want {
				t.Errorf("contentTypeToExt(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveImageExt(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		url         string
		want        string
	}{
		{"content type wins", "image/jpeg", "https://example.com/a.png", ".jpg"},
		{"url path fallback when content type unknown", "application/octet-stream", "https://example.com/a.PNG", ".png"},
		{"default .bin when neither", "", "https://example.com/a", defaultImageExt},
		{"default .bin when url path has no ext", "foo/bar", "https://example.com/noext", defaultImageExt},
		{"url path ext lowercased", "", "https://example.com/a.JPEG", ".jpeg"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := resolveImageExt(tc.contentType, tc.url)
			if got != tc.want {
				t.Errorf("resolveImageExt(%q, %q) = %q, want %q", tc.contentType, tc.url, got, tc.want)
			}
		})
	}
}

func TestBuildImageCacheKey(t *testing.T) {
	const url = "https://example.com/a.jpg"
	const ts int64 = 1700000000

	key := buildImageCacheKey(url, "image/jpeg", ts)

	if !strings.HasPrefix(key, "images/") {
		t.Errorf("expected key to start with 'images/' prefix, got %q", key)
	}
	if !strings.HasSuffix(key, ".jpg") {
		t.Errorf("expected key to end with .jpg, got %q", key)
	}
	// Format: images/<hash>/<ts>.<ext>
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		t.Fatalf("expected 3 path segments (images/<hash>/<ts>.<ext>), got %d in %q", len(parts), key)
	}
	if parts[0] != "images" {
		t.Errorf("expected segment 0 = 'images', got %q", parts[0])
	}
	if len(parts[1]) != 64 {
		t.Errorf("expected segment 1 (hash) to be 64 hex chars, got %d in %q", len(parts[1]), parts[1])
	}
	if parts[2] != "1700000000.jpg" {
		t.Errorf("expected segment 2 = '1700000000.jpg', got %q", parts[2])
	}

	// Different URL → different key.
	otherKey := buildImageCacheKey("https://example.com/b.jpg", "image/jpeg", ts)
	if otherKey == key {
		t.Errorf("expected different URLs to yield different keys, got same: %q", key)
	}

	// Same URL at different ts → different key (no overwrite).
	laterKey := buildImageCacheKey(url, "image/jpeg", ts+1)
	if laterKey == key {
		t.Errorf("expected different timestamps to yield different keys, got same: %q", key)
	}
}

func TestBuildImageCacheKey_SchemeHostCaseOnlyDiff(t *testing.T) {
	// After normalizeImageURL, scheme/host case-only differences produce the
	// same input to the key builder. Verify the keys match.
	n1, err := normalizeImageURL("HTTPS://Example.com/a.jpg", "https://example.com/")
	if err != nil {
		t.Fatalf("normalize 1: %v", err)
	}
	n2, err := normalizeImageURL("https://example.com/a.jpg", "https://example.com/")
	if err != nil {
		t.Fatalf("normalize 2: %v", err)
	}
	k1 := buildImageCacheKey(n1, "image/jpeg", 1700000000)
	k2 := buildImageCacheKey(n2, "image/jpeg", 1700000000)
	if k1 != k2 {
		t.Errorf("scheme/host case-only diff should yield same key after normalization, got %q vs %q", k1, k2)
	}
}

func TestBuildImageCacheKey_DifferentQueryYieldsDifferentKey(t *testing.T) {
	// Decision 6: queries are preserved; different query = different hash.
	n1, _ := normalizeImageURL("https://cdn.example.com/a.jpg?w=800", "https://example.com/")
	n2, _ := normalizeImageURL("https://cdn.example.com/a.jpg?w=400", "https://example.com/")
	k1 := buildImageCacheKey(n1, "image/jpeg", 1700000000)
	k2 := buildImageCacheKey(n2, "image/jpeg", 1700000000)
	if k1 == k2 {
		t.Errorf("different query parameters should yield different keys, got same: %q", k1)
	}
}
