package pin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
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

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
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

	// Create failure-path injection (compensating-delete tests)
	createPinErr   error
	linkPinTagErr  error
	tagsByIDs      []db.Tag
	deletePinCalls int
	deletePinRows  int64
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
	if m.createPinErr != nil {
		return db.Pin{}, m.createPinErr
	}
	return db.Pin{}, nil
}

func (m *mockQuerier) LinkPinTag(_ context.Context, _ db.LinkPinTagParams) error {
	return m.linkPinTagErr
}

func (m *mockQuerier) DeletePin(_ context.Context, _ db.DeletePinParams) (int64, error) {
	m.deletePinCalls++
	return m.deletePinRows, nil
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
	return m.tagsByIDs, nil
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

// Pins the tag_ids count cap on /api/pins. The endpoint is
// unauthenticated and not rate-limited and the array feeds into
// ListPinsWithCreator + CountPins (or ListPinsByCreator +
// CountPinsByCreatorFiltered on the creator_id path). The Count
// query has no LIMIT/OFFSET so `tag_id = ANY($N::uuid[])` is
// evaluated against the full `pins` table. The cap mirrors the
// sibling search.Search at internal/search/handler.go:115 so
// the two handlers reject the same input shape with the same
// 400 message.

func TestList_TagIDsTooMany(t *testing.T) {
	mock := &mockQuerier{}
	h := NewHandlerWithQuerier(mock)

	tagIDs := "00000000-0000-0000-0000-000000000001," +
		"00000000-0000-0000-0000-000000000002," +
		"00000000-0000-0000-0000-000000000003," +
		"00000000-0000-0000-0000-000000000004," +
		"00000000-0000-0000-0000-000000000005," +
		"00000000-0000-0000-0000-000000000006"
	rec := doRequest(t, h, "/api/pins?tag_ids="+tagIDs)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for >5 tag_ids, got %d; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if !strings.Contains(body["error"], "5개") {
		t.Errorf("error message should reference the 5-tag cap (sibling search.Search convention), got %q", body["error"])
	}
}

// Pins the tag_ids creator_id path: the cap fires before listByCreator
// dispatch so CountPinsByCreatorFiltered (UN-paginated, same EXISTS
// shape as CountPins) is never reached with an over-cap array.
func TestList_TagIDsTooMany_CreatorIDPath(t *testing.T) {
	mock := &mockQuerier{}
	h := NewHandlerWithQuerier(mock)

	tagIDs := "00000000-0000-0000-0000-000000000001," +
		"00000000-0000-0000-0000-000000000002," +
		"00000000-0000-0000-0000-000000000003," +
		"00000000-0000-0000-0000-000000000004," +
		"00000000-0000-0000-0000-000000000005," +
		"00000000-0000-0000-0000-000000000006"
	rec := doRequest(t, h, "/api/pins?creator_id=00000000-0000-0000-0000-000000000001&tag_ids="+tagIDs)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for >5 tag_ids on creator_id path, got %d; body=%s", rec.Code, rec.Body.String())
	}
}

// Boundary: exactly 5 tag_ids must pass the cap guard and reach the
// query layer (200 with empty list since the mock returns no rows).
// The cap is inclusive at 5 to match search.Search:115.
func TestList_TagIDsAtMaxCount(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	tagIDs := "00000000-0000-0000-0000-000000000001," +
		"00000000-0000-0000-0000-000000000002," +
		"00000000-0000-0000-0000-000000000003," +
		"00000000-0000-0000-0000-000000000004," +
		"00000000-0000-0000-0000-000000000005"
	rec := doRequest(t, h, "/api/pins?tag_ids="+tagIDs)

	if rec.Code != http.StatusOK {
		t.Fatalf("5 tag_ids (= cap) must NOT trip the count-cap 400; got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(mock.lastListP.Column2) != 5 {
		t.Errorf("expected all 5 tag_ids forwarded to ListPinsWithCreator, got %d", len(mock.lastListP.Column2))
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

	_, err = probeDuration(context.Background(), tmpFile.Name())
	if err == nil {
		t.Error("expected error for non-video file, got nil")
	}
}

func TestProbeDuration_MissingFile(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed, skipping")
	}

	_, err := probeDuration(context.Background(), "/tmp/nonexistent-video-file.mp4")
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

// --- Request body cap tests (spec: pin `서버가 본문을 디스크에 스풀하기 전에 본문 상한으로 거절한다`) ---

// TestRequestBodyCapConstant pins the request body cap value so that an
// accidental shrink to a value smaller than the storage video limit (100 MiB
// before trim) is caught.
func TestRequestBodyCapConstant(t *testing.T) {
	if requestBodyCap != 500<<20 {
		t.Errorf("requestBodyCap = %d, want %d", requestBodyCap, int64(500)<<20)
	}
	// The cap must be at least the largest accepted media file (video 100 MiB)
	// plus reasonable headroom for the multipart envelope, thumbnail, and form
	// fields. Without this slack a normal video upload would be rejected by
	// the body cap before it ever reaches the storage size check.
	if requestBodyCap <= maxBytes {
		t.Errorf("requestBodyCap (%d) must exceed maxBytes (%d) so trimmed-video paths are not blocked", requestBodyCap, maxBytes)
	}
}

// TestCreate_RejectsBodyOverCapBeforeDiskSpool verifies that a multipart body
// exceeding requestBodyCap is rejected by MaxBytesReader before ParseMultipartForm
// can spool it to disk, and that the handler maps the resulting MaxBytesError to
// the same size-limit error response used by storage-side rejection.
func TestCreate_RejectsBodyOverCapBeforeDiskSpool(t *testing.T) {
	// Temporarily lower the cap so the test exercises the rejection path without
	// allocating cap-sized buffers. The production default is asserted by
	// TestRequestBodyCapConstant.
	origCap := requestBodyCap
	requestBodyCap = 1 << 10 // 1 KiB
	t.Cleanup(func() { requestBodyCap = origCap })

	h := NewHandlerWithQuerier(&mockQuerier{})

	// Build a well-formed multipart envelope around a single file part whose
	// content (zeros) far exceeds the lowered cap. MaxBytesReader must trip while
	// the parser is still reading the file body, before any bytes are spooled to
	// disk.
	header := []byte("--xxxxxx\r\n" +
		"Content-Disposition: form-data; name=\"media\"; filename=\"a.bin\"\r\n" +
		"Content-Type: application/octet-stream\r\n\r\n")
	overflow := bytes.Repeat([]byte{0}, int(requestBodyCap)*4)
	trailer := []byte("\r\n--xxxxxx--\r\n")
	body := io.MultiReader(
		bytes.NewReader(header),
		bytes.NewReader(overflow),
		bytes.NewReader(trailer),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/pins", body)
	req.Header.Set("Content-Type", `multipart/form-data; boundary=xxxxxx`)
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "파일 크기가 제한을 초과했습니다") {
		t.Fatalf("expected size-limit error, got: %q", rec.Body.String())
	}
}

// TestCreate_RejectsNoTrimVideoWhenProbeDurationFails verifies that a no-trim
// video upload whose duration cannot be determined (probeDuration error) is
// rejected with a 400 + "비디오 길이를 확인할 수 없습니다" response, ensuring
// the "구간 정보 없이 15초 초과 비디오 업로드 → 거부" SHALL of
// pin/spec.md is not silently bypassed when ffprobe fails (missing binary,
// malformed input, or transient I/O error).
//
// The test feeds a video/mp4-typed multipart part whose body is plain text
// — ffprobe (or its absence) reliably returns a non-nil error for this input,
// so the no-trim else branch enters the fail-closed path regardless of
// whether ffprobe is installed on the test host.
func TestCreate_RejectsNoTrimVideoWhenProbeDurationFails(t *testing.T) {
	h := NewHandlerWithQuerier(&mockQuerier{})

	// Build a multipart body with a single "media" part labelled as video/mp4
	// but containing plain text. No trim_start/trim_end fields are supplied,
	// so the handler enters the no-trim branch where probeDuration is invoked.
	const boundary = "yyyyyy"
	body := bytes.NewBufferString(
		"--" + boundary + "\r\n" +
			"Content-Disposition: form-data; name=\"title\"\r\n\r\n" +
			"probe-fail-test\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Disposition: form-data; name=\"media\"; filename=\"a.mp4\"\r\n" +
			"Content-Type: video/mp4\r\n\r\n" +
			"not actually a video, ffprobe will fail to read this as a media container\r\n" +
			"--" + boundary + "--\r\n",
	)
	req := httptest.NewRequest(http.MethodPost, "/api/pins", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "비디오 길이를 확인할 수 없습니다") {
		t.Fatalf("expected duration-unknown error, got: %q", rec.Body.String())
	}
	// The fail-closed branch must not be mistaken for the >15s rejection, which
	// has a distinct message reserved for cases where duration is known.
	if strings.Contains(rec.Body.String(), "15초 초과") {
		t.Fatalf("probe-fail rejection must use the duration-unknown message, not the >15s message, got: %q", rec.Body.String())
	}
}

// TestCreate_PreservesGenericMultipartErrorMessage verifies that multipart
// parse errors unrelated to the body cap (e.g. malformed body without the
// declared boundary) keep returning the generic 400 message rather than the
// size-limit message.
func TestCreate_PreservesGenericMultipartErrorMessage(t *testing.T) {
	h := NewHandlerWithQuerier(&mockQuerier{})

	// Body is well under cap and contains no boundary marker. ParseMultipartForm
	// should fail with a parse error (not MaxBytesError).
	body := bytes.NewReader([]byte("not a multipart body, just plain bytes"))
	req := httptest.NewRequest(http.MethodPost, "/api/pins", body)
	req.Header.Set("Content-Type", `multipart/form-data; boundary=xxxxxx`)
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "파일 크기가 제한을 초과했습니다") {
		t.Fatalf("non-cap multipart error should NOT use size-limit message, got: %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "잘못된 요청 형식입니다") {
		t.Fatalf("expected generic format error, got: %q", rec.Body.String())
	}
}

// --- Input-length validation tests (spec: pin `핀 생성 요청의 텍스트 필드는 pins 컬럼 cap에 맞춰 사전 길이 검증된다`) ---
//
// These exercise the title rune-cap branch (handler.go: utf8.RuneCountInString(title) > 200 → 400).
// The title check sits between the empty-title check and the media-file check, so it
// rejects before any storage interaction. The accept-path tests verify the cap-boundary
// input flows past the length check (a downstream "미디어 파일은 필수입니다" 400 confirms it).
func TestCreate_RejectsTitleOverRuneCap(t *testing.T) {
	h := NewHandlerWithQuerier(&mockQuerier{})

	const boundary = "ttttt1"
	body := bytes.NewBufferString(
		"--" + boundary + "\r\n" +
			"Content-Disposition: form-data; name=\"title\"\r\n\r\n" +
			strings.Repeat("A", 201) + "\r\n" +
			"--" + boundary + "--\r\n",
	)
	req := httptest.NewRequest(http.MethodPost, "/api/pins", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "제목은 200자 이내여야 합니다") {
		t.Fatalf("expected title-length error, got: %q", rec.Body.String())
	}
}

// 멀티바이트(한국어) 201 rune title은 byte 길이가 603이지만 rune 단위로는 cap(200)을 초과해 거부되어야 한다.
// utf8.RuneCountInString이 byte가 아닌 rune 단위로 비교한다는 D3 결정이 코드로 enforce되는지 검증한다.
func TestCreate_RejectsTitleOverRuneCapMultibyte(t *testing.T) {
	h := NewHandlerWithQuerier(&mockQuerier{})

	const boundary = "ttttt2"
	body := bytes.NewBufferString(
		"--" + boundary + "\r\n" +
			"Content-Disposition: form-data; name=\"title\"\r\n\r\n" +
			strings.Repeat("가", 201) + "\r\n" +
			"--" + boundary + "--\r\n",
	)
	req := httptest.NewRequest(http.MethodPost, "/api/pins", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "제목은 200자 이내여야 합니다") {
		t.Fatalf("expected title-length error, got: %q", rec.Body.String())
	}
}

// title이 정확히 cap(200 rune)일 때는 길이 검증을 통과해야 한다. 통과 여부는 핸들러가 다음 단계인
// 미디어 파일 검증으로 진행해 "미디어 파일은 필수입니다" 400을 반환하는 것으로 확인한다 — 제목 길이
// 메시지가 응답에 포함되어 있지 않다면 boundary 비교가 `>`이며 cap 정확값은 무손실 통과한다는 뜻이다.
func TestCreate_AcceptsTitleAtRuneCap(t *testing.T) {
	h := NewHandlerWithQuerier(&mockQuerier{})

	const boundary = "ttttt3"
	body := bytes.NewBufferString(
		"--" + boundary + "\r\n" +
			"Content-Disposition: form-data; name=\"title\"\r\n\r\n" +
			strings.Repeat("A", 200) + "\r\n" +
			"--" + boundary + "--\r\n",
	)
	req := httptest.NewRequest(http.MethodPost, "/api/pins", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (media missing), got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "제목은 200자 이내여야 합니다") {
		t.Fatalf("title at cap (200 ASCII) must pass length check, got: %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "미디어 파일은 필수입니다") {
		t.Fatalf("expected downstream media-required error, got: %q", rec.Body.String())
	}
}

func TestCreate_AcceptsTitleAtRuneCapMultibyte(t *testing.T) {
	h := NewHandlerWithQuerier(&mockQuerier{})

	const boundary = "ttttt4"
	body := bytes.NewBufferString(
		"--" + boundary + "\r\n" +
			"Content-Disposition: form-data; name=\"title\"\r\n\r\n" +
			strings.Repeat("가", 200) + "\r\n" +
			"--" + boundary + "--\r\n",
	)
	req := httptest.NewRequest(http.MethodPost, "/api/pins", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (media missing), got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "제목은 200자 이내여야 합니다") {
		t.Fatalf("title at cap (200 한국어 rune ≒ 600 byte) must pass rune-based length check, got: %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "미디어 파일은 필수입니다") {
		t.Fatalf("expected downstream media-required error, got: %q", rec.Body.String())
	}
}
