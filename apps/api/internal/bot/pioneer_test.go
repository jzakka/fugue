package bot

import (
	"testing"
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

		// Detail pages (numeric ID) - should not match if keyword present
		{"https://example.com/item/12345", NodeTypeDetail},
		{"https://example.com/post/987654321", NodeTypeDetail},

		// Skip pages
		{"https://example.com/login", NodeTypeSkip},
		{"https://example.com/signup", NodeTypeSkip},
		{"https://example.com/ad/banner", NodeTypeSkip},
		{"https://example.com/cart", NodeTypeSkip},
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
