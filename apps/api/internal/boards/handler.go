package boards

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/interaction"
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

type CreatorSummary struct {
	ID        string  `json:"id"`
	Nickname  string  `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
}

type TagResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Category string `json:"category"`
}

type PinResponse struct {
	ID          string           `json:"id"`
	URL         *string          `json:"url"`
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	MediaURL    string           `json:"media_url"`
	MediaType   string           `json:"media_type"`
	OgImage     *string          `json:"og_image"`
	OgData      *json.RawMessage `json:"og_data"`
	Tags        []TagResponse    `json:"tags"`
	CreatedAt   time.Time        `json:"created_at"`
	Creator     CreatorSummary   `json:"creator"`
}

type BoardDetailResponse struct {
	Board   BoardResponse `json:"board"`
	Pins    []PinResponse `json:"pins"`
	HasMore bool          `json:"has_more"`
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

	// Parse pagination params for pins
	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		if l > 0 && l <= 50 {
			limit = l
		} else if l > 50 {
			limit = 50
		}
	}
	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 {
		offset = o
	}

	// Fetch pins belonging to this board
	pinRows, err := q.ListBoardPins(r.Context(), db.ListBoardPinsParams{
		BoardID: boardID,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		log.Printf("boards.GetByID: list pins error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드 핀 목록을 불러올 수 없습니다")
		return
	}

	pins := make([]PinResponse, 0, len(pinRows))
	for _, row := range pinRows {
		pins = append(pins, toBoardPinResponse(row))
	}

	// Batch load tags for all pins
	hydratePinTags(r.Context(), q, pins)

	hasMore := (int64(offset) + int64(len(pinRows))) < pinCount

	writeJSON(w, http.StatusOK, BoardDetailResponse{
		Board:   toBoardResponse(board, pinCount, images),
		Pins:    pins,
		HasMore: hasMore,
	})
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
		writeError(w, http.StatusForbidden, "보드를 수정할 권한이 없습니다")
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

	writeJSON(w, http.StatusOK, toBoardResponse(updated, pinCount, images))
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
		results = append(results, toBoardResponse(b, pinCount, images))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"boards": results,
	})
}

// ---------------------------------------------------------------------------
// ListByPin – GET /api/pins/{id}/boards
// ---------------------------------------------------------------------------

type PinBoardResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	CreatorID       string `json:"creator_id"`
	CreatorNickname string `json:"creator_nickname"`
}

func (h *Handler) ListByPin(w http.ResponseWriter, r *http.Request) {
	pinID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	q := db.New(h.database)
	rows, err := q.ListPublicBoardsByPin(r.Context(), pinID)
	if err != nil {
		log.Printf("boards.ListByPin: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "보드 목록을 불러올 수 없습니다")
		return
	}

	boards := make([]PinBoardResponse, 0, len(rows))
	for _, row := range rows {
		boards = append(boards, PinBoardResponse{
			ID:              row.ID.String(),
			Name:            row.Name,
			CreatorID:       row.CreatorID.String(),
			CreatorNickname: row.CreatorNickname,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"boards": boards,
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

	rowsAffected, err := q.AddPinToBoard(r.Context(), db.AddPinToBoardParams{
		BoardID: boardID,
		PinID:   workID,
	})
	if err != nil {
		log.Printf("boards.AddPin: DB error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀을 추가할 수 없습니다")
		return
	}

	if rowsAffected == 0 {
		writeError(w, http.StatusConflict, "이미 보드에 추가된 핀입니다")
		return
	}

	// spec: interaction `인증된 호출자의 핀 조회·핀 생성·보드 추가에 interaction row가 piggyback된다`
	interaction.Record(r.Context(), q, creatorID, workID, "board_add")

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
		PinID:   workID,
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

func toBoardPinResponse(row db.ListBoardPinsRow) PinResponse {
	var url *string
	if row.Url.Valid {
		url = &row.Url.String
	}
	var desc *string
	if row.Description.Valid {
		desc = &row.Description.String
	}
	var ogImage *string
	if row.OgImage.Valid {
		ogImage = &row.OgImage.String
	}
	var ogData *json.RawMessage
	if row.OgData.Valid {
		raw := json.RawMessage(row.OgData.RawMessage)
		ogData = &raw
	}
	var avatarURL *string
	if row.CreatorAvatarUrl.Valid {
		avatarURL = &row.CreatorAvatarUrl.String
	}
	return PinResponse{
		ID:          row.ID.String(),
		URL:         url,
		Title:       row.Title,
		Description: desc,
		MediaURL:    row.MediaUrl,
		MediaType:   row.MediaType,
		OgImage:     ogImage,
		OgData:      ogData,
		Tags:        []TagResponse{},
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func hydratePinTags(ctx context.Context, q *db.Queries, pins []PinResponse) {
	if len(pins) == 0 {
		return
	}
	pinIDs := make([]uuid.UUID, len(pins))
	for i, p := range pins {
		pinIDs[i] = uuid.MustParse(p.ID)
	}
	rows, err := q.GetTagsForPins(ctx, pinIDs)
	if err != nil {
		log.Printf("boards.hydratePinTags: error: %v", err)
		return
	}
	tagMap := make(map[string][]TagResponse)
	for _, r := range rows {
		pid := r.PinID.String()
		tagMap[pid] = append(tagMap[pid], TagResponse{
			ID:       r.ID.String(),
			Name:     r.Name,
			Slug:     r.Slug,
			Category: r.Category,
		})
	}
	for i := range pins {
		if tags, ok := tagMap[pins[i].ID]; ok {
			pins[i].Tags = tags
		}
	}
}

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
