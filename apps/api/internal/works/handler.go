package works

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// WorksQuerier abstracts the DB queries the handler needs.
// Satisfied by *db.Queries in production, mocked in tests.
type WorksQuerier interface {
	ListWorksWithCreator(ctx context.Context, arg db.ListWorksWithCreatorParams) ([]db.ListWorksWithCreatorRow, error)
	ListWorksByCreator(ctx context.Context, arg db.ListWorksByCreatorParams) ([]db.ListWorksByCreatorRow, error)
	CountWorks(ctx context.Context, arg db.CountWorksParams) (int64, error)
	CountWorksByCreatorFiltered(ctx context.Context, arg db.CountWorksByCreatorFilteredParams) (int64, error)
}

type Handler struct {
	q WorksQuerier
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{q: db.New(database)}
}

// NewHandlerWithQuerier creates a handler with a custom querier (for testing).
func NewHandlerWithQuerier(q WorksQuerier) *Handler {
	return &Handler{q: q}
}

// List handles GET /api/works?field=&tags=&limit=&offset=&creator_id=
//
// Data flow:
//
//	query params → validate/clamp → ListWorksWithCreator (JOIN) → CountWorks → JSON
//	If creator_id is present → ListWorksByCreator instead.
//
// Error paths:
//
//	DB error → 500 + structured log
//	Empty result → 200 { works: [], has_more: false }
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

	// Branch: creator_id filter uses dedicated query
	if creatorIDStr := r.URL.Query().Get("creator_id"); creatorIDStr != "" {
		creatorID, err := uuid.Parse(creatorIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "유효하지 않은 크리에이터 ID입니다")
			return
		}
		h.listByCreator(w, r, creatorID, field, tags, limit, offset)
		return
	}

	rows, err := h.q.ListWorksWithCreator(r.Context(), db.ListWorksWithCreatorParams{
		Column1: field,
		Column2: tags,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		log.Printf("works.List: query error: %v (field=%q tags=%v limit=%d offset=%d)", err, field, tags, limit, offset)
		writeError(w, http.StatusInternalServerError, "작품 목록을 불러올 수 없습니다")
		return
	}

	count, err := h.q.CountWorks(r.Context(), db.CountWorksParams{
		Column1: field,
		Column2: tags,
	})
	if err != nil {
		log.Printf("works.List: count error: %v (field=%q tags=%v)", err, field, tags)
		writeError(w, http.StatusInternalServerError, "작품 수를 확인할 수 없습니다")
		return
	}

	works := make([]WorkResponse, 0, len(rows))
	for _, row := range rows {
		works = append(works, toWorkResponse(row))
	}

	hasMore := (int64(offset) + int64(len(rows))) < count

	writeJSON(w, http.StatusOK, ListWorksResponse{
		Works:   works,
		HasMore: hasMore,
	})
}

func (h *Handler) listByCreator(w http.ResponseWriter, r *http.Request, creatorID uuid.UUID, field string, tags []string, limit, offset int) {
	rows, err := h.q.ListWorksByCreator(r.Context(), db.ListWorksByCreatorParams{
		CreatorID: creatorID,
		Column2:   field,
		Column3:   tags,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		log.Printf("works.listByCreator: query error: %v (creator=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "작품 목록을 불러올 수 없습니다")
		return
	}

	count, err := h.q.CountWorksByCreatorFiltered(r.Context(), db.CountWorksByCreatorFilteredParams{
		CreatorID: creatorID,
		Column2:   field,
		Column3:   tags,
	})
	if err != nil {
		log.Printf("works.listByCreator: count error: %v (creator=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "작품 수를 확인할 수 없습니다")
		return
	}

	works := make([]WorkResponse, 0, len(rows))
	for _, row := range rows {
		works = append(works, toCreatorWorkResponse(row))
	}

	hasMore := (int64(offset) + int64(len(rows))) < count

	writeJSON(w, http.StatusOK, ListWorksResponse{
		Works:   works,
		HasMore: hasMore,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("works: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
