package search

import (
	"context"
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

// SearchQuerier abstracts the db.Queries methods the search handler invokes.
// Mirrors the FeedQuerier pattern in apps/api/internal/feed/handler.go so that
// silent-error log contracts (e.g. the top_tags fallback) can be pinned with a
// mock querier in unit tests without standing up a real Postgres.
type SearchQuerier interface {
	SearchPinsBySimilarity(ctx context.Context, arg db.SearchPinsBySimilarityParams) ([]db.SearchPinsBySimilarityRow, error)
	SearchPinsByILIKE(ctx context.Context, arg db.SearchPinsByILIKEParams) ([]db.SearchPinsByILIKERow, error)
	SearchPinsWithTagFilter(ctx context.Context, arg db.SearchPinsWithTagFilterParams) ([]db.SearchPinsWithTagFilterRow, error)
	SearchPinsILIKEWithTagFilter(ctx context.Context, arg db.SearchPinsILIKEWithTagFilterParams) ([]db.SearchPinsILIKEWithTagFilterRow, error)
	SearchCreatorsBySimilarity(ctx context.Context, arg db.SearchCreatorsBySimilarityParams) ([]db.SearchCreatorsBySimilarityRow, error)
	SearchCreatorsByILIKE(ctx context.Context, arg db.SearchCreatorsByILIKEParams) ([]db.SearchCreatorsByILIKERow, error)
	SearchBoardsBySimilarity(ctx context.Context, arg db.SearchBoardsBySimilarityParams) ([]db.SearchBoardsBySimilarityRow, error)
	SearchBoardsByILIKE(ctx context.Context, arg db.SearchBoardsByILIKEParams) ([]db.SearchBoardsByILIKERow, error)
	SearchTopTags(ctx context.Context, dollar_1 []uuid.UUID) ([]db.SearchTopTagsRow, error)
}

type Handler struct {
	q SearchQuerier
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{q: db.New(database)}
}

// NewHandlerWithQuerier constructs a Handler bound to a custom SearchQuerier.
// It is used by tests that exercise silent-error log contracts with a fake
// querier (see handler_top_tags_log_test.go).
func NewHandlerWithQuerier(q SearchQuerier) *Handler {
	return &Handler{q: q}
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

// maxSearchQueryRunes caps the `q` parameter at the same length as
// `pins.title` (VARCHAR(200), migration 000004_pivot_pins.up.sql). The
// endpoint is unauthenticated and not rate-limited and the query feeds
// directly into pg_trgm `similarity($1)` plus `ILIKE '%' || $1 || '%'`
// across pins+creators+boards (search.sql L1-104), so unbounded input
// amplifies into a 3-4 query DB load per request. Mirrors the established
// project convention — creators.nickname (50), creators.avatar_url (500),
// pins.title (200), pins.media_url (500). 200 runes is the most
// spec-grounded bound because that is the upper length of the search
// target column itself.
const maxSearchQueryRunes = 200

// maxSearchOffset caps the `offset` pagination parameter. Sister-handler
// convention from pin/handler.go:568 (`o > 0 && o <= 100000`). Postgres
// LIMIT/OFFSET pagination must sort-and-skip the entire OFFSET prefix
// before returning rows; combined with this endpoint's pg_trgm similarity
// scoring across pins/creators/boards (3-4 queries per request), an
// unbounded offset like 999999999 forces a full multi-table scan even
// when the result set is empty. The endpoint is unauthenticated
// (cmd/server/main.go:135) so a single IP can repeatedly issue such
// requests. Silent clamp (out-of-range falls back to offset=0) matches
// pin/handler.go contract — out-of-range pagination has no meaningful
// successful response anyway, so a 400 would be a stricter behavior
// change than the sister.
const maxSearchOffset = 100000

// Search handles GET /api/search?q=&type=&tag_ids=&limit=&offset=
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "검색어를 입력해주세요")
		return
	}
	if utf8.RuneCountInString(q) > maxSearchQueryRunes {
		writeError(w, http.StatusBadRequest, "검색어는 200자 이내여야 합니다")
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
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 && o <= maxSearchOffset {
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
			// top_tags is a fan-out enrichment (related-tag discovery); the
			// outer `response["top_tags"] = []TopTag{}` fallback below keeps
			// the response shape stable even when SearchTopTags errors. The
			// fallback alone, however, hides the difference between "no tags
			// match the result set" and "pin_tags/tags join is degraded" —
			// operators only notice when the discovery UX silently collapses.
			// Mirrors the additive-logging contract from the sibling pins/
			// creators/boards branches above (search.go L137 / L186 / L201)
			// and the feed handler's media-type fallback (cycle for PR #216).
			topTags, err := h.q.SearchTopTags(r.Context(), pinIDs)
			if err != nil {
				log.Printf("search.SearchTopTags: %v (q=%q pin_count=%d)", err, q, len(pinIDs))
			} else {
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
