package bot

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Fetcher abstracts page retrieval for Pioneer.
// The default (nil) falls back to fetchHTMLShared.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (html, finalURL string, err error)
}

// HTTPFetcher retrieves pages via the standard net/http client.
type HTTPFetcher struct{}

// Fetch implements Fetcher.
func (HTTPFetcher) Fetch(ctx context.Context, u string) (string, string, error) {
	return fetchHTMLShared(ctx, u)
}

// FileSaver persists HTML under a sitemap directory, mirroring the URL's
// host + path structure. Numeric path segments are replaced with {id} so
// that one file per node template is written (matching Pioneer's templatePath).
//
// Examples:
//   https://www.pixiv.net/                   -> <base>/pixiv.net/index.html
//   https://www.pixiv.net/tags/漫画           -> <base>/pixiv.net/tags/漫画/index.html
//   https://www.pixiv.net/artworks/12345     -> <base>/pixiv.net/artworks/{id}/index.html
type FileSaver struct {
	BaseDir string
}

// Save writes html to disk using a sitemap-style path derived from finalURL.
func (s *FileSaver) Save(finalURL, html string) error {
	if s == nil || s.BaseDir == "" {
		return nil
	}
	rel, err := sitemapPath(finalURL)
	if err != nil {
		return err
	}
	full := filepath.Join(s.BaseDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(html), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", full, err)
	}
	return nil
}

// sitemapPath maps a URL to a sitemap-relative path like
// "pixiv.net/tags/漫画/index.html". Query string and fragment are ignored.
func sitemapPath(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	if host == "" {
		return "", fmt.Errorf("missing host in url: %s", rawURL)
	}

	parts := []string{host}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == "" {
			continue
		}
		if decoded, err := url.PathUnescape(seg); err == nil {
			seg = decoded
		}
		if isNumeric(seg) {
			seg = "{id}"
		}
		seg = sanitizeSegment(seg)
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		parts = append(parts, seg)
	}
	parts = append(parts, "index.html")
	return filepath.Join(parts...), nil
}

// sanitizeSegment strips characters unsafe for local filenames while
// preserving Unicode (e.g. Japanese tag names).
func sanitizeSegment(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '/', '\\', '\x00', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SavingFetcher wraps another Fetcher and writes the returned HTML to disk.
// Save failures are logged but do not fail the fetch.
type SavingFetcher struct {
	Inner Fetcher
	Saver *FileSaver
}

// Fetch implements Fetcher.
func (f *SavingFetcher) Fetch(ctx context.Context, u string) (string, string, error) {
	html, finalURL, err := f.Inner.Fetch(ctx, u)
	if err != nil {
		return html, finalURL, err
	}
	if saveErr := f.Saver.Save(finalURL, html); saveErr != nil {
		fmt.Printf("⚠️  sitemap save failed for %s: %v\n", finalURL, saveErr)
	}
	return html, finalURL, nil
}
