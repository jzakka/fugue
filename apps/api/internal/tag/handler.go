package tag

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

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
