package creator

import (
	"time"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// BoardSummary is the per-board entry inside the public profile response's
// `boards` array.
//
// spec: profile `공개 프로필 조회 응답에 보드 요약과 핀 요약을 포함한다`
type BoardSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	IsPublic    bool      `json:"is_public"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PinSummary is the per-pin entry inside the public profile response's `pins`
// array. The outer payload is already the creator, so per-pin creator data is
// intentionally omitted to avoid duplication.
//
// spec: profile `공개 프로필 조회 응답에 보드 요약과 핀 요약을 포함한다`
type PinSummary struct {
	ID        string    `json:"id"`
	MediaURL  string    `json:"media_url"`
	MediaType string    `json:"media_type"`
	Title     string    `json:"title"`
	OgImage   *string   `json:"og_image"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatorPublicDTO struct {
	ID        string         `json:"id"`
	Nickname  string         `json:"nickname"`
	AvatarURL *string        `json:"avatar_url"`
	PinCount  int64          `json:"pin_count"`
	CreatedAt time.Time      `json:"created_at"`
	Boards    []BoardSummary `json:"boards"`
	Pins      []PinSummary   `json:"pins"`
}

type CreatorPrivateDTO struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	AvatarURL *string   `json:"avatar_url"`
	Email     *string   `json:"email"`
	PinCount  int64     `json:"pin_count"`
	CreatedAt time.Time `json:"created_at"`
}

func toPublicDTO(c db.Creator, pinCount int64, boards []BoardSummary, pins []PinSummary) CreatorPublicDTO {
	var avatarURL *string
	if c.AvatarUrl.Valid {
		avatarURL = &c.AvatarUrl.String
	}
	if boards == nil {
		boards = []BoardSummary{}
	}
	if pins == nil {
		pins = []PinSummary{}
	}
	return CreatorPublicDTO{
		ID:        c.ID.String(),
		Nickname:  c.Nickname,
		AvatarURL: avatarURL,
		PinCount:  pinCount,
		CreatedAt: c.CreatedAt,
		Boards:    boards,
		Pins:      pins,
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

func toBoardSummary(b db.Board) BoardSummary {
	var description *string
	if b.Description.Valid {
		description = &b.Description.String
	}
	return BoardSummary{
		ID:          b.ID.String(),
		Name:        b.Name,
		Description: description,
		IsPublic:    b.IsPublic,
		UpdatedAt:   b.UpdatedAt,
	}
}

func toPinSummary(row db.ListPinsByCreatorRow) PinSummary {
	var ogImage *string
	if row.OgImage.Valid {
		ogImage = &row.OgImage.String
	}
	return PinSummary{
		ID:        row.ID.String(),
		MediaURL:  row.MediaUrl,
		MediaType: row.MediaType,
		Title:     row.Title,
		OgImage:   ogImage,
		CreatedAt: row.CreatedAt,
	}
}
