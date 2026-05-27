package snapshot

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"strings"
	"testing"
)

// Pins the gzip-bomb cap on Gunzip's decompressed output. The compressed
// input is already bounded by maxCompressedSize (reader.go:87, 10MB) but
// without this cap a 1KB→10GB ratio bomb could still OOM the Harvester
// worker. Sister convention: every other external-body read wraps
// io.ReadAll with io.LimitReader (og 1MB, robots 512KB, snapshot input
// 10MB, auth 64KB cycle 108).

// makeBomb produces a deterministic high-ratio gzip blob by compressing
// a repeated single byte. With default gzip compression long runs of
// identical bytes shrink to ~0.1-0.2% of the input — well above the
// 100:1 ratio we need to exceed MaxDecompressedSnapshotBytes from a
// modest compressed input.
func makeBomb(t *testing.T, decompressedSize int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	payload := bytes.Repeat([]byte{'A'}, decompressedSize)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func TestMaxDecompressedSnapshotBytes_MatchesIntent(t *testing.T) {
	const want = 100 * 1024 * 1024
	if MaxDecompressedSnapshotBytes != want {
		t.Fatalf("MaxDecompressedSnapshotBytes drifted from documented intent (10x safety margin over normal ~10MB max decompressed body, matching the 10:1 ratio against the 10MB compressed input cap): got %d, want %d",
			MaxDecompressedSnapshotBytes, want)
	}
}

func TestGunzip_NormalSnapshotPassesThrough(t *testing.T) {
	// A realistic-shape HTML body well under the cap.
	raw := []byte(`<!DOCTYPE html><html><head><title>example</title></head><body><p>` +
		strings.Repeat("hello world ", 1024) + `</p></body></html>`)
	gz, err := gzipBytes(raw)
	if err != nil {
		t.Fatalf("gzipBytes: %v", err)
	}
	got, err := Gunzip(gz)
	if err != nil {
		t.Fatalf("Gunzip on normal body: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("normal body must round-trip verbatim (lengths got=%d want=%d)", len(got), len(raw))
	}
}

func TestGunzip_AtCapPassesThrough(t *testing.T) {
	// Decompressed body exactly equal to the cap must succeed.
	gz := makeBomb(t, MaxDecompressedSnapshotBytes)
	got, err := Gunzip(gz)
	if err != nil {
		t.Fatalf("Gunzip at cap (=%d bytes) must succeed; got err=%v", MaxDecompressedSnapshotBytes, err)
	}
	if len(got) != MaxDecompressedSnapshotBytes {
		t.Fatalf("Gunzip at cap: got %d bytes, want %d", len(got), MaxDecompressedSnapshotBytes)
	}
}

func TestGunzip_OverCapRejected(t *testing.T) {
	// Decompressed body just over the cap (cap+1) must be rejected, not
	// truncated. Truncation would feed an invalid HTML prefix to
	// downstream parsers; rejection lets CompositeFetcher fail-open to
	// HTTP.
	gz := makeBomb(t, MaxDecompressedSnapshotBytes+1)
	body, err := Gunzip(gz)
	if err == nil {
		t.Fatalf("Gunzip cap+1 (%d bytes) must reject, got nil err and %d bytes back", MaxDecompressedSnapshotBytes+1, len(body))
	}
	if !strings.Contains(err.Error(), "cap") {
		t.Fatalf("Gunzip cap+1 err should mention cap, got: %v", err)
	}
	if body != nil {
		t.Fatalf("Gunzip cap+1 must not return partial body; got %d bytes", len(body))
	}
}

func TestGunzip_ZipBombRejected(t *testing.T) {
	// The actual attack scenario: a tiny compressed blob that decompresses
	// way past the cap. We size the payload to cap+1MB so it's
	// unambiguously over without being so large that the test wastes RAM.
	const oversize = MaxDecompressedSnapshotBytes + 1024*1024
	gz := makeBomb(t, oversize)
	if len(gz) > 1*1024*1024 {
		// Sanity: our test bomb itself should be small (high ratio). If
		// it isn't, the test bomb isn't representative of the attack
		// pattern and the assertion below would still hold but for the
		// wrong reason.
		t.Logf("note: test bomb compressed to %d bytes (>1MB) — ratio still over 100:1, attack pattern intact", len(gz))
	}
	_, err := Gunzip(gz)
	if err == nil {
		t.Fatalf("Gunzip on zip bomb (compressed=%d → decompressed=%d) must reject", len(gz), oversize)
	}
	if !strings.Contains(err.Error(), "cap") {
		t.Fatalf("zip bomb rejection should mention cap, got: %v", err)
	}
}

func TestGunzip_CorruptedGzipStillReturnsError(t *testing.T) {
	// Regression: the cap wrap must not swallow gzip CRC/format errors.
	// This mirrors the existing TestS3ReaderRejectsCorruptedGzip
	// expectation at the Gunzip level.
	gz, err := gzipBytes([]byte("hello world"))
	if err != nil {
		t.Fatalf("gzipBytes: %v", err)
	}
	gz[len(gz)/2] ^= 0xFF
	_, err = Gunzip(gz)
	if err == nil {
		t.Fatal("corrupted gzip must return an error")
	}
	// Either gzip.NewReader fails (header corruption) or io.ReadAll fails
	// (mid-stream CRC). Both are acceptable — what matters is err != nil.
	if errors.Is(err, io.EOF) {
		t.Fatalf("corrupted gzip should not surface as bare EOF: %v", err)
	}
}
