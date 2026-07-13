package bot

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

const (
	cleanupTestCanonical = "https://example.com/post/1"
	cleanupTestNewURL    = "https://cdn.example.com/images/hash/2.png"
	cleanupTestOldURL    = "https://cdn.example.com/images/hash/1.png"
)

// newCleanupImageServer serves a valid PNG so cacheImage succeeds.
func newCleanupImageServer(t *testing.T) *httptest.Server {
	t.Helper()
	pngBytes := harvestTestPNG(64, 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newCleanupFailingImageServer always 500s so cacheImage falls back.
func newCleanupFailingImageServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newCleanupPipeline(t *testing.T, mockDB *MockBotDB, srv *httptest.Server) (*HarvestPipeline, *MockStorage) {
	t.Helper()
	ms := NewMockStorage()
	ms.UploadFunc = func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
		return cleanupTestNewURL, nil
	}
	p := NewHarvestPipeline(mockDB, ms)
	p.client = srv.Client()
	return p, ms
}

// Task 4.1 (a): upsert 실패 + 캐시 성공 → 새 객체 보상 삭제.
// spec: bot `미참조가 된 이미지 캐시 객체는 처리 경로에서 정리된다` — "Pin 영속화
// 실패 시 새 캐시 객체가 보상 삭제된다"
func TestProcessDocument_CompensatingDeleteOnUpsertFailure(t *testing.T) {
	srv := newCleanupImageServer(t)
	mockDB := NewMockBotDB()
	upsertErr := errors.New("db down")
	mockDB.CreateErr = upsertErr
	p, ms := newCleanupPipeline(t, mockDB, srv)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	created, pinID, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if !errors.Is(err, upsertErr) {
		t.Fatalf("expected upsert error to propagate, got %v", err)
	}
	if created || pinID != uuid.Nil {
		t.Errorf("failure semantics changed: created=%v pinID=%v", created, pinID)
	}
	if len(ms.DeletedURLs) != 1 || ms.DeletedURLs[0] != cleanupTestNewURL {
		t.Errorf("expected compensating delete of %q, got %v", cleanupTestNewURL, ms.DeletedURLs)
	}
}

// Task 4.1 (b): upsert 실패 + fallback → 삭제 호출 없음.
// spec: "캐시 저장 자체가 실패(fallback)한 처리의 영속화 실패에는 보상 삭제가 없다"
func TestProcessDocument_NoCompensatingDeleteWhenCacheFellBack(t *testing.T) {
	srv := newCleanupFailingImageServer(t)
	mockDB := NewMockBotDB()
	mockDB.CreateErr = errors.New("db down")
	p, ms := newCleanupPipeline(t, mockDB, srv)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err == nil {
		t.Fatal("expected upsert error")
	}
	if len(ms.DeletedURLs) != 0 {
		t.Errorf("expected no delete calls on fallback, got %v", ms.DeletedURLs)
	}
}

func seedExistingPin(mockDB *MockBotDB, canonical, ogImage string) {
	mockDB.ExistingURLs[canonical] = true
	mockDB.OgImages[canonical] = sql.NullString{String: ogImage, Valid: true}
}

// Task 4.2 (a): 재수집이 새 캐시 객체로 교체 → prev 삭제.
// spec: "재수집으로 새 캐시 객체로 교체되면 이전 객체가 삭제된다"
func TestProcessDocument_ReplacementDeletesPrev_NewCacheObject(t *testing.T) {
	srv := newCleanupImageServer(t)
	mockDB := NewMockBotDB()
	seedExistingPin(mockDB, cleanupTestCanonical, cleanupTestOldURL)
	p, ms := newCleanupPipeline(t, mockDB, srv)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	created, pinID, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if created {
		t.Errorf("created = true, want false on update")
	}
	if pinID == uuid.Nil {
		t.Errorf("pinID is Nil")
	}
	if len(ms.DeletedURLs) != 1 || ms.DeletedURLs[0] != cleanupTestOldURL {
		t.Errorf("expected delete of prev %q, got %v", cleanupTestOldURL, ms.DeletedURLs)
	}
}

// Task 4.2 (b): 재수집이 원본 URL fallback으로 교체 → prev 삭제.
// spec: "재수집이 원본 URL fallback으로 귀결되어도 이전 객체가 삭제된다"
func TestProcessDocument_ReplacementDeletesPrev_FallbackURL(t *testing.T) {
	srv := newCleanupFailingImageServer(t)
	mockDB := NewMockBotDB()
	seedExistingPin(mockDB, cleanupTestCanonical, cleanupTestOldURL)
	p, ms := newCleanupPipeline(t, mockDB, srv)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(ms.DeletedURLs) != 1 || ms.DeletedURLs[0] != cleanupTestOldURL {
		t.Errorf("expected delete of prev %q, got %v", cleanupTestOldURL, ms.DeletedURLs)
	}
}

// Task 4.2 (c): 재수집에서 대표 이미지 참조가 부재(NULL)로 교체 → prev 삭제.
// spec: "재수집에서 대표 이미지 후보가 사라져 참조가 비워져도 이전 객체가 삭제된다"
func TestProcessDocument_ReplacementDeletesPrev_NullReference(t *testing.T) {
	mockDB := NewMockBotDB()
	seedExistingPin(mockDB, cleanupTestCanonical, cleanupTestOldURL)
	ms := NewMockStorage()
	p := NewHarvestPipeline(mockDB, ms)

	// No ThumbnailURL → og_image stays NULL; media comes from a candidate.
	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		MediaCandidates: []MediaCandidate{
			{Type: "video", URL: "https://cdn.example.com/v.mp4"},
		},
	}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(ms.DeletedURLs) != 1 || ms.DeletedURLs[0] != cleanupTestOldURL {
		t.Errorf("expected delete of prev %q, got %v", cleanupTestOldURL, ms.DeletedURLs)
	}
}

// Task 4.4 (a): prev == new → 삭제 호출 없음.
// spec: "참조 값이 교체되지 않은 재수집에서는 삭제하지 않는다"
func TestProcessDocument_NoDeleteWhenReferenceUnchanged(t *testing.T) {
	srv := newCleanupImageServer(t)
	mockDB := NewMockBotDB()
	seedExistingPin(mockDB, cleanupTestCanonical, cleanupTestNewURL) // prev == what the cache will return
	p, ms := newCleanupPipeline(t, mockDB, srv)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(ms.DeletedURLs) != 0 {
		t.Errorf("expected no delete when prev == new, got %v", ms.DeletedURLs)
	}
}

// Task 4.4 (b): 신규 insert(prev NULL) → 삭제 호출 없음.
// spec: "신규 수집(이전 참조 부재)에서는 삭제하지 않는다"
func TestProcessDocument_NoDeleteOnFreshInsert(t *testing.T) {
	srv := newCleanupImageServer(t)
	mockDB := NewMockBotDB()
	p, ms := newCleanupPipeline(t, mockDB, srv)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	created, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if !created {
		t.Errorf("created = false, want true on fresh insert")
	}
	if len(ms.DeletedURLs) != 0 {
		t.Errorf("expected no delete on fresh insert, got %v", ms.DeletedURLs)
	}
}

// Task 4.5: 삭제 실패 시 반환값 불변 + 로그에 대상 URL과 사유 포함.
// spec: "이전 객체 삭제 실패는 Pin 처리 결과에 영향을 주지 않는다"
func TestProcessDocument_DeleteFailureDoesNotAffectResult(t *testing.T) {
	srv := newCleanupImageServer(t)
	mockDB := NewMockBotDB()
	seedExistingPin(mockDB, cleanupTestCanonical, cleanupTestOldURL)
	p, ms := newCleanupPipeline(t, mockDB, srv)
	ms.DeleteByURLFunc = func(ctx context.Context, url string) error {
		return errors.New("storage unavailable")
	}

	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	created, pinID, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("delete failure must not fail the pipeline: %v", err)
	}
	if created {
		t.Errorf("created = true, want false on update")
	}
	if pinID == uuid.Nil {
		t.Errorf("pinID is Nil")
	}
	logged := logBuf.String()
	if !strings.Contains(logged, cleanupTestOldURL) {
		t.Errorf("log missing target URL %q: %s", cleanupTestOldURL, logged)
	}
	if !strings.Contains(logged, "storage unavailable") {
		t.Errorf("log missing failure reason: %s", logged)
	}
}

// Task 4.5 (보상 삭제 변형): 보상 삭제 실패가 upsert 오류 반환을 바꾸지 않는다.
func TestProcessDocument_CompensatingDeleteFailureKeepsUpsertError(t *testing.T) {
	srv := newCleanupImageServer(t)
	mockDB := NewMockBotDB()
	upsertErr := errors.New("db down")
	mockDB.CreateErr = upsertErr
	p, ms := newCleanupPipeline(t, mockDB, srv)
	ms.DeleteByURLFunc = func(ctx context.Context, url string) error {
		return errors.New("storage unavailable")
	}

	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
	}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if !errors.Is(err, upsertErr) {
		t.Fatalf("expected original upsert error, got %v", err)
	}
	logged := logBuf.String()
	if !strings.Contains(logged, cleanupTestNewURL) || !strings.Contains(logged, "storage unavailable") {
		t.Errorf("log missing target URL or reason: %s", logged)
	}
}

// Task 4.7: og_data 직렬화 실패 시 캐시 업로드가 발생하지 않는다 — 직렬화가
// 캐시 저장보다 앞에 있어야 무보상 고아 창이 없다는 D3 순서의 회귀 방지.
func TestProcessDocument_MarshalFailurePreventsCacheUpload(t *testing.T) {
	srv := newCleanupImageServer(t)
	mockDB := NewMockBotDB()
	p, ms := newCleanupPipeline(t, mockDB, srv)

	// time.Time with year 10000 makes json.Marshal fail
	// ("Time.MarshalJSON: year outside of range [0,9999]").
	badTime := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	doc := PinDocument{
		Title:        "t",
		CanonicalURL: cleanupTestCanonical,
		ThumbnailURL: srv.URL + "/thumb.png",
		OGData:       OGData{PublishedAt: &badTime},
	}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if ms.CallCount != 0 {
		t.Errorf("expected no cache upload after marshal failure, got %d uploads", ms.CallCount)
	}
	if len(ms.DeletedURLs) != 0 {
		t.Errorf("expected no delete calls, got %v", ms.DeletedURLs)
	}
}

// Task 4.3: StorageAdapter.DeleteByURL의 네임스페이스 판정. 삭제는 자사
// public URL 공간의 이미지 캐시 네임스페이스 key에만 허용된다.
// spec: "이전 참조가 외부 URL이면 삭제를 시도하지 않는다" / "자사 저장소의
// 캐시 네임스페이스 밖 객체는 삭제하지 않는다"
func TestStorageAdapter_DeleteByURL_NamespaceScoping(t *testing.T) {
	var deleteCalls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalls = append(deleteCalls, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client, err := storage.NewClient(storage.Config{
		Endpoint:  srv.URL,
		Bucket:    "test-bucket",
		AccessKey: "test",
		SecretKey: "test",
		PublicURL: "https://cdn.example.com/test-bucket",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	adapter := NewStorageAdapter(client)
	ctx := context.Background()

	// External URL → no delete, success.
	if err := adapter.DeleteByURL(ctx, "https://other.example.com/images/x.png"); err != nil {
		t.Errorf("external URL: %v", err)
	}
	// Own URL but user-media namespace → no delete, success.
	if err := adapter.DeleteByURL(ctx, "https://cdn.example.com/test-bucket/image/user.png"); err != nil {
		t.Errorf("user media key: %v", err)
	}
	// Own URL, prefix-similar namespace without separator → no delete, success.
	if err := adapter.DeleteByURL(ctx, "https://cdn.example.com/test-bucket/imagesfoo/x.png"); err != nil {
		t.Errorf("imagesfoo key: %v", err)
	}
	if len(deleteCalls) != 0 {
		t.Fatalf("expected no S3 deletes for out-of-namespace URLs, got %v", deleteCalls)
	}

	// Image cache namespace key → delete performed.
	if err := adapter.DeleteByURL(ctx, "https://cdn.example.com/test-bucket/images/hash/1.png"); err != nil {
		t.Errorf("cache key delete: %v", err)
	}
	if len(deleteCalls) != 1 || deleteCalls[0] != "/test-bucket/images/hash/1.png" {
		t.Errorf("expected S3 delete of cache key, got %v", deleteCalls)
	}
}
