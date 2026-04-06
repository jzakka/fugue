package interaction

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/google/uuid"
)

type Handler struct {
	database *sql.DB
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{database: database}
}

type createInteractionRequest struct {
	PinID string `json:"pin_id"`
	Type  string `json:"type"`
}

// Create handles POST /api/interactions [auth]
//
// Body: { "pin_id": "uuid", "type": "view"|"pin"|"board_add" }
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "인증이 필요합니다")
		return
	}

	var req createInteractionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	if !isValidInteractionType(req.Type) {
		writeError(w, http.StatusBadRequest, "유효하지 않은 인터랙션 타입입니다 (view, pin, board_add)")
		return
	}

	pinUUID, err := uuid.Parse(req.PinID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	err = db.New(h.database).CreateInteraction(r.Context(), db.CreateInteractionParams{
		UserID: creatorID,
		PinID:  uuid.NullUUID{UUID: pinUUID, Valid: true},
		Type:   req.Type,
	})
	if err != nil {
		log.Printf("interaction.Create: db error: %v (user=%s pin=%s type=%s)", err, creatorID, req.PinID, req.Type)
		writeError(w, http.StatusInternalServerError, "인터랙션을 저장할 수 없습니다")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func isValidInteractionType(t string) bool {
	switch t {
	case "view", "pin", "board_add":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("interaction: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
