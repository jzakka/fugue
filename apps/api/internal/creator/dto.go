package creator

import (
	"encoding/json"
	"time"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// CreatorPublicDTO is returned for public profile views (no email).
type CreatorPublicDTO struct {
	ID        string           `json:"id"`
	Nickname  string           `json:"nickname"`
	Bio       *string          `json:"bio"`
	Roles     []string         `json:"roles"`
	Contacts  json.RawMessage  `json:"contacts"`
	AvatarURL *string          `json:"avatar_url"`
	WorkCount int64            `json:"work_count"`
	CreatedAt time.Time        `json:"created_at"`
}

// CreatorPrivateDTO is returned for the authenticated user's own profile.
type CreatorPrivateDTO struct {
	ID        string           `json:"id"`
	Nickname  string           `json:"nickname"`
	Bio       *string          `json:"bio"`
	Roles     []string         `json:"roles"`
	Contacts  json.RawMessage  `json:"contacts"`
	AvatarURL *string          `json:"avatar_url"`
	Email     *string          `json:"email"`
	WorkCount int64            `json:"work_count"`
	CreatedAt time.Time        `json:"created_at"`
}

func toPublicDTO(c db.Creator, workCount int64) CreatorPublicDTO {
	var bio *string
	if c.Bio.Valid {
		bio = &c.Bio.String
	}
	var avatarURL *string
	if c.AvatarUrl.Valid {
		avatarURL = &c.AvatarUrl.String
	}
	roles := c.Roles
	if roles == nil {
		roles = []string{}
	}
	contacts := c.Contacts
	if contacts == nil {
		contacts = json.RawMessage(`{}`)
	}
	return CreatorPublicDTO{
		ID:        c.ID.String(),
		Nickname:  c.Nickname,
		Bio:       bio,
		Roles:     roles,
		Contacts:  contacts,
		AvatarURL: avatarURL,
		WorkCount: workCount,
		CreatedAt: c.CreatedAt,
	}
}

func toPrivateDTO(c db.Creator, workCount int64) CreatorPrivateDTO {
	var bio *string
	if c.Bio.Valid {
		bio = &c.Bio.String
	}
	var avatarURL *string
	if c.AvatarUrl.Valid {
		avatarURL = &c.AvatarUrl.String
	}
	var email *string
	if c.Email.Valid {
		email = &c.Email.String
	}
	roles := c.Roles
	if roles == nil {
		roles = []string{}
	}
	contacts := c.Contacts
	if contacts == nil {
		contacts = json.RawMessage(`{}`)
	}
	return CreatorPrivateDTO{
		ID:        c.ID.String(),
		Nickname:  c.Nickname,
		Bio:       bio,
		Roles:     roles,
		Contacts:  contacts,
		AvatarURL: avatarURL,
		Email:     email,
		WorkCount: workCount,
		CreatedAt: c.CreatedAt,
	}
}
