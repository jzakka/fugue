package snapshot

import (
	"fmt"
	"regexp"
	"testing"
	"time"
)

func TestHashNormalizedURL_DeterministicAnd64HexLowercase(t *testing.T) {
	const u = "https://example.com/page"

	h1 := HashNormalizedURL(u)
	h2 := HashNormalizedURL(u)
	if h1 != h2 {
		t.Fatalf("expected deterministic hash, got %q vs %q", h1, h2)
	}

	hexRe := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !hexRe.MatchString(h1) {
		t.Fatalf("expected 64-char lowercase hex, got %q (len=%d)", h1, len(h1))
	}

	other := HashNormalizedURL("https://example.com/different")
	if other == h1 {
		t.Fatalf("expected distinct hash for different URL")
	}
}

func TestSnapshotKey_FormatAndUTCDate(t *testing.T) {
	t1 := time.Date(2026, 4, 17, 23, 30, 0, 0, time.UTC)

	got := SnapshotKey("https://example.com/page", t1)

	wantPrefix := "snapshots/"
	wantSuffix := "/20260417.html.gz"
	if len(got) <= len(wantPrefix)+len(wantSuffix) {
		t.Fatalf("key too short: %q", got)
	}
	if got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("missing prefix: %q", got)
	}
	if got[len(got)-len(wantSuffix):] != wantSuffix {
		t.Fatalf("missing suffix: %q", got)
	}

	hexSeg := got[len(wantPrefix) : len(got)-len(wantSuffix)]
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hexSeg) {
		t.Fatalf("hex segment invalid: %q", hexSeg)
	}
}

func TestSnapshotKey_NonUTCInputNormalizedToUTC(t *testing.T) {
	// 2026-04-17 03:00 in JST (UTC+9) = 2026-04-16 18:00 UTC
	jst := time.FixedZone("JST", 9*60*60)
	tIn := time.Date(2026, 4, 17, 3, 0, 0, 0, jst)

	got := SnapshotKey("https://example.com/x", tIn)

	want := "20260416.html.gz"
	if got[len(got)-len(want):] != want {
		t.Fatalf("expected UTC-normalized date %q, got key %q", want, got)
	}
}

func TestSnapshotKey_MatchesPattern(t *testing.T) {
	t1 := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	got := SnapshotKey("https://example.com/page", t1)

	want := fmt.Sprintf(SnapshotKeyPattern, HashNormalizedURL("https://example.com/page"), "20260417")
	if got != want {
		t.Fatalf("SnapshotKey not consistent with SnapshotKeyPattern:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestSnapshotKey_SameURLSameDayProducesSameKey(t *testing.T) {
	t1 := time.Date(2026, 4, 17, 1, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 17, 23, 59, 0, 0, time.UTC)

	if SnapshotKey("https://e.com/a", t1) != SnapshotKey("https://e.com/a", t2) {
		t.Fatalf("same URL on same UTC day must produce same key")
	}
}
