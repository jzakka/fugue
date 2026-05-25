package tag

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"unicode/utf8"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// maxListQueryRunes mirrors the sister handler search.Search
// (`maxSearchQueryRunes = 200`, internal/search/handler.go) which capped
// its `q` parameter at the upper length of `pins.title` (VARCHAR(200),
// migration 000004_pivot_pins.up.sql) to keep unauthenticated,
// un-rate-limited search endpoints from amplifying pattern-based SQL
// matching. The same surface exists here: /api/tags is unauthenticated
// and not rate-limited (cmd/server/main.go:138-139) and feeds the raw
// query string into SearchTags (db/queries/tags.sql:15)
// `WHERE name ILIKE '%' || $1 || '%' LIMIT 50`. LIMIT 50 only caps the
// result set; per-row LIKE matching cost is amplified by pattern length
// (postgres cannot use the GIN trigram index for a double-wildcard
// pattern), so an unbounded q drives a full-scan match cost proportional
// to len(q). 200 runes is the most spec-grounded bound — it matches the
// sister cap and aligns with the project string-input convention
// (creators.nickname 50, creators.avatar_url 500, pins.title 200,
// pins.media_url 500, search.q 200).
const maxListQueryRunes = 200

type Handler struct {
	q *db.Queries
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{q: db.New(database)}
}

type TagResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Category     string `json:"category"`
	DisplayOrder int32  `json:"display_order"`
}

// List handles GET /api/tags?category=&q=
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	query := r.URL.Query().Get("q")

	if utf8.RuneCountInString(query) > maxListQueryRunes {
		writeError(w, http.StatusBadRequest, "검색어는 200자 이내여야 합니다")
		return
	}

	var tags []db.Tag
	var err error

	switch {
	case query != "":
		tags, err = h.q.SearchTags(r.Context(), sql.NullString{String: query, Valid: true})
	case category != "":
		tags, err = h.q.ListTagsByCategory(r.Context(), category)
	default:
		tags, err = h.q.ListTags(r.Context())
	}

	if err != nil {
		if r.Context().Err() == context.Canceled {
			return // client disconnected, not a server error
		}
		log.Printf("tag.List: query error: %v", err)
		writeError(w, http.StatusInternalServerError, "태그 목록을 불러올 수 없습니다")
		return
	}

	result := make([]TagResponse, 0, len(tags))
	for _, t := range tags {
		order := int32(0)
		if t.DisplayOrder.Valid {
			order = t.DisplayOrder.Int32
		}
		result = append(result, TagResponse{
			ID:           t.ID.String(),
			Name:         t.Name,
			Slug:         t.Slug,
			Category:     t.Category,
			DisplayOrder: order,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"tags": result})
}

type PopularTagResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Category string `json:"category"`
	PinCount int64  `json:"pin_count"`
}

// PopularTags handles GET /api/tags/popular?limit=
func (h *Handler) PopularTags(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		if l > 0 && l <= 50 {
			limit = int32(l)
		} else if l > 50 {
			limit = 50
		}
	}

	rows, err := h.q.GetPopularTags(r.Context(), limit)
	if err != nil {
		log.Printf("tag.PopularTags: query error: %v", err)
		writeError(w, http.StatusInternalServerError, "인기 태그를 불러올 수 없습니다")
		return
	}

	if rows == nil {
		rows = []db.GetPopularTagsRow{}
	}
	result := make([]PopularTagResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, PopularTagResponse{
			ID:       row.ID.String(),
			Name:     row.Name,
			Slug:     row.Slug,
			Category: row.Category,
			PinCount: row.PinCount,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"tags": result})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("tag: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
