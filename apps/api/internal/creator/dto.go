package creator

import (
	"time"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type CreatorPublicDTO struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	AvatarURL *string   `json:"avatar_url"`
	PinCount  int64     `json:"pin_count"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatorPrivateDTO struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	AvatarURL *string   `json:"avatar_url"`
	Email     *string   `json:"email"`
	PinCount  int64     `json:"pin_count"`
	CreatedAt time.Time `json:"created_at"`
}

func toPublicDTO(c db.Creator, pinCount int64) CreatorPublicDTO {
	var avatarURL *string
	if c.AvatarUrl.Valid {
		avatarURL = &c.AvatarUrl.String
	}
	return CreatorPublicDTO{
		ID:        c.ID.String(),
		Nickname:  c.Nickname,
		AvatarURL: avatarURL,
		PinCount:  pinCount,
		CreatedAt: c.CreatedAt,
	}
}

func toPrivateDTO(c db.Creator, pinCount int64) CreatorPrivateDTO {
	var avatarURL *string
	if c.AvatarUrl.Valid {
		avatarURL = &c.AvatarUrl.String
	}
	var email *string
	if c.Email.Valid {
		email = &c.Email.String
	}
	return CreatorPrivateDTO{
		ID:        c.ID.String(),
		Nickname:  c.Nickname,
		AvatarURL: avatarURL,
		Email:     email,
		PinCount:  pinCount,
		CreatedAt: c.CreatedAt,
	}
}
