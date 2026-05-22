package search

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Pins the additive-logging contract for search.Search's top_tags enrichment
// branch. The branch is fail-open by design (the response shape stays stable
// via the outer `response["top_tags"] = []TopTag{}` fallback so the discovery
// UX degrades silently rather than 500-ing), but a SearchTopTags DB error
// MUST emit a timestamped operator log line so the silent degradation is
// visible before related-tag fan-out collapses across all queries.
//
// Mirrors the contract from feed/handler_mediatype_log_test.go (PR #216) and
// the auth package silent-error series (cycles C / F / G / I).

func captureSearchLog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	return &buf, func() { log.SetOutput(prev) }
}

// recordingSearchQuerier is a minimal SearchQuerier mock for the top_tags
// silent-error tests. It returns a single pin from searchPins so that the
// downstream top_tags branch (gated on `len(topPins) > 0`) actually executes,
// and lets the test inject an error or a row set for SearchTopTags.
type recordingSearchQuerier struct {
	pinID         uuid.UUID
	topTagsErr    error
	topTagsRows   []db.SearchTopTagsRow
	topTagsCalls  int
	topTagsPinIDs [][]uuid.UUID
}

func (r *recordingSearchQuerier) onePin() []db.SearchPinsByILIKERow {
	return []db.SearchPinsByILIKERow{{
		ID:               r.pinID,
		CreatorID:        uuid.New(),
		MediaUrl:         "https://example.com/x.jpg",
		MediaType:        "image",
		Url:              sql.NullString{},
		Title:            "x",
		Description:      sql.NullString{},
		OgImage:          sql.NullString{},
		CreatorIDRef:     uuid.New(),
		CreatorNickname:  "creator",
		CreatorAvatarUrl: sql.NullString{},
	}}
}

func (r *recordingSearchQuerier) SearchPinsByILIKE(ctx context.Context, arg db.SearchPinsByILIKEParams) ([]db.SearchPinsByILIKERow, error) {
	return r.onePin(), nil
}

func (r *recordingSearchQuerier) SearchPinsBySimilarity(ctx context.Context, arg db.SearchPinsBySimilarityParams) ([]db.SearchPinsBySimilarityRow, error) {
	return nil, nil
}

func (r *recordingSearchQuerier) SearchPinsWithTagFilter(ctx context.Context, arg db.SearchPinsWithTagFilterParams) ([]db.SearchPinsWithTagFilterRow, error) {
	return nil, nil
}

func (r *recordingSearchQuerier) SearchPinsILIKEWithTagFilter(ctx context.Context, arg db.SearchPinsILIKEWithTagFilterParams) ([]db.SearchPinsILIKEWithTagFilterRow, error) {
	return nil, nil
}

func (r *recordingSearchQuerier) SearchCreatorsBySimilarity(ctx context.Context, arg db.SearchCreatorsBySimilarityParams) ([]db.SearchCreatorsBySimilarityRow, error) {
	return nil, nil
}

func (r *recordingSearchQuerier) SearchCreatorsByILIKE(ctx context.Context, arg db.SearchCreatorsByILIKEParams) ([]db.SearchCreatorsByILIKERow, error) {
	return nil, nil
}

func (r *recordingSearchQuerier) SearchBoardsBySimilarity(ctx context.Context, arg db.SearchBoardsBySimilarityParams) ([]db.SearchBoardsBySimilarityRow, error) {
	return nil, nil
}

func (r *recordingSearchQuerier) SearchBoardsByILIKE(ctx context.Context, arg db.SearchBoardsByILIKEParams) ([]db.SearchBoardsByILIKERow, error) {
	return nil, nil
}

func (r *recordingSearchQuerier) SearchTopTags(ctx context.Context, dollar_1 []uuid.UUID) ([]db.SearchTopTagsRow, error) {
	r.topTagsCalls++
	r.topTagsPinIDs = append(r.topTagsPinIDs, dollar_1)
	if r.topTagsErr != nil {
		return nil, r.topTagsErr
	}
	return r.topTagsRows, nil
}

func TestSearch_LogsOnSearchTopTagsError(t *testing.T) {
	q := &recordingSearchQuerier{
		pinID:      uuid.New(),
		topTagsErr: errors.New("simulated SearchTopTags failure"),
	}
	h := NewHandlerWithQuerier(q)

	// type=pins so `skipTopTags` is false (it only short-circuits when
	// type=all && limit<=5), and `useSimilarity` is false (q="ab" has 2
	// runes, below the >2 threshold), so SearchPinsByILIKE is the chosen
	// pin source — matching the recordingSearchQuerier.
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&limit=20", nil)
	rec := httptest.NewRecorder()

	buf, restore := captureSearchLog(t)
	defer restore()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (SearchTopTags failure must remain fail-open); body=%s", rec.Code, rec.Body.String())
	}

	out := buf.String()
	if !strings.Contains(out, "search.SearchTopTags:") {
		t.Fatalf("SearchTopTags failure must emit operator log; got: %q", out)
	}
	if !strings.Contains(out, "q=\"ab\"") {
		t.Fatalf("log line must include the original query so the operator can scope the failure; got: %q", out)
	}
	if !strings.Contains(out, "pin_count=1") {
		t.Fatalf("log line must include the pin_count so the operator can gauge fan-out; got: %q", out)
	}

	// SearchTopTags MUST have been called exactly once with the single pin
	// id from the upstream search result.
	if q.topTagsCalls != 1 {
		t.Fatalf("SearchTopTags must be called exactly once; got %d calls", q.topTagsCalls)
	}

	// Response shape must still include the fallback empty list — the
	// silent-discard behavior is preserved end-to-end; only the operator
	// log is additive.
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	tags, ok := body["top_tags"].([]any)
	if !ok {
		t.Fatalf("response must include top_tags array; got %T (%v)", body["top_tags"], body["top_tags"])
	}
	if len(tags) != 0 {
		t.Fatalf("top_tags must fall back to empty list on SearchTopTags error; got %d entries", len(tags))
	}
}

func TestSearch_NoLogOnHealthyTopTags(t *testing.T) {
	// Happy-path regression: when SearchTopTags succeeds the handler MUST
	// NOT emit any `search.SearchTopTags:` lines. Mirrors the cache-success
	// no-log contract from feed/handler_cache_log_test.go and the
	// media-type fallback no-log test in handler_mediatype_log_test.go.
	q := &recordingSearchQuerier{
		pinID: uuid.New(),
		topTagsRows: []db.SearchTopTagsRow{{
			ID: uuid.New(), Name: "tag1", Slug: "tag1", Category: "general", Count: 3,
		}},
	}
	h := NewHandlerWithQuerier(q)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&limit=20", nil)
	rec := httptest.NewRecorder()

	buf, restore := captureSearchLog(t)
	defer restore()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if out := buf.String(); strings.Contains(out, "search.SearchTopTags:") {
		t.Fatalf("happy path must not emit search.SearchTopTags log lines, got: %q", out)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	tags, ok := body["top_tags"].([]any)
	if !ok {
		t.Fatalf("response must include top_tags array; got %T", body["top_tags"])
	}
	if len(tags) != 1 {
		t.Fatalf("top_tags must contain the row returned by SearchTopTags; got %d entries", len(tags))
	}
}
