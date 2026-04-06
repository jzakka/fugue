package pin

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

func (m *mockQuerier) DeletePin(_ context.Context, _ db.DeletePinParams) (int64, error) {
	return 0, nil
}

func (m *mockQuerier) GetPinWithCreator(_ context.Context, _ uuid.UUID) (db.GetPinWithCreatorRow, error) {
	return db.GetPinWithCreatorRow{}, sql.ErrNoRows
}

func (m *mockQuerier) GetPinURL(_ context.Context, _ uuid.UUID) (string, error) {
	return "", sql.ErrNoRows
}

func (m *mockQuerier) UpdatePinCountByURL(_ context.Context, _ string) error {
	return nil
}

func (m *mockQuerier) RelatedPins(_ context.Context, _ db.RelatedPinsParams) ([]db.RelatedPinsRow, error) {
	return nil, nil
}

func sampleRow() db.ListPinsWithCreatorRow {
	return db.ListPinsWithCreatorRow{
		ID:               uuid.MustParse("20000000-0000-0000-0000-000000000001"),
		CreatorID:        uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Url:              "https://soundcloud.com/haru/dreamscape",
		Title:            "Dreamscape",
		Description:      sql.NullString{String: "몽환적인 신스팝", Valid: true},
		Field:            "음악",
		Tags:             []string{"신스팝", "몽환"},
		OgImage:          sql.NullString{},
		OgData:           pqtype.NullRawMessage{},
		PinCount:         1,
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

func TestList_FieldFilter(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	doRequest(t, h, "/api/pins?field=음악")

	if mock.lastListP.Column1 != "음악" {
		t.Errorf("expected field '음악', got %q", mock.lastListP.Column1)
	}
}

func TestList_EmptyResult(t *testing.T) {
	mock := &mockQuerier{listRows: nil, countVal: 0}
	h := NewHandlerWithQuerier(mock)

	rec := doRequest(t, h, "/api/pins?field=nonexistent")
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
	if p.PinCount != 1 {
		t.Errorf("expected pin_count 1, got %d", p.PinCount)
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
