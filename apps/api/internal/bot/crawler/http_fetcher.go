package crawler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// HTTPFetcher implements Fetcher by making actual HTTP requests.
// Used in production for real crawling.
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher creates a new HTTPFetcher with the given HTTP client.
// If client is nil, http.DefaultClient is used.
func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPFetcher{
		client: client,
	}
}

// Fetch retrieves a page via HTTP GET request.
func (f *HTTPFetcher) Fetch(ctx context.Context, url string, headers map[string][]string) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if headers != nil {
		req.Header = http.Header(headers)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Extract content type
	contentType := resp.Header.Get("Content-Type")
	// Remove charset and other parameters
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return &FetchResult{
		Body:        resp.Body,
		ContentType: contentType,
	}, nil
}
