package crawler

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// FileFetcher implements Fetcher by reading HTML files from the filesystem.
// Used for testing without needing actual HTTP requests.
type FileFetcher struct {
	basePath string
}

// NewFileFetcher creates a new FileFetcher that reads files from the given base path.
func NewFileFetcher(basePath string) *FileFetcher {
	return &FileFetcher{
		basePath: basePath,
	}
}

// Fetch retrieves a file from the filesystem based on the URL path.
// The URL is converted to a file path relative to basePath.
func (f *FileFetcher) Fetch(ctx context.Context, urlStr string) (*FetchResult, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Convert URL path to file path
	// /index.html -> basePath/index.html
	// /page1 -> basePath/page1.html (add .html if not present)
	path := parsed.Path
	if path == "" || path == "/" {
		path = "/index.html"
	}

	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Add .html extension if not present and no extension exists
	if filepath.Ext(path) == "" {
		path = path + ".html"
	}

	fullPath := filepath.Join(f.basePath, path)

	// Open the file
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	return &FetchResult{
		Body:        file,
		ContentType: "text/html; charset=utf-8",
	}, nil
}
