package bot

import (
	"strings"
	"testing"
)

func longBody(n int) string {
	return strings.Repeat("a", n)
}

func TestClassifier_Listing(t *testing.T) {
	c := NewClassifier()
	doc := PinDocument{BodyText: longBody(500)}
	pinnable, reason := c.Classify(doc, LinkStats{Links: 60, Words: 100})
	if pinnable {
		t.Fatalf("expected pinnable=false")
	}
	if reason != ClassifierReasonListing {
		t.Fatalf("reason = %q, want listing", reason)
	}
}

func TestClassifier_ListingDivisionByZeroGuard(t *testing.T) {
	c := NewClassifier()
	// Thumbnail present so neither empty_body nor no_primary_media masks
	// the division-by-zero regression we are guarding against.
	doc := PinDocument{
		BodyText:     longBody(500),
		ThumbnailURL: "https://cdn.example.com/x.jpg",
	}
	// Word count zero must NOT trigger listing — even with many links.
	pinnable, reason := c.Classify(doc, LinkStats{Links: 100, Words: 0})
	if !pinnable {
		t.Fatalf("expected pinnable=true (reason=%q)", reason)
	}
}

func TestClassifier_EmptyBody(t *testing.T) {
	c := NewClassifier()
	doc := PinDocument{
		BodyText:     "short",
		ThumbnailURL: "https://cdn.example.com/x.jpg",
	}
	pinnable, reason := c.Classify(doc, LinkStats{})
	if pinnable {
		t.Fatalf("expected pinnable=false")
	}
	if reason != ClassifierReasonEmptyBody {
		t.Fatalf("reason = %q, want empty_body", reason)
	}
}

func TestClassifier_BodyJustAtThreshold(t *testing.T) {
	c := NewClassifier()
	// At exactly the threshold the body is NOT < threshold, so it passes.
	doc := PinDocument{
		BodyText:     longBody(DefaultBodyTextMinBytes),
		ThumbnailURL: "https://cdn.example.com/x.jpg",
	}
	pinnable, _ := c.Classify(doc, LinkStats{})
	if !pinnable {
		t.Fatalf("body at threshold should pass")
	}
}

func TestClassifier_PriorityListingOverEmptyBody(t *testing.T) {
	c := NewClassifier()
	// Body short AND link density high → listing wins.
	doc := PinDocument{BodyText: "short"}
	pinnable, reason := c.Classify(doc, LinkStats{Links: 60, Words: 100})
	if pinnable {
		t.Fatal("expected pinnable=false")
	}
	if reason != ClassifierReasonListing {
		t.Fatalf("reason = %q, want listing (priority over empty_body)", reason)
	}
}

func TestClassifier_NoPrimaryMedia(t *testing.T) {
	c := NewClassifier()
	// Body long enough to skip empty_body, no thumbnail, no candidates.
	doc := PinDocument{BodyText: longBody(500)}
	pinnable, reason := c.Classify(doc, LinkStats{Links: 1, Words: 100})
	if pinnable {
		t.Fatalf("expected pinnable=false")
	}
	if reason != ClassifierReasonNoPrimaryMedia {
		t.Fatalf("reason = %q, want no_primary_media", reason)
	}
}

func TestClassifier_NormalPagePasses(t *testing.T) {
	c := NewClassifier()
	doc := PinDocument{
		BodyText:     longBody(500),
		ThumbnailURL: "https://cdn.example.com/x.jpg",
	}
	pinnable, reason := c.Classify(doc, LinkStats{Links: 5, Words: 100})
	if !pinnable {
		t.Fatalf("expected pinnable=true, reason=%q", reason)
	}
	if reason != "" {
		t.Fatalf("reason should be empty when pinnable, got %q", reason)
	}
}

func TestClassifier_DependsOnDocOnly(t *testing.T) {
	// Method signature regression: Classify must accept (PinDocument, LinkStats)
	// and nothing more. node_type or external state must not be a parameter.
	c := NewClassifier()
	_, _ = c.Classify(PinDocument{}, LinkStats{})
}
