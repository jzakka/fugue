package creator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

	publicBoards     []db.Board
	publicBoardsErr  error
	creatorPins      []db.ListPinsByCreatorRow
	creatorPinsErr   error
	lastBoardsParams db.ListPublicBoardsByCreatorLimitedParams
	lastPinsParams   db.ListPinsByCreatorParams
	boardsCalls      int
	pinsCalls        int
}

func (m *mockQuerier) GetCreator(_ context.Context, _ uuid.UUID) (db.Creator, error) {
	return m.creator, m.getErr
}

func (m *mockQuerier) UpdateCreator(_ context.Context, arg db.UpdateCreatorParams) (db.Creator, error) {
	m.lastUpdate = arg
	return m.updated, m.updateErr
}

func (m *mockQuerier) CountPinsByCreator(_ context.Context, _ uuid.UUID) (int64, error) {
	return m.workCount, m.countErr
}

func (m *mockQuerier) ListPublicBoardsByCreatorLimited(_ context.Context, arg db.ListPublicBoardsByCreatorLimitedParams) ([]db.Board, error) {
	m.lastBoardsParams = arg
	m.boardsCalls++
	return m.publicBoards, m.publicBoardsErr
}

func (m *mockQuerier) ListPinsByCreator(_ context.Context, arg db.ListPinsByCreatorParams) ([]db.ListPinsByCreatorRow, error) {
	m.lastPinsParams = arg
	m.pinsCalls++
	return m.creatorPins, m.creatorPinsErr
}

var testCreatorID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func sampleCreator() db.Creator {
	return db.Creator{
		ID:        testCreatorID,
		Nickname:  "하루",
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
	if resp.Nickname != "하루" {
		t.Errorf("expected nickname '하루', got %s", resp.Nickname)
	}
	if resp.PinCount != 5 {
		t.Errorf("expected work_count 5, got %d", resp.PinCount)
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

// The following tests pin the ADDED Requirement "avatar_url 입력은
// creators.avatar_url 컬럼 cap에 맞춰 사전 길이 검증된다" of the change
// `fix-creator-update-avatar-url-input-length-validation`. They assert
// that PUT /api/creators/me rejects avatar_url inputs exceeding the
// VARCHAR(500) rune cap with 400 (UpdateCreator never called), accepts
// boundary input, preserves the empty-string clear semantics, and
// preserves the omitted-field merge semantics.

func decodeUpdateMeError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v (raw=%q)", err, rec.Body.String())
	}
	return body["error"]
}

func TestUpdateMe_RejectsAvatarURLOverRuneCap(t *testing.T) {
	c := sampleCreator()
	mock := &mockQuerier{creator: c}
	h := NewHandlerWithQuerier(mock)

	over := strings.Repeat("A", 501)
	body, err := json.Marshal(map[string]any{"avatar_url": over})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if msg := decodeUpdateMeError(t, rec); msg != "아바타 URL은 500자 이내여야 합니다" {
		t.Fatalf("error message: got %q, want %q", msg, "아바타 URL은 500자 이내여야 합니다")
	}
	if (mock.lastUpdate != db.UpdateCreatorParams{}) {
		t.Fatalf("UpdateCreator must not be called on reject; lastUpdate=%+v", mock.lastUpdate)
	}
}

func TestUpdateMe_RejectsAvatarURLOverRuneCapMultibyte(t *testing.T) {
	// 한국어 501 rune ≈ 1503 byte. byte-count로 cap을 비교하면 정상 입력
	// (가*167 = 501 byte)을 잘못 거부하거나 본 케이스를 통과시킨다.
	// rune-count만이 PostgreSQL VARCHAR(500) 규칙과 일치.
	c := sampleCreator()
	mock := &mockQuerier{creator: c}
	h := NewHandlerWithQuerier(mock)

	over := strings.Repeat("가", 501)
	body, err := json.Marshal(map[string]any{"avatar_url": over})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if msg := decodeUpdateMeError(t, rec); msg != "아바타 URL은 500자 이내여야 합니다" {
		t.Fatalf("error message: got %q, want %q", msg, "아바타 URL은 500자 이내여야 합니다")
	}
	if (mock.lastUpdate != db.UpdateCreatorParams{}) {
		t.Fatalf("UpdateCreator must not be called on reject; lastUpdate=%+v", mock.lastUpdate)
	}
}

func TestUpdateMe_AcceptsAvatarURLAtRuneCap(t *testing.T) {
	c := sampleCreator()
	atCap := strings.Repeat("A", 500)
	updated := c
	updated.AvatarUrl = sql.NullString{String: atCap, Valid: true}
	mock := &mockQuerier{creator: c, updated: updated, workCount: 1}
	h := NewHandlerWithQuerier(mock)

	body, err := json.Marshal(map[string]any{"avatar_url": atCap})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !mock.lastUpdate.AvatarUrl.Valid {
		t.Fatalf("AvatarUrl.Valid: got false, want true")
	}
	if got := utf8.RuneCountInString(mock.lastUpdate.AvatarUrl.String); got != 500 {
		t.Fatalf("AvatarUrl rune length: got %d, want 500", got)
	}
}

func TestUpdateMe_AcceptsAvatarURLEmptyAsClear(t *testing.T) {
	c := sampleCreator()
	updated := c
	updated.AvatarUrl = sql.NullString{}
	mock := &mockQuerier{creator: c, updated: updated, workCount: 1}
	h := NewHandlerWithQuerier(mock)

	body := `{"avatar_url":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if mock.lastUpdate.AvatarUrl.Valid {
		t.Fatalf("empty-string avatar_url must be cleared (Valid=false); got %+v", mock.lastUpdate.AvatarUrl)
	}
}

func TestUpdateMe_AcceptsAvatarURLOmitted(t *testing.T) {
	c := sampleCreator()
	mock := &mockQuerier{creator: c, updated: c, workCount: 1}
	h := NewHandlerWithQuerier(mock)

	body := `{"nickname":"새이름"}`
	req := httptest.NewRequest(http.MethodPut, "/api/creators/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCreatorID(req, testCreatorID)
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if mock.lastUpdate.AvatarUrl != c.AvatarUrl {
		t.Fatalf("omitted avatar_url must preserve current value; got %+v want %+v",
			mock.lastUpdate.AvatarUrl, c.AvatarUrl)
	}
}

// The following tests pin the public-profile payload invariants enforced by:
//   profile `공개 프로필 조회 응답에 보드 요약과 핀 요약을 포함한다`
//
// They assert that GET /api/creators/{id} responses always include the boards
// and pins arrays, normalize empty results to [] rather than null, apply the
// system-side upper bounds via the SQL LIMIT parameters, and surface query
// errors as 500 while skipping the two additional fetches when the creator
// itself is not found.

func sampleBoard(id uuid.UUID, name string, isPublic bool) db.Board {
	return db.Board{
		ID:        id,
		CreatorID: testCreatorID,
		Name:      name,
		IsPublic:  isPublic,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
}

func samplePinRow(id uuid.UUID, title string) db.ListPinsByCreatorRow {
	return db.ListPinsByCreatorRow{
		ID:               id,
		CreatorID:        testCreatorID,
		MediaUrl:         "https://example.com/" + title + ".jpg",
		MediaType:        "image",
		Title:            title,
		CreatedAt:        time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		CreatorIDRef:     testCreatorID,
		CreatorNickname:  "하루",
		CreatorAvatarUrl: sql.NullString{String: "https://example.com/avatar.jpg", Valid: true},
	}
}

func TestGetByID_ReturnsBoardsAndPins(t *testing.T) {
	boardIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	pinIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	mock := &mockQuerier{
		creator:   sampleCreator(),
		workCount: 5,
		publicBoards: []db.Board{
			sampleBoard(boardIDs[0], "노을 모음", true),
			sampleBoard(boardIDs[1], "스튜디오 사진", true),
			sampleBoard(boardIDs[2], "음악 큐레이션", true),
		},
		creatorPins: []db.ListPinsByCreatorRow{
			samplePinRow(pinIDs[0], "p1"),
			samplePinRow(pinIDs[1], "p2"),
			samplePinRow(pinIDs[2], "p3"),
			samplePinRow(pinIDs[3], "p4"),
			samplePinRow(pinIDs[4], "p5"),
		},
	}
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
	if len(resp.Boards) != 3 {
		t.Fatalf("expected 3 boards in response, got %d", len(resp.Boards))
	}
	if len(resp.Pins) != 5 {
		t.Fatalf("expected 5 pins in response, got %d", len(resp.Pins))
	}
	if resp.Boards[0].Name != "노을 모음" {
		t.Errorf("unexpected first board name: %s", resp.Boards[0].Name)
	}
	if resp.Pins[0].Title != "p1" {
		t.Errorf("unexpected first pin title: %s", resp.Pins[0].Title)
	}
}

func TestGetByID_EmptyBoardsAndPinsSerializeAsArrays(t *testing.T) {
	mock := &mockQuerier{creator: sampleCreator(), workCount: 0}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// Raw JSON check: spec requires the keys to serialize as [], not null.
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"boards":[]`)) {
		t.Errorf("expected boards key to serialize as [], body=%s", body)
	}
	if !bytes.Contains([]byte(body), []byte(`"pins":[]`)) {
		t.Errorf("expected pins key to serialize as [], body=%s", body)
	}
}

func TestGetByID_AppliesUpperBoundsToBoardsAndPinsQueries(t *testing.T) {
	mock := &mockQuerier{creator: sampleCreator(), workCount: 0}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastBoardsParams.Limit != maxPublicProfileBoards {
		t.Errorf("expected boards LIMIT %d, got %d", maxPublicProfileBoards, mock.lastBoardsParams.Limit)
	}
	if mock.lastBoardsParams.CreatorID != testCreatorID {
		t.Errorf("expected boards CreatorID %s, got %s", testCreatorID, mock.lastBoardsParams.CreatorID)
	}
	if mock.lastPinsParams.Limit != maxPublicProfileRecentPins {
		t.Errorf("expected pins LIMIT %d, got %d", maxPublicProfileRecentPins, mock.lastPinsParams.Limit)
	}
	if mock.lastPinsParams.Offset != 0 {
		t.Errorf("expected pins OFFSET 0, got %d", mock.lastPinsParams.Offset)
	}
}

func TestGetByID_BoardsQueryErrorReturns500(t *testing.T) {
	mock := &mockQuerier{
		creator:         sampleCreator(),
		workCount:       0,
		publicBoardsErr: errors.New("boards query failed"),
	}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetByID_PinsQueryErrorReturns500(t *testing.T) {
	mock := &mockQuerier{
		creator:        sampleCreator(),
		workCount:      0,
		creatorPinsErr: errors.New("pins query failed"),
	}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetByID_NotFoundSkipsBoardsAndPinsFetch(t *testing.T) {
	mock := &mockQuerier{getErr: sql.ErrNoRows}
	h := NewHandlerWithQuerier(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/creators/"+testCreatorID.String(), nil)
	req = withChiParam(req, "id", testCreatorID.String())
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if mock.boardsCalls != 0 {
		t.Errorf("expected boards fetch to be skipped on 404, got %d calls", mock.boardsCalls)
	}
	if mock.pinsCalls != 0 {
		t.Errorf("expected pins fetch to be skipped on 404, got %d calls", mock.pinsCalls)
	}
}
