package boards

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// DTO
// ---------------------------------------------------------------------------

type BoardResponse struct {
	ID          string    `json:"id"`
	CreatorID   string    `json:"creator_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	IsPublic    bool      `json:"is_public"`
	PinCount    int64     `json:"pin_count"`
	CoverImages []string  `json:"cover_images"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

type Handler struct {
	database *sql.DB
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{database: database}
}

// ---------------------------------------------------------------------------
// Create – POST /api/boards [auth]
// ---------------------------------------------------------------------------

type createBoardRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsPublic    *bool   `json:"is_public"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "인증이 필요합니다")
		return
	}

	var req createBoardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	name := req.Name
	if name == "" {
		writeError(w, http.StatusBadRequest, "보드 이름은 필수입니다")
		return
	}
	if utf8.RuneCountInString(name) > 100 {
		writeError(w, http.StatusBadRequest, "보드 이름은 100자 이내여야 합니다")
		return
	}

	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	var description sql.NullString
	if req.Description != nil {
		description = sql.NullString{String: *req.Description, Valid: true}
	}

	q := db.New(h.database)
	board, err := q.CreateBoard(r.Context(), db.CreateBoardParams{
		CreatorID:   creatorID,
		Name:        name,
		Description: description,
		IsPublic:    isPublic,
	})
	if err != nil {
		log.Printf("boards.Create: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드를 생성할 수 없습니다")
		return
	}

	writeJSON(w, http.StatusCreated, toBoardResponse(board, 0, nil))
}

// ---------------------------------------------------------------------------
// GetByID – GET /api/boards/{id}
// ---------------------------------------------------------------------------

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	boardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 보드 ID입니다")
		return
	}

	q := db.New(h.database)
	board, err := q.GetBoard(r.Context(), boardID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}
	if err != nil {
		log.Printf("boards.GetByID: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드를 불러올 수 없습니다")
		return
	}

	// Private board: only owner can view
	if !board.IsPublic {
		callerID, ok := auth.CreatorIDFromContext(r.Context())
		if !ok || callerID != board.CreatorID {
			writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
			return
		}
	}

	pinCount, err := q.CountBoardPins(r.Context(), boardID)
	if err != nil {
		log.Printf("boards.GetByID: count pins error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드 정보를 불러올 수 없습니다")
		return
	}

	images, err := q.ListBoardPinImages(r.Context(), boardID)
	if err != nil {
		log.Printf("boards.GetByID: list images error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드 정보를 불러올 수 없습니다")
		return
	}

	coverImages := toStringSlice(images)

	writeJSON(w, http.StatusOK, toBoardResponse(board, pinCount, coverImages))
}

// ---------------------------------------------------------------------------
// Update – PUT /api/boards/{id} [auth]
// ---------------------------------------------------------------------------

type updateBoardRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsPublic    *bool   `json:"is_public"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "인증이 필요합니다")
		return
	}

	boardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 보드 ID입니다")
		return
	}

	q := db.New(h.database)

	// Fetch current board to merge partial updates
	current, err := q.GetBoard(r.Context(), boardID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}
	if err != nil {
		log.Printf("boards.Update: get error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드를 불러올 수 없습니다")
		return
	}

	if current.CreatorID != creatorID {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}

	var req updateBoardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	// Merge: use current values as defaults
	name := current.Name
	if req.Name != nil {
		name = *req.Name
	}
	if name == "" {
		writeError(w, http.StatusBadRequest, "보드 이름은 필수입니다")
		return
	}
	if utf8.RuneCountInString(name) > 100 {
		writeError(w, http.StatusBadRequest, "보드 이름은 100자 이내여야 합니다")
		return
	}

	description := current.Description
	if req.Description != nil {
		description = sql.NullString{String: *req.Description, Valid: true}
	}

	isPublic := current.IsPublic
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	updated, err := q.UpdateBoard(r.Context(), db.UpdateBoardParams{
		ID:          boardID,
		CreatorID:   creatorID,
		Name:        name,
		Description: description,
		IsPublic:    isPublic,
	})
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}
	if err != nil {
		log.Printf("boards.Update: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드를 수정할 수 없습니다")
		return
	}

	pinCount, _ := q.CountBoardPins(r.Context(), boardID)
	images, _ := q.ListBoardPinImages(r.Context(), boardID)
	coverImages := toStringSlice(images)

	writeJSON(w, http.StatusOK, toBoardResponse(updated, pinCount, coverImages))
}

// ---------------------------------------------------------------------------
// Delete – DELETE /api/boards/{id} [auth]
// ---------------------------------------------------------------------------

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "인증이 필요합니다")
		return
	}

	boardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 보드 ID입니다")
		return
	}

	q := db.New(h.database)
	rowsAffected, err := q.DeleteBoard(r.Context(), db.DeleteBoardParams{
		ID:        boardID,
		CreatorID: creatorID,
	})
	if err != nil {
		log.Printf("boards.Delete: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드를 삭제할 수 없습니다")
		return
	}

	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// ListByCreator – GET /api/boards?creator_id=uuid
// ---------------------------------------------------------------------------

func (h *Handler) ListByCreator(w http.ResponseWriter, r *http.Request) {
	creatorIDStr := r.URL.Query().Get("creator_id")
	if creatorIDStr == "" {
		writeError(w, http.StatusBadRequest, "creator_id 파라미터가 필요합니다")
		return
	}

	creatorID, err := uuid.Parse(creatorIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 크리에이터 ID입니다")
		return
	}

	q := db.New(h.database)

	// If the authenticated user is the owner, show all boards; otherwise public only
	callerID, authenticated := auth.CreatorIDFromContext(r.Context())
	isOwner := authenticated && callerID == creatorID

	var boards []db.Board
	if isOwner {
		boards, err = q.ListBoardsByCreator(r.Context(), creatorID)
	} else {
		boards, err = q.ListPublicBoardsByCreator(r.Context(), creatorID)
	}
	if err != nil {
		log.Printf("boards.ListByCreator: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드 목록을 불러올 수 없습니다")
		return
	}

	results := make([]BoardResponse, 0, len(boards))
	for _, b := range boards {
		pinCount, _ := q.CountBoardPins(r.Context(), b.ID)
		images, _ := q.ListBoardPinImages(r.Context(), b.ID)
		coverImages := toStringSlice(images)
		results = append(results, toBoardResponse(b, pinCount, coverImages))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"boards": results,
	})
}

// ---------------------------------------------------------------------------
// AddPin – POST /api/boards/{id}/pins [auth]
// ---------------------------------------------------------------------------

type addPinRequest struct {
	PinID string `json:"pin_id"`
}

func (h *Handler) AddPin(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "인증이 필요합니다")
		return
	}

	boardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 보드 ID입니다")
		return
	}

	var req addPinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	workID, err := uuid.Parse(req.PinID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 작품 ID입니다")
		return
	}

	q := db.New(h.database)

	// Verify board ownership
	board, err := q.GetBoard(r.Context(), boardID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}
	if err != nil {
		log.Printf("boards.AddPin: get board error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드를 불러올 수 없습니다")
		return
	}
	if board.CreatorID != creatorID {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}

	// ON CONFLICT DO NOTHING makes this idempotent
	if err := q.AddPinToBoard(r.Context(), db.AddPinToBoardParams{
		BoardID: boardID,
		PinID:  workID,
	}); err != nil {
		log.Printf("boards.AddPin: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀을 추가할 수 없습니다")
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// ---------------------------------------------------------------------------
// RemovePin – DELETE /api/boards/{id}/pins/{pin_id} [auth]
// ---------------------------------------------------------------------------

func (h *Handler) RemovePin(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "인증이 필요합니다")
		return
	}

	boardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 보드 ID입니다")
		return
	}

	workID, err := uuid.Parse(chi.URLParam(r, "pin_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 작품 ID입니다")
		return
	}

	q := db.New(h.database)

	// Verify board ownership
	board, err := q.GetBoard(r.Context(), boardID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}
	if err != nil {
		log.Printf("boards.RemovePin: get board error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드를 불러올 수 없습니다")
		return
	}
	if board.CreatorID != creatorID {
		writeError(w, http.StatusNotFound, "보드를 찾을 수 없습니다")
		return
	}

	rowsAffected, err := q.RemovePinFromBoard(r.Context(), db.RemovePinFromBoardParams{
		BoardID: boardID,
		PinID:  workID,
	})
	if err != nil {
		log.Printf("boards.RemovePin: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀을 제거할 수 없습니다")
		return
	}

	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "핀을 찾을 수 없습니다")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func toBoardResponse(b db.Board, pinCount int64, coverImages []string) BoardResponse {
	var desc *string
	if b.Description.Valid {
		desc = &b.Description.String
	}

	if coverImages == nil {
		coverImages = []string{}
	}

	return BoardResponse{
		ID:          b.ID.String(),
		CreatorID:   b.CreatorID.String(),
		Name:        b.Name,
		Description: desc,
		IsPublic:    b.IsPublic,
		PinCount:    pinCount,
		CoverImages: coverImages,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}

func toStringSlice(nullStrings []sql.NullString) []string {
	result := make([]string, 0, len(nullStrings))
	for _, ns := range nullStrings {
		if ns.Valid {
			result = append(result, ns.String)
		}
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("boards: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
