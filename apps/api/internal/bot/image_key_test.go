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
	}{
		{
			name:    "lowercase host + strip fragment + preserve query",
			raw:     "https://Example.COM/a.jpg?w=800#hero",
			pageURL: "https://example.com/",
			want:    "https://example.com/a.jpg?w=800",
		},
		{
			name:    "relative url resolved",
			raw:     "/static/img.png",
			pageURL: "https://example.com/path/",
			want:    "https://example.com/static/img.png",
		},
		{
			name:    "query preserved as-is (no sort)",
			raw:     "https://example.com/i.jpg?b=2&a=1",
			pageURL: "https://example.com/",
			want:    "https://example.com/i.jpg?b=2&a=1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeImageURL(tc.raw, tc.pageURL)
			if got != tc.want {
				t.Errorf("normalizeImageURL() = %q want %q", got, tc.want)
			}
		})
	}
}

func TestSha256Hex(t *testing.T) {
	h := sha256Hex("https://example.com/a.jpg")
	if len(h) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(h))
	}
	if strings.ToLower(h) != h {
		t.Errorf("expected lowercase hex")
	}
}

func TestExtensionForImage(t *testing.T) {
	tests := []struct {
		ct   string
		url  string
		want string
	}{
		{"image/jpeg", "https://x/a.jpg", ".jpg"},
		{"image/png", "https://x/a.png", ".png"},
		{"image/webp", "https://x/a.webp", ".webp"},
		{"image/gif", "https://x/a.gif", ".gif"},
		{"image/jpeg; charset=utf-8", "https://x/a", ".jpg"},
		{"", "https://x/a.png", ".png"},
		{"", "https://x/a.JPG", ".jpg"},
		{"", "https://x/a", ".bin"},
		{"application/weird", "https://x/a", ".bin"},
	}
	for _, tc := range tests {
		got := extensionForImage(tc.ct, tc.url)
		if got != tc.want {
			t.Errorf("extensionForImage(%q,%q) = %q want %q", tc.ct, tc.url, got, tc.want)
		}
	}
}

func TestImageCacheKey(t *testing.T) {
	key := imageCacheKey("abc123", 1700000000, ".jpg")
	want := "images/abc123/1700000000.jpg"
	if key != want {
		t.Errorf("imageCacheKey = %q want %q", key, want)
	}
	// Accepts ext without leading dot
	key2 := imageCacheKey("abc", 1, "png")
	if key2 != "images/abc/1.png" {
		t.Errorf("imageCacheKey unexpected %q", key2)
	}
}
