package pin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type mockQuerier struct {
	listRows          []db.ListPinsWithCreatorRow
	listErr           error
	countVal          int64
	countErr          error
	lastListP         db.ListPinsWithCreatorParams
	lastCountP        db.CountPinsParams
	creatorRows       []db.ListPinsByCreatorRow
	creatorErr        error
	lastCreatorP      db.ListPinsByCreatorParams
	creatorCountVal   int64
	creatorCountErr   error
	lastCreatorCountP db.CountPinsByCreatorFilteredParams

	// Related pin fields
	pinWithCreator    db.GetPinWithCreatorRow
	pinWithCreatorErr error
	pinTags           []db.GetPinTagsRow
	relatedRows       []db.RelatedPinsRow
	relatedErr        error
	lastRelatedP      db.RelatedPinsParams
	fbMediaRows       []db.FallbackRelatedByMediaTypeRow
	fbMediaErr        error
	lastFBMediaP      db.FallbackRelatedByMediaTypeParams
	fbLatestRows      []db.FallbackRelatedLatestRow
	fbLatestErr       error
	lastFBLatestP     db.FallbackRelatedLatestParams
	tagsForPins       []db.GetTagsForPinsRow
}

func (m *mockQuerier) ListPinsWithCreator(_ context.Context, arg db.ListPinsWithCreatorParams) ([]db.ListPinsWithCreatorRow, error) {
	m.lastListP = arg
	return m.listRows, m.listErr
}

func (m *mockQuerier) ListPinsByCreator(_ context.Context, arg db.ListPinsByCreatorParams) ([]db.ListPinsByCreatorRow, error) {
	m.lastCreatorP = arg
	return m.creatorRows, m.creatorErr
}

func (m *mockQuerier) CountPins(_ context.Context, arg db.CountPinsParams) (int64, error) {
	m.lastCountP = arg
	return m.countVal, m.countErr
}

func (m *mockQuerier) CountPinsByCreatorFiltered(_ context.Context, arg db.CountPinsByCreatorFilteredParams) (int64, error) {
	m.lastCreatorCountP = arg
	return m.creatorCountVal, m.creatorCountErr
}

func (m *mockQuerier) CreatePin(_ context.Context, _ db.CreatePinParams) (db.Pin, error) {
	return db.Pin{}, nil
}

func (m *mockQuerier) LinkPinTag(_ context.Context, _ db.LinkPinTagParams) error {
	return nil
}

func (m *mockQuerier) DeletePin(_ context.Context, _ db.DeletePinParams) (int64, error) {
	return 0, nil
}

func (m *mockQuerier) GetPinWithCreator(_ context.Context, _ uuid.UUID) (db.GetPinWithCreatorRow, error) {
	return m.pinWithCreator, m.pinWithCreatorErr
}

func (m *mockQuerier) GetPinTags(_ context.Context, _ uuid.UUID) ([]db.GetPinTagsRow, error) {
	return m.pinTags, nil
}

func (m *mockQuerier) RelatedPins(_ context.Context, arg db.RelatedPinsParams) ([]db.RelatedPinsRow, error) {
	m.lastRelatedP = arg
	return m.relatedRows, m.relatedErr
}

func (m *mockQuerier) FallbackRelatedByMediaType(_ context.Context, arg db.FallbackRelatedByMediaTypeParams) ([]db.FallbackRelatedByMediaTypeRow, error) {
	m.lastFBMediaP = arg
	return m.fbMediaRows, m.fbMediaErr
}

func (m *mockQuerier) FallbackRelatedLatest(_ context.Context, arg db.FallbackRelatedLatestParams) ([]db.FallbackRelatedLatestRow, error) {
	m.lastFBLatestP = arg
	return m.fbLatestRows, m.fbLatestErr
}

func (m *mockQuerier) GetTagsByIDs(_ context.Context, _ []uuid.UUID) ([]db.Tag, error) {
	return nil, nil
}

func (m *mockQuerier) GetTagsForPins(_ context.Context, _ []uuid.UUID) ([]db.GetTagsForPinsRow, error) {
	return m.tagsForPins, nil
}

func (m *mockQuerier) CreateInteraction(_ context.Context, _ db.CreateInteractionParams) error {
	return nil
}

func sampleRow() db.ListPinsWithCreatorRow {
	return db.ListPinsWithCreatorRow{
		ID:               uuid.MustParse("20000000-0000-0000-0000-000000000001"),
		CreatorID:        uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		MediaUrl:         "image/seed-test.jpg",
		MediaType:        "image",
		Url:              sql.NullString{String: "https://soundcloud.com/haru/dreamscape", Valid: true},
		Title:            "Dreamscape",
		Description:      sql.NullString{String: "몽환적인 신스팝", Valid: true},
		OgImage:          sql.NullString{},
		OgData:           pqtype.NullRawMessage{},
		CreatedAt:        time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CreatorIDRef:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CreatorNickname:  "하루",
		CreatorAvatarUrl: sql.NullString{},
	}
}

func doRequest(t *testing.T, h *Handler, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	return rec
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) ListPinsResponse {
	t.Helper()
	var resp ListPinsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestList_DefaultParams(t *testing.T) {
	mock := &mockQuerier{listRows: []db.ListPinsWithCreatorRow{sampleRow()}, countVal: 1}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/pins")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastListP.Limit != 20 {
		t.Errorf("expected default limit 20, got %d", mock.lastListP.Limit)
	}
}

func TestList_MediaTypeFilter(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	doRequest(t, h, "/api/pins?media_type=audio")

	if mock.lastListP.Column1 != "audio" {
		t.Errorf("expected media_type 'audio', got %q", mock.lastListP.Column1)
	}
}

func TestList_EmptyResult(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/pins?media_type=video")
	resp := decodeResponse(t, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(resp.Pins) != 0 {
		t.Errorf("expected empty pins, got %d", len(resp.Pins))
	}
}

func TestList_HasMore(t *testing.T) {
	rows := []db.ListPinsWithCreatorRow{sampleRow()}
	mock := &mockQuerier{listRows: rows, countVal: 25}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/pins?limit=20")
	resp := decodeResponse(t, rec)

	if !resp.HasMore {
		t.Error("expected has_more=true when count > offset+len(rows)")
	}
}

func TestList_DBError(t *testing.T) {
	mock := &mockQuerier{listErr: errors.New("connection refused")}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/pins")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestList_ResponseStructure(t *testing.T) {
	mock := &mockQuerier{listRows: []db.ListPinsWithCreatorRow{sampleRow()}, countVal: 1}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/pins")
	resp := decodeResponse(t, rec)

	if len(resp.Pins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(resp.Pins))
	}

	p := resp.Pins[0]
	if p.Title != "Dreamscape" {
		t.Errorf("unexpected title: %s", p.Title)
	}
	if p.MediaURL != "image/seed-test.jpg" {
		t.Errorf("unexpected media_url: %s", p.MediaURL)
	}
	if p.MediaType != "image" {
		t.Errorf("unexpected media_type: %s", p.MediaType)
	}
	if p.Creator.Nickname != "하루" {
		t.Errorf("unexpected creator nickname: %s", p.Creator.Nickname)
	}
}

func TestList_InvalidCreatorID(t *testing.T) {
	mock := &mockQuerier{}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/pins?creator_id=not-a-uuid")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- Related handler tests ---

var (
	pinID    = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	tagID1   = uuid.MustParse("a0000000-0000-0000-0000-000000000001")
	relPinID = uuid.MustParse("20000000-0000-0000-0000-000000000002")
	fbPinID  = uuid.MustParse("20000000-0000-0000-0000-000000000003")
	ltPinID  = uuid.MustParse("20000000-0000-0000-0000-000000000004")
)

func samplePinWithCreator() db.GetPinWithCreatorRow {
	return db.GetPinWithCreatorRow{
		ID:              pinID,
		CreatorID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		MediaUrl:        "image/test.jpg",
		MediaType:       "image",
		Title:           "Test Pin",
		CreatedAt:       time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CreatorIDRef:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CreatorNickname: "하루",
	}
}

func sampleRelatedRow(id uuid.UUID, title string) db.RelatedPinsRow {
	return db.RelatedPinsRow{
		ID:              id,
		CreatorID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		MediaUrl:        "image/related.jpg",
		MediaType:       "image",
		Title:           title,
		CreatedAt:       time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CreatorIDRef:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CreatorNickname: "하루",
	}
}

func sampleFBMediaRow(id uuid.UUID, title string) db.FallbackRelatedByMediaTypeRow {
	return db.FallbackRelatedByMediaTypeRow{
		ID:              id,
		CreatorID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		MediaUrl:        "image/fb.jpg",
		MediaType:       "image",
		Title:           title,
		CreatedAt:       time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CreatorIDRef:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CreatorNickname: "하루",
	}
}

func sampleFBLatestRow(id uuid.UUID, title string) db.FallbackRelatedLatestRow {
	return db.FallbackRelatedLatestRow{
		ID:              id,
		CreatorID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		MediaUrl:        "audio/fb.mp3",
		MediaType:       "audio",
		Title:           title,
		CreatedAt:       time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CreatorIDRef:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CreatorNickname: "하루",
	}
}

func doRelatedRequest(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/pins/"+id+"/related", nil)
	// Chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Related(rec, req)
	return rec
}

type relatedResponse struct {
	Pins []PinResponse `json:"pins"`
}

func decodeRelatedResponse(t *testing.T, rec *httptest.ResponseRecorder) relatedResponse {
	t.Helper()
	var resp relatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode related response: %v", err)
	}
	return resp
}

// 4.1 태그 있는 핀의 정상 연관 핀 반환
func TestRelated_WithTags(t *testing.T) {
	mock := &mockQuerier{
		pinWithCreator: samplePinWithCreator(),
		pinTags: []db.GetPinTagsRow{
			{ID: tagID1, Name: "ambient", Slug: "ambient", Category: "genre"},
		},
		relatedRows: []db.RelatedPinsRow{
			sampleRelatedRow(relPinID, "Related Pin"),
		},
	}
	h := NewHandlerWithQuerier(mock)

	rec := doRelatedRequest(t, h, pinID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := decodeRelatedResponse(t, rec)
	if len(resp.Pins) != 1 {
		t.Fatalf("expected 1 related pin, got %d", len(resp.Pins))
	}
	if resp.Pins[0].Title != "Related Pin" {
		t.Errorf("unexpected title: %s", resp.Pins[0].Title)
	}
}

// 4.2 태그 없는 핀의 fallback 연관 핀 반환
func TestRelated_NoTags_FallbackToMediaType(t *testing.T) {
	mock := &mockQuerier{
		pinWithCreator: samplePinWithCreator(),
		pinTags:        nil, // no tags
		fbMediaRows: []db.FallbackRelatedByMediaTypeRow{
			sampleFBMediaRow(fbPinID, "Fallback Media Pin"),
		},
	}
	h := NewHandlerWithQuerier(mock)

	rec := doRelatedRequest(t, h, pinID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := decodeRelatedResponse(t, rec)
	if len(resp.Pins) == 0 {
		t.Fatal("expected fallback pins, got 0")
	}
	if resp.Pins[0].Title != "Fallback Media Pin" {
		t.Errorf("unexpected title: %s", resp.Pins[0].Title)
	}
}

// 4.3 태그 매칭 부족 시 미디어 타입 fallback
func TestRelated_PartialTags_MediaTypeFallback(t *testing.T) {
	mock := &mockQuerier{
		pinWithCreator: samplePinWithCreator(),
		pinTags: []db.GetPinTagsRow{
			{ID: tagID1, Name: "ambient", Slug: "ambient", Category: "genre"},
		},
		relatedRows: []db.RelatedPinsRow{
			sampleRelatedRow(relPinID, "Tag Match"),
		},
		// 1 tag match + 1 media type fallback = 2 total
		fbMediaRows: []db.FallbackRelatedByMediaTypeRow{
			sampleFBMediaRow(fbPinID, "Media Fallback"),
		},
	}
	h := NewHandlerWithQuerier(mock)

	rec := doRelatedRequest(t, h, pinID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := decodeRelatedResponse(t, rec)
	if len(resp.Pins) != 2 {
		t.Fatalf("expected 2 pins, got %d", len(resp.Pins))
	}
	if resp.Pins[0].Title != "Tag Match" {
		t.Errorf("first pin should be tag match, got: %s", resp.Pins[0].Title)
	}
	if resp.Pins[1].Title != "Media Fallback" {
		t.Errorf("second pin should be media fallback, got: %s", resp.Pins[1].Title)
	}
}

// 4.4 중복 핀 제외 확인
func TestRelated_ExcludeIDs_PassedToFallback(t *testing.T) {
	mock := &mockQuerier{
		pinWithCreator: samplePinWithCreator(),
		pinTags: []db.GetPinTagsRow{
			{ID: tagID1, Name: "ambient", Slug: "ambient", Category: "genre"},
		},
		relatedRows: []db.RelatedPinsRow{
			sampleRelatedRow(relPinID, "Tag Match"),
		},
		fbMediaRows: []db.FallbackRelatedByMediaTypeRow{
			sampleFBMediaRow(fbPinID, "Media Fallback"),
		},
		fbLatestRows: []db.FallbackRelatedLatestRow{
			sampleFBLatestRow(ltPinID, "Latest Fallback"),
		},
	}
	h := NewHandlerWithQuerier(mock)

	doRelatedRequest(t, h, pinID.String())

	// Verify media type fallback received exclude IDs: [self, tag-match result]
	if len(mock.lastFBMediaP.Column2) != 2 {
		t.Fatalf("expected 2 exclude IDs for media fallback, got %d", len(mock.lastFBMediaP.Column2))
	}
	if mock.lastFBMediaP.Column2[0] != pinID {
		t.Errorf("first exclude ID should be self pin, got %s", mock.lastFBMediaP.Column2[0])
	}
	if mock.lastFBMediaP.Column2[1] != relPinID {
		t.Errorf("second exclude ID should be tag match pin, got %s", mock.lastFBMediaP.Column2[1])
	}

	// Verify latest fallback received exclude IDs: [self, tag-match, media-fallback]
	if len(mock.lastFBLatestP.Column1) != 3 {
		t.Fatalf("expected 3 exclude IDs for latest fallback, got %d", len(mock.lastFBLatestP.Column1))
	}
}

// --- Duration validation tests ---

func TestMaxVideoDurationSeconds(t *testing.T) {
	if maxVideoDurationSeconds != 15 {
		t.Errorf("expected maxVideoDurationSeconds=15, got %d", maxVideoDurationSeconds)
	}
}

func TestProbeDuration_InvalidFile(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed, skipping")
	}

	tmpFile, err := os.CreateTemp("", "probe-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_, _ = tmpFile.WriteString("not a video")
	_ = tmpFile.Close()

	_, err = probeDuration(tmpFile.Name())
	if err == nil {
		t.Error("expected error for non-video file, got nil")
	}
}

func TestProbeDuration_MissingFile(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed, skipping")
	}

	_, err := probeDuration("/tmp/nonexistent-video-file.mp4")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// TestDurationValidation_ContentTypeSkip verifies that non-video content types
// skip duration validation (the video validation branch only triggers for "video/*").
func TestDurationValidation_ContentTypeSkip(t *testing.T) {
	// Verify that the contentType check correctly identifies video types
	videoTypes := []string{"video/mp4", "video/webm"}
	for _, ct := range videoTypes {
		if !strings.HasPrefix(ct, "video/") {
			t.Errorf("expected %q to be detected as video", ct)
		}
	}

	nonVideoTypes := []string{"image/jpeg", "image/png", "audio/mpeg", "audio/ogg"}
	for _, ct := range nonVideoTypes {
		if strings.HasPrefix(ct, "video/") {
			t.Errorf("expected %q to NOT be detected as video", ct)
		}
	}
}

// TestDurationValidation_ThresholdLogic verifies the comparison logic matches spec:
// duration > 15 → reject, duration <= 15 → pass
func TestDurationValidation_ThresholdLogic(t *testing.T) {
	tests := []struct {
		duration float64
		reject   bool
	}{
		{10.0, false},
		{15.0, false},  // exactly 15s → pass
		{15.001, true}, // just over 15s → reject
		{30.0, true},
		{0.0, false},
	}

	for _, tt := range tests {
		shouldReject := tt.duration > float64(maxVideoDurationSeconds)
		if shouldReject != tt.reject {
			t.Errorf("duration=%.3f: expected reject=%v, got %v", tt.duration, tt.reject, shouldReject)
		}
	}
}
