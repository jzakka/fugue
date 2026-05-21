package bot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestFilterOverlongMediaURLs covers the four Scenarios from the spec ADDED
// Requirement "ProcessDocument의 media_url 후보는 pins.media_url 컬럼 cap에
// 맞춰 사전 차단된다":
//
//   - 501 rune ThumbnailURL → cleared (picker falls back to MediaCandidates).
//   - All candidates > 500 runes → ThumbnailURL "" + MediaCandidates empty
//     (classifier no_primary_media short-circuits to skipped+harvested).
//   - ≤500 rune URLs → preserved lossless.
//   - Multibyte URL (percent-encoded UTF-8) → rune-count comparison, not
//     byte-count.
func TestFilterOverlongMediaURLs(t *testing.T) {
	const runeCap = pinsMediaURLRuneCap // 500

	// A 501-char ASCII URL: passes URL syntax checks elsewhere; the only
	// thing that matters here is rune length > cap.
	overlongASCII := "https://cdn.example.com/" + strings.Repeat("a", runeCap-len("https://cdn.example.com/")+1)
	if got := utf8.RuneCountInString(overlongASCII); got != runeCap+1 {
		t.Fatalf("test setup: overlongASCII rune len = %d, want %d", got, runeCap+1)
	}
	shortURL := "https://cdn.example.com/image.jpg"

	// A multibyte URL that overflows on rune count but whose byte count
	// far exceeds the rune count. The cap is rune-based so this must be
	// dropped; a byte-based check would erroneously drop a much shorter URL.
	multibyteOverlong := "https://cdn.example.com/" + strings.Repeat("가", runeCap)
	if got := utf8.RuneCountInString(multibyteOverlong); got <= runeCap {
		t.Fatalf("test setup: multibyteOverlong rune len = %d, expected > %d", got, runeCap)
	}

	t.Run("overlong ThumbnailURL is cleared", func(t *testing.T) {
		doc := PinDocument{
			ThumbnailURL: overlongASCII,
			MediaCandidates: []MediaCandidate{
				{Type: "image", URL: shortURL},
			},
		}
		filterOverlongMediaURLs(&doc, "http://src.example/page")

		if doc.ThumbnailURL != "" {
			t.Errorf("ThumbnailURL = %q, want \"\" (overlong should be cleared)", doc.ThumbnailURL)
		}
		// Short candidate must survive so picker can fall back to it.
		if len(doc.MediaCandidates) != 1 || doc.MediaCandidates[0].URL != shortURL {
			t.Errorf("MediaCandidates = %+v, want [{image %q}]", doc.MediaCandidates, shortURL)
		}
	})

	t.Run("short ThumbnailURL is preserved", func(t *testing.T) {
		doc := PinDocument{ThumbnailURL: shortURL}
		filterOverlongMediaURLs(&doc, "http://src.example/page")

		if doc.ThumbnailURL != shortURL {
			t.Errorf("ThumbnailURL = %q, want %q (within cap must be preserved)", doc.ThumbnailURL, shortURL)
		}
	})

	t.Run("exactly cap-rune URL is preserved (boundary)", func(t *testing.T) {
		exactCap := "https://cdn.example.com/" + strings.Repeat("b", runeCap-len("https://cdn.example.com/"))
		if got := utf8.RuneCountInString(exactCap); got != runeCap {
			t.Fatalf("test setup: exactCap rune len = %d, want %d", got, runeCap)
		}
		doc := PinDocument{
			ThumbnailURL: exactCap,
			MediaCandidates: []MediaCandidate{
				{Type: "image", URL: exactCap},
			},
		}
		filterOverlongMediaURLs(&doc, "http://src.example/page")

		if doc.ThumbnailURL != exactCap {
			t.Errorf("ThumbnailURL = %q, want %q (= cap should be preserved)", doc.ThumbnailURL, exactCap)
		}
		if len(doc.MediaCandidates) != 1 || doc.MediaCandidates[0].URL != exactCap {
			t.Errorf("MediaCandidates = %+v, want one entry (= cap should be preserved)", doc.MediaCandidates)
		}
	})

	t.Run("all candidates overlong → empty result (classifier no_primary_media)", func(t *testing.T) {
		doc := PinDocument{
			ThumbnailURL: overlongASCII,
			MediaCandidates: []MediaCandidate{
				{Type: "image", URL: overlongASCII},
				{Type: "video", URL: overlongASCII},
			},
		}
		filterOverlongMediaURLs(&doc, "http://src.example/page")

		if doc.ThumbnailURL != "" {
			t.Errorf("ThumbnailURL = %q, want \"\"", doc.ThumbnailURL)
		}
		if len(doc.MediaCandidates) != 0 {
			t.Errorf("MediaCandidates = %+v, want [] (all overlong)", doc.MediaCandidates)
		}
	})

	t.Run("mixed candidates: overlong dropped, short kept in order", func(t *testing.T) {
		good1 := "https://cdn.example.com/good1.jpg"
		good2 := "https://cdn.example.com/good2.mp4"
		doc := PinDocument{
			ThumbnailURL: shortURL,
			MediaCandidates: []MediaCandidate{
				{Type: "image", URL: good1},
				{Type: "image", URL: overlongASCII},
				{Type: "video", URL: good2},
				{Type: "video", URL: overlongASCII},
			},
		}
		filterOverlongMediaURLs(&doc, "http://src.example/page")

		if doc.ThumbnailURL != shortURL {
			t.Errorf("ThumbnailURL = %q, want preserved %q", doc.ThumbnailURL, shortURL)
		}
		if len(doc.MediaCandidates) != 2 {
			t.Fatalf("MediaCandidates len = %d, want 2 (two overlong dropped)", len(doc.MediaCandidates))
		}
		if doc.MediaCandidates[0].URL != good1 || doc.MediaCandidates[1].URL != good2 {
			t.Errorf("MediaCandidates = %+v, want order [good1, good2]", doc.MediaCandidates)
		}
		if doc.MediaCandidates[0].Type != "image" || doc.MediaCandidates[1].Type != "video" {
			t.Errorf("MediaCandidates Type lost: %+v", doc.MediaCandidates)
		}
	})

	t.Run("multibyte URL overflowing on runes is dropped (not byte-count)", func(t *testing.T) {
		doc := PinDocument{
			ThumbnailURL: multibyteOverlong,
			MediaCandidates: []MediaCandidate{
				{Type: "image", URL: multibyteOverlong},
				{Type: "image", URL: shortURL},
			},
		}
		filterOverlongMediaURLs(&doc, "http://src.example/page")

		if doc.ThumbnailURL != "" {
			t.Errorf("ThumbnailURL = %q, want \"\" (multibyte > 500 runes must be dropped)", doc.ThumbnailURL)
		}
		if len(doc.MediaCandidates) != 1 || doc.MediaCandidates[0].URL != shortURL {
			t.Errorf("MediaCandidates = %+v, want [{image %q}] (multibyte overlong dropped)", doc.MediaCandidates, shortURL)
		}
	})

	t.Run("empty document is a no-op", func(t *testing.T) {
		doc := PinDocument{}
		filterOverlongMediaURLs(&doc, "http://src.example/page")
		if doc.ThumbnailURL != "" {
			t.Errorf("ThumbnailURL unexpected: %q", doc.ThumbnailURL)
		}
		if len(doc.MediaCandidates) != 0 {
			t.Errorf("MediaCandidates unexpected: %+v", doc.MediaCandidates)
		}
	})

	t.Run("nil pointer is safe", func(t *testing.T) {
		// Should not panic.
		filterOverlongMediaURLs(nil, "http://src.example/page")
	})
}
