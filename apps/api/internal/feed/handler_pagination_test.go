package feed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Tests in this file cover:
//   feed `개인화 피드의 페이지네이션은 페이지 간 작품 중복을 반환하지 않는다`
//
// The personalized branch of GetFeed must propagate the cursor's page offset
// to every underlying query so consecutive pages return disjoint pin id sets.

type recordingQuerier struct {
	tagFreq []db.GetUserTagFrequencyRow
	mtFreq  []db.GetUserMediaTypeFrequencyRow

	// allRec / allLatest are the global ordered universes. The querier slices
	// them according to (offset, limit) on each call so different page offsets
	// produce different results.
	allRec    []db.RecommendByTagsRow
	allMT     []db.RecommendByMediaTypeRow
	allLatest []db.ListPinsWithCreatorRow

	pinCount int64

	tagCalls    []db.RecommendByTagsParams
	mtCalls     []db.RecommendByMediaTypeParams
	latestCalls []db.ListPinsWithCreatorParams
}

func (r *recordingQuerier) CountUserPins(_ context.Context, _ uuid.UUID) (int64, error) {
	return r.pinCount, nil
}

func (r *recordingQuerier) GetUserTagFrequency(_ context.Context, _ db.GetUserTagFrequencyParams) ([]db.GetUserTagFrequencyRow, error) {
	return r.tagFreq, nil
}

func (r *recordingQuerier) GetUserMediaTypeFrequency(_ context.Context, _ db.GetUserMediaTypeFrequencyParams) ([]db.GetUserMediaTypeFrequencyRow, error) {
	return r.mtFreq, nil
}

func (r *recordingQuerier) RecommendByTags(_ context.Context, p db.RecommendByTagsParams) ([]db.RecommendByTagsRow, error) {
	r.tagCalls = append(r.tagCalls, p)
	return sliceRows(r.allRec, int(p.Offset), int(p.Limit)), nil
}

func (r *recordingQuerier) RecommendByMediaType(_ context.Context, p db.RecommendByMediaTypeParams) ([]db.RecommendByMediaTypeRow, error) {
	r.mtCalls = append(r.mtCalls, p)
	return sliceRowsMT(r.allMT, int(p.Offset), int(p.Limit)), nil
}

func (r *recordingQuerier) ListPinsWithCreator(_ context.Context, p db.ListPinsWithCreatorParams) ([]db.ListPinsWithCreatorRow, error) {
	r.latestCalls = append(r.latestCalls, p)
	return sliceRowsLatest(r.allLatest, int(p.Offset), int(p.Limit)), nil
}

func sliceRows[T any](src []T, offset, limit int) []T {
	if offset >= len(src) {
		return nil
	}
	end := offset + limit
	if end > len(src) {
		end = len(src)
	}
	out := make([]T, end-offset)
	copy(out, src[offset:end])
	return out
}

func sliceRowsMT(src []db.RecommendByMediaTypeRow, offset, limit int) []db.RecommendByMediaTypeRow {
	return sliceRows(src, offset, limit)
}

func sliceRowsLatest(src []db.ListPinsWithCreatorRow, offset, limit int) []db.ListPinsWithCreatorRow {
	return sliceRows(src, offset, limit)
}

func makeRecRow(seed int) db.RecommendByTagsRow {
	id := uuid.New()
	creatorID := uuid.New()
	return db.RecommendByTagsRow{
		ID:              id,
		CreatorID:       creatorID,
		MediaUrl:        fmt.Sprintf("https://example.test/rec/%d", seed),
		MediaType:       "image",
		Title:           fmt.Sprintf("rec-%d", seed),
		CreatorIDRef:    creatorID,
		CreatorNickname: fmt.Sprintf("creator-%d", seed),
	}
}

func makeLatestRow(seed int) db.ListPinsWithCreatorRow {
	id := uuid.New()
	creatorID := uuid.New()
	return db.ListPinsWithCreatorRow{
		ID:              id,
		CreatorID:       creatorID,
		MediaUrl:        fmt.Sprintf("https://example.test/latest/%d", seed),
		MediaType:       "image",
		Title:           fmt.Sprintf("latest-%d", seed),
		CreatorIDRef:    creatorID,
		CreatorNickname: fmt.Sprintf("creator-%d", seed),
	}
}

func newTestHandler(t *testing.T, q FeedQuerier) (*Handler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewHandlerWithQuerier(q, rdb), mr
}

func authenticatedRequest(t *testing.T, target string, userID uuid.UUID) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = req.WithContext(auth.WithCreatorID(req.Context(), userID))
	return req
}

func decodeFeedResp(t *testing.T, body []byte) FeedResponse {
	t.Helper()
	var resp FeedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode feed response: %v\nbody=%s", err, body)
	}
	return resp
}

func pinIDSet(pins []PinResponse) map[string]struct{} {
	out := make(map[string]struct{}, len(pins))
	for _, p := range pins {
		out[p.ID] = struct{}{}
	}
	return out
}

// TestPersonalizedFeed_PagesAreDisjoint asserts the SHALL from
// `개인화 피드의 페이지네이션은 페이지 간 작품 중복을 반환하지 않는다`:
// the pin id set of page N must be disjoint from the pin id set of page N-1
// when following next_cursor.
func TestPersonalizedFeed_PagesAreDisjoint(t *testing.T) {
	const limit = 20
	allRec := make([]db.RecommendByTagsRow, 60)
	for i := range allRec {
		allRec[i] = makeRecRow(i)
	}
	allLatest := make([]db.ListPinsWithCreatorRow, 80)
	for i := range allLatest {
		allLatest[i] = makeLatestRow(i)
	}

	q := &recordingQuerier{
		pinCount: 15, // above the cold-start threshold of 10
		tagFreq: []db.GetUserTagFrequencyRow{
			{TagID: uuid.New(), Freq: 5},
		},
		allRec:    allRec,
		allLatest: allLatest,
	}

	h, _ := newTestHandler(t, q)

	userID := uuid.New()

	// Page 1: no cursor → offset 0
	req1 := authenticatedRequest(t, fmt.Sprintf("/api/feed?limit=%d", limit), userID)
	rec1 := httptest.NewRecorder()
	h.GetFeed(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("page 1 status: got %d, want 200; body=%s", rec1.Code, rec1.Body.String())
	}
	resp1 := decodeFeedResp(t, rec1.Body.Bytes())
	if resp1.NextCursor == nil {
		t.Fatalf("page 1: expected next_cursor to be non-nil (returned %d items)", len(resp1.Pins))
	}
	if len(resp1.Pins) != limit {
		t.Fatalf("page 1: expected %d pins, got %d", limit, len(resp1.Pins))
	}

	// Page 2: forward the cursor returned by page 1
	req2 := authenticatedRequest(t, fmt.Sprintf("/api/feed?limit=%d&cursor=%s", limit, *resp1.NextCursor), userID)
	rec2 := httptest.NewRecorder()
	h.GetFeed(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("page 2 status: got %d, want 200; body=%s", rec2.Code, rec2.Body.String())
	}
	resp2 := decodeFeedResp(t, rec2.Body.Bytes())

	page1IDs := pinIDSet(resp1.Pins)
	for _, p := range resp2.Pins {
		if _, dup := page1IDs[p.ID]; dup {
			t.Fatalf("page 2 contains pin id %s that already appeared on page 1; SHALL `이전 페이지에 포함되지 않은 작품` violated", p.ID)
		}
	}
}

// TestPersonalizedFeed_OffsetPropagatesToAllSources asserts the SHALL
// "cursor가 표현하는 페이지 위치(offset)는 응답을 만들 때 사용되는 모든
// underlying 쿼리에 일관되게 전파되어야 한다" by inspecting the offset values
// captured by the recording querier on page 2.
func TestPersonalizedFeed_OffsetPropagatesToAllSources(t *testing.T) {
	const limit = 20
	const pageOffset = limit

	allRec := make([]db.RecommendByTagsRow, 60)
	for i := range allRec {
		allRec[i] = makeRecRow(i)
	}
	allLatest := make([]db.ListPinsWithCreatorRow, 80)
	for i := range allLatest {
		allLatest[i] = makeLatestRow(i)
	}

	q := &recordingQuerier{
		pinCount: 15,
		tagFreq: []db.GetUserTagFrequencyRow{
			{TagID: uuid.New(), Freq: 5},
		},
		allRec:    allRec,
		allLatest: allLatest,
	}
	h, _ := newTestHandler(t, q)

	userID := uuid.New()
	cursor := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("offset:%d", pageOffset)))
	req := authenticatedRequest(t, fmt.Sprintf("/api/feed?limit=%d&cursor=%s", limit, cursor), userID)
	rec := httptest.NewRecorder()
	h.GetFeed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	if len(q.tagCalls) == 0 {
		t.Fatalf("RecommendByTags was never called")
	}
	for i, c := range q.tagCalls {
		if int(c.Offset) != pageOffset {
			t.Fatalf("RecommendByTags call %d: Offset=%d, want %d (page offset must propagate)", i, c.Offset, pageOffset)
		}
	}

	if len(q.latestCalls) == 0 {
		t.Fatalf("ListPinsWithCreator was never called")
	}
	// The first latest call (non-fill-gap) should receive the page offset.
	firstLatest := q.latestCalls[0]
	if int(firstLatest.Offset) != pageOffset {
		t.Fatalf("ListPinsWithCreator first call: Offset=%d, want %d (page offset must propagate to latest source)", firstLatest.Offset, pageOffset)
	}
	// If a fill-gap call happened, its offset must be page offset + latest consumed,
	// not 0 or len(latestRows).
	if len(q.latestCalls) > 1 {
		fill := q.latestCalls[1]
		if int(fill.Offset) < pageOffset {
			t.Fatalf("ListPinsWithCreator fill-gap call: Offset=%d, must be >= page offset %d", fill.Offset, pageOffset)
		}
	}
}

// TestPersonalizedFeed_FallbackToMediaTypeAlsoReceivesOffset ensures the
// media-type fallback path also propagates page offset.
func TestPersonalizedFeed_FallbackToMediaTypeAlsoReceivesOffset(t *testing.T) {
	const limit = 20
	const pageOffset = limit

	// Empty allRec → media-type fallback path is exercised. But emptyrec also
	// triggers the "fall back to latest" early return. To make the media-type
	// path execute we need recRows to be non-empty but smaller than recLimit.
	allRec := make([]db.RecommendByTagsRow, 3) // recLimit = (20+1)/2 = 10, so 3 < 10
	for i := range allRec {
		allRec[i] = makeRecRow(i)
	}
	allMT := make([]db.RecommendByMediaTypeRow, 40)
	for i := range allMT {
		row := db.RecommendByMediaTypeRow{
			ID:              uuid.New(),
			CreatorID:       uuid.New(),
			MediaUrl:        fmt.Sprintf("https://example.test/mt/%d", i),
			MediaType:       "image",
			Title:           fmt.Sprintf("mt-%d", i),
			CreatorIDRef:    uuid.New(),
			CreatorNickname: fmt.Sprintf("creator-mt-%d", i),
		}
		allMT[i] = row
	}
	allLatest := make([]db.ListPinsWithCreatorRow, 80)
	for i := range allLatest {
		allLatest[i] = makeLatestRow(i)
	}

	q := &recordingQuerier{
		pinCount: 15,
		tagFreq: []db.GetUserTagFrequencyRow{
			{TagID: uuid.New(), Freq: 5},
		},
		mtFreq: []db.GetUserMediaTypeFrequencyRow{
			{MediaType: "image", Freq: 7},
		},
		allRec:    allRec,
		allMT:     allMT,
		allLatest: allLatest,
	}
	h, _ := newTestHandler(t, q)

	userID := uuid.New()
	cursor := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("offset:%d", pageOffset)))
	req := authenticatedRequest(t, fmt.Sprintf("/api/feed?limit=%d&cursor=%s", limit, cursor), userID)
	rec := httptest.NewRecorder()
	h.GetFeed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	if len(q.mtCalls) == 0 {
		t.Fatalf("RecommendByMediaType was never called (path not exercised)")
	}
	for i, c := range q.mtCalls {
		if int(c.Offset) != pageOffset {
			t.Fatalf("RecommendByMediaType call %d: Offset=%d, want %d (page offset must propagate)", i, c.Offset, pageOffset)
		}
	}
}
