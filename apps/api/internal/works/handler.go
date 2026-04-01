package works

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// WorksQuerier abstracts the DB queries the handler needs.
// Satisfied by *db.Queries in production, mocked in tests.
type WorksQuerier interface {
	ListWorksWithCreator(ctx context.Context, arg db.ListWorksWithCreatorParams) ([]db.ListWorksWithCreatorRow, error)
	CountWorks(ctx context.Context, arg db.CountWorksParams) (int64, error)
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

// List handles GET /api/works?field=&tags=&limit=&offset=
//
// Data flow:
//
//	query params → validate/clamp → ListWorksWithCreator (JOIN) → CountWorks → JSON
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
