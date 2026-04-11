package crawler

import (
	"context"
	"io"
)

// Fetcher abstracts page retrieval, allowing different implementations
// for testing (file-based) and production (HTTP).
type Fetcher interface {
	// Fetch retrieves the content at the given URL.
	// Returns FetchResult containing the body and content type, or an error.
	Fetch(ctx context.Context, url string) (*FetchResult, error)
}

// FetchResult represents the result of fetching a page.
type FetchResult struct {
	// Body is the content of the fetched page.
	// The caller is responsible for closing it.
	Body io.ReadCloser

	// ContentType is the MIME type of the content (e.g., "text/html").
	ContentType string
}
