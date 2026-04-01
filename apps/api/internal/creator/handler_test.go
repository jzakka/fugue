package creator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type mockQuerier struct {
	creator    db.Creator
	getErr     error
	updated    db.Creator
	updateErr  error
	workCount  int64
	countErr   error
	lastUpdate db.UpdateCreatorParams
}

func (m *mockQuerier) GetCreator(_ context.Context, _ uuid.UUID) (db.Creator, error) {
	return m.creator, m.getErr
}

func (m *mockQuerier) UpdateCreator(_ context.Context, arg db.UpdateCreatorParams) (db.Creator, error) {
	m.lastUpdate = arg
	return m.updated, m.updateErr
}

func (m *mockQuerier) CountWorksByCreator(_ context.Context, _ uuid.UUID) (int64, error) {
	return m.workCount, m.countErr
}

var testCreatorID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func sampleCreator() db.Creator {
	return db.Creator{
		ID:        testCreatorID,
		Nickname:  "하루",
		Bio:       sql.NullString{String: "음악하는 사람", Valid: true},
		Roles:     []string{"작곡", "보컬"},
		Contacts:  json.RawMessage(`{"twitter":"@haru"}`),
		AvatarUrl: sql.NullString{String: "https://example.com/avatar.jpg", Valid: true},
		Email:     sql.NullString{String: "haru@example.com", Valid: true},
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
}

func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func withCreatorID(r *http.Request, id uuid.UUID) *http.Request {
	ctx := auth.SetCreatorIDForTest(r.Context(), id)
	return r.WithContext(ctx)
}

// --- GetByID tests ---

func TestGetByID_Success(t *testing.T) {
	mock := &mockQuerier{creator: sampleCreator(), workCount: 5}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp CreatorPublicDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != testCreatorID.String() {
		t.Errorf("expected ID %s, got %s", testCreatorID, resp.ID)
	}
	if resp.Nickname != "하루" {
		t.Errorf("expected nickname '하루', got %s", resp.Nickname)
	}
	if resp.WorkCount != 5 {
		t.Errorf("expected work_count 5, got %d", resp.WorkCount)
	}
	// Public DTO should not have email
	raw, _ := json.Marshal(resp)
	if bytes.Contains(raw, []byte(`"email"`)) {
		t.Error("public DTO should not contain email field")
	}
}

func TestGetByID_InvalidUUID(t *testing.T) {
	mock := &mockQuerier{}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/not-a-uuid", nil)
	req = withChiParam(req, "id", "not-a-uuid")
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	mock := &mockQuerier{getErr: sql.ErrNoRows}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetByID_DBError(t *testing.T) {
	mock := &mockQuerier{getErr: errors.New("connection refused")}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// --- GetMe tests ---

func TestGetMe_Success(t *testing.T) {
	mock := &mockQuerier{creator: sampleCreator(), workCount: 3}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/me", nil)
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.GetMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp CreatorPrivateDTO
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Email == nil || *resp.Email != "haru@example.com" {
		t.Errorf("expected email 'haru@example.com', got %v", resp.Email)
	}
	if resp.WorkCount != 3 {
		t.Errorf("expected work_count 3, got %d", resp.WorkCount)
	}
}

func TestGetMe_Unauthorized(t *testing.T) {
	mock := &mockQuerier{}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/me", nil)
	rec := httptest.NewRecorder()

	h.GetMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// --- UpdateMe tests ---

func TestUpdateMe_Success(t *testing.T) {
	c := sampleCreator()
	updated := c
	updated.Nickname = "새이름"
	mock := &mockQuerier{creator: c, updated: updated, workCount: 2}
	h := NewHandlerWithQuerier(mock)

	body := `{"nickname":"새이름"}`
	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastUpdate.Nickname != "새이름" {
		t.Errorf("expected nickname '새이름', got %s", mock.lastUpdate.Nickname)
	}
}

func TestUpdateMe_EmptyNickname(t *testing.T) {
	c := sampleCreator()
	mock := &mockQuerier{creator: c}
	h := NewHandlerWithQuerier(mock)

	body := `{"nickname":"  "}`
	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateMe_NicknameTooLong(t *testing.T) {
	c := sampleCreator()
	mock := &mockQuerier{creator: c}
	h := NewHandlerWithQuerier(mock)

	longName := make([]byte, 201)
	for i := range longName {
		longName[i] = 'a'
	}
	body := `{"nickname":"` + string(longName) + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateMe_EmptyRoles(t *testing.T) {
	c := sampleCreator()
	mock := &mockQuerier{creator: c}
	h := NewHandlerWithQuerier(mock)

	body := `{"roles":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateMe_Unauthorized(t *testing.T) {
	mock := &mockQuerier{}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestUpdateMe_PartialUpdate(t *testing.T) {
	c := sampleCreator()
	mock := &mockQuerier{creator: c, updated: c, workCount: 0}
	h := NewHandlerWithQuerier(mock)

	// Only update bio, nickname should remain unchanged
	body := `{"bio":"새로운 자기소개"}`
	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// Nickname should remain the original
	if mock.lastUpdate.Nickname != "하루" {
		t.Errorf("expected unchanged nickname '하루', got %s", mock.lastUpdate.Nickname)
	}
	if !mock.lastUpdate.Bio.Valid || mock.lastUpdate.Bio.String != "새로운 자기소개" {
		t.Errorf("expected bio '새로운 자기소개', got %v", mock.lastUpdate.Bio)
	}
}
