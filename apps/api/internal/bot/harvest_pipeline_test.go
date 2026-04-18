package bot

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// MockBotDB implements BotDB for testing.
type MockBotDB struct {
	ExistingURLs map[string]bool // sourceURL -> exists
	CreatedPins  []db.CreatePinParams
	CreateErr    error
	DedupErr     error
}

func NewMockBotDB() *MockBotDB {
	return &MockBotDB{
		ExistingURLs: make(map[string]bool),
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

func TestHarvestPipeline_NewItems(t *testing.T) {
	// Set up a mock media server
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-image-data"))
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
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
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
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
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
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
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
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
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
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.jpg" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("data"))
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

func TestHarvestPipeline_ImageCache_Success(t *testing.T) {
	// Media server returns media bytes; image server returns a cacheable image.
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("media-bytes"))
	}))
	defer mediaServer.Close()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image-bytes"))
	}))
	defer imageServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	html := `<html><head><meta property="og:image" content="` + imageServer.URL + `/cover.png"></head></html>`
	items := []RawItem{
		{Title: "A", MediaURL: mediaServer.URL + "/m.jpg", MediaType: "image",
			SourceURL: "https://example.com/a", PageHTML: html},
	}
	_, _, _, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockDB.CreatedPins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(mockDB.CreatedPins))
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid {
		t.Fatalf("expected og_image set")
	}
	// Success path: og_image should point at storage (contains "images/" prefix in the key).
	if !strings.Contains(pin.OgImage.String, "images/") {
		t.Errorf("expected og_image to contain images/ prefix, got %q", pin.OgImage.String)
	}
}

func TestHarvestPipeline_ImageCache_DownloadFallback(t *testing.T) {
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("media"))
	}))
	defer mediaServer.Close()

	// Image server returns 403 → download fallback path.
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer imageServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	candidate := imageServer.URL + "/cover.jpg"
	html := `<html><head><meta property="og:image" content="` + candidate + `"></head></html>`

	items := []RawItem{
		{Title: "B", MediaURL: mediaServer.URL + "/m.jpg", MediaType: "image",
			SourceURL: "https://example.com/b", PageHTML: html},
	}
	_, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failed != 0 {
		t.Errorf("expected Pin creation to continue, failed=%d", failed)
	}
	if len(mockDB.CreatedPins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(mockDB.CreatedPins))
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid || pin.OgImage.String != candidate {
		t.Errorf("expected og_image fallback to original candidate %q, got valid=%v value=%q",
			candidate, pin.OgImage.Valid, pin.OgImage.String)
	}
}

func TestHarvestPipeline_ImageCache_NoCandidate(t *testing.T) {
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("media"))
	}))
	defer mediaServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	items := []RawItem{
		{Title: "C", MediaURL: mediaServer.URL + "/m.jpg", MediaType: "image",
			SourceURL: "https://example.com/c", PageHTML: "<html><body>empty</body></html>"},
	}
	_, _, _, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockDB.CreatedPins) != 1 {
		t.Fatalf("expected 1 pin")
	}
	if mockDB.CreatedPins[0].OgImage.Valid {
		t.Errorf("expected NULL og_image when no candidate")
	}
}

func TestHarvestPipeline_ImageCache_Oversize(t *testing.T) {
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("m"))
	}))
	defer mediaServer.Close()

	// Image server returns Content-Length way above the cap.
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "999999")
		_, _ = w.Write(make([]byte, 999999))
	}))
	defer imageServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()
	pipeline := NewHarvestPipeline(mockDB, mockStorage, WithImageCacheMaxBytes(1024))
	pipeline.client = mediaServer.Client()

	candidate := imageServer.URL + "/big.png"
	html := `<html><head><meta property="og:image" content="` + candidate + `"></head></html>`
	items := []RawItem{
		{Title: "D", MediaURL: mediaServer.URL + "/m.jpg", MediaType: "image",
			SourceURL: "https://example.com/d", PageHTML: html},
	}
	_, _, _, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid || pin.OgImage.String != candidate {
		t.Errorf("expected oversize fallback to original, got valid=%v value=%q",
			pin.OgImage.Valid, pin.OgImage.String)
	}
}

func TestHarvestPipeline_ImageCache_UploadFailure(t *testing.T) {
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("media"))
	}))
	defer mediaServer.Close()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("imgbytes"))
	}))
	defer imageServer.Close()

	mockDB := NewMockBotDB()
	mockStorage := NewMockStorage()

	candidate := imageServer.URL + "/c.png"

	// Fail uploads only for the image cache (key starts with "images/"), succeed for bot/ uploads.
	mockStorage.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
		if strings.HasPrefix(filename, "images/") {
			return "", fmt.Errorf("storage down")
		}
		return "https://cdn.example.com/media/" + filename, nil
	}

	pipeline := NewHarvestPipeline(mockDB, mockStorage)
	pipeline.client = mediaServer.Client()

	html := `<html><head><meta property="og:image" content="` + candidate + `"></head></html>`
	items := []RawItem{
		{Title: "E", MediaURL: mediaServer.URL + "/m.jpg", MediaType: "image",
			SourceURL: "https://example.com/e", PageHTML: html},
	}
	_, _, failed, err := pipeline.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failed != 0 {
		t.Errorf("pin creation must not fail, got failed=%d", failed)
	}
	pin := mockDB.CreatedPins[0]
	if !pin.OgImage.Valid || pin.OgImage.String != candidate {
		t.Errorf("expected upload-fail fallback to original candidate, got %q", pin.OgImage.String)
	}
}

func TestHarvestPipeline_EmptyDescription(t *testing.T) {
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
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
