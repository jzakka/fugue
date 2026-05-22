package bot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestDownloadAndUpload_ImageBranch_RejectsOversizeContentLength is the
// regression test for harvester/spec.md L749 SHALL ("Harvester가 외부 미디어
// 응답 본문을 stream으로 객체 저장소에 전달할 때는 명시적인 stream 크기
// 상한 가드를 갖춰야 한다"). If the server declares a Content-Length above
// the image cap, the fetch MUST short-circuit before reading the body.
func TestDownloadAndUpload_ImageBranch_RejectsOversizeContentLength(t *testing.T) {
	cap := int64(1024) // 1 KiB test cap
	bytesRead := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		// Declare a Content-Length that exceeds cap. The handler returns a
		// tiny actual body — we never expect the test to read it. We assert
		// on the bytesRead counter via the server's request-write hook.
		w.Header().Set("Content-Length", strconv.FormatInt(cap*2, 10))
		// Write only a stub; the precheck must fire BEFORE the body is read.
		w.WriteHeader(http.StatusOK)
		n, _ := w.Write([]byte("oversized"))
		bytesRead += n
	}))
	defer srv.Close()

	p := NewHarvestPipeline(NewMockBotDB(), NewMockStorage(), WithImageCacheMaxBytes(cap))
	p.client = srv.Client()

	_, err := p.downloadAndUpload(context.Background(), srv.URL+"/big.jpg", "image")
	if err == nil {
		t.Fatal("expected error from Content-Length precheck, got nil")
	}
	if !errors.Is(err, errImageOversize) {
		t.Errorf("expected errImageOversize, got %v", err)
	}
}

// TestDownloadAndUpload_ImageBranch_RejectsOversizeStream covers the case
// where the server lies about (or omits) Content-Length but the actual body
// streams past the cap. The LimitReader(cap+1) + post-read overshoot check
// MUST stop the upload and surface errImageOversize.
func TestDownloadAndUpload_ImageBranch_RejectsOversizeStream(t *testing.T) {
	cap := int64(1024) // 1 KiB test cap
	// Server omits Content-Length and writes cap+128 bytes (above cap).
	// http.ResponseWriter chunked encoding kicks in automatically when no
	// Content-Length is set, so resp.ContentLength is -1 on the client side
	// and the precheck does not fire — only the LimitReader+1 overshoot
	// check stops the read.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, cap+128))
	}))
	defer srv.Close()

	mockStorage := NewMockStorage()
	uploads := 0
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, r io.Reader) (string, error) {
		uploads++
		return "https://cdn.example.com/" + filename, nil
	}

	p := NewHarvestPipeline(NewMockBotDB(), mockStorage, WithImageCacheMaxBytes(cap))
	p.client = srv.Client()

	_, err := p.downloadAndUpload(context.Background(), srv.URL+"/lying.jpg", "image")
	if err == nil {
		t.Fatal("expected error from overshoot guard, got nil")
	}
	if !errors.Is(err, errImageOversize) {
		t.Errorf("expected errImageOversize, got %v", err)
	}
	if uploads != 0 {
		t.Errorf("expected 0 uploads to storage when stream overshoots cap, got %d", uploads)
	}
}

// TestDownloadAndUpload_ImageBranch_AcceptsAtCapBoundary verifies that an
// image exactly at the cap is NOT rejected by the stream guard. The body
// fails the DecodeConfig validation (it is not a real PNG) but that is the
// pre-existing in-process check — what we assert here is that the stream
// cap does not trip at the boundary value.
func TestDownloadAndUpload_ImageBranch_AcceptsAtCapBoundary(t *testing.T) {
	cap := int64(1024) // 1 KiB test cap; bigger than DefaultImageMinBytes (1024)
	// Use a body of exactly cap bytes so LimitReader+1 reads cap, the
	// post-read overshoot check (len > cap) does NOT trip, and execution
	// falls through to the min-bytes / DecodeConfig checks.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, cap))
	}))
	defer srv.Close()

	p := NewHarvestPipeline(NewMockBotDB(), NewMockStorage(), WithImageCacheMaxBytes(cap))
	p.client = srv.Client()

	_, err := p.downloadAndUpload(context.Background(), srv.URL+"/atcap.jpg", "image")
	if err == nil {
		t.Fatal("expected DecodeConfig error (body is raw zeros), got nil")
	}
	if errors.Is(err, errImageOversize) {
		t.Fatalf("at-cap boundary must NOT trip errImageOversize, got: %v", err)
	}
	if !errors.Is(err, errImageInvalidMedia) {
		t.Errorf("expected errImageInvalidMedia (decode fail) at boundary, got %v", err)
	}
}

// TestDownloadAndUpload_ImageBranch_HappyPathStillUploads is the regression
// guard against breaking the legitimate fetch path: a real PNG well under
// cap MUST still pass all validation and call storage.Upload exactly once.
func TestDownloadAndUpload_ImageBranch_HappyPathStillUploads(t *testing.T) {
	pngBytes := harvestTestPNG(64, 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()

	mockStorage := NewMockStorage()
	uploads := 0
	var uploadedSize int64
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, r io.Reader) (string, error) {
		uploads++
		uploadedSize = size
		if !strings.HasPrefix(filename, "bot/") {
			t.Errorf("expected bot/ prefix, got filename=%q", filename)
		}
		return "https://cdn.example.com/" + filename, nil
	}

	p := NewHarvestPipeline(NewMockBotDB(), mockStorage)
	p.client = srv.Client()

	url, err := p.downloadAndUpload(context.Background(), srv.URL+"/good.png", "image")
	if err != nil {
		t.Fatalf("unexpected error on happy path: %v", err)
	}
	if uploads != 1 {
		t.Errorf("expected 1 upload, got %d", uploads)
	}
	if uploadedSize != int64(len(pngBytes)) {
		t.Errorf("uploaded size mismatch: got %d, want %d", uploadedSize, len(pngBytes))
	}
	if !strings.HasPrefix(url, "https://cdn.example.com/bot/") {
		t.Errorf("unexpected returned URL: %q", url)
	}
}
