package bot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// fetchHTMLShared fetches HTML content with timeout, redirect limits, and size limits.
// Returns (html, finalURL, error) where finalURL is the URL after any redirects.
// Shared by Pioneer and Harvester.
func fetchHTMLShared(ctx context.Context, rawURL string) (string, string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "FugueBot/1.0 (+https://fugue.app)")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	// Preserve the final URL after redirects
	finalURL := resp.Request.URL.String()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// HTTPStatusError is defined in snapshot_first_fetch.go and lets the
		// harvester entry point classify 4xx/5xx via errors.As without
		// re-parsing the error message.
		return "", "", &HTTPStatusError{Code: resp.StatusCode}
	}

	// Limit response body to 5MB to prevent memory spikes
	const maxBodySize = 5 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) == 0 {
		return "", "", fmt.Errorf("empty response body")
	}

	return string(body), finalURL, nil
}
