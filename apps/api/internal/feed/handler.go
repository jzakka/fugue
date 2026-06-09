package feed

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// FeedQuerier is the subset of db.Queries the feed handler depends on. Tests
// substitute it to exercise pagination wiring without a live Postgres.
type FeedQuerier interface {
	CountUserPins(ctx context.Context, creatorID uuid.UUID) (int64, error)
	GetUserTagFrequency(ctx context.Context, arg db.GetUserTagFrequencyParams) ([]db.GetUserTagFrequencyRow, error)
	GetUserMediaTypeFrequency(ctx context.Context, arg db.GetUserMediaTypeFrequencyParams) ([]db.GetUserMediaTypeFrequencyRow, error)
	RecommendByTags(ctx context.Context, arg db.RecommendByTagsParams) ([]db.RecommendByTagsRow, error)
	RecommendByMediaType(ctx context.Context, arg db.RecommendByMediaTypeParams) ([]db.RecommendByMediaTypeRow, error)
	ListPinsWithCreator(ctx context.Context, arg db.ListPinsWithCreatorParams) ([]db.ListPinsWithCreatorRow, error)
	ListLatestPinsExcludingRecommended(ctx context.Context, arg db.ListLatestPinsExcludingRecommendedParams) ([]db.ListLatestPinsExcludingRecommendedRow, error)
}

type Handler struct {
	q   FeedQuerier
	rdb *redis.Client
}

func NewHandler(database *sql.DB, rdb *redis.Client) *Handler {
	return &Handler{q: db.New(database), rdb: rdb}
}

// NewHandlerWithQuerier constructs a Handler bound to a custom FeedQuerier. It
// is used by tests that exercise pagination wiring with a fake querier.
func NewHandlerWithQuerier(q FeedQuerier, rdb *redis.Client) *Handler {
	return &Handler{q: q, rdb: rdb}
}

type FeedResponse struct {
	Pins       []PinResponse `json:"pins"`
	NextCursor *string       `json:"next_cursor"`
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
	CreatedAt   time.Time        `json:"created_at"`
	Creator     CreatorSummary   `json:"creator"`
}

type CreatorSummary struct {
	ID        string  `json:"id"`
	Nickname  string  `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
}

func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	creatorID, authenticated := auth.CreatorIDFromContext(r.Context())

	if authenticated {
		cacheKey := feedCacheKey(creatorID, limit, offset)
		cached, err := h.rdb.Get(r.Context(), cacheKey).Bytes()
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
		// Cache-aside fallthrough is intentional, but redis.Nil (normal cache
		// miss) and Redis degradation (OOM / MISCONF / network / read-only
		// replica failover) must be distinguished so operators can see when
		// every request is amplifying to the full personalized path
		// (CountUserPins + GetUserTagFrequency + RecommendByTags +
		// (conditional) RecommendByMediaType + ListPinsWithCreator). Mirrors
		// the additive-logging contract from cycle K's cache Set branch
		// (L126-128) and auth/service.go:226-237 which distinguishes
		// redis.Nil from other errors via fmt.Errorf wrap. redis.Nil stays
		// silent because that is the documented cache-miss signal.
		if err != redis.Nil {
			log.Printf("feed.GetFeed: cache get error: %v (key=%s)", err, cacheKey)
		}
	}

	var resp FeedResponse

	if authenticated {
		pinCount, err := h.q.CountUserPins(r.Context(), creatorID)
		if err != nil {
			log.Printf("feed.GetFeed: CountUserPins error: %v (user=%s)", err, creatorID)
			writeError(w, http.StatusInternalServerError, "피드를 불러올 수 없습니다")
			return
		}

		if pinCount < 10 {
			resp, err = h.buildLatestFeed(r.Context(), limit, offset)
			if err != nil {
				log.Printf("feed.GetFeed: buildLatestFeed error: %v", err)
				writeError(w, http.StatusInternalServerError, "피드를 불러올 수 없습니다")
				return
			}
		} else {
			// spec: feed `개인화 피드의 페이지네이션은 페이지 간 작품 중복을 반환하지 않는다`
			resp, err = h.buildPersonalizedFeed(r.Context(), creatorID, limit, offset)
			if err != nil {
				log.Printf("feed.GetFeed: buildPersonalizedFeed error: %v (user=%s)", err, creatorID)
				writeError(w, http.StatusInternalServerError, "피드를 불러올 수 없습니다")
				return
			}
		}

		if data, err := json.Marshal(resp); err == nil {
			cacheKey := feedCacheKey(creatorID, limit, offset)
			// Cache write is opportunistic (no spec contract) but a silent
			// discard hides Redis degradation (OOM / MISCONF / network /
			// read-only replica failover) until the operator notices DB load
			// amplification: every subsequent same (sub, limit, offset)
			// request re-runs the full personalized path (CountUserPins +
			// GetUserTagFrequency + RecommendByTags + (conditional)
			// RecommendByMediaType + ListPinsWithCreator). Mirrors the
			// additive-logging contract from cycle C / F / G / I in the
			// auth package (RevokeRefreshToken / RotateRefreshToken /
			// RateLimiter / StoreRefreshToken).
			if setErr := h.rdb.Set(r.Context(), cacheKey, data, 5*time.Minute).Err(); setErr != nil {
				log.Printf("feed.GetFeed: cache set error: %v (key=%s)", setErr, cacheKey)
			}
		}
	} else {
		var err error
		resp, err = h.buildLatestFeed(r.Context(), limit, offset)
		if err != nil {
			log.Printf("feed.GetFeed: buildLatestFeed error: %v", err)
			writeError(w, http.StatusInternalServerError, "피드를 불러올 수 없습니다")
			return
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) buildLatestFeed(ctx context.Context, limit, offset int) (FeedResponse, error) {
	rows, err := h.q.ListPinsWithCreator(ctx, db.ListPinsWithCreatorParams{
		Column1: "",
		Column2: nil,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return FeedResponse{}, err
	}

	pins := make([]PinResponse, 0, len(rows))
	for _, row := range rows {
		pins = append(pins, listRowToPinResponse(row))
	}

	return FeedResponse{
		Pins:       pins,
		NextCursor: buildNextCursor(offset, limit, len(pins)),
	}, nil
}

// buildExcludedLatestFeed serves a latest-only page for a personalized caller
// whose recommendation source is empty at this offset. It draws from the
// recommendation-excluded pool (the same pool buildPersonalizedFeed's mixed
// branch uses for its latest portion) so pages stay disjoint across the
// personalized→latest transition. With empty tagIDs and mediaTypes the
// exclusion predicate is a no-op, degrading to plain latest.
func (h *Handler) buildExcludedLatestFeed(ctx context.Context, creatorID uuid.UUID, tagIDs []uuid.UUID, mediaTypes []string, limit, offset int) (FeedResponse, error) {
	rows, err := h.q.ListLatestPinsExcludingRecommended(ctx, db.ListLatestPinsExcludingRecommendedParams{
		CreatorID: creatorID,
		Column2:   tagIDs,
		Column3:   mediaTypes,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return FeedResponse{}, fmt.Errorf("ListLatestPinsExcludingRecommended (no-rec fallback): %w", err)
	}

	pins := make([]PinResponse, 0, len(rows))
	for _, row := range rows {
		pins = append(pins, listRowToPinResponse(db.ListPinsWithCreatorRow(row)))
	}

	return FeedResponse{
		Pins:       pins,
		NextCursor: buildNextCursor(offset, limit, len(pins)),
	}, nil
}

// buildPersonalizedFeed assembles a personalized feed page for an authenticated
// caller whose pin count crosses the cold-start threshold. The `offset` argument
// is the page position decoded from the cursor; it is propagated to every
// underlying source so consecutive pages return disjoint pin id sets.
//
// spec: feed `개인화 피드의 페이지네이션은 페이지 간 작품 중복을 반환하지 않는다` —
// "cursor가 표현하는 페이지 위치(offset)는 응답을 만들 때 사용되는 모든 underlying
// 쿼리에 일관되게 전파되어야 한다"
func (h *Handler) buildPersonalizedFeed(ctx context.Context, creatorID uuid.UUID, limit, offset int) (FeedResponse, error) {
	// Get user's top tag IDs
	tagRows, err := h.q.GetUserTagFrequency(ctx, db.GetUserTagFrequencyParams{
		CreatorID: creatorID,
		Limit:     10,
	})
	if err != nil {
		return FeedResponse{}, fmt.Errorf("GetUserTagFrequency: %w", err)
	}

	tagIDs := make([]uuid.UUID, 0, len(tagRows))
	for _, row := range tagRows {
		tagIDs = append(tagIDs, row.TagID)
	}

	// Resolve the user's top media types up front. Together with tagIDs they
	// define the recommendation population that the latest ("보충") source must
	// exclude on EVERY page — otherwise a pin matching the user's tags/media
	// appears in both the recommended source and the latest source, which order
	// it differently, so the same pin surfaces in different pages and violates
	// the feed `페이지 간 작품 중복을 반환하지 않는다` SHALL. mtRows is reused by the
	// fallback below so GetUserMediaTypeFrequency runs once. Fail-open: a DB
	// error degrades the exclusion to tag-only (still correct for the tag
	// population) but stays operator-visible — mirroring the additive-logging
	// contract from this file's cache branches and the auth silent-error series.
	var mediaTypes []string
	mtRows, mtErr := h.q.GetUserMediaTypeFrequency(ctx, db.GetUserMediaTypeFrequencyParams{
		CreatorID: creatorID,
		Limit:     3,
	})
	if mtErr != nil {
		log.Printf("feed.buildPersonalizedFeed: GetUserMediaTypeFrequency error: %v (user=%s)", mtErr, creatorID)
	} else {
		for _, mr := range mtRows {
			mediaTypes = append(mediaTypes, mr.MediaType)
		}
	}

	recLimit := (limit + 1) / 2
	var recRows []db.RecommendByTagsRow

	if len(tagIDs) > 0 {
		recRows, err = h.q.RecommendByTags(ctx, db.RecommendByTagsParams{
			Column1:   tagIDs,
			CreatorID: creatorID,
			Limit:     int32(recLimit),
			Offset:    int32(offset),
		})
		if err != nil {
			return FeedResponse{}, fmt.Errorf("RecommendByTags: %w", err)
		}
	}

	// Fallback: if tags produced insufficient results, supplement with
	// media-type-based recommendations using the media types resolved above.
	if len(recRows) < recLimit && len(mediaTypes) > 0 {
		deficit := int32(recLimit - len(recRows))
		mtRecs, err := h.q.RecommendByMediaType(ctx, db.RecommendByMediaTypeParams{
			Column1:   mediaTypes,
			CreatorID: creatorID,
			Limit:     deficit,
			Offset:    int32(offset),
		})
		if err != nil {
			log.Printf("feed.buildPersonalizedFeed: RecommendByMediaType error: %v (user=%s)", err, creatorID)
		} else {
			for _, mr := range mtRecs {
				recRows = append(recRows, db.RecommendByTagsRow(mr))
			}
		}
	}

	// If still nothing, fall back to latest — but for a personalized caller the
	// fallback MUST draw from the SAME recommendation-excluded pool that the
	// mixed branch below uses. A later page often depletes the recommendation
	// source (its offset runs past the recommendation population) and lands
	// here; using the unfiltered buildLatestFeed would reintroduce pins that
	// the EARLIER pages already served from their excluded-latest portion (the
	// two pools are positioned differently, so the same pin resurfaces) —
	// violating the `페이지 간 작품 중복을 반환하지 않는다` SHALL. The unfiltered
	// buildLatestFeed stays the right choice only for the cold-start / unauth
	// branches in GetFeed, which carry no recommendation population.
	if len(recRows) == 0 {
		return h.buildExcludedLatestFeed(ctx, creatorID, tagIDs, mediaTypes, limit, offset)
	}

	latestLimit := limit / 2
	if latestLimit < 1 {
		latestLimit = 1
	}
	latestRows, err := h.q.ListLatestPinsExcludingRecommended(ctx, db.ListLatestPinsExcludingRecommendedParams{
		CreatorID: creatorID,
		Column2:   tagIDs,
		Column3:   mediaTypes,
		Limit:     int32(latestLimit),
		Offset:    int32(offset),
	})
	if err != nil {
		return FeedResponse{}, fmt.Errorf("ListLatestPinsExcludingRecommended: %w", err)
	}

	recommended := make([]PinResponse, 0, len(recRows))
	for _, row := range recRows {
		recommended = append(recommended, recRowToPinResponse(row))
	}

	latest := make([]PinResponse, 0, len(latestRows))
	for _, row := range latestRows {
		latest = append(latest, listRowToPinResponse(db.ListPinsWithCreatorRow(row)))
	}

	pins := interleave(recommended, latest, limit)

	if len(pins) < limit {
		deficit := limit - len(pins)
		extraRows, err := h.q.ListLatestPinsExcludingRecommended(ctx, db.ListLatestPinsExcludingRecommendedParams{
			CreatorID: creatorID,
			Column2:   tagIDs,
			Column3:   mediaTypes,
			Limit:     int32(deficit),
			Offset:    int32(offset + len(latestRows)),
		})
		if err != nil {
			return FeedResponse{}, fmt.Errorf("ListLatestPinsExcludingRecommended (fill): %w", err)
		}
		for _, row := range extraRows {
			pins = append(pins, listRowToPinResponse(db.ListPinsWithCreatorRow(row)))
		}
	}

	return FeedResponse{
		Pins:       pins,
		NextCursor: buildNextCursor(offset, limit, len(pins)),
	}, nil
}

func interleave(recommended, latest []PinResponse, limit int) []PinResponse {
	result := make([]PinResponse, 0, limit)
	ri, li := 0, 0
	for len(result) < limit && (ri < len(recommended) || li < len(latest)) {
		if ri < len(recommended) {
			result = append(result, recommended[ri])
			ri++
			if len(result) >= limit {
				break
			}
		}
		if li < len(latest) {
			result = append(result, latest[li])
			li++
		}
	}
	return result
}

// maxFeedOffset caps the offset value decoded from the `cursor` query
// parameter on GET /api/feed. Sister-handler convention from
// pin/handler.go:568 (`o > 0 && o <= 100000`), search/handler.go (cycle 105
// PR #295, `maxSearchOffset`), and boards/handler.go (cycle 106 PR #301,
// `maxBoardsOffset`). buildNextCursor only emits well-formed cursors in
// the normal pagination range, but the cursor is base64 (not signed) and
// reaches us straight from the client — a hand-crafted
// base64("offset:999999999") is indistinguishable from a legitimate
// next_cursor at decode time. The endpoint is unauthenticated
// (cmd/server/main.go OptionalJWTMiddleware), and both
// ListPinsWithCreator and RecommendByTags / RecommendByMediaType are
// OFFSET-based — Postgres must sort-and-skip the entire OFFSET prefix
// before LIMIT applies, and the personalized branch fans out the same
// offset across multiple queries (multiplying the amplification). Silent
// clamp (out-of-range → offset=0) matches the sister contracts and
// preserves the existing parsePagination behavior for malformed cursors
// (silent fallback to offset=0) — a 400 here would be a stricter behavior
// change than the cycle 105/106 sisters chose.
const maxFeedOffset = 100000

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		if l > 0 && l <= 50 {
			limit = l
		} else if l > 50 {
			limit = 50
		}
	}
	offset = 0
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		decoded, err := base64.URLEncoding.DecodeString(cursor)
		if err == nil {
			var o int
			if _, err := fmt.Sscanf(string(decoded), "offset:%d", &o); err == nil && o > 0 && o <= maxFeedOffset {
				offset = o
			}
		}
	}
	return limit, offset
}

func buildNextCursor(currentOffset, limit, returnedCount int) *string {
	if returnedCount < limit {
		return nil
	}
	nextOffset := currentOffset + returnedCount
	raw := fmt.Sprintf("offset:%d", nextOffset)
	encoded := base64.URLEncoding.EncodeToString([]byte(raw))
	return &encoded
}

func feedCacheKey(userID uuid.UUID, limit, offset int) string {
	return fmt.Sprintf("feed:%s:%d:%d", userID.String(), limit, offset)
}

func listRowToPinResponse(row db.ListPinsWithCreatorRow) PinResponse {
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
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func recRowToPinResponse(row db.RecommendByTagsRow) PinResponse {
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
		CreatedAt:   row.CreatedAt,
		Creator: CreatorSummary{
			ID:        row.CreatorIDRef.String(),
			Nickname:  row.CreatorNickname,
			AvatarURL: avatarURL,
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("feed: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
