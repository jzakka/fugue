package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type AuthEvent struct {
	Event     string    `json:"event"`
	Provider  string    `json:"provider,omitempty"`
	CreatorID string    `json:"creator_id,omitempty"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

func LogAuthEvent(r *http.Request, event, provider string, creatorID uuid.UUID, err error) {
	e := AuthEvent{
		Event:     event,
		Provider:  provider,
		IP:        extractIP(r),
		UserAgent: r.UserAgent(),
		Timestamp: time.Now().UTC(),
	}
	if creatorID != uuid.Nil {
		e.CreatorID = creatorID.String()
	}
	if err != nil {
		e.Error = err.Error()
	}

	data, _ := json.Marshal(e)
	log.Println(string(data))
}
