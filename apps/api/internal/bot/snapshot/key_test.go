package snapshot

import (
	"regexp"
	"testing"
	"time"
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestHashNormalizedURL_IsDeterministic(t *testing.T) {
	t.Parallel()
	const url = "https://example.com/a/b"
	h1 := HashNormalizedURL(url)
	h2 := HashNormalizedURL(url)
	if h1 != h2 {
		t.Fatalf("expected deterministic hash, got %q and %q", h1, h2)
	}
	if !hex64.MatchString(h1) {
		t.Fatalf("expected 64-char lowercase hex, got %q", h1)
	}
}

func TestHashNormalizedURL_DiffersForDiffInputs(t *testing.T) {
	t.Parallel()
	a := HashNormalizedURL("https://example.com/a")
	b := HashNormalizedURL("https://example.com/b")
	if a == b {
		t.Fatalf("different URLs produced same hash: %q", a)
	}
}

func TestSnapshotKey_FormatAndUTC(t *testing.T) {
	t.Parallel()
	// A time in Asia/Seoul (UTC+9) that is still 2026-04-17 in UTC.
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Skip("Asia/Seoul tzdata not available:", err)
	}
	local := time.Date(2026, 4, 18, 5, 0, 0, 0, loc) // 2026-04-17 20:00 UTC

	got := SnapshotKey("https://example.com/page", local)

	// Build expected manually to lock the behavior contract.
	hash := HashNormalizedURL("https://example.com/page")
	want := "snapshots/" + hash + "/20260417.html.gz"
	if got != want {
		t.Fatalf("SnapshotKey mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestSnapshotKey_SameDayOverwrites(t *testing.T) {
	t.Parallel()
	morning := time.Date(2026, 4, 17, 1, 0, 0, 0, time.UTC)
	evening := time.Date(2026, 4, 17, 23, 59, 0, 0, time.UTC)
	if SnapshotKey("https://x.example/y", morning) != SnapshotKey("https://x.example/y", evening) {
		t.Fatal("same-day keys should collide to enable overwrite semantics")
	}
}

func TestSnapshotKey_MatchesPattern(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	key := SnapshotKey("https://example.com/", now)
	re := regexp.MustCompile(`^snapshots/[0-9a-f]{64}/\d{8}\.html\.gz$`)
	if !re.MatchString(key) {
		t.Fatalf("key %q does not match SnapshotKeyPattern shape", key)
	}
}
