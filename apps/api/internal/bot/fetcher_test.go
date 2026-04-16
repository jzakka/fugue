package bot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSitemapPath(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "root path",
			url:  "https://www.pixiv.net/",
			want: filepath.Join("pixiv.net", "index.html"),
		},
		{
			name: "root without trailing slash",
			url:  "https://www.pixiv.net",
			want: filepath.Join("pixiv.net", "index.html"),
		},
		{
			name: "tag page with japanese characters",
			url:  "https://www.pixiv.net/tags/%E6%BC%AB%E7%94%BB",
			want: filepath.Join("pixiv.net", "tags", "漫画", "index.html"),
		},
		{
			name: "numeric segment replaced with {id}",
			url:  "https://www.pixiv.net/artworks/12345",
			want: filepath.Join("pixiv.net", "artworks", "{id}", "index.html"),
		},
		{
			name: "query string ignored",
			url:  "https://www.pixiv.net/artworks/12345?mode=manga",
			want: filepath.Join("pixiv.net", "artworks", "{id}", "index.html"),
		},
		{
			name: "www stripped",
			url:  "https://WWW.Pixiv.NET/en",
			want: filepath.Join("pixiv.net", "en", "index.html"),
		},
		{
			name: "path traversal segment dropped",
			url:  "https://example.com/a/../b",
			want: filepath.Join("example.com", "a", "b", "index.html"),
		},
		{
			name: "unsafe characters sanitized",
			url:  "https://example.com/foo:bar*baz",
			want: filepath.Join("example.com", "foo_bar_baz", "index.html"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sitemapPath(tc.url)
			if err != nil {
				t.Fatalf("sitemapPath(%q) error: %v", tc.url, err)
			}
			if got != tc.want {
				t.Errorf("sitemapPath(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestSitemapPathErrors(t *testing.T) {
	_, err := sitemapPath("not-a-url")
	if err == nil {
		t.Errorf("expected error for url without host")
	}
}

func TestFileSaverSave(t *testing.T) {
	dir := t.TempDir()
	saver := &FileSaver{BaseDir: dir}
	if err := saver.Save("https://www.pixiv.net/tags/art", "<html>hi</html>"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "pixiv.net", "tags", "art", "index.html"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(got) != "<html>hi</html>" {
		t.Errorf("saved content = %q, want <html>hi</html>", got)
	}
}

func TestFileSaverNilBaseDir(t *testing.T) {
	// Empty BaseDir is a no-op (useful when saving is disabled).
	saver := &FileSaver{}
	if err := saver.Save("https://example.com/", "<html/>"); err != nil {
		t.Errorf("expected no error when BaseDir empty, got %v", err)
	}
}
