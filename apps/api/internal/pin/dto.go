package pin

import (
	"encoding/json"
	"time"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type CreatorSummary struct {
	ID        string  `json:"id"`
	Nickname  string  `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
}

type PinResponse struct {
	ID          string           `json:"id"`
	URL         string           `json:"url"`
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	Field       string           `json:"field"`
	Tags        []string         `json:"tags"`
	OgImage     *string          `json:"og_image"`
	OgData      *json.RawMessage `json:"og_data"`
	PinCount    int32            `json:"pin_count"`
	CreatedAt   time.Time        `json:"created_at"`
	Creator     CreatorSummary   `json:"creator"`
}

type ListPinsResponse struct {
	Pins    []PinResponse `json:"pins"`
	HasMore bool          `json:"has_more"`
}

type CreatePinRequest struct {
	URL         string           `json:"url"`
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	Field       string           `json:"field"`
	Tags        []string         `json:"tags"`
	OgImage     *string          `json:"og_image"`
	OgData      *json.RawMessage `json:"og_data"`
}

type CreatedPinResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Field     string    `json:"field"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
}

func toCreatedResponse(p db.Pin) CreatedPinResponse {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	return CreatedPinResponse{
		ID:        p.ID.String(),
		URL:       p.Url,
		Title:     p.Title,
		Field:     p.Field,
		Tags:      tags,
		CreatedAt: p.CreatedAt,
	}
}

func toPinDetailResponse(row db.GetPinWithCreatorRow) PinResponse {
	var desc *string
	if row.Description.Valid {
		desc = &row.Description.String
	}
	var ogImage *string
	if row.OgImage.Valid {
		ogImage = &row.OgImage.String
	}
	var ogData *json.RawMessage
	if row.OgData.Valid {
		raw := json.RawMessage(row.OgData.RawMessage)
		ogData = &raw
	}
	var avatarURL *string
	if row.CreatorAvatarUrl.Valid {
		avatarURL = &row.CreatorAvatarUrl.String
	}
	tags := row.Tags
	if tags == nil {
		tags = []string{}
	}
	return PinResponse{
		ID:          row.ID.String(),
		URL:         row.Url,
		Title:       row.Title,
		Description: desc,
		Field:       row.Field,
		Tags:        tags,
		OgImage:     ogImage,
		OgData:      ogData,
		PinCount:    row.PinCount,
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func toRelatedPinResponse(row db.RelatedPinsRow) PinResponse {
	var desc *string
	if row.Description.Valid {
		desc = &row.Description.String
	}
	var ogImage *string
	if row.OgImage.Valid {
		ogImage = &row.OgImage.String
	}
	var ogData *json.RawMessage
	if row.OgData.Valid {
		raw := json.RawMessage(row.OgData.RawMessage)
		ogData = &raw
	}
	var avatarURL *string
	if row.CreatorAvatarUrl.Valid {
		avatarURL = &row.CreatorAvatarUrl.String
	}
	tags := row.Tags
	if tags == nil {
		tags = []string{}
	}
	return PinResponse{
		ID:          row.ID.String(),
		URL:         row.Url,
		Title:       row.Title,
		Description: desc,
		Field:       row.Field,
		Tags:        tags,
		OgImage:     ogImage,
		OgData:      ogData,
		PinCount:    row.PinCount,
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func toCreatorPinResponse(row db.ListPinsByCreatorRow) PinResponse {
	var desc *string
	if row.Description.Valid {
		desc = &row.Description.String
	}
	var ogImage *string
	if row.OgImage.Valid {
		ogImage = &row.OgImage.String
	}
	var ogData *json.RawMessage
	if row.OgData.Valid {
		raw := json.RawMessage(row.OgData.RawMessage)
		ogData = &raw
	}
	var avatarURL *string
	if row.CreatorAvatarUrl.Valid {
		avatarURL = &row.CreatorAvatarUrl.String
	}
	tags := row.Tags
	if tags == nil {
		tags = []string{}
	}
	return PinResponse{
		ID:          row.ID.String(),
		URL:         row.Url,
		Title:       row.Title,
		Description: desc,
		Field:       row.Field,
		Tags:        tags,
		OgImage:     ogImage,
		OgData:      ogData,
		PinCount:    row.PinCount,
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func toPinResponse(row db.ListPinsWithCreatorRow) PinResponse {
	var desc *string
	if row.Description.Valid {
		desc = &row.Description.String
	}
	var ogImage *string
	if row.OgImage.Valid {
		ogImage = &row.OgImage.String
	}
	var ogData *json.RawMessage
	if row.OgData.Valid {
		raw := json.RawMessage(row.OgData.RawMessage)
		ogData = &raw
	}
	var avatarURL *string
	if row.CreatorAvatarUrl.Valid {
		avatarURL = &row.CreatorAvatarUrl.String
	}
	tags := row.Tags
	if tags == nil {
		tags = []string{}
	}
	return PinResponse{
		ID:          row.ID.String(),
		URL:         row.Url,
		Title:       row.Title,
		Description: desc,
		Field:       row.Field,
		Tags:        tags,
		OgImage:     ogImage,
		OgData:      ogData,
		PinCount:    row.PinCount,
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}
