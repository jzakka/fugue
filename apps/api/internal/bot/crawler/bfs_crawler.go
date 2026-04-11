package crawler

import (
	"context"
	"fmt"
)

// BFSCrawler implements the Crawler interface using breadth-first search.
type BFSCrawler struct {
	fetcher Fetcher
}

// NewBFSCrawler creates a new BFSCrawler with the given Fetcher.
func NewBFSCrawler(fetcher Fetcher) *BFSCrawler {
	return &BFSCrawler{
		fetcher: fetcher,
	}
}

// queueItem represents a URL in the BFS queue.
type queueItem struct {
	url       string
	depth     int
	parentURL string
}

// Crawl performs a breadth-first traversal of the website starting from rootURL.
// It respects the maxDepth limit and only follows links within the same domain.
func (c *BFSCrawler) Crawl(ctx context.Context, rootURL string, maxDepth int) (*Result, error) {
	// Normalize root URL
	normalizedRoot, err := normalizeURL(rootURL)
	if err != nil {
		return nil, fmt.Errorf("invalid root URL: %w", err)
	}

	// Initialize queue with root URL at depth 0
	queue := []queueItem{{url: normalizedRoot, depth: 0, parentURL: ""}}

	// Track visited URLs to avoid duplicates
	visited := make(map[string]bool)
	visited[normalizedRoot] = true

	// Store results
	var results []VisitedURL

	// BFS main loop
	for len(queue) > 0 {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Dequeue (FIFO)
		current := queue[0]
		queue = queue[1:]

		// Fetch the page
		fetchResult, err := c.fetcher.Fetch(ctx, current.url)
		if err != nil {
			// Record the error but continue
			results = append(results, VisitedURL{
				URL:       current.url,
				Depth:     current.depth,
				ParentURL: current.parentURL,
				Error:     err,
			})
			continue
		}

		// Check if content is HTML
		if !isHTMLContent(fetchResult.ContentType) {
			// Record as visited but don't extract links
			_ = fetchResult.Body.Close()
			results = append(results, VisitedURL{
				URL:       current.url,
				Depth:     current.depth,
				ParentURL: current.parentURL,
				Error:     nil,
			})
			continue
		}

		// Extract links from HTML
		links, err := extractLinks(fetchResult.Body, current.url)
		_ = fetchResult.Body.Close()

		if err != nil {
			// Record error but mark as visited
			results = append(results, VisitedURL{
				URL:       current.url,
				Depth:     current.depth,
				ParentURL: current.parentURL,
				Error:     fmt.Errorf("extract links: %w", err),
			})
			continue
		}

		// Successfully processed
		results = append(results, VisitedURL{
			URL:       current.url,
			Depth:     current.depth,
			ParentURL: current.parentURL,
			Error:     nil,
		})

		// Don't follow links if we're at max depth
		if current.depth >= maxDepth {
			continue
		}

		// Process discovered links
		for _, link := range links {
			// Skip if already visited
			if visited[link] {
				continue
			}

			// Skip if not same domain
			if !isSameDomain(normalizedRoot, link) {
				continue
			}

			// Skip if file extension should be filtered
			if shouldSkipURL(link) {
				continue
			}

			// Add to queue and mark as visited
			visited[link] = true
			queue = append(queue, queueItem{
				url:       link,
				depth:     current.depth + 1,
				parentURL: current.url,
			})
		}
	}

	return &Result{URLs: results}, nil
}
