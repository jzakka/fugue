package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/httpclient"
)

// fetchHTMLShared fetches HTML content with timeout, redirect limits, and size limits.
// Returns (html, finalURL, error) where finalURL is the URL after any redirects.
// Shared by Pioneer and Harvester.
//
// rawURL is caller-untrusted — it originates from the harvester_frontier, whose
// rows are URLs Pioneer extracted from arbitrary external HTML. The HTTP client
// MUST therefore be the SSRF-safe factory (httpclient.NewSSRFSafeClient): its
// dialer re-resolves every host and refuses to dial private/reserved/metadata IP
// ranges, and its CheckRedirect re-runs the same check on every redirect hop. The
// options (ConnectTimeout 5s, TotalTimeout 10s, MaxRedirects 5) mirror Pioneer's
// DefaultConsumerFetcher (pioneer_consumer.go) so both fetch stages of the bot
// pipeline share one SSRF policy, satisfying the httpclient package contract that
// "any code that fetches caller-untrusted URLs ... extracted from crawled HTML"
// route through this client. TotalTimeout 10s preserves the prior 10s deadline.
//
// client is injectable for tests: production callers pass nil to get the
// SSRF-safe default, while tests can inject an httptest.Server's client to
// exercise body-limit/2xx/finalURL handling against a loopback server that
// the SSRF dialer would otherwise (correctly) refuse to dial.
func fetchHTMLShared(ctx context.Context, client *http.Client, rawURL string) (string, string, error) {
	if client == nil {
		client = httpclient.NewSSRFSafeClient(httpclient.Options{
			ConnectTimeout: 5 * time.Second,
			TotalTimeout:   10 * time.Second,
			MaxRedirects:   5,
		})
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
			log.Printf("fetchHTMLShared: failed to close response body: %v", closeErr)
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
