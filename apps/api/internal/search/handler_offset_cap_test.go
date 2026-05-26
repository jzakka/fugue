package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Pins the `offset` cap on GET /api/search. Sister-handler convention from
// pin/handler.go:568 (`o > 0 && o <= 100000`) is silent clamp — out-of-range
// offset is treated as offset=0 rather than 400. This file verifies the
// clamp via a mock querier that captures arg.Offset from SearchPinsByILIKE.

// offsetCapturingQuerier is a SearchQuerier mock that records the Offset
// passed to SearchPinsByILIKE / SearchPinsBySimilarity on the FIRST call only.
// search.Search may call searchPins twice (once for the user result, once for
// top_tags fan-out with offset=0) — capturing only the first call is enough
// to verify the user offset path. All other methods return nil so search.Search
// proceeds without I/O side effects.
type offsetCapturingQuerier struct {
	captured        bool
	firstPinsOffset int32
}

func (q *offsetCapturingQuerier) SearchPinsByILIKE(ctx context.Context, arg db.SearchPinsByILIKEParams) ([]db.SearchPinsByILIKERow, error) {
	if !q.captured {
		q.firstPinsOffset = arg.Offset
		q.captured = true
	}
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchPinsBySimilarity(ctx context.Context, arg db.SearchPinsBySimilarityParams) ([]db.SearchPinsBySimilarityRow, error) {
	if !q.captured {
		q.firstPinsOffset = arg.Offset
		q.captured = true
	}
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchPinsWithTagFilter(ctx context.Context, arg db.SearchPinsWithTagFilterParams) ([]db.SearchPinsWithTagFilterRow, error) {
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchPinsILIKEWithTagFilter(ctx context.Context, arg db.SearchPinsILIKEWithTagFilterParams) ([]db.SearchPinsILIKEWithTagFilterRow, error) {
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchCreatorsBySimilarity(ctx context.Context, arg db.SearchCreatorsBySimilarityParams) ([]db.SearchCreatorsBySimilarityRow, error) {
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchCreatorsByILIKE(ctx context.Context, arg db.SearchCreatorsByILIKEParams) ([]db.SearchCreatorsByILIKERow, error) {
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchBoardsBySimilarity(ctx context.Context, arg db.SearchBoardsBySimilarityParams) ([]db.SearchBoardsBySimilarityRow, error) {
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchBoardsByILIKE(ctx context.Context, arg db.SearchBoardsByILIKEParams) ([]db.SearchBoardsByILIKERow, error) {
	return nil, nil
}

func (q *offsetCapturingQuerier) SearchTopTags(ctx context.Context, dollar_1 []uuid.UUID) ([]db.SearchTopTagsRow, error) {
	return nil, nil
}

// TestSearch_OffsetSilentlyClampedAboveCap verifies that offset > maxSearchOffset
// is silently treated as offset=0 (sister pin/handler.go:568 contract). The mock
// captures arg.Offset from SearchPinsByILIKE — the only observable signal that
// the offset guard fired.
func TestSearch_OffsetSilentlyClampedAboveCap(t *testing.T) {
	q := &offsetCapturingQuerier{}
	h := NewHandlerWithQuerier(q)

	// type=pins + 2-rune q → useSimilarity=false → SearchPinsByILIKE is the
	// pin source (matches handler_top_tags_log_test.go scoping).
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&offset=100001", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("offset out-of-range must silently clamp (not 4xx); got %d body=%s", rec.Code, rec.Body.String())
	}
	if q.firstPinsOffset != 0 {
		t.Fatalf("offset=100001 (above cap=%d) must clamp to 0; got %d", maxSearchOffset, q.firstPinsOffset)
	}
}

// TestSearch_OffsetAtCapPassesThrough verifies the cap is inclusive — offset
// exactly equal to maxSearchOffset is forwarded unchanged.
func TestSearch_OffsetAtCapPassesThrough(t *testing.T) {
	q := &offsetCapturingQuerier{}
	h := NewHandlerWithQuerier(q)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&offset=100000", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("offset at cap must pass; got %d body=%s", rec.Code, rec.Body.String())
	}
	if q.firstPinsOffset != int32(maxSearchOffset) {
		t.Fatalf("offset=%d (= cap) must pass through; got %d", maxSearchOffset, q.firstPinsOffset)
	}
}

// TestSearch_OffsetNormalRangePassesThrough is a happy-path regression that
// confirms the cap addition didn't break ordinary offset values.
func TestSearch_OffsetNormalRangePassesThrough(t *testing.T) {
	q := &offsetCapturingQuerier{}
	h := NewHandlerWithQuerier(q)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&offset=40", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("normal offset must pass; got %d body=%s", rec.Code, rec.Body.String())
	}
	if q.firstPinsOffset != 40 {
		t.Fatalf("offset=40 must pass through; got %d", q.firstPinsOffset)
	}
}
