package admin

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Handler provides admin API endpoints for bot source management.
type Handler struct {
	q *db.Queries
}

// NewHandler creates a new admin Handler.
func NewHandler(database *sql.DB) *Handler {
	return &Handler{q: db.New(database)}
}

// statusResponse represents the bot status overview.
type statusResponse struct {
	Sources []sourceResponse `json:"sources"`
}

// sourceResponse represents a single bot source.
type sourceResponse struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Platform      string      `json:"platform"`
	SeedURLs      []string    `json:"seed_urls"`
	IntervalHours int32       `json:"interval_hours"`
	Enabled       bool        `json:"enabled"`
	LastCrawledAt *string     `json:"last_crawled_at"`
	Stats         interface{} `json:"stats"`
	CreatedAt     string      `json:"created_at"`
}

func toSourceResponse(bs db.BotSource) sourceResponse {
	resp := sourceResponse{
		ID:            bs.ID.String(),
		Name:          bs.Name,
		Platform:      bs.Platform,
		SeedURLs:      bs.SeedUrls,
		IntervalHours: bs.IntervalHours,
		Enabled:       bs.Enabled,
		CreatedAt:     bs.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if bs.LastCrawledAt.Valid {
		t := bs.LastCrawledAt.Time.Format("2006-01-02T15:04:05Z")
		resp.LastCrawledAt = &t
	}

	if bs.Stats.Valid {
		var stats interface{}
		if err := json.Unmarshal(bs.Stats.RawMessage, &stats); err == nil {
			resp.Stats = stats
		} else {
			resp.Stats = map[string]interface{}{}
		}
	} else {
		resp.Stats = map[string]interface{}{}
	}

	return resp
}

// Status handles GET /api/admin/bot/status
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	sources, err := h.q.ListAllBotSources(r.Context())
	if err != nil {
		log.Printf("admin.Status: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get bot status")
		return
	}

	resp := statusResponse{
		Sources: make([]sourceResponse, 0, len(sources)),
	}
	for _, s := range sources {
		resp.Sources = append(resp.Sources, toSourceResponse(s))
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListSources handles GET /api/admin/bot/sources
func (h *Handler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.q.ListAllBotSources(r.Context())
	if err != nil {
		log.Printf("admin.ListSources: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list sources")
		return
	}

	resp := make([]sourceResponse, 0, len(sources))
	for _, s := range sources {
		resp = append(resp, toSourceResponse(s))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"sources": resp})
}

// createSourceRequest is the JSON body for creating a new source.
type createSourceRequest struct {
	Name          string   `json:"name"`
	Platform      string   `json:"platform"`
	SeedURLs      []string `json:"seed_urls"`
	IntervalHours int32    `json:"interval_hours"`
	Enabled       bool     `json:"enabled"`
}

// CreateSource handles POST /api/admin/bot/sources
func (h *Handler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var req createSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" || req.Platform == "" || len(req.SeedURLs) == 0 {
		writeError(w, http.StatusBadRequest, "name, platform, and seed_urls are required")
		return
	}
	if req.IntervalHours <= 0 {
		req.IntervalHours = 24
	}

	source, err := h.q.CreateBotSource(r.Context(), db.CreateBotSourceParams{
		Name:          req.Name,
		Platform:      req.Platform,
		SeedUrls:      req.SeedURLs,
		IntervalHours: req.IntervalHours,
		Enabled:       req.Enabled,
	})
	if err != nil {
		log.Printf("admin.CreateSource: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create source")
		return
	}

	writeJSON(w, http.StatusCreated, toSourceResponse(source))
}

// toggleRequest is the JSON body for toggling source enabled status.
type toggleRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleSource handles PATCH /api/admin/bot/sources/{id}
func (h *Handler) ToggleSource(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid source ID")
		return
	}

	var req toggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	source, err := h.q.ToggleBotSource(r.Context(), db.ToggleBotSourceParams{
		ID:      id,
		Enabled: req.Enabled,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Source not found")
			return
		}
		log.Printf("admin.ToggleSource: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to toggle source")
		return
	}

	writeJSON(w, http.StatusOK, toSourceResponse(source))
}

// DeleteSource handles DELETE /api/admin/bot/sources/{id}
func (h *Handler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid source ID")
		return
	}

	rows, err := h.q.DeleteBotSource(r.Context(), id)
	if err != nil {
		log.Printf("admin.DeleteSource: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete source")
		return
	}

	if rows == 0 {
		writeError(w, http.StatusNotFound, "Source not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("admin: json encode error: %v", err)
	}
}

