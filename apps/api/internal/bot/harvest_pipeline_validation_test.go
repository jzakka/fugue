package bot

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// build1x1GIFForPipeline is a copy of the 37-byte 1×1 GIF placeholder seen
// in the 2026-04-27 QA report. Kept local to avoid cross-test coupling.
func build1x1GIFForPipeline(t *testing.T) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Transparent, color.Black})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("gif encode: %v", err)
	}
	return buf.Bytes()
}

// TestCacheImage_Rejects1x1Placeholder_FallsBackInsteadOfUploading is the
// regression test for the QA-reported 1×1 GIF: when the candidate is the
// canonical placeholder shape (decodable but tiny), cacheImage MUST NOT
// upload and MUST return the original candidate URL via the fallback path.
//
// Spec contract: "검증 탈락 후보는 정본 키에 업로드되지 않는다" — verifies the
// MockStorage.Upload was never invoked with an images/ prefix key.
func TestCacheImage_Rejects1x1Placeholder_FallsBackInsteadOfUploading(t *testing.T) {
	placeholder := build1x1GIFForPipeline(t)
	// The legacy downloadAndUpload path now also validates image bytes, so
	// /whatever serves a real PNG to keep the Pin row creatable. Only the
	// og:image candidate (cacheImage path) is the 1×1 placeholder we want
	// to assert is rejected.
	validPNG := harvestTestPNG(64, 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/placeholder.gif" {
			w.Header().Set("Content-Type", "image/gif")
			_, _ = w.Write(placeholder)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(validPNG)
	}))
	defer srv.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	uploadCalls := 0
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, r io.Reader) (string, error) {
		if strings.HasPrefix(filename, "images/") {
			uploadCalls++
		}
		return "https://cdn.example.com/" + filename, nil
	}

	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = srv.Client()

	candidate := srv.URL + "/placeholder.gif"
	html := fmt.Sprintf(`<html><head><meta property="og:image" content="%s"></head></html>`, candidate)
	items := []RawItem{
		{
			Title:     "Placeholder Page",
			MediaURL:  srv.URL + "/whatever",
			MediaType: "image",
			SourceURL: "https://example.com/qa-1x1",
			PageHTML:  []byte(html),
		},
	}

	_, _, _, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected pipeline error: %v", err)
	}
	if uploadCalls != 0 {
		t.Fatalf("expected 0 uploads to canonical images/ key for placeholder, got %d", uploadCalls)
	}
	if len(mockDB.CreatedPins) != 1 {
		t.Fatalf("expected 1 pin row, got %d", len(mockDB.CreatedPins))
	}
	// Pin should still exist (Process doesn't kill the row on placeholder)
	// but og_image must point to the original candidate (fallback), NOT to
	// the canonical storage prefix.
	pin := mockDB.CreatedPins[0]
	if pin.OgImage.Valid && strings.Contains(pin.OgImage.String, "/images/") {
		t.Fatalf("og_image leaked canonical key for invalid media: %q", pin.OgImage.String)
	}
}
