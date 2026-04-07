package search

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type Handler struct {
	q *db.Queries
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{q: db.New(database)}
}

// PinResult is the JSON shape for a pin search result.
type PinResult struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	MediaURL    string  `json:"media_url"`
	MediaType   string  `json:"media_type"`
	URL         *string `json:"url"`
	Description *string `json:"description"`
	OgImage     *string `json:"og_image"`
	CreatorID   string  `json:"creator_id"`
	Nickname    string  `json:"creator_nickname"`
	AvatarURL   *string `json:"creator_avatar_url"`
	CreatedAt   string  `json:"created_at"`
}

type CreatorResult struct {
	ID        string  `json:"id"`
	Nickname  string  `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
	CreatedAt string  `json:"created_at"`
}

type BoardResult struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	CreatorID       string  `json:"creator_id"`
	CreatorNickname string  `json:"creator_nickname"`
	CreatedAt       string  `json:"created_at"`
}

type TopTag struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// Search handles GET /api/search?q=&type=&tag_ids=&limit=&offset=
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "검색어를 입력해주세요")
		return
	}

	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "all"
	}
	if searchType != "all" && searchType != "pins" && searchType != "creators" && searchType != "boards" {
		writeError(w, http.StatusBadRequest, "유효하지 않은 type입니다")
		return
	}

	limit := int32(20)
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = int32(l)
		if limit > 50 {
			limit = 50
		}
	}

	offset := int32(0)
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 {
		offset = int32(o)
	}

	// Parse tag_ids (pins only, max 5)
	var tagIDs []uuid.UUID
	if tagParam := r.URL.Query().Get("tag_ids"); tagParam != "" {
		parts := strings.Split(tagParam, ",")
		if len(parts) > 5 {
			writeError(w, http.StatusBadRequest, "태그 필터는 최대 5개까지 가능합니다")
			return
		}
		for _, s := range parts {
			id, err := uuid.Parse(strings.TrimSpace(s))
			if err != nil {
				writeError(w, http.StatusBadRequest, "유효하지 않은 태그 ID입니다")
				return
			}
			tagIDs = append(tagIDs, id)
		}
	}

	useSimilarity := utf8.RuneCountInString(q) > 2

	response := make(map[string]any)

	// Search pins
	if searchType == "all" || searchType == "pins" {
		pins, err := h.searchPins(r, q, useSimilarity, tagIDs, limit, offset)
		if err != nil {
			log.Printf("search: pins error: %v", err)
			writeError(w, http.StatusInternalServerError, "검색에 실패했습니다")
			return
		}
		response["pins"] = pins
		if searchType == "pins" {
			response["has_more"] = len(pins) == int(limit)
		}

		// Top tags: skip for small autocomplete requests (limit<=5) to avoid extra DB work
		skipTopTags := searchType == "all" && limit <= 5
		topPins := pins
		if !skipTopTags {
			if offset > 0 || limit < 100 {
				var err2 error
				topPins, err2 = h.searchPins(r, q, useSimilarity, tagIDs, 100, 0)
				if err2 != nil {
					log.Printf("search: top_tags query error: %v", err2)
					topPins = pins
				}
			}
		}
		if !skipTopTags && len(topPins) > 0 {
			pinIDs := make([]uuid.UUID, 0, len(topPins))
			for _, p := range topPins {
				pinIDs = append(pinIDs, uuid.MustParse(p.ID))
			}
			topTags, err := h.q.SearchTopTags(r.Context(), pinIDs)
			if err == nil {
				tags := make([]TopTag, 0, len(topTags))
				for _, t := range topTags {
					tags = append(tags, TopTag{
						ID: t.ID.String(), Name: t.Name,
						Slug: t.Slug, Category: t.Category, Count: t.Count,
					})
				}
				response["top_tags"] = tags
			}
		}
		if _, ok := response["top_tags"]; !ok {
			response["top_tags"] = []TopTag{}
		}
	}

	// Search creators (tag_ids ignored for creators)
	if searchType == "all" || searchType == "creators" {
		creatorLimit := limit
		creators, err := h.searchCreators(r, q, useSimilarity, creatorLimit, offset)
		if err != nil {
			log.Printf("search: creators error: %v", err)
			writeError(w, http.StatusInternalServerError, "검색에 실패했습니다")
			return
		}
		response["creators"] = creators
		if searchType == "creators" {
			response["has_more"] = len(creators) == int(limit)
		}
	}

	// Search boards (tag_ids ignored for boards)
	if searchType == "all" || searchType == "boards" {
		boardLimit := limit
		boards, err := h.searchBoards(r, q, useSimilarity, boardLimit, offset)
		if err != nil {
			log.Printf("search: boards error: %v", err)
			writeError(w, http.StatusInternalServerError, "검색에 실패했습니다")
			return
		}
		response["boards"] = boards
		if searchType == "boards" {
			response["has_more"] = len(boards) == int(limit)
		}
	}

	// Ensure top_tags is always present
	if _, ok := response["top_tags"]; !ok {
		response["top_tags"] = []TopTag{}
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) searchPins(r *http.Request, q string, useSimilarity bool, tagIDs []uuid.UUID, limit, offset int32) ([]PinResult, error) {
	if useSimilarity && len(tagIDs) > 0 {
		rows, err := h.q.SearchPinsWithTagFilter(r.Context(), db.SearchPinsWithTagFilterParams{
			Similarity: q, Limit: limit, Offset: offset, Column4: tagIDs,
		})
		if err != nil {
			return nil, err
		}
		return mapPinsWithTag(rows), nil
	}
	if !useSimilarity && len(tagIDs) > 0 {
		rows, err := h.q.SearchPinsILIKEWithTagFilter(r.Context(), db.SearchPinsILIKEWithTagFilterParams{
			Column1: sql.NullString{String: q, Valid: true}, Limit: limit, Offset: offset, Column4: tagIDs,
		})
		if err != nil {
			return nil, err
		}
		return mapPinsILIKEWithTag(rows), nil
	}
	if useSimilarity {
		rows, err := h.q.SearchPinsBySimilarity(r.Context(), db.SearchPinsBySimilarityParams{
			Similarity: q, Limit: limit, Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		return mapPinsSim(rows), nil
	}
	rows, err := h.q.SearchPinsByILIKE(r.Context(), db.SearchPinsByILIKEParams{
		Column1: sql.NullString{String: q, Valid: true}, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	return mapPinsILIKE(rows), nil
}

func (h *Handler) searchCreators(r *http.Request, q string, useSimilarity bool, limit, offset int32) ([]CreatorResult, error) {
	if useSimilarity {
		rows, err := h.q.SearchCreatorsBySimilarity(r.Context(), db.SearchCreatorsBySimilarityParams{
			Similarity: q, Limit: limit, Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		result := make([]CreatorResult, 0, len(rows))
		for _, r := range rows {
			result = append(result, CreatorResult{
				ID: r.ID.String(), Nickname: r.Nickname,
				AvatarURL: nullStr(r.AvatarUrl), CreatedAt: r.CreatedAt.Format(time.RFC3339),
			})
		}
		return result, nil
	}
	rows, err := h.q.SearchCreatorsByILIKE(r.Context(), db.SearchCreatorsByILIKEParams{
		Column1: sql.NullString{String: q, Valid: true}, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]CreatorResult, 0, len(rows))
	for _, r := range rows {
		result = append(result, CreatorResult{
			ID: r.ID.String(), Nickname: r.Nickname,
			AvatarURL: nullStr(r.AvatarUrl), CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func (h *Handler) searchBoards(r *http.Request, q string, useSimilarity bool, limit, offset int32) ([]BoardResult, error) {
	if useSimilarity {
		rows, err := h.q.SearchBoardsBySimilarity(r.Context(), db.SearchBoardsBySimilarityParams{
			Similarity: q, Limit: limit, Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		result := make([]BoardResult, 0, len(rows))
		for _, r := range rows {
			result = append(result, BoardResult{
				ID: r.ID.String(), Name: r.Name,
				Description: nullStr(r.Description), CreatorID: r.CreatorID.String(),
				CreatorNickname: r.CreatorNickname, CreatedAt: r.CreatedAt.Format(time.RFC3339),
			})
		}
		return result, nil
	}
	rows, err := h.q.SearchBoardsByILIKE(r.Context(), db.SearchBoardsByILIKEParams{
		Column1: sql.NullString{String: q, Valid: true}, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]BoardResult, 0, len(rows))
	for _, r := range rows {
		result = append(result, BoardResult{
			ID: r.ID.String(), Name: r.Name,
			Description: nullStr(r.Description), CreatorID: r.CreatorID.String(),
			CreatorNickname: r.CreatorNickname, CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func mapPinsSim(rows []db.SearchPinsBySimilarityRow) []PinResult {
	result := make([]PinResult, 0, len(rows))
	for _, r := range rows {
		result = append(result, PinResult{
			ID: r.ID.String(), Title: r.Title, MediaURL: r.MediaUrl, MediaType: r.MediaType,
			URL: nullStr(r.Url), Description: nullStr(r.Description), OgImage: nullStr(r.OgImage),
			CreatorID: r.CreatorIDRef.String(), Nickname: r.CreatorNickname,
			AvatarURL: nullStr(r.CreatorAvatarUrl), CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func mapPinsILIKE(rows []db.SearchPinsByILIKERow) []PinResult {
	result := make([]PinResult, 0, len(rows))
	for _, r := range rows {
		result = append(result, PinResult{
			ID: r.ID.String(), Title: r.Title, MediaURL: r.MediaUrl, MediaType: r.MediaType,
			URL: nullStr(r.Url), Description: nullStr(r.Description), OgImage: nullStr(r.OgImage),
			CreatorID: r.CreatorIDRef.String(), Nickname: r.CreatorNickname,
			AvatarURL: nullStr(r.CreatorAvatarUrl), CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func mapPinsWithTag(rows []db.SearchPinsWithTagFilterRow) []PinResult {
	result := make([]PinResult, 0, len(rows))
	for _, r := range rows {
		result = append(result, PinResult{
			ID: r.ID.String(), Title: r.Title, MediaURL: r.MediaUrl, MediaType: r.MediaType,
			URL: nullStr(r.Url), Description: nullStr(r.Description), OgImage: nullStr(r.OgImage),
			CreatorID: r.CreatorIDRef.String(), Nickname: r.CreatorNickname,
			AvatarURL: nullStr(r.CreatorAvatarUrl), CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func mapPinsILIKEWithTag(rows []db.SearchPinsILIKEWithTagFilterRow) []PinResult {
	result := make([]PinResult, 0, len(rows))
	for _, r := range rows {
		result = append(result, PinResult{
			ID: r.ID.String(), Title: r.Title, MediaURL: r.MediaUrl, MediaType: r.MediaType,
			URL: nullStr(r.Url), Description: nullStr(r.Description), OgImage: nullStr(r.OgImage),
			CreatorID: r.CreatorIDRef.String(), Nickname: r.CreatorNickname,
			AvatarURL: nullStr(r.CreatorAvatarUrl), CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func nullStr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
