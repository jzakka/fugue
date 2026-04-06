package pin

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type PinQuerier interface {
	ListPinsWithCreator(ctx context.Context, arg db.ListPinsWithCreatorParams) ([]db.ListPinsWithCreatorRow, error)
	ListPinsByCreator(ctx context.Context, arg db.ListPinsByCreatorParams) ([]db.ListPinsByCreatorRow, error)
	CountPins(ctx context.Context, arg db.CountPinsParams) (int64, error)
	CountPinsByCreatorFiltered(ctx context.Context, arg db.CountPinsByCreatorFilteredParams) (int64, error)
	CreatePin(ctx context.Context, arg db.CreatePinParams) (db.Pin, error)
	DeletePin(ctx context.Context, arg db.DeletePinParams) (int64, error)
	GetPinWithCreator(ctx context.Context, id uuid.UUID) (db.GetPinWithCreatorRow, error)
	GetPinURL(ctx context.Context, id uuid.UUID) (string, error)
	UpdatePinCountByURL(ctx context.Context, url string) error
	RelatedPins(ctx context.Context, arg db.RelatedPinsParams) ([]db.RelatedPinsRow, error)
}

type Handler struct {
	q PinQuerier
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{q: db.New(database)}
}

func NewHandlerWithQuerier(q PinQuerier) *Handler {
	return &Handler{q: q}
}

// Create handles POST /api/pins [auth]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreatePinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	req.Title = strings.TrimSpace(req.Title)
	req.Field = strings.TrimSpace(req.Field)

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "URL은 필수입니다")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "제목은 필수입니다")
		return
	}
	if req.Field == "" {
		writeError(w, http.StatusBadRequest, "분야는 필수입니다")
		return
	}
	if len(req.Tags) > 5 {
		writeError(w, http.StatusBadRequest, "태그는 최대 5개까지 가능합니다")
		return
	}
	for _, tag := range req.Tags {
		if utf8.RuneCountInString(tag) > 30 {
			writeError(w, http.StatusBadRequest, "태그는 30자를 초과할 수 없습니다")
			return
		}
	}

	var desc sql.NullString
	if req.Description != nil && *req.Description != "" {
		desc = sql.NullString{String: *req.Description, Valid: true}
	}

	var ogImage sql.NullString
	if req.OgImage != nil && *req.OgImage != "" {
		ogImage = sql.NullString{String: *req.OgImage, Valid: true}
	}

	var ogData pqtype.NullRawMessage
	if req.OgData != nil {
		ogData = pqtype.NullRawMessage{RawMessage: *req.OgData, Valid: true}
	}

	p, err := h.q.CreatePin(r.Context(), db.CreatePinParams{
		CreatorID:   creatorID,
		Url:         req.URL,
		Title:       req.Title,
		Description: desc,
		Field:       req.Field,
		Tags:        req.Tags,
		OgImage:     ogImage,
		OgData:      ogData,
	})
	if err != nil {
		log.Printf("pin.Create: insert error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀 등록에 실패했습니다")
		return
	}

	if err := h.q.UpdatePinCountByURL(r.Context(), p.Url); err != nil {
		log.Printf("pin.Create: pin count update error: %v", err)
	}

	writeJSON(w, http.StatusCreated, toCreatedResponse(p))
}

// GetByID handles GET /api/pins/{id}
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	row, err := h.q.GetPinWithCreator(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "핀을 찾을 수 없습니다")
			return
		}
		log.Printf("pin.GetByID: query error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "핀을 불러올 수 없습니다")
		return
	}

	writeJSON(w, http.StatusOK, toPinDetailResponse(row))
}

// Delete handles DELETE /api/pins/{id} [auth]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	pinURL, urlErr := h.q.GetPinURL(r.Context(), id)

	rowsAffected, err := h.q.DeletePin(r.Context(), db.DeletePinParams{
		ID:        id,
		CreatorID: creatorID,
	})
	if err != nil {
		log.Printf("pin.Delete: delete error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "핀 삭제에 실패했습니다")
		return
	}
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "핀을 찾을 수 없습니다")
		return
	}

	if urlErr == nil {
		if err := h.q.UpdatePinCountByURL(r.Context(), pinURL); err != nil {
			log.Printf("pin.Delete: pin count update error: %v", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// Related handles GET /api/pins/{id}/related
func (h *Handler) Related(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	row, err := h.q.GetPinWithCreator(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "핀을 찾을 수 없습니다")
			return
		}
		log.Printf("pin.Related: get pin error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "핀을 불러올 수 없습니다")
		return
	}

	if len(row.Tags) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"pins": []any{}})
		return
	}

	related, err := h.q.RelatedPins(r.Context(), db.RelatedPinsParams{
		ID:      id,
		Column2: row.Tags,
		Field:   row.Field,
	})
	if err != nil {
		log.Printf("pin.Related: query error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "연관 핀을 불러올 수 없습니다")
		return
	}

	pins := make([]PinResponse, 0, len(related))
	for _, r := range related {
		pins = append(pins, toRelatedPinResponse(r))
	}

	writeJSON(w, http.StatusOK, map[string]any{"pins": pins})
}

// List handles GET /api/pins?field=&tags=&limit=&offset=&creator_id=
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")

	var tags []string
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
	}

	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		if l > 0 && l <= 50 {
			limit = l
		} else if l > 50 {
			limit = 50
		}
	}

	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 && o <= 100000 {
		offset = o
	}

	if creatorIDStr := r.URL.Query().Get("creator_id"); creatorIDStr != "" {
		creatorID, err := uuid.Parse(creatorIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "유효하지 않은 크리에이터 ID입니다")
			return
		}
		h.listByCreator(w, r, creatorID, field, tags, limit, offset)
		return
	}

	rows, err := h.q.ListPinsWithCreator(r.Context(), db.ListPinsWithCreatorParams{
		Column1: field,
		Column2: tags,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		log.Printf("pin.List: query error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀 목록을 불러올 수 없습니다")
		return
	}

	count, err := h.q.CountPins(r.Context(), db.CountPinsParams{
		Column1: field,
		Column2: tags,
	})
	if err != nil {
		log.Printf("pin.List: count error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀 수를 확인할 수 없습니다")
		return
	}

	pins := make([]PinResponse, 0, len(rows))
	for _, row := range rows {
		pins = append(pins, toPinResponse(row))
	}

	hasMore := (int64(offset) + int64(len(rows))) < count

	writeJSON(w, http.StatusOK, ListPinsResponse{
		Pins:    pins,
		HasMore: hasMore,
	})
}

func (h *Handler) listByCreator(w http.ResponseWriter, r *http.Request, creatorID uuid.UUID, field string, tags []string, limit, offset int) {
	rows, err := h.q.ListPinsByCreator(r.Context(), db.ListPinsByCreatorParams{
		CreatorID: creatorID,
		Column2:   field,
		Column3:   tags,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		log.Printf("pin.listByCreator: query error: %v (creator=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "핀 목록을 불러올 수 없습니다")
		return
	}

	count, err := h.q.CountPinsByCreatorFiltered(r.Context(), db.CountPinsByCreatorFilteredParams{
		CreatorID: creatorID,
		Column2:   field,
		Column3:   tags,
	})
	if err != nil {
		log.Printf("pin.listByCreator: count error: %v (creator=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "핀 수를 확인할 수 없습니다")
		return
	}

	pins := make([]PinResponse, 0, len(rows))
	for _, row := range rows {
		pins = append(pins, toCreatorPinResponse(row))
	}

	hasMore := (int64(offset) + int64(len(rows))) < count

	writeJSON(w, http.StatusOK, ListPinsResponse{
		Pins:    pins,
		HasMore: hasMore,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("pin: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
