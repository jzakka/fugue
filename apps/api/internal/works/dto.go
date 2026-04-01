package works

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

type WorkResponse struct {
	ID          string           `json:"id"`
	URL         string           `json:"url"`
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	Field       string           `json:"field"`
	Tags        []string         `json:"tags"`
	OgImage     *string          `json:"og_image"`
	OgData      *json.RawMessage `json:"og_data"`
	CreatedAt   time.Time        `json:"created_at"`
	Creator     CreatorSummary   `json:"creator"`
}

type ListWorksResponse struct {
	Works   []WorkResponse `json:"works"`
	HasMore bool           `json:"has_more"`
}

func toCreatorWorkResponse(row db.ListWorksByCreatorRow) WorkResponse {
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

	return WorkResponse{
		ID:          row.ID.String(),
		URL:         row.Url,
		Title:       row.Title,
		Description: desc,
		Field:       row.Field,
		Tags:        tags,
		OgImage:     ogImage,
		OgData:      ogData,
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func toWorkResponse(row db.ListWorksWithCreatorRow) WorkResponse {
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

	return WorkResponse{
		ID:          row.ID.String(),
		URL:         row.Url,
		Title:       row.Title,
		Description: desc,
		Field:       row.Field,
		Tags:        tags,
		OgImage:     ogImage,
		OgData:      ogData,
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}
