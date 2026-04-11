package crawler

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBFSCrawler_DepthZero(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 0)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Should only visit root URL
	if len(result.URLs) != 1 {
		t.Errorf("Expected 1 URL, got %d", len(result.URLs))
	}

	if result.URLs[0].Depth != 0 {
		t.Errorf("Expected depth 0, got %d", result.URLs[0].Depth)
	}
}

func TestBFSCrawler_DepthOne(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 1)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Should visit root + 2 pages (page1, page2)
	if len(result.URLs) != 3 {
		t.Errorf("Expected 3 URLs, got %d", len(result.URLs))
	}

	// Check depths
	depthCounts := make(map[int]int)
	for _, url := range result.URLs {
		depthCounts[url.Depth]++
	}

	if depthCounts[0] != 1 {
		t.Errorf("Expected 1 URL at depth 0, got %d", depthCounts[0])
	}
	if depthCounts[1] != 2 {
		t.Errorf("Expected 2 URLs at depth 1, got %d", depthCounts[1])
	}
}

func TestBFSCrawler_DepthTwo(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 2)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Should visit root + page1 + page2 + sub1
	if len(result.URLs) != 4 {
		t.Errorf("Expected 4 URLs, got %d", len(result.URLs))
	}

	// Verify BFS order: all depth 0, then all depth 1, then all depth 2
	for i := 0; i < len(result.URLs)-1; i++ {
		if result.URLs[i].Depth > result.URLs[i+1].Depth {
			t.Errorf("BFS order violated: depth %d followed by depth %d",
				result.URLs[i].Depth, result.URLs[i+1].Depth)
		}
	}
}

func TestBFSCrawler_MaxDepthEnforced(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 1)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// No URL should have depth > 1
	for _, url := range result.URLs {
		if url.Depth > 1 {
			t.Errorf("Found URL with depth %d > maxDepth 1: %s", url.Depth, url.URL)
		}
	}
}

func TestBFSCrawler_SameDomainOnly(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 2)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// All URLs should be from example.com
	for _, url := range result.URLs {
		if !isSameDomain("http://example.com/", url.URL) {
			t.Errorf("Found URL from different domain: %s", url.URL)
		}
	}
}

func TestBFSCrawler_ExternalDomainExcluded(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 2)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Should not contain external.com
	for _, url := range result.URLs {
		if isSameDomain("https://external.com/", url.URL) {
			t.Errorf("External domain was crawled: %s", url.URL)
		}
	}
}

func TestBFSCrawler_SubdomainExcluded(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 2)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Should not contain sub.example.com
	for _, url := range result.URLs {
		if isSameDomain("https://sub.example.com/", url.URL) {
			t.Errorf("Subdomain was crawled: %s", url.URL)
		}
	}
}

func TestBFSCrawler_DuplicatePrevention(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/", 2)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Check for duplicate URLs
	seen := make(map[string]bool)
	for _, url := range result.URLs {
		normalized, _ := normalizeURL(url.URL)
		if seen[normalized] {
			t.Errorf("Duplicate URL found: %s", url.URL)
		}
		seen[normalized] = true
	}
}

func TestURLNormalization_RelativePath(t *testing.T) {
	base := "http://example.com/page1"
	rel := "/page2"

	abs, err := makeAbsoluteURL(base, rel)
	if err != nil {
		t.Fatalf("makeAbsoluteURL failed: %v", err)
	}

	expected := "http://example.com/page2"
	if abs != expected {
		t.Errorf("Expected %s, got %s", expected, abs)
	}
}

func TestURLNormalization_TrailingSlash(t *testing.T) {
	url1 := "http://example.com/page/"
	url2 := "http://example.com/page"

	norm1, _ := normalizeURL(url1)
	norm2, _ := normalizeURL(url2)

	if norm1 != norm2 {
		t.Errorf("Trailing slash normalization failed: %s vs %s", norm1, norm2)
	}
}

func TestFileExtensionFiltering(t *testing.T) {
	tests := []struct {
		url    string
		should string
	}{
		{"http://example.com/image.jpg", "skip"},
		{"http://example.com/page.html", "keep"},
		{"http://example.com/page", "keep"},
		{"http://example.com/doc.pdf", "skip"},
		{"http://example.com/style.css", "skip"},
	}

	for _, tt := range tests {
		shouldSkip := shouldSkipURL(tt.url)
		if tt.should == "skip" && !shouldSkip {
			t.Errorf("Expected to skip %s, but didn't", tt.url)
		}
		if tt.should == "keep" && shouldSkip {
			t.Errorf("Expected to keep %s, but skipped", tt.url)
		}
	}
}

func TestBFSCrawler_FetchError(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	// Try to crawl a non-existent page
	result, err := crawler.Crawl(context.Background(), "http://example.com/nonexistent", 0)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Should have 1 result with error
	if len(result.URLs) != 1 {
		t.Errorf("Expected 1 URL, got %d", len(result.URLs))
	}

	if result.URLs[0].Error == nil {
		t.Error("Expected error for non-existent page, got nil")
	}
}

func TestBFSCrawler_EmptySite(t *testing.T) {
	testdataPath, _ := filepath.Abs("testdata")
	fetcher := NewFileFetcher(testdataPath)
	crawler := NewBFSCrawler(fetcher)

	result, err := crawler.Crawl(context.Background(), "http://example.com/empty", 1)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Should only have the root page
	if len(result.URLs) != 1 {
		t.Errorf("Expected 1 URL for empty site, got %d", len(result.URLs))
	}
}

func TestContentTypeFiltering(t *testing.T) {
	tests := []struct {
		contentType string
		isHTML      bool
	}{
		{"text/html", true},
		{"text/html; charset=utf-8", true},
		{"application/xhtml+xml", true},
		{"image/jpeg", false},
		{"application/json", false},
		{"text/plain", false},
	}

	for _, tt := range tests {
		result := isHTMLContent(tt.contentType)
		if result != tt.isHTML {
			t.Errorf("For %s: expected isHTML=%v, got %v", tt.contentType, tt.isHTML, result)
		}
	}
}
