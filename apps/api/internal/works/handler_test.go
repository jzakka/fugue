package works

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// mockQuerier implements WorksQuerier for testing.
type mockQuerier struct {
	listRows             []db.ListWorksWithCreatorRow
	listErr              error
	countVal             int64
	countErr             error
	lastListP            db.ListWorksWithCreatorParams
	lastCountP           db.CountWorksParams
	creatorRows          []db.ListWorksByCreatorRow
	creatorErr           error
	lastCreatorP         db.ListWorksByCreatorParams
	creatorCountVal      int64
	creatorCountErr      error
	lastCreatorCountP    db.CountWorksByCreatorFilteredParams
}

func (m *mockQuerier) ListWorksWithCreator(_ context.Context, arg db.ListWorksWithCreatorParams) ([]db.ListWorksWithCreatorRow, error) {
	m.lastListP = arg
	return m.listRows, m.listErr
}

func (m *mockQuerier) ListWorksByCreator(_ context.Context, arg db.ListWorksByCreatorParams) ([]db.ListWorksByCreatorRow, error) {
	m.lastCreatorP = arg
	return m.creatorRows, m.creatorErr
}

func (m *mockQuerier) CountWorks(_ context.Context, arg db.CountWorksParams) (int64, error) {
	m.lastCountP = arg
	return m.countVal, m.countErr
}

func (m *mockQuerier) CountWorksByCreatorFiltered(_ context.Context, arg db.CountWorksByCreatorFilteredParams) (int64, error) {
	m.lastCreatorCountP = arg
	return m.creatorCountVal, m.creatorCountErr
}

func sampleRow() db.ListWorksWithCreatorRow {
	return db.ListWorksWithCreatorRow{
		ID:               uuid.MustParse("20000000-0000-0000-0000-000000000001"),
		CreatorID:        uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Url:              "https://soundcloud.com/haru/dreamscape",
		Title:            "Dreamscape",
		Description:      sql.NullString{String: "몽환적인 신스팝", Valid: true},
		Field:            "음악",
		Tags:             []string{"신스팝", "몽환"},
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

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) ListWorksResponse {
	t.Helper()
	var resp ListWorksResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- Tests ---

func TestList_DefaultParams(t *testing.T) {
	mock := &mockQuerier{listRows: []db.ListWorksWithCreatorRow{sampleRow()}, countVal: 1}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	// Default limit=20, offset=0
	if mock.lastListP.Limit != 20 {
		t.Errorf("expected default limit 20, got %d", mock.lastListP.Limit)
	}
	if mock.lastListP.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", mock.lastListP.Offset)
	}
}

func TestList_FieldFilter(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	doRequest(t, h, "/api/works?field=음악")

	if mock.lastListP.Column1 != "음악" {
		t.Errorf("expected field '음악', got %q", mock.lastListP.Column1)
	}
}

func TestList_TagsParsing(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	doRequest(t, h, "/api/works?tags=신스팝,몽환")

	if len(mock.lastListP.Column2) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(mock.lastListP.Column2))
	}
	if mock.lastListP.Column2[0] != "신스팝" || mock.lastListP.Column2[1] != "몽환" {
		t.Errorf("unexpected tags: %v", mock.lastListP.Column2)
	}
}

func TestList_EmptyTags(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	doRequest(t, h, "/api/works")

	if mock.lastListP.Column2 != nil {
		t.Errorf("expected nil tags for no tags param, got %v", mock.lastListP.Column2)
	}
}

func TestList_LimitClamping(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int32
	}{
		{"over 50 clamped to 50", "/api/works?limit=100", 50},
		{"negative uses default 20", "/api/works?limit=-5", 20},
		{"zero uses default 20", "/api/works?limit=0", 20},
		{"valid limit passed through", "/api/works?limit=10", 10},
		{"non-numeric uses default 20", "/api/works?limit=abc", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockQuerier{listRows: nil, countVal: 0}
			h := NewHandlerWithQuerier(mock)
			doRequest(t, h, tt.query)
			if mock.lastListP.Limit != tt.expected {
				t.Errorf("expected limit %d, got %d", tt.expected, mock.lastListP.Limit)
			}
		})
	}
}

func TestList_OffsetClamping(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int32
	}{
		{"negative clamped to 0", "/api/works?offset=-10", 0},
		{"zero stays 0", "/api/works?offset=0", 0},
		{"valid offset passed through", "/api/works?offset=20", 20},
		{"non-numeric uses default 0", "/api/works?offset=abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockQuerier{listRows: nil, countVal: 0}
			h := NewHandlerWithQuerier(mock)
			doRequest(t, h, tt.query)
			if mock.lastListP.Offset != tt.expected {
				t.Errorf("expected offset %d, got %d", tt.expected, mock.lastListP.Offset)
			}
		})
	}
}

func TestList_EmptyResult(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works?field=nonexistent")
	resp := decodeResponse(t, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(resp.Works) != 0 {
		t.Errorf("expected empty works, got %d", len(resp.Works))
	}
	if resp.HasMore {
		t.Error("expected has_more=false for empty result")
	}
}

func TestList_HasMore(t *testing.T) {
	rows := []db.ListWorksWithCreatorRow{sampleRow()}
	mock := &mockQuerier{listRows: rows, countVal: 25} // 25 total, first page has 1 row
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works?limit=20")
	resp := decodeResponse(t, rec)

	if !resp.HasMore {
		t.Error("expected has_more=true when count > offset+len(rows)")
	}
}

func TestList_NoMore(t *testing.T) {
	rows := []db.ListWorksWithCreatorRow{sampleRow()}
	mock := &mockQuerier{listRows: rows, countVal: 1}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works")
	resp := decodeResponse(t, rec)

	if resp.HasMore {
		t.Error("expected has_more=false when count <= offset+len(rows)")
	}
}

func TestList_DBError(t *testing.T) {
	mock := &mockQuerier{listErr: errors.New("connection refused")}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestList_CountError(t *testing.T) {
	mock := &mockQuerier{
		listRows: []db.ListWorksWithCreatorRow{sampleRow()},
		countErr: errors.New("connection refused"),
	}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestList_ResponseStructure(t *testing.T) {
	mock := &mockQuerier{listRows: []db.ListWorksWithCreatorRow{sampleRow()}, countVal: 1}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works")
	resp := decodeResponse(t, rec)

	if len(resp.Works) != 1 {
		t.Fatalf("expected 1 work, got %d", len(resp.Works))
	}

	w := resp.Works[0]
	if w.ID != "20000000-0000-0000-0000-000000000001" {
		t.Errorf("unexpected ID: %s", w.ID)
	}
	if w.Title != "Dreamscape" {
		t.Errorf("unexpected title: %s", w.Title)
	}
	if w.Field != "음악" {
		t.Errorf("unexpected field: %s", w.Field)
	}
	if w.Creator.Nickname != "하루" {
		t.Errorf("unexpected creator nickname: %s", w.Creator.Nickname)
	}
	if w.Creator.ID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("unexpected creator ID: %s", w.Creator.ID)
	}
}

func TestList_NullFields(t *testing.T) {
	row := sampleRow()
	row.Description = sql.NullString{}      // NULL
	row.OgImage = sql.NullString{}          // NULL
	row.OgData = pqtype.NullRawMessage{}    // NULL
	row.CreatorAvatarUrl = sql.NullString{} // NULL

	mock := &mockQuerier{listRows: []db.ListWorksWithCreatorRow{row}, countVal: 1}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works")

	// Parse raw JSON to check null fields
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}

	var works []map[string]json.RawMessage
	if err := json.Unmarshal(raw["works"], &works); err != nil {
		t.Fatalf("decode works array: %v", err)
	}

	w := works[0]
	if string(w["description"]) != "null" {
		t.Errorf("expected null description, got %s", string(w["description"]))
	}
	if string(w["og_image"]) != "null" {
		t.Errorf("expected null og_image, got %s", string(w["og_image"]))
	}
	if string(w["og_data"]) != "null" {
		t.Errorf("expected null og_data, got %s", string(w["og_data"]))
	}

	var creator map[string]json.RawMessage
	if err := json.Unmarshal(w["creator"], &creator); err != nil {
		t.Fatalf("decode creator: %v", err)
	}
	if string(creator["avatar_url"]) != "null" {
		t.Errorf("expected null avatar_url, got %s", string(creator["avatar_url"]))
	}
}

func TestList_NilTagsReturnsEmptyArray(t *testing.T) {
	row := sampleRow()
	row.Tags = nil

	mock := &mockQuerier{listRows: []db.ListWorksWithCreatorRow{row}, countVal: 1}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works")

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var works []map[string]json.RawMessage
	if err := json.Unmarshal(raw["works"], &works); err != nil {
		t.Fatalf("decode works: %v", err)
	}
	// tags should be [] not null
	if string(works[0]["tags"]) == "null" {
		t.Error("expected tags to be [] not null")
	}
}

func sampleCreatorRow() db.ListWorksByCreatorRow {
	return db.ListWorksByCreatorRow{
		ID:               uuid.MustParse("20000000-0000-0000-0000-000000000001"),
		CreatorID:        uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Url:              "https://soundcloud.com/haru/dreamscape",
		Title:            "Dreamscape",
		Description:      sql.NullString{String: "몽환적인 신스팝", Valid: true},
		Field:            "음악",
		Tags:             []string{"신스팝", "몽환"},
		OgImage:          sql.NullString{},
		OgData:           pqtype.NullRawMessage{},
		CreatedAt:        time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CreatorIDRef:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CreatorNickname:  "하루",
		CreatorAvatarUrl: sql.NullString{},
	}
}

func TestList_CreatorIDFilter(t *testing.T) {
	creatorID := "00000000-0000-0000-0000-000000000001"
	mock := &mockQuerier{
		creatorRows:     []db.ListWorksByCreatorRow{sampleCreatorRow()},
		creatorCountVal: 1,
	}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works?creator_id="+creatorID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeResponse(t, rec)
	if len(resp.Works) != 1 {
		t.Fatalf("expected 1 work, got %d", len(resp.Works))
	}
	if mock.lastCreatorP.CreatorID.String() != creatorID {
		t.Errorf("expected creator_id %s, got %s", creatorID, mock.lastCreatorP.CreatorID)
	}
}

func TestList_InvalidCreatorID(t *testing.T) {
	mock := &mockQuerier{}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/works?creator_id=not-a-uuid")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestList_CreatorIDWithFieldFilter(t *testing.T) {
	mock := &mockQuerier{
		creatorRows: nil,
	}
	h := NewHandlerWithQuerier(mock)

	doRequest(t, h, "/api/works?creator_id=00000000-0000-0000-0000-000000000001&field=음악")

	if mock.lastCreatorP.Column2 != "음악" {
		t.Errorf("expected field '음악', got %q", mock.lastCreatorP.Column2)
	}
}
