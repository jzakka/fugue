package storage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// PNG magic bytes (8-byte signature) — http.DetectContentType returns "image/png"
var pngBytes = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}

// JPEG SOI + APP0 — http.DetectContentType returns "image/jpeg"
var jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01}

// WebM EBML header — http.DetectContentType returns "video/webm"
var webmBytes = []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0xF7, 0x81, 0x01, 0x42, 0xF2, 0x81, 0x04, 0x42, 0xF3, 0x81, 0x08, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'}

// WAV: "RIFF<size>WAVE" — http.DetectContentType returns "audio/wave"
var wavBytes = []byte{'R', 'I', 'F', 'F', 0x24, 0x00, 0x00, 0x00, 'W', 'A', 'V', 'E', 'f', 'm', 't', ' ', 0x10, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x44, 0xAC, 0x00, 0x00, 0x88, 0x58, 0x01, 0x00, 0x02, 0x00, 0x10, 0x00, 'd', 'a', 't', 'a', 0x00, 0x00, 0x00, 0x00}

// MP3: ID3v2 header — http.DetectContentType returns "audio/mpeg"
var mp3Bytes = []byte{'I', 'D', '3', 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

func newTestClient() *Client {
	// s3 stays nil — these tests cover validation paths that reject before any S3 call.
	return &Client{bucket: "test-bucket", pubURL: "http://example.test/test-bucket"}
}

func TestNormalizeMIME(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"image/jpg", "image/jpeg"},
		{"image/JPG", "image/jpeg"},
		{"image/pjpeg", "image/jpeg"},
		{"audio/x-wav", "audio/wav"},
		{"audio/wave", "audio/wav"},
		{"audio/mp3", "audio/mpeg"},
		{"audio/x-flac", "audio/flac"},
		{"image/png", "image/png"},
		{"VIDEO/MP4", "video/mp4"},
		{"  image/jpeg  ", "image/jpeg"},
	}
	for _, c := range cases {
		got := normalizeMIME(c.in)
		if got != c.want {
			t.Errorf("normalizeMIME(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUpload_RejectsMimeMismatch_WebMAsPNG(t *testing.T) {
	c := newTestClient()
	_, err := c.Upload(context.Background(), "fake.png", "image/png", int64(len(webmBytes)), bytes.NewReader(webmBytes))
	if err == nil {
		t.Fatal("expected error for declared=image/png with WebM bytes, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("error message missing 'unsupported file type' prefix: %v", err)
	}
	if !strings.Contains(err.Error(), "content type mismatch") {
		t.Errorf("error message missing mismatch detail: %v", err)
	}
}

func TestUpload_RejectsMimeMismatch_PNGAsVideoMP4(t *testing.T) {
	c := newTestClient()
	_, err := c.Upload(context.Background(), "fake.mp4", "video/mp4", int64(len(pngBytes)), bytes.NewReader(pngBytes))
	if err == nil {
		t.Fatal("expected error for declared=video/mp4 with PNG bytes, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("error message missing 'unsupported file type' prefix: %v", err)
	}
}

func TestUpload_RejectsMimeMismatch_WAVAsImageJPEG(t *testing.T) {
	c := newTestClient()
	_, err := c.Upload(context.Background(), "fake.jpg", "image/jpeg", int64(len(wavBytes)), bytes.NewReader(wavBytes))
	if err == nil {
		t.Fatal("expected error for declared=image/jpeg with WAV bytes, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("error message missing 'unsupported file type' prefix: %v", err)
	}
}

func TestUpload_RejectsMimeMismatch_SameCategoryDifferentFormat(t *testing.T) {
	c := newTestClient()
	_, err := c.Upload(context.Background(), "fake.jpg", "image/jpeg", int64(len(pngBytes)), bytes.NewReader(pngBytes))
	if err == nil {
		t.Fatal("expected error for declared=image/jpeg with PNG bytes, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("error message missing 'unsupported file type' prefix: %v", err)
	}
}

// validateAcceptThroughMismatch verifies that the mismatch check does NOT fire
// for the given inputs. We exercise the validation by calling Upload with a
// nil S3 client: if the mismatch branch rejects, we get an "unsupported file
// type" error before any S3 call. If validation passes, control reaches
// PutObject which panics on the nil s3 client — we recover and treat that as
// proof the mismatch branch let the request through.
func validateAcceptThroughMismatch(t *testing.T, name, declared string, body []byte) {
	t.Helper()
	c := newTestClient()
	defer func() {
		if r := recover(); r != nil {
			// reached S3 call without a mismatch error — that's the success signal here.
			return
		}
	}()
	_, err := c.Upload(context.Background(), name, declared, int64(len(body)), bytes.NewReader(body))
	if err != nil {
		if strings.Contains(err.Error(), "content type mismatch") {
			t.Errorf("expected mismatch check to pass for declared=%q, got mismatch error: %v", declared, err)
		}
		// Any non-mismatch error (e.g. nil-client deref converted to error) is also acceptable.
	}
}

func TestUpload_AllowsDeclaredEmpty(t *testing.T) {
	// Empty declared Content-Type — comparison must be skipped, sniff alone drives allowlist.
	validateAcceptThroughMismatch(t, "anon.png", "", pngBytes)
}

func TestUpload_AllowsDeclaredOctetStream(t *testing.T) {
	// generic octet-stream behaves the same as empty declared (comparison skipped).
	validateAcceptThroughMismatch(t, "anon.png", "application/octet-stream", pngBytes)
}

func TestUpload_AllowsAliasedDeclared_ImageJPG(t *testing.T) {
	// declared=image/jpg (alias), sniff=image/jpeg → normalize equal → pass.
	validateAcceptThroughMismatch(t, "photo.jpg", "image/jpg", jpegBytes)
}

func TestUpload_AllowsAliasedDeclared_ImagePJPEG(t *testing.T) {
	validateAcceptThroughMismatch(t, "photo.jpg", "image/pjpeg", jpegBytes)
}

func TestUpload_AllowsAliasedDeclared_AudioMP3(t *testing.T) {
	// declared=audio/mp3 (alias), sniff=audio/mpeg → normalize equal → pass.
	validateAcceptThroughMismatch(t, "song.mp3", "audio/mp3", mp3Bytes)
}

func TestUpload_AllowsAliasedDeclared_AudioXWav(t *testing.T) {
	// http.DetectContentType returns "audio/wave" for WAV bytes — normalize to audio/wav.
	// declared=audio/x-wav (alias) → normalize to audio/wav → equal.
	validateAcceptThroughMismatch(t, "song.wav", "audio/x-wav", wavBytes)
}

func TestUpload_AllowsExactMatch_PNG(t *testing.T) {
	validateAcceptThroughMismatch(t, "photo.png", "image/png", pngBytes)
}

// Delete has no pre-S3 validation logic, so unlike the Upload tests above it is
// verified against a fake S3 endpoint (Config.Endpoint injection) that captures
// the outgoing DeleteObject request.
func TestDelete_IssuesDeleteObjectForBucketAndKey(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		Endpoint:  srv.URL,
		Bucket:    "test-bucket",
		AccessKey: "test",
		SecretKey: "test",
		PublicURL: srv.URL + "/test-bucket",
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	if err := c.Delete(context.Background(), "image/abc123.png"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE request, got %s", gotMethod)
	}
	// UsePathStyle=true → DELETE /<bucket>/<key>
	if gotPath != "/test-bucket/image/abc123.png" {
		t.Errorf("expected path /test-bucket/image/abc123.png, got %s", gotPath)
	}
}

func TestDelete_ReturnsErrorOnS3Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		Endpoint:  srv.URL,
		Bucket:    "test-bucket",
		AccessKey: "test",
		SecretKey: "test",
		PublicURL: srv.URL + "/test-bucket",
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	if err := c.Delete(context.Background(), "image/abc123.png"); err == nil {
		t.Fatal("expected error on S3 500 response, got nil")
	}
}
