package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Test URL classification
func TestClassifyURL(t *testing.T) {
	tests := []struct {
		url      string
		expected NodeType
	}{
		// Listing pages
		{"https://example.com/trending", NodeTypeListing},
		{"https://example.com/popular", NodeTypeListing},
		{"https://example.com/shots/recent", NodeTypeListing},

		// Gallery pages
		{"https://example.com/gallery", NodeTypeGallery},
		{"https://example.com/collections/art", NodeTypeGallery},

		// Category pages
		{"https://example.com/category/design", NodeTypeCategory},
		{"https://example.com/tags/illustration", NodeTypeCategory},
		{"https://example.com/contest/magicalparty", NodeTypeCategory},
		{"https://example.com/event/summer2026", NodeTypeCategory},

		// Detail pages (numeric ID in path)
		{"https://example.com/item/12345", NodeTypeDetail},
		{"https://example.com/post/987654321", NodeTypeDetail},

		// Detail pages (query parameter ID)
		{"https://www.pixiv.net/member.php?id=30988235", NodeTypeDetail},
		{"https://www.pixiv.net/artworks/12345678", NodeTypeDetail},
		{"https://www.pixiv.net/en/artworks/99999", NodeTypeDetail},
		{"https://example.com/view?illust_id=456789", NodeTypeDetail},

		// Detail pages (content singular path)
		{"https://unsplash.com/photos/abc123", NodeTypeDetail},
		{"https://example.com/works/my-piece", NodeTypeDetail},

		// Pagination is NOT detail (?p= should remain listing)
		{"https://example.com/page", NodeTypeListing},

		// Skip pages
		{"https://example.com/login", NodeTypeSkip},
		{"https://example.com/signup", NodeTypeSkip},
		{"https://example.com/ad/banner", NodeTypeSkip},
		{"https://example.com/cart", NodeTypeSkip},
		{"https://example.com/checkout/step1", NodeTypeSkip},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := classifyURL(tt.url)
			if result != tt.expected {
				t.Errorf("classifyURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

// Test domain validation
func TestIsSameDomain(t *testing.T) {
	tests := []struct {
		url        string
		rootDomain string
		expected   bool
	}{
		// Same domain (with www normalization)
		{"https://example.com/page", "example.com", true},
		{"https://www.example.com/page", "example.com", true},
		{"https://example.com/page", "www.example.com", true},

		// Different subdomain - should be blocked
		{"https://ads.example.com/page", "example.com", false},
		{"https://blog.example.com/page", "example.com", false},

		// External domain
		{"https://other.com/page", "example.com", false},
		{"https://google.com/page", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := isSameDomain(tt.url, tt.rootDomain)
			if result != tt.expected {
				t.Errorf("isSameDomain(%q, %q) = %v, want %v", tt.url, tt.rootDomain, result, tt.expected)
			}
		})
	}
}

// Test file extension filtering
func TestHasExcludedExtension(t *testing.T) {
	tests := []struct {
		url      string
		excluded bool
	}{
		// Should be excluded
		{"https://example.com/image.jpg", true},
		{"https://example.com/photo.PNG", true},
		{"https://example.com/video.mp4", true},
		{"https://example.com/audio.mp3", true},
		{"https://example.com/doc.pdf", true},
		{"https://example.com/style.css", true},
		{"https://example.com/script.js", true},

		// Should NOT be excluded
		{"https://example.com/page", false},
		{"https://example.com/article", false},
		{"https://example.com/shots/12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := hasExcludedExtension(tt.url)
			if result != tt.excluded {
				t.Errorf("hasExcludedExtension(%q) = %v, want %v", tt.url, result, tt.excluded)
			}
		})
	}
}

// Test script validation threshold
func TestEstimateItemCount(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected int
	}{
		{
			name:     "HTML with images",
			html:     `<img src="1.jpg"><img src="2.jpg"><img src="3.jpg">`,
			expected: 3,
		},
		{
			name:     "HTML with cards",
			html:     `<div class="card"></div><div class="card"></div>`,
			expected: 2,
		},
		{
			name:     "HTML with articles",
			html:     `<article></article><article></article><article></article><article></article>`,
			expected: 4,
		},
		{
			name:     "Empty HTML",
			html:     ``,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateItemCount(tt.html)
			if result != tt.expected {
				t.Errorf("estimateItemCount() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test that Pioneer creates edges during crawl
func TestPioneerCreatesEdges(t *testing.T) {
	// Create test HTTP server with pages that link to each other
	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body><a href="%s/trending">trending</a><a href="%s/popular">popular</a></body></html>`, serverURL, serverURL)
	})
	mux.HandleFunc("/trending", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body><a href="%s/item/12345">detail</a></body></html>`, serverURL)
	})
	mux.HandleFunc("/popular", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body><a href="%s/trending">trending again</a></body></html>`, serverURL)
	})
	mux.HandleFunc("/item/12345", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>detail page</body></html>`)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	serverURL = ts.URL

	// Setup mocks
	siteID := uuid.New()
	siteRepo := NewMockSiteRepository()
	siteRepo.Sites[siteID] = db.BotSite{
		ID:      siteID,
		Domain:  "127.0.0.1",
		RootUrl: serverURL + "/",
		Active:  true,
	}

	graphRepo := NewMockGraphRepository()
	scriptRepo := NewMockScriptRepository()
	aiClient := NewMockAIClient()
	executor := NewMockScriptExecutor()

	pioneer := NewPioneer(siteRepo, graphRepo, scriptRepo, aiClient, executor, PioneerConfig{
		MaxNodesPerSite:  10,
		RateLimitMs:      0,
		SuccessThreshold: 0.7,
	})

	err := pioneer.Run(context.Background(), siteID)
	if err != nil {
		t.Fatalf("Pioneer.Run() error: %v", err)
	}

	// Verify nodes were created
	if len(graphRepo.Nodes) == 0 {
		t.Fatal("Expected nodes to be created, got 0")
	}

	// Verify edges were created
	if len(graphRepo.Edges) == 0 {
		t.Fatal("Expected edges to be created, got 0")
	}

	// Verify at least one edge exists (root → child)
	t.Logf("Created %d nodes and %d edges", len(graphRepo.Nodes), len(graphRepo.Edges))

	// Verify no duplicate edges
	edgeSet := make(map[string]bool)
	for _, e := range graphRepo.Edges {
		key := e.FromNodeID.String() + "->" + e.ToNodeID.String()
		if edgeSet[key] {
			t.Errorf("Duplicate edge found: %s", key)
		}
		edgeSet[key] = true
	}
}

// Test NodeType priority
func TestNodeTypePriority(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		priority int
	}{
		{NodeTypeListing, 100},
		{NodeTypeGallery, 80},
		{NodeTypeCategory, 60},
		{NodeTypeDetail, 10},
		{NodeTypeSkip, 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.nodeType), func(t *testing.T) {
			result := NodeTypePriority(tt.nodeType)
			if result != tt.priority {
				t.Errorf("NodeTypePriority(%v) = %v, want %v", tt.nodeType, result, tt.priority)
			}
		})
	}
}
