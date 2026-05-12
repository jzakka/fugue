package bot

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// build1x1GIF returns the bytes of a 1×1 GIF89a — the exact placeholder
// pattern observed in QA report 2026-04-27 (b2136cc2-... 37 bytes).
func build1x1GIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Transparent, color.Black})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("gif encode: %v", err)
	}
	return buf.Bytes()
}

// buildLargePNG returns a synthesized PNG of the given dimensions filled
// with deterministic pseudo-random pixel data so PNG deflate cannot
// compress below the validator's byte threshold (1024 bytes by default).
// Solid colors and gradients compress too well for that purpose.
func buildLargePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Linear-congruential PRNG for deterministic noise. Good enough to
	// defeat zlib without pulling math/rand.
	seed := uint32(1234567)
	next := func() uint8 {
		seed = seed*1664525 + 1013904223
		return uint8(seed >> 24)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: next(), G: next(), B: next(), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

// fixedBodyServer serves the same payload (with given Content-Type) for
// every request so the validator can exercise its download path.
func fixedBodyServer(t *testing.T, body []byte, contentType string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write(body)
	}))
}

func TestImageValidator_Reject1x1GIF(t *testing.T) {
	srv := fixedBodyServer(t, build1x1GIF(t), "image/gif")
	defer srv.Close()

	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), srv.URL, "image")

	if r.Valid {
		t.Fatalf("expected 1×1 GIF to be rejected, got Valid=true")
	}
	// 37-byte 1×1 GIF should fail the byte-count threshold first; if
	// thresholds change to allow it past bytes, the dimension check should
	// catch it. Either is acceptable evidence of rejection.
	if r.Reason != MediaValidationImageBytesTooFew && r.Reason != MediaValidationImageTooSmall {
		t.Fatalf("unexpected reject reason %q", r.Reason)
	}
}

func TestImageValidator_AcceptNormalPNG(t *testing.T) {
	srv := fixedBodyServer(t, buildLargePNG(t, 128, 128), "image/png")
	defer srv.Close()

	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), srv.URL, "image")

	if !r.Valid {
		t.Fatalf("expected 128x128 PNG to be accepted, got rejected with reason=%q", r.Reason)
	}
	if r.Width != 128 || r.Height != 128 {
		t.Fatalf("expected 128x128 dims, got %dx%d", r.Width, r.Height)
	}
}

func TestImageValidator_RejectCorruptedBytes(t *testing.T) {
	// Random 4 KiB of non-image bytes; passes the byte-length threshold so
	// rejection must come from the decoder.
	body := bytes.Repeat([]byte{0x42}, 4096)
	srv := fixedBodyServer(t, body, "image/png")
	defer srv.Close()

	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), srv.URL, "image")

	if r.Valid {
		t.Fatal("expected corrupted bytes to be rejected")
	}
	if r.Reason != MediaValidationDecodeFailed {
		t.Fatalf("expected decode_failed, got %q", r.Reason)
	}
}

func TestImageValidator_RejectSmallDimensions(t *testing.T) {
	// 16x16 PNG: passes byte-count (image bytes are well over 1KiB) but
	// fails dimensions.
	srv := fixedBodyServer(t, buildLargePNG(t, 16, 16), "image/png")
	defer srv.Close()

	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), srv.URL, "image")

	if r.Valid {
		t.Fatal("expected sub-threshold dimensions to be rejected")
	}
	// Either bytes_too_few (depending on PNG compression of solid color)
	// or image_too_small is acceptable since both signal the same defect.
	if r.Reason != MediaValidationImageTooSmall && r.Reason != MediaValidationImageBytesTooFew {
		t.Fatalf("unexpected reason %q", r.Reason)
	}
}

func TestImageValidator_DownloadFailure(t *testing.T) {
	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), "http://127.0.0.1:1/nope", "image")
	if r.Valid {
		t.Fatal("expected download failure to be rejected")
	}
	if r.Reason != MediaValidationDownloadFailed {
		t.Fatalf("expected download_failed, got %q", r.Reason)
	}
}

func TestImageValidator_UnsupportedType(t *testing.T) {
	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), "http://example.invalid", "weird")
	if r.Valid {
		t.Fatal("expected unsupported type to be rejected")
	}
	if r.Reason != MediaValidationUnsupportedType {
		t.Fatalf("expected unsupported_type, got %q", r.Reason)
	}
}

// stubValidator returns canned results indexed by URL. Tests use it to drive
// FilterValidMedia without the network/ffprobe dependency.
type stubValidator struct {
	results map[string]MediaValidationResult
	calls   int
}

func (s *stubValidator) Validate(_ context.Context, url string, _ string) MediaValidationResult {
	s.calls++
	if r, ok := s.results[url]; ok {
		return r
	}
	return MediaValidationResult{Valid: false, Reason: MediaValidationDownloadFailed}
}

func TestFilterValidMedia_All1x1RejectedClearsDoc(t *testing.T) {
	doc := PinDocument{
		ThumbnailURL: "https://x/thumb.gif",
		MediaCandidates: []MediaCandidate{
			{Type: "image", URL: "https://x/thumb.gif"},
		},
	}
	stub := &stubValidator{results: map[string]MediaValidationResult{
		"https://x/thumb.gif": {Valid: false, Reason: MediaValidationImageTooSmall},
	}}
	FilterValidMedia(context.Background(), stub, &doc)

	if doc.ThumbnailURL != "" {
		t.Fatalf("expected ThumbnailURL cleared, got %q", doc.ThumbnailURL)
	}
	if len(doc.MediaCandidates) != 0 {
		t.Fatalf("expected MediaCandidates emptied, got %d", len(doc.MediaCandidates))
	}
	if doc.OGData.MediaValidation == nil {
		t.Fatal("expected MediaValidation record")
	}
	// Rejection tally deduplicates by URL: when the same URL is referenced
	// from both ThumbnailURL and a MediaCandidates entry, it counts once.
	// This keeps RejectedCount aligned with "distinct rejected media refs".
	if doc.OGData.MediaValidation.RejectedCount != 1 {
		t.Fatalf("expected 1 rejection after URL dedup, got %d", doc.OGData.MediaValidation.RejectedCount)
	}
	if got := doc.OGData.MediaValidation.Reasons["image_too_small"]; got != 1 {
		t.Fatalf("expected image_too_small=1 after URL dedup, got %d", got)
	}
}

func TestFilterValidMedia_MixedKeepsValidOnly(t *testing.T) {
	doc := PinDocument{
		ThumbnailURL: "https://x/good.png",
		MediaCandidates: []MediaCandidate{
			{Type: "image", URL: "https://x/good.png"},
			{Type: "image", URL: "https://x/bad.gif"},
		},
	}
	stub := &stubValidator{results: map[string]MediaValidationResult{
		"https://x/good.png": {Valid: true, Reason: MediaValidationOK, Width: 800, Height: 600},
		"https://x/bad.gif":  {Valid: false, Reason: MediaValidationImageTooSmall},
	}}
	FilterValidMedia(context.Background(), stub, &doc)

	if doc.ThumbnailURL != "https://x/good.png" {
		t.Fatalf("expected good thumbnail retained, got %q", doc.ThumbnailURL)
	}
	if len(doc.MediaCandidates) != 1 {
		t.Fatalf("expected 1 candidate retained, got %d", len(doc.MediaCandidates))
	}
	if doc.MediaCandidates[0].URL != "https://x/good.png" {
		t.Fatalf("expected good URL, got %q", doc.MediaCandidates[0].URL)
	}
	if doc.MediaCandidates[0].Width != 800 || doc.MediaCandidates[0].Height != 600 {
		t.Fatalf("expected width/height backfilled, got %dx%d", doc.MediaCandidates[0].Width, doc.MediaCandidates[0].Height)
	}
	if doc.OGData.MediaValidation == nil || doc.OGData.MediaValidation.RejectedCount != 1 {
		t.Fatalf("expected 1 rejection recorded, got %+v", doc.OGData.MediaValidation)
	}
}

func TestFilterValidMedia_DedupesValidationByURL(t *testing.T) {
	// ThumbnailURL == MediaCandidates[0].URL: validator should be invoked
	// once per unique URL, not twice.
	doc := PinDocument{
		ThumbnailURL: "https://x/img.png",
		MediaCandidates: []MediaCandidate{
			{Type: "image", URL: "https://x/img.png"},
		},
	}
	stub := &stubValidator{results: map[string]MediaValidationResult{
		"https://x/img.png": {Valid: true, Reason: MediaValidationOK, Width: 100, Height: 100},
	}}
	FilterValidMedia(context.Background(), stub, &doc)
	if stub.calls != 1 {
		t.Fatalf("expected 1 validator call, got %d", stub.calls)
	}
}

func TestMediaValidationMetrics_RecordsAllAxes(t *testing.T) {
	m := NewMediaValidationMetrics()
	m.RecordRejection(MediaValidationImageTooSmall)
	m.RecordRejection(MediaValidationImageTooSmall)
	m.RecordRejection(MediaValidationDecodeFailed)
	m.RecordClassification(true, "")
	m.RecordClassification(false, ClassifierReasonNoPrimaryMedia)
	m.RecordClassification(false, ClassifierReasonEmptyBody) // unrelated reason — not tracked

	total, perReason, pinnable, noPrimary := m.Snapshot()
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if perReason["image_too_small"] != 2 || perReason["decode_failed"] != 1 {
		t.Fatalf("per-reason wrong: %+v", perReason)
	}
	// Positive assertion that "unrelated" classifier reasons are silently
	// ignored: empty_body must not have leaked into pinnable, no_primary,
	// or any per-reason rejection bucket.
	if _, found := perReason["empty_body"]; found {
		t.Fatalf("empty_body unexpectedly recorded as a rejection reason: %+v", perReason)
	}
	if pinnable != 1 || noPrimary != 1 {
		t.Fatalf("classification counters: pinnable=%d no_primary=%d", pinnable, noPrimary)
	}

	// RecordRejectionN should fold counts in O(1) per reason and sum
	// correctly into the existing tallies.
	m.RecordRejectionN(MediaValidationImageTooSmall, 5)
	total2, perReason2, _, _ := m.Snapshot()
	if total2 != 8 {
		t.Fatalf("expected total 8 after RecordRejectionN(5), got %d", total2)
	}
	if perReason2["image_too_small"] != 7 {
		t.Fatalf("image_too_small: expected 7, got %d", perReason2["image_too_small"])
	}

	// Reset must zero every counter.
	m.Reset()
	total3, perReason3, pinnable3, noPrimary3 := m.Snapshot()
	if total3 != 0 || pinnable3 != 0 || noPrimary3 != 0 || len(perReason3) != 0 {
		t.Fatalf("Reset did not zero all counters: total=%d pinnable=%d noPrimary=%d perReason=%+v",
			total3, pinnable3, noPrimary3, perReason3)
	}
}

// TestImageValidator_RespectsClientTimeout verifies the validator returns
// a download_failed (not a panic / hang) when the server stalls beyond the
// HTTP client deadline.
func TestImageValidator_RespectsClientTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer srv.Close()
	v := NewDefaultMediaValidator()
	v.HTTP = &http.Client{Timeout: 50 * time.Millisecond}
	r := v.Validate(context.Background(), srv.URL, "image")
	if r.Valid {
		t.Fatal("expected timeout to mark invalid")
	}
	if r.Reason != MediaValidationDownloadFailed {
		t.Fatalf("expected download_failed, got %q", r.Reason)
	}
}
