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

type TagResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Category string `json:"category"`
}

type PinResponse struct {
	ID          string           `json:"id"`
	URL         *string          `json:"url"`
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	MediaURL    string           `json:"media_url"`
	MediaType   string           `json:"media_type"`
	OgImage     *string          `json:"og_image"`
	OgData      *json.RawMessage `json:"og_data"`
	Tags        []TagResponse    `json:"tags"`
	CreatedAt   time.Time        `json:"created_at"`
	Creator     CreatorSummary   `json:"creator"`
}

type ListPinsResponse struct {
	Pins    []PinResponse `json:"pins"`
	HasMore bool          `json:"has_more"`
}

type CreatePinRequest struct {
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	OgImage     *string          `json:"og_image"`
	OgData      *json.RawMessage `json:"og_data"`
}

type CreatedPinResponse struct {
	ID        string    `json:"id"`
	URL       *string   `json:"url"`
	Title     string    `json:"title"`
	MediaURL  string    `json:"media_url"`
	MediaType string    `json:"media_type"`
	CreatedAt time.Time `json:"created_at"`
}

func toCreatedResponse(p db.Pin) CreatedPinResponse {
	var url *string
	if p.Url.Valid {
		url = &p.Url.String
	}
	return CreatedPinResponse{
		ID:        p.ID.String(),
		URL:       url,
		Title:     p.Title,
		MediaURL:  p.MediaUrl,
		MediaType: p.MediaType,
		CreatedAt: p.CreatedAt,
	}
}

func toPinDetailResponse(row db.GetPinWithCreatorRow) PinResponse {
	var url *string
	if row.Url.Valid {
		url = &row.Url.String
	}
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
	return PinResponse{
		ID:          row.ID.String(),
		URL:         url,
		Title:       row.Title,
		Description: desc,
		MediaURL:    row.MediaUrl,
		MediaType:   row.MediaType,
		OgImage:     ogImage,
		OgData:      ogData,
		Tags:        []TagResponse{},
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func toRelatedPinResponse(row db.RelatedPinsRow) PinResponse {
	var url *string
	if row.Url.Valid {
		url = &row.Url.String
	}
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
	return PinResponse{
		ID:          row.ID.String(),
		URL:         url,
		Title:       row.Title,
		Description: desc,
		MediaURL:    row.MediaUrl,
		MediaType:   row.MediaType,
		OgImage:     ogImage,
		OgData:      ogData,
		Tags:        []TagResponse{},
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func toCreatorPinResponse(row db.ListPinsByCreatorRow) PinResponse {
	var url *string
	if row.Url.Valid {
		url = &row.Url.String
	}
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
	return PinResponse{
		ID:          row.ID.String(),
		URL:         url,
		Title:       row.Title,
		Description: desc,
		MediaURL:    row.MediaUrl,
		MediaType:   row.MediaType,
		OgImage:     ogImage,
		OgData:      ogData,
		Tags:        []TagResponse{},
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func toPinResponse(row db.ListPinsWithCreatorRow) PinResponse {
	var url *string
	if row.Url.Valid {
		url = &row.Url.String
	}
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
	return PinResponse{
		ID:          row.ID.String(),
		URL:         url,
		Title:       row.Title,
		Description: desc,
		MediaURL:    row.MediaUrl,
		MediaType:   row.MediaType,
		OgImage:     ogImage,
		OgData:      ogData,
		Tags:        []TagResponse{},
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}
