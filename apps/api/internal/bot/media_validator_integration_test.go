package bot

import (
	"context"
	"testing"
)

// TestFilterValidMedia_FlowsThroughClassifier verifies that when every media
// candidate is invalid, the resulting PinDocument matches the existing
// classifier's `no_primary_media` precondition (empty thumbnail + empty
// candidates). This is the regression contract for the QA-reported 1×1 GIF
// bug — the spec contract is "PinDocument의 candidates/thumbnail이 빈 상태로
// 구성됨" (tasks 2.4).
func TestFilterValidMedia_AllInvalid_TriggersNoPrimaryMedia(t *testing.T) {
	doc := PinDocument{
		Title:    "Page",
		BodyText: longBody(500),
		// Every media URL is the QA-reported placeholder pattern.
		ThumbnailURL: "https://x/placeholder.gif",
		MediaCandidates: []MediaCandidate{
			{Type: "image", URL: "https://x/placeholder.gif"},
		},
	}
	stub := &stubValidator{results: map[string]MediaValidationResult{
		"https://x/placeholder.gif": {Valid: false, Reason: MediaValidationImageTooSmall},
	}}
	FilterValidMedia(context.Background(), stub, &doc)

	// Spec: candidates/thumbnail empty after rejection.
	if doc.ThumbnailURL != "" {
		t.Fatalf("ThumbnailURL not cleared: %q", doc.ThumbnailURL)
	}
	if len(doc.MediaCandidates) != 0 {
		t.Fatalf("MediaCandidates not emptied: %d remain", len(doc.MediaCandidates))
	}

	// Existing classifier should now route this to no_primary_media (regression test).
	c := NewClassifier()
	pinnable, reason := c.Classify(doc, LinkStats{Links: 1, Words: 100})
	if pinnable {
		t.Fatal("expected pinnable=false after validation rejected all candidates")
	}
	if reason != ClassifierReasonNoPrimaryMedia {
		t.Fatalf("expected no_primary_media reason, got %q", reason)
	}

	// Spec: og_data exposes rejection count + reasons. The same URL appears
	// as both ThumbnailURL and MediaCandidates[0].URL, but the rejection
	// tally deduplicates by URL so the operator sees one rejected media
	// reference, not two.
	if doc.OGData.MediaValidation == nil {
		t.Fatal("expected MediaValidation record on og_data")
	}
	if doc.OGData.MediaValidation.RejectedCount != 1 {
		t.Fatalf("expected 1 rejection (dedup by URL), got %d", doc.OGData.MediaValidation.RejectedCount)
	}
	if doc.OGData.MediaValidation.Reasons["image_too_small"] != 1 {
		t.Fatalf("expected image_too_small=1 after dedup, got %d", doc.OGData.MediaValidation.Reasons["image_too_small"])
	}
}

// TestFilterValidMedia_PartialInvalid_StillPinnable verifies that a mix of
// valid and invalid candidates produces a pinnable document with only the
// valid ones retained — and that og_data still records the rejection (tasks 2.5,
// 4.2).
func TestFilterValidMedia_PartialInvalid_StillPinnable(t *testing.T) {
	doc := PinDocument{
		Title:        "Page",
		BodyText:     longBody(500),
		ThumbnailURL: "https://x/good.png",
		MediaCandidates: []MediaCandidate{
			{Type: "image", URL: "https://x/good.png"},
			{Type: "image", URL: "https://x/bad.gif"},
		},
	}
	stub := &stubValidator{results: map[string]MediaValidationResult{
		"https://x/good.png": {Valid: true, Reason: MediaValidationOK, Width: 800, Height: 600},
		"https://x/bad.gif":  {Valid: false, Reason: MediaValidationDecodeFailed},
	}}
	FilterValidMedia(context.Background(), stub, &doc)

	if doc.ThumbnailURL != "https://x/good.png" {
		t.Fatalf("expected good thumbnail kept, got %q", doc.ThumbnailURL)
	}
	if len(doc.MediaCandidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(doc.MediaCandidates))
	}

	c := NewClassifier()
	pinnable, _ := c.Classify(doc, LinkStats{Links: 1, Words: 100})
	if !pinnable {
		t.Fatal("expected pinnable=true with one valid candidate")
	}

	if doc.OGData.MediaValidation == nil ||
		doc.OGData.MediaValidation.RejectedCount != 1 ||
		doc.OGData.MediaValidation.Reasons["decode_failed"] != 1 {
		t.Fatalf("og_data validation record wrong: %+v", doc.OGData.MediaValidation)
	}
}
