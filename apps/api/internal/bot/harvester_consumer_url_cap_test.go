package bot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestCapCanonicalURLForPin covers the pins.url VARCHAR(1000) cap enforcement
// on doc.CanonicalURL. CanonicalURL (and its fetchURL fallback) derive from
// harvester_frontier.url (TEXT, unbounded), so without this guard an overlong
// URL fails the pins INSERT and burns retries. Policy mirrors the media_url
// skip-not-truncate decision, with a fetchURL fallback because pins.url is
// NOT NULL and the dedup key:
//
//   - CanonicalURL ≤ cap → kept as-is (true).
//   - CanonicalURL > cap, fetchURL ≤ cap → rewritten to fetchURL (true).
//   - both > cap → page skipped (false).
func TestCapCanonicalURLForPin(t *testing.T) {
	const runeCap = pinsURLRuneCap // 1000

	shortURL := "https://example.com/article/123"

	overlongASCII := "https://example.com/" + strings.Repeat("a", runeCap-len("https://example.com/")+1)
	if got := utf8.RuneCountInString(overlongASCII); got != runeCap+1 {
		t.Fatalf("test setup: overlongASCII rune len = %d, want %d", got, runeCap+1)
	}

	// Multibyte URL that overflows on rune count; the cap is rune-based, so a
	// byte-based check would erroneously trip on a much shorter URL.
	multibyteOverlong := "https://example.com/" + strings.Repeat("가", runeCap)
	if got := utf8.RuneCountInString(multibyteOverlong); got <= runeCap {
		t.Fatalf("test setup: multibyteOverlong rune len = %d, expected > %d", got, runeCap)
	}

	t.Run("CanonicalURL within cap is kept", func(t *testing.T) {
		doc := PinDocument{CanonicalURL: shortURL}
		if ok := capCanonicalURLForPin(&doc, "https://example.com/fetch"); !ok {
			t.Fatalf("ok = false, want true (within cap)")
		}
		if doc.CanonicalURL != shortURL {
			t.Errorf("CanonicalURL = %q, want preserved %q", doc.CanonicalURL, shortURL)
		}
	})

	t.Run("exactly cap-rune CanonicalURL is kept (boundary)", func(t *testing.T) {
		exactCap := "https://example.com/" + strings.Repeat("b", runeCap-len("https://example.com/"))
		if got := utf8.RuneCountInString(exactCap); got != runeCap {
			t.Fatalf("test setup: exactCap rune len = %d, want %d", got, runeCap)
		}
		doc := PinDocument{CanonicalURL: exactCap}
		if ok := capCanonicalURLForPin(&doc, "https://example.com/fetch"); !ok {
			t.Fatalf("ok = false, want true (= cap must be kept)")
		}
		if doc.CanonicalURL != exactCap {
			t.Errorf("CanonicalURL = %q, want preserved (= cap)", doc.CanonicalURL)
		}
	})

	t.Run("overlong CanonicalURL falls back to bounded fetchURL", func(t *testing.T) {
		doc := PinDocument{CanonicalURL: overlongASCII}
		if ok := capCanonicalURLForPin(&doc, shortURL); !ok {
			t.Fatalf("ok = false, want true (fetchURL within cap)")
		}
		if doc.CanonicalURL != shortURL {
			t.Errorf("CanonicalURL = %q, want rewritten to fetchURL %q", doc.CanonicalURL, shortURL)
		}
	})

	t.Run("multibyte overlong CanonicalURL falls back on rune count (not bytes)", func(t *testing.T) {
		doc := PinDocument{CanonicalURL: multibyteOverlong}
		if ok := capCanonicalURLForPin(&doc, shortURL); !ok {
			t.Fatalf("ok = false, want true (fetchURL within cap)")
		}
		if doc.CanonicalURL != shortURL {
			t.Errorf("CanonicalURL = %q, want rewritten to fetchURL %q", doc.CanonicalURL, shortURL)
		}
	})

	t.Run("both overlong → skip (false), CanonicalURL left unchanged", func(t *testing.T) {
		doc := PinDocument{CanonicalURL: overlongASCII}
		if ok := capCanonicalURLForPin(&doc, overlongASCII); ok {
			t.Fatalf("ok = true, want false (both over cap → caller must skip)")
		}
		// On skip we do not rewrite; the caller drops the page entirely.
		if doc.CanonicalURL != overlongASCII {
			t.Errorf("CanonicalURL = %q, want unchanged on skip", doc.CanonicalURL)
		}
	})

	t.Run("exactly cap-rune fetchURL fallback is accepted (fetch boundary)", func(t *testing.T) {
		exactCapFetch := "https://example.com/" + strings.Repeat("c", runeCap-len("https://example.com/"))
		if got := utf8.RuneCountInString(exactCapFetch); got != runeCap {
			t.Fatalf("test setup: exactCapFetch rune len = %d, want %d", got, runeCap)
		}
		doc := PinDocument{CanonicalURL: overlongASCII}
		if ok := capCanonicalURLForPin(&doc, exactCapFetch); !ok {
			t.Fatalf("ok = false, want true (= cap fetchURL must be accepted)")
		}
		if doc.CanonicalURL != exactCapFetch {
			t.Errorf("CanonicalURL = %q, want rewritten to exactCapFetch", doc.CanonicalURL)
		}
	})
}
