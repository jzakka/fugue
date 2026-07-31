package boards

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// mockQuerier implements boardsQuerier with overridable funcs. Methods the
// AddPin flow never touches return sql.ErrConnDone so an unexpected call
// surfaces as a test failure instead of silently succeeding.
type mockQuerier struct {
	getBoard          func(ctx context.Context, id uuid.UUID) (db.Board, error)
	getPin            func(ctx context.Context, id uuid.UUID) (db.Pin, error)
	addPinToBoard     func(ctx context.Context, arg db.AddPinToBoardParams) (int64, error)
	createInteraction func(ctx context.Context, arg db.CreateInteractionParams) error
}

func (m *mockQuerier) GetBoard(ctx context.Context, id uuid.UUID) (db.Board, error) {
	return m.getBoard(ctx, id)
}

func (m *mockQuerier) GetPin(ctx context.Context, id uuid.UUID) (db.Pin, error) {
	return m.getPin(ctx, id)
}

func (m *mockQuerier) AddPinToBoard(ctx context.Context, arg db.AddPinToBoardParams) (int64, error) {
	return m.addPinToBoard(ctx, arg)
}

func (m *mockQuerier) CreateInteraction(ctx context.Context, arg db.CreateInteractionParams) error {
	if m.createInteraction != nil {
		return m.createInteraction(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) CreateBoard(ctx context.Context, arg db.CreateBoardParams) (db.Board, error) {
	return db.Board{}, sql.ErrConnDone
}

func (m *mockQuerier) UpdateBoard(ctx context.Context, arg db.UpdateBoardParams) (db.Board, error) {
	return db.Board{}, sql.ErrConnDone
}

func (m *mockQuerier) DeleteBoard(ctx context.Context, arg db.DeleteBoardParams) (int64, error) {
	return 0, sql.ErrConnDone
}

func (m *mockQuerier) ListBoardsByCreator(ctx context.Context, creatorID uuid.UUID) ([]db.Board, error) {
	return nil, sql.ErrConnDone
}

func (m *mockQuerier) ListPublicBoardsByCreator(ctx context.Context, creatorID uuid.UUID) ([]db.Board, error) {
	return nil, sql.ErrConnDone
}

func (m *mockQuerier) ListPublicBoardsByPin(ctx context.Context, pinID uuid.UUID) ([]db.ListPublicBoardsByPinRow, error) {
	return nil, sql.ErrConnDone
}

func (m *mockQuerier) CountBoardPins(ctx context.Context, boardID uuid.UUID) (int64, error) {
	return 0, sql.ErrConnDone
}

func (m *mockQuerier) ListBoardPinImages(ctx context.Context, boardID uuid.UUID) ([]string, error) {
	return nil, sql.ErrConnDone
}

func (m *mockQuerier) ListBoardPins(ctx context.Context, arg db.ListBoardPinsParams) ([]db.ListBoardPinsRow, error) {
	return nil, sql.ErrConnDone
}

func (m *mockQuerier) GetTagsForPins(ctx context.Context, pinIDs []uuid.UUID) ([]db.GetTagsForPinsRow, error) {
	return nil, sql.ErrConnDone
}

func (m *mockQuerier) RemovePinFromBoard(ctx context.Context, arg db.RemovePinFromBoardParams) (int64, error) {
	return 0, sql.ErrConnDone
}

var _ boardsQuerier = (*mockQuerier)(nil)

func doAddPin(t *testing.T, q boardsQuerier, callerID uuid.UUID, boardID uuid.UUID, pinID string) *httptest.ResponseRecorder {
	t.Helper()

	h := &Handler{q: q}

	body, err := json.Marshal(map[string]string{"pin_id": pinID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/boards/"+boardID.String()+"/pins", bytes.NewReader(body))
	req = req.WithContext(auth.WithCreatorID(req.Context(), callerID))

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", boardID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	rec := httptest.NewRecorder()
	h.AddPin(rec, req)
	return rec
}

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error body: %v (body=%q)", err, rec.Body.String())
	}
	return resp["error"]
}

func TestAddPin_NonexistentPinReturns404(t *testing.T) {
	owner := uuid.New()
	boardID := uuid.New()

	q := &mockQuerier{
		getBoard: func(ctx context.Context, id uuid.UUID) (db.Board, error) {
			return db.Board{ID: boardID, CreatorID: owner}, nil
		},
		getPin: func(ctx context.Context, id uuid.UUID) (db.Pin, error) {
			return db.Pin{}, sql.ErrNoRows
		},
		addPinToBoard: func(ctx context.Context, arg db.AddPinToBoardParams) (int64, error) {
			t.Fatal("AddPinToBoard must not be called when the pin does not exist")
			return 0, nil
		},
	}

	rec := doAddPin(t, q, owner, boardID, uuid.NewString())

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if msg := errorMessage(t, rec); msg != "핀을 찾을 수 없습니다" {
		t.Fatalf("expected error %q, got %q", "핀을 찾을 수 없습니다", msg)
	}
}

func TestAddPin_ExistingPinReturns201(t *testing.T) {
	owner := uuid.New()
	boardID := uuid.New()
	pinID := uuid.New()

	q := &mockQuerier{
		getBoard: func(ctx context.Context, id uuid.UUID) (db.Board, error) {
			return db.Board{ID: boardID, CreatorID: owner}, nil
		},
		getPin: func(ctx context.Context, id uuid.UUID) (db.Pin, error) {
			return db.Pin{ID: pinID}, nil
		},
		addPinToBoard: func(ctx context.Context, arg db.AddPinToBoardParams) (int64, error) {
			if arg.BoardID != boardID || arg.PinID != pinID {
				t.Fatalf("unexpected AddPinToBoard args: %+v", arg)
			}
			return 1, nil
		},
	}

	rec := doAddPin(t, q, owner, boardID, pinID.String())

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body=%q)", rec.Code, rec.Body.String())
	}
}

func TestAddPin_DuplicatePinReturns409(t *testing.T) {
	owner := uuid.New()
	boardID := uuid.New()
	pinID := uuid.New()

	q := &mockQuerier{
		getBoard: func(ctx context.Context, id uuid.UUID) (db.Board, error) {
			return db.Board{ID: boardID, CreatorID: owner}, nil
		},
		getPin: func(ctx context.Context, id uuid.UUID) (db.Pin, error) {
			return db.Pin{ID: pinID}, nil
		},
		addPinToBoard: func(ctx context.Context, arg db.AddPinToBoardParams) (int64, error) {
			return 0, nil
		},
	}

	rec := doAddPin(t, q, owner, boardID, pinID.String())

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if msg := errorMessage(t, rec); msg != "이미 보드에 추가된 핀입니다" {
		t.Fatalf("expected error %q, got %q", "이미 보드에 추가된 핀입니다", msg)
	}
}

func TestAddPin_OtherOwnersBoardReturns404(t *testing.T) {
	owner := uuid.New()
	caller := uuid.New()
	boardID := uuid.New()

	q := &mockQuerier{
		getBoard: func(ctx context.Context, id uuid.UUID) (db.Board, error) {
			return db.Board{ID: boardID, CreatorID: owner}, nil
		},
		getPin: func(ctx context.Context, id uuid.UUID) (db.Pin, error) {
			t.Fatal("GetPin must not be called when the caller does not own the board")
			return db.Pin{}, nil
		},
		addPinToBoard: func(ctx context.Context, arg db.AddPinToBoardParams) (int64, error) {
			t.Fatal("AddPinToBoard must not be called when the caller does not own the board")
			return 0, nil
		},
	}

	rec := doAddPin(t, q, caller, boardID, uuid.NewString())

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if msg := errorMessage(t, rec); msg != "보드를 찾을 수 없습니다" {
		t.Fatalf("expected error %q, got %q", "보드를 찾을 수 없습니다", msg)
	}
}
