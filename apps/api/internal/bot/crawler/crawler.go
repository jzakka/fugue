// Package crawler provides a breadth-first web crawler with configurable page fetching.
//
// The crawler separates the BFS traversal logic from the page retrieval mechanism
// through the Fetcher interface, enabling testability without requiring HTTP requests.
//
// Example usage with file-based testing:
//
//	fetcher := crawler.NewFileFetcher("testdata")
//	c := crawler.NewBFSCrawler(fetcher)
//	result, err := c.Crawl(ctx, "http://example.com/", 2)
//
// Example usage with HTTP fetching:
//
//	fetcher := crawler.NewHTTPFetcher(http.DefaultClient)
//	c := crawler.NewBFSCrawler(fetcher)
//	result, err := c.Crawl(ctx, "http://example.com/", 2)
package crawler

import "context"

// Crawler performs breadth-first traversal of a website.
type Crawler interface {
	// Crawl traverses the website starting from rootURL up to maxDepth.
	// Returns a Result containing all visited URLs with their metadata, or an error.
	Crawl(ctx context.Context, rootURL string, maxDepth int) (*Result, error)
}

// Result represents the outcome of a crawl operation.
type Result struct {
	// URLs contains all URLs that were visited during the crawl.
	URLs []VisitedURL
}

// VisitedURL represents a single URL visited during crawling.
type VisitedURL struct {
	// URL is the absolute URL that was visited.
	URL string

	// Depth is the distance from the root URL (root = 0).
	Depth int

	// ParentURL is the URL of the page that linked to this URL.
	// Empty for the root URL.
	ParentURL string

	// Error contains any error encountered when fetching or processing this URL.
	// Nil if the URL was successfully processed.
	Error error
}
