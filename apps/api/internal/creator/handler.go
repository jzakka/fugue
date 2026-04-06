package creator

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type CreatorQuerier interface {
	GetCreator(ctx context.Context, id uuid.UUID) (db.Creator, error)
	UpdateCreator(ctx context.Context, arg db.UpdateCreatorParams) (db.Creator, error)
	CountPinsByCreator(ctx context.Context, creatorID uuid.UUID) (int64, error)
}

type Handler struct {
	q CreatorQuerier
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{q: db.New(database)}
}

func NewHandlerWithQuerier(q CreatorQuerier) *Handler {
	return &Handler{q: q}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 크리에이터 ID입니다")
		return
	}

	creator, err := h.q.GetCreator(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "크리에이터를 찾을 수 없습니다")
			return
		}
		log.Printf("creator.GetByID: query error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "크리에이터 정보를 불러올 수 없습니다")
		return
	}

	workCount, err := h.q.CountPinsByCreator(r.Context(), id)
	if err != nil {
		log.Printf("creator.GetByID: count error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "크리에이터 정보를 불러올 수 없습니다")
		return
	}

	writeJSON(w, http.StatusOK, toPublicDTO(creator, workCount))
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	creator, err := h.q.GetCreator(r.Context(), creatorID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "크리에이터를 찾을 수 없습니다")
			return
		}
		log.Printf("creator.GetMe: query error: %v (id=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "프로필을 불러올 수 없습니다")
		return
	}

	workCount, err := h.q.CountPinsByCreator(r.Context(), creatorID)
	if err != nil {
		log.Printf("creator.GetMe: count error: %v (id=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "프로필을 불러올 수 없습니다")
		return
	}

	writeJSON(w, http.StatusOK, toPrivateDTO(creator, workCount))
}

type updateRequest struct {
	Nickname  *string `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	current, err := h.q.GetCreator(r.Context(), creatorID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "크리에이터를 찾을 수 없습니다")
			return
		}
		log.Printf("creator.UpdateMe: get error: %v (id=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "프로필을 불러올 수 없습니다")
		return
	}

	nickname := current.Nickname
	if req.Nickname != nil {
		nickname = strings.TrimSpace(*req.Nickname)
	}

	if nickname == "" {
		writeError(w, http.StatusBadRequest, "닉네임은 비어있을 수 없습니다")
		return
	}
	if utf8.RuneCountInString(nickname) > 50 {
		writeError(w, http.StatusBadRequest, "닉네임은 50자를 초과할 수 없습니다")
		return
	}

	avatarURL := current.AvatarUrl
	if req.AvatarURL != nil {
		if *req.AvatarURL == "" {
			avatarURL = sql.NullString{}
		} else {
			avatarURL = sql.NullString{String: *req.AvatarURL, Valid: true}
		}
	}

	updated, err := h.q.UpdateCreator(r.Context(), db.UpdateCreatorParams{
		ID:        creatorID,
		Nickname:  nickname,
		AvatarUrl: avatarURL,
	})
	if err != nil {
		log.Printf("creator.UpdateMe: update error: %v (id=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "프로필 업데이트에 실패했습니다")
		return
	}

	workCount, err := h.q.CountPinsByCreator(r.Context(), creatorID)
	if err != nil {
		log.Printf("creator.UpdateMe: count error: %v (id=%s)", err, creatorID)
		workCount = 0
	}

	writeJSON(w, http.StatusOK, toPrivateDTO(updated, workCount))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("creator: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
