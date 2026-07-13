package bot

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// harvestTestPNG returns a deterministic noise-filled PNG of the given
// dimensions. Used by image-cache tests so the validation that
// harvester-media-validation added rejects fake byte strings while still
// allowing real images through the cacheImage path.
func harvestTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(987654321)
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
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// MockBotDB implements BotDB for testing.
type MockBotDB struct {
	ExistingURLs map[string]bool           // sourceURL -> exists
	OgImages     map[string]sql.NullString // sourceURL -> current og_image (upsert history)
	CreatedPins  []db.CreatePinParams
	CreateErr    error
	DedupErr     error
}

func NewMockBotDB() *MockBotDB {
	return &MockBotDB{
		ExistingURLs: make(map[string]bool),
		OgImages:     make(map[string]sql.NullString),
	}
}

func (m *MockBotDB) BotPinExistsByURL(ctx context.Context, arg db.BotPinExistsByURLParams) (bool, error) {
	if m.DedupErr != nil {
		return false, m.DedupErr
	}
	return m.ExistingURLs[arg.Url.String], nil
}

func (m *MockBotDB) CreatePin(ctx context.Context, arg db.CreatePinParams) (db.Pin, error) {
	if m.CreateErr != nil {
		return db.Pin{}, m.CreateErr
	}
	m.CreatedPins = append(m.CreatedPins, arg)
	return db.Pin{
		ID:          uuid.New(),
		CreatorID:   arg.CreatorID,
		MediaUrl:    arg.MediaUrl,
		MediaType:   arg.MediaType,
		Url:         arg.Url,
		Title:       arg.Title,
		Description: arg.Description,
		OgImage:     arg.OgImage,
		OgData:      arg.OgData,
	}, nil
}

func (m *MockBotDB) UpsertBotPinByURL(ctx context.Context, arg db.UpsertBotPinByURLParams) (db.UpsertBotPinByURLRow, error) {
	if m.CreateErr != nil {
		return db.UpsertBotPinByURLRow{}, m.CreateErr
	}
	url := arg.Url.String
	inserted := !m.ExistingURLs[url]
	// Mirror the CTE snapshot semantics: prev_og_image is the value the row
	// held BEFORE this upsert, NULL on a fresh insert.
	prev := sql.NullString{}
	if !inserted {
		prev = m.OgImages[url]
	}
	m.ExistingURLs[url] = true
	m.OgImages[url] = arg.OgImage
	m.CreatedPins = append(m.CreatedPins, db.CreatePinParams(arg))
	return db.UpsertBotPinByURLRow{
		ID:          uuid.New(),
		CreatorID:   arg.CreatorID,
		Url:         arg.Url,
		Title:       arg.Title,
		Description: arg.Description,
		OgImage:     arg.OgImage,
		OgData:      arg.OgData,
		MediaUrl:    arg.MediaUrl,
		MediaType:   arg.MediaType,
		Inserted:    inserted,
		PrevOgImage: prev,
	}, nil
}

func TestHarvestPipeline_NewItems(t *testing.T) {
	// Set up a mock media server. After harvester-media-validation,
	// downloadAndUpload validates image bytes before upload, so the
	// fixture serves a real noise PNG that satisfies the minimum-bytes
	// and minimum-dim thresholds.
	pngBytes := harvestTestPNG(64, 64)
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{
			Title:     "Test Image 1",
			MediaURL:  mediaServer.URL + "/img1.jpg",
			MediaType: "image",
			SourceURL: "https://example.com/page1",
		},
		{
			Title:       "Test Image 2",
			Description: "A description",
			MediaURL:    mediaServer.URL + "/img2.png",
			MediaType:   "image",
			SourceURL:   "https://example.com/page2",
		},
	}

	created, deduped, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 2 {
		t.Errorf("expected 2 pins created, got %d", created)
	}
	if deduped != 0 {
		t.Errorf("expected 0 deduped, got %d", deduped)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
	if len(mockDB.CreatedPins) != 2 {
		t.Fatalf("expected 2 pins in DB, got %d", len(mockDB.CreatedPins))
	}
	if mockDB.CreatedPins[0].CreatorID != BotCreatorID {
		t.Errorf("expected bot creator ID, got %v", mockDB.CreatedPins[0].CreatorID)
	}
	if mockDB.CreatedPins[0].Title != "Test Image 1" {
		t.Errorf("expected title 'Test Image 1', got %q", mockDB.CreatedPins[0].Title)
	}
	if !mockDB.CreatedPins[1].Description.Valid || mockDB.CreatedPins[1].Description.String != "A description" {
		t.Errorf("expected description 'A description', got %v", mockDB.CreatedPins[1].Description)
	}
	// og_image and og_data should be NULL
	if mockDB.CreatedPins[0].OgImage.Valid {
		t.Errorf("expected NULL og_image")
	}
	if mockDB.CreatedPins[0].OgData.Valid {
		t.Errorf("expected NULL og_data")
	}
	if mockStorage.CallCount != 2 {
		t.Errorf("expected 2 storage uploads, got %d", mockStorage.CallCount)
	}
}

func TestHarvestPipeline_DBDedup(t *testing.T) {
	pngBytes := harvestTestPNG(64, 64)
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockDB.ExistingURLs["https://example.com/existing"] = true

	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{Title: "Existing", MediaURL: mediaServer.URL + "/img.jpg", MediaType: "image", SourceURL: "https://example.com/existing"},
		{Title: "New", MediaURL: mediaServer.URL + "/img2.jpg", MediaType: "image", SourceURL: "https://example.com/new"},
	}

	created, deduped, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 1 {
		t.Errorf("expected 1 created, got %d", created)
	}
	if deduped != 1 {
		t.Errorf("expected 1 deduped, got %d", deduped)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
}

func TestHarvestPipeline_BatchDedup(t *testing.T) {
	pngBytes := harvestTestPNG(64, 64)
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	// Same sourceURL in batch → second should be deduped
	items := []RawItem{
		{Title: "First", MediaURL: mediaServer.URL + "/img.jpg", MediaType: "image", SourceURL: "https://example.com/page"},
		{Title: "Duplicate", MediaURL: mediaServer.URL + "/img2.jpg", MediaType: "image", SourceURL: "https://example.com/page"},
	}

	created, deduped, _, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 1 {
		t.Errorf("expected 1 created, got %d", created)
	}
	if deduped != 1 {
		t.Errorf("expected 1 deduped, got %d", deduped)
	}
}

func TestHarvestPipeline_DownloadFailure(t *testing.T) {
	// Server returns 404
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{Title: "Bad", MediaURL: mediaServer.URL + "/missing.jpg", MediaType: "image", SourceURL: "https://example.com/page"},
	}

	created, deduped, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 0 {
		t.Errorf("expected 0 created, got %d", created)
	}
	if deduped != 0 {
		t.Errorf("expected 0 deduped, got %d", deduped)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
}

func TestHarvestPipeline_UploadFailure(t *testing.T) {
	pngBytes := harvestTestPNG(64, 64)
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
		return "", fmt.Errorf("storage error")
	}
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{Title: "Upload Fail", MediaURL: mediaServer.URL + "/img.jpg", MediaType: "image", SourceURL: "https://example.com/page"},
	}

	created, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 0 {
		t.Errorf("expected 0 created, got %d", created)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
}

func TestHarvestPipeline_DBCreateError(t *testing.T) {
	pngBytes := harvestTestPNG(64, 64)
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockDB.CreateErr = fmt.Errorf("db insert error")
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{Title: "DB Fail", MediaURL: mediaServer.URL + "/img.jpg", MediaType: "image", SourceURL: "https://example.com/page"},
	}

	created, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 0 {
		t.Errorf("expected 0 created, got %d", created)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
}

func TestHarvestPipeline_DBDedupError(t *testing.T) {
	mockDB := NewMockBotDB()
	mockDB.DedupErr = fmt.Errorf("db connection error")
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)

	items := []RawItem{
		{Title: "Check Fail", MediaURL: "http://example.com/img.jpg", MediaType: "image", SourceURL: "https://example.com/page"},
	}

	created, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 0 {
		t.Errorf("expected 0 created, got %d", created)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
}

func TestHarvestPipeline_MixedResults(t *testing.T) {
	// 5 items: 2 dedup (1 DB + 1 batch), 1 download failure, 2 new
	pngBytes := harvestTestPNG(64, 64)
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.jpg" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockDB.ExistingURLs["https://example.com/existing"] = true
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{Title: "New1", MediaURL: mediaServer.URL + "/img1.jpg", MediaType: "image", SourceURL: "https://example.com/new1"},
		{Title: "DBDup", MediaURL: mediaServer.URL + "/img2.jpg", MediaType: "image", SourceURL: "https://example.com/existing"},
		{Title: "DlFail", MediaURL: mediaServer.URL + "/missing.jpg", MediaType: "image", SourceURL: "https://example.com/fail"},
		{Title: "New2", MediaURL: mediaServer.URL + "/img3.jpg", MediaType: "image", SourceURL: "https://example.com/new2"},
		{Title: "BatchDup", MediaURL: mediaServer.URL + "/img4.jpg", MediaType: "image", SourceURL: "https://example.com/new1"},
	}

	created, deduped, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 2 {
		t.Errorf("expected 2 created, got %d", created)
	}
	if deduped != 2 {
		t.Errorf("expected 2 deduped (1 DB + 1 batch), got %d", deduped)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
}

// --- Section 5 of harvester-image-cache: primary image cache integration ---

// imageCacheTestServer returns a httptest server that serves
//   - /media   → 200 body real noise PNG (after harvester-media-validation
//     downloadAndUpload validates image bytes before upload, so a real PNG
//     is required for the legacy Process() path to succeed)
//   - /image.jpg → 200 body real noise PNG (≥1024 bytes, ≥32x32) so the
//     harvester-media-validation check passes
//   - /notfound-image.jpg → 404
//   - /huge.jpg → 200 with Content-Length > threshold
//   - /stream-huge.jpg → 200 with no Content-Length but streamed body > threshold
func imageCacheTestServer(threshold int) *httptest.Server {
	validPNG := harvestTestPNG(64, 64)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/media":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(validPNG)
		case "/image.jpg":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(validPNG)
		case "/notfound-image.jpg":
			w.WriteHeader(http.StatusNotFound)
		case "/huge.jpg":
			body := make([]byte, threshold+100)
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			_, _ = w.Write(body)
		case "/stream-huge.jpg":
			// No Content-Length → forces read-side size check.
			w.Header().Set("Content-Type", "image/jpeg")
			// Write threshold+100 bytes in chunks.
			chunk := make([]byte, 1024)
			written := 0
			for written < threshold+100 {
				n := threshold + 100 - written
				if n > len(chunk) {
					n = len(chunk)
				}
				_, _ = w.Write(chunk[:n])
				written += n
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestHarvestPipeline_ImageCache_Success(t *testing.T) {
	server := imageCacheTestServer(1024)
	defer server.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	// Record image upload keys to verify "images/" prefix is used.
	var imageUploadKey string
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
		if strings.HasPrefix(filename, "images/") {
			imageUploadKey = filename
		}
		return "https://cdn.example.com/" + filename, nil
	}
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = server.Client()

	html := fmt.Sprintf(`<html><head><meta property="og:image" content="%s/image.jpg"></head></html>`, server.URL)

	items := []RawItem{
		{
			Title:     "Cached Image",
			MediaURL:  server.URL + "/media",
			MediaType: "image",
			SourceURL: "https://example.com/page1",
			PageHTML:  []byte(html),
		},
	}

	created, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 1 || failed != 0 {
		t.Fatalf("expected created=1 failed=0, got created=%d failed=%d", created, failed)
	}
	if len(mockDB.CreatedPins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(mockDB.CreatedPins))
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid {
		t.Fatalf("expected OgImage to be set on cache success, got NULL")
	}
	if !strings.HasPrefix(pin.OgImage.String, "https://cdn.example.com/images/") {
		t.Errorf("expected OgImage to be storage URL with images/ prefix, got %q", pin.OgImage.String)
	}
	if imageUploadKey == "" {
		t.Errorf("expected storage.Upload called with images/ prefix key")
	}
	// Key format: images/<hash>/<ts>.png (real PNG payload after the
	// harvester-media-validation change replaced the fake "image-bytes"
	// fixture with a synthesized noise PNG).
	parts := strings.Split(imageUploadKey, "/")
	if len(parts) != 3 || parts[0] != "images" || len(parts[1]) != 64 || !strings.HasSuffix(parts[2], ".png") {
		t.Errorf("unexpected key format: %q", imageUploadKey)
	}
}

func TestHarvestPipeline_ImageCache_DownloadFail_FallbackToOriginalURL(t *testing.T) {
	server := imageCacheTestServer(1024)
	defer server.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = server.Client()

	candidate := server.URL + "/notfound-image.jpg"
	html := fmt.Sprintf(`<html><head><meta property="og:image" content="%s"></head></html>`, candidate)

	items := []RawItem{
		{
			Title:     "Download Fail",
			MediaURL:  server.URL + "/media",
			MediaType: "image",
			SourceURL: "https://example.com/page2",
			PageHTML:  []byte(html),
		},
	}

	created, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Pin must still be created — image failure does NOT block pin creation.
	if created != 1 || failed != 0 {
		t.Fatalf("expected created=1 failed=0, got created=%d failed=%d", created, failed)
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid {
		t.Fatalf("expected OgImage to be set to original candidate URL on fallback")
	}
	// Fallback: original candidate URL is recorded (not storage URL).
	if pin.OgImage.String != candidate {
		t.Errorf("expected OgImage = %q (original candidate), got %q", candidate, pin.OgImage.String)
	}
}

func TestHarvestPipeline_ImageCache_NoCandidate_OgImageNull(t *testing.T) {
	server := imageCacheTestServer(1024)
	defer server.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = server.Client()

	// HTML with no image metadata at all.
	html := `<html><head><title>nothing</title></head><body><p>no imgs</p></body></html>`

	items := []RawItem{
		{
			Title:     "No Image",
			MediaURL:  server.URL + "/media",
			MediaType: "image",
			SourceURL: "https://example.com/page3",
			PageHTML:  []byte(html),
		},
	}

	created, _, _, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 pin created, got %d", created)
	}
	pin := mockDB.CreatedPins[0]
	if pin.OgImage.Valid {
		t.Errorf("expected OgImage to be NULL when no candidate, got %q", pin.OgImage.String)
	}
}

func TestHarvestPipeline_ImageCache_Oversize_FallbackToOriginalURL(t *testing.T) {
	// 16 KiB: above the 64x64 noise PNG (~12 KiB) served on /media so the
	// primary media path (downloadAndUpload image branch) is not tripped by
	// the same cap. The /huge.jpg fixture writes threshold+100 = 16484
	// bytes which still exceeds 16384, preserving the cacheImage oversize
	// fallback assertion. (downloadAndUpload's image-branch stream cap was
	// added 2026-05-22 to enforce harvester/spec.md L749 SHALL; it shares
	// the imageCacheMaxBytes source-of-truth with cacheImage.)
	const threshold = 16384
	server := imageCacheTestServer(threshold)
	defer server.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	// Image upload must NOT be called for oversized payloads (partial bytes
	// are not uploaded per spec). Only body-media upload is expected.
	imageUploadCalled := false
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
		if strings.HasPrefix(filename, "images/") {
			imageUploadCalled = true
		}
		return "https://cdn.example.com/" + filename, nil
	}
	pipeline := NewHarvestPipeline(mockDB, mockStorage, WithImageCacheMaxBytes(threshold))
	pipeline.client = server.Client()

	candidate := server.URL + "/huge.jpg"
	html := fmt.Sprintf(`<html><head><meta property="og:image" content="%s"></head></html>`, candidate)

	items := []RawItem{
		{
			Title:     "Huge Image",
			MediaURL:  server.URL + "/media",
			MediaType: "image",
			SourceURL: "https://example.com/page4",
			PageHTML:  []byte(html),
		},
	}

	created, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 1 || failed != 0 {
		t.Fatalf("expected created=1 failed=0, got created=%d failed=%d", created, failed)
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid || pin.OgImage.String != candidate {
		t.Errorf("expected OgImage = %q (original candidate on oversize fallback), got %v", candidate, pin.OgImage)
	}
	if imageUploadCalled {
		t.Errorf("expected NO image upload on oversize (partial bytes must not be uploaded)")
	}
}

func TestHarvestPipeline_ImageCache_UploadFail_FallbackToOriginalURL(t *testing.T) {
	server := imageCacheTestServer(1024)
	defer server.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	// Make only image upload fail; media upload should still succeed.
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
		if strings.HasPrefix(filename, "images/") {
			return "", fmt.Errorf("simulated storage failure")
		}
		return "https://cdn.example.com/" + filename, nil
	}
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = server.Client()

	candidate := server.URL + "/image.jpg"
	html := fmt.Sprintf(`<html><head><meta property="og:image" content="%s"></head></html>`, candidate)

	items := []RawItem{
		{
			Title:     "Upload Fail Image",
			MediaURL:  server.URL + "/media",
			MediaType: "image",
			SourceURL: "https://example.com/page5",
			PageHTML:  []byte(html),
		},
	}

	created, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 1 || failed != 0 {
		t.Fatalf("expected created=1 failed=0, got created=%d failed=%d", created, failed)
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid || pin.OgImage.String != candidate {
		t.Errorf("expected OgImage = %q (original candidate on upload fail), got %v", candidate, pin.OgImage)
	}
}

func TestHarvestPipeline_EmptyDescription(t *testing.T) {
	pngBytes := harvestTestPNG(64, 64)
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{Title: "No Desc", MediaURL: mediaServer.URL + "/img.jpg", MediaType: "image", SourceURL: "https://example.com/page", Description: ""},
	}

	_, _, _, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockDB.CreatedPins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(mockDB.CreatedPins))
	}
	if mockDB.CreatedPins[0].Description != (sql.NullString{}) {
		t.Errorf("expected NULL description for empty string, got %v", mockDB.CreatedPins[0].Description)
	}
}

// TestNewHarvestPipeline_ImageCacheTTLDays covers HARVESTER_IMAGE_CACHE_TTL_DAYS
// parsing per harvester-image-cache-ttl Decision D3: valid > 0, zero,
// negative, non-numeric, and unset all resolve to a configured TTL exposed
// by ImageCacheTTLDays(). Invalid inputs fall back to the default.
func TestNewHarvestPipeline_ImageCacheTTLDays(t *testing.T) {
	cases := []struct {
		name  string
		env   string
		unset bool
		want  int
	}{
		{name: "unset uses default", unset: true, want: DefaultImageCacheTTLDays},
		{name: "valid positive", env: "30", want: 30},
		{name: "valid large", env: "365", want: 365},
		{name: "leading/trailing whitespace trimmed", env: "  45\t", want: 45},
		{name: "zero falls back", env: "0", want: DefaultImageCacheTTLDays},
		{name: "negative falls back", env: "-7", want: DefaultImageCacheTTLDays},
		{name: "non-numeric falls back", env: "forever", want: DefaultImageCacheTTLDays},
		{name: "blank string treated as unset", env: "   ", want: DefaultImageCacheTTLDays},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unset {
				// Register a cleanup via t.Setenv then genuinely unset the
				// variable so NewHarvestPipeline takes the "os.Getenv returns
				// empty because the variable was never set" branch. t.Setenv
				// ensures the prior value is restored even though we remove
				// the var within the test body.
				t.Setenv(imageCacheTTLDaysEnv, "restore-marker")
				if err := os.Unsetenv(imageCacheTTLDaysEnv); err != nil {
					t.Fatalf("os.Unsetenv failed: %v", err)
				}
			} else {
				t.Setenv(imageCacheTTLDaysEnv, tc.env)
			}
			p := NewHarvestPipeline(NewMockBotDB(), NewMockStorage())
			if got := p.ImageCacheTTLDays(); got != tc.want {
				t.Errorf("ImageCacheTTLDays() = %d, want %d (env=%q, unset=%v)", got, tc.want, tc.env, tc.unset)
			}
		})
	}
}

// TestWithImageCacheTTLDays exercises the programmatic option override of
// the image cache TTL, mirroring WithImageCacheMaxBytes.
func TestWithImageCacheTTLDays(t *testing.T) {
	t.Setenv(imageCacheTTLDaysEnv, "")
	if err := os.Unsetenv(imageCacheTTLDaysEnv); err != nil {
		t.Fatalf("os.Unsetenv failed: %v", err)
	}

	p := NewHarvestPipeline(NewMockBotDB(), NewMockStorage(), WithImageCacheTTLDays(7))
	if got := p.ImageCacheTTLDays(); got != 7 {
		t.Errorf("ImageCacheTTLDays() after option = %d, want 7", got)
	}

	// Zero or negative values are ignored; default or env survives.
	p2 := NewHarvestPipeline(NewMockBotDB(), NewMockStorage(), WithImageCacheTTLDays(0))
	if got := p2.ImageCacheTTLDays(); got != DefaultImageCacheTTLDays {
		t.Errorf("ImageCacheTTLDays() after zero option = %d, want default %d", got, DefaultImageCacheTTLDays)
	}
	p3 := NewHarvestPipeline(NewMockBotDB(), NewMockStorage(), WithImageCacheTTLDays(-5))
	if got := p3.ImageCacheTTLDays(); got != DefaultImageCacheTTLDays {
		t.Errorf("ImageCacheTTLDays() after negative option = %d, want default %d", got, DefaultImageCacheTTLDays)
	}
}

func TestExtensionFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"plain path", "https://example.com/a.png", ".png"},
		{"query stripped", "https://example.com/a.png?width=200", ".png"},
		{"fragment stripped", "https://example.com/a.png#frag", ".png"},
		{"query then fragment", "https://example.com/a.png?x=1#y", ".png"},
		{"question mark inside fragment", "https://example.com/a.png#y?z", ".png"},
		{"no extension", "https://example.com/image", ".bin"},
		{"percent-encoded hash preserved", "https://example.com/a%23b.jpg", ".jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extensionFromURL(tt.url); got != tt.want {
				t.Errorf("extensionFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
