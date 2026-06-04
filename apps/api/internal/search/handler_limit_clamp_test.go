package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Pins the `limit` clamp on GET /api/search. The clamp must operate on the
// parsed int value BEFORE narrowing to int32: a limit beyond int32 max (e.g.
// 2147483648) would otherwise wrap to a negative int32, bypass the `> 50`
// clamp, and forward a negative LIMIT to Postgres ("LIMIT must not be
// negative"). This file verifies the clamp via a mock querier that captures
// arg.Limit from the pin search query.

// limitCapturingQuerier records the Limit passed to SearchPinsByILIKE /
// SearchPinsBySimilarity on the FIRST call only. type=pins requests don't fan
// out top_tags (that runs only for type=all), so the first call carries the
// user limit. All other methods return nil so search.Search proceeds without
// I/O side effects.
type limitCapturingQuerier struct {
	captured       bool
	firstPinsLimit int32
}

func (q *limitCapturingQuerier) SearchPinsByILIKE(ctx context.Context, arg db.SearchPinsByILIKEParams) ([]db.SearchPinsByILIKERow, error) {
	if !q.captured {
		q.firstPinsLimit = arg.Limit
		q.captured = true
	}
	return nil, nil
}

func (q *limitCapturingQuerier) SearchPinsBySimilarity(ctx context.Context, arg db.SearchPinsBySimilarityParams) ([]db.SearchPinsBySimilarityRow, error) {
	if !q.captured {
		q.firstPinsLimit = arg.Limit
		q.captured = true
	}
	return nil, nil
}

func (q *limitCapturingQuerier) SearchPinsWithTagFilter(ctx context.Context, arg db.SearchPinsWithTagFilterParams) ([]db.SearchPinsWithTagFilterRow, error) {
	return nil, nil
}

func (q *limitCapturingQuerier) SearchPinsILIKEWithTagFilter(ctx context.Context, arg db.SearchPinsILIKEWithTagFilterParams) ([]db.SearchPinsILIKEWithTagFilterRow, error) {
	return nil, nil
}

func (q *limitCapturingQuerier) SearchCreatorsBySimilarity(ctx context.Context, arg db.SearchCreatorsBySimilarityParams) ([]db.SearchCreatorsBySimilarityRow, error) {
	return nil, nil
}

func (q *limitCapturingQuerier) SearchCreatorsByILIKE(ctx context.Context, arg db.SearchCreatorsByILIKEParams) ([]db.SearchCreatorsByILIKERow, error) {
	return nil, nil
}

func (q *limitCapturingQuerier) SearchBoardsBySimilarity(ctx context.Context, arg db.SearchBoardsBySimilarityParams) ([]db.SearchBoardsBySimilarityRow, error) {
	return nil, nil
}

func (q *limitCapturingQuerier) SearchBoardsByILIKE(ctx context.Context, arg db.SearchBoardsByILIKEParams) ([]db.SearchBoardsByILIKERow, error) {
	return nil, nil
}

func (q *limitCapturingQuerier) SearchTopTags(ctx context.Context, dollar_1 []uuid.UUID) ([]db.SearchTopTagsRow, error) {
	return nil, nil
}

// TestSearch_LimitOverflowClampedNotNegative is the regression guard: a limit
// exceeding int32 max must clamp to 50, never wrap to a negative int32 that
// would reach the SQL LIMIT.
func TestSearch_LimitOverflowClampedNotNegative(t *testing.T) {
	q := &limitCapturingQuerier{}
	h := NewHandlerWithQuerier(q)

	// 2147483648 = int32 max + 1; a cast-before-clamp would wrap to
	// -2147483648 and slip past `> 50`.
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&limit=2147483648", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("overflow limit must clamp (not 4xx/5xx); got %d body=%s", rec.Code, rec.Body.String())
	}
	if q.firstPinsLimit != 50 {
		t.Fatalf("limit=2147483648 must clamp to 50, got %d", q.firstPinsLimit)
	}
}

// TestSearch_LimitAboveCapClamped verifies an ordinary above-cap value (no
// int32 overflow) still clamps to 50.
func TestSearch_LimitAboveCapClamped(t *testing.T) {
	q := &limitCapturingQuerier{}
	h := NewHandlerWithQuerier(q)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&limit=1000000", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("above-cap limit must clamp; got %d body=%s", rec.Code, rec.Body.String())
	}
	if q.firstPinsLimit != 50 {
		t.Fatalf("limit=1000000 must clamp to 50, got %d", q.firstPinsLimit)
	}
}

// TestSearch_LimitNormalRangePassesThrough is a happy-path regression that
// confirms ordinary limit values are forwarded unchanged.
func TestSearch_LimitNormalRangePassesThrough(t *testing.T) {
	q := &limitCapturingQuerier{}
	h := NewHandlerWithQuerier(q)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=ab&type=pins&limit=10", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("normal limit must pass; got %d body=%s", rec.Code, rec.Body.String())
	}
	if q.firstPinsLimit != 10 {
		t.Fatalf("limit=10 must pass through, got %d", q.firstPinsLimit)
	}
}

// TestSearch_LimitDefaultsWhenAbsentOrInvalid verifies the default (20) holds
// for missing, zero, negative, and non-numeric limit inputs.
func TestSearch_LimitDefaultsWhenAbsentOrInvalid(t *testing.T) {
	cases := []string{
		"/api/search?q=ab&type=pins",
		"/api/search?q=ab&type=pins&limit=0",
		"/api/search?q=ab&type=pins&limit=-5",
		"/api/search?q=ab&type=pins&limit=abc",
	}
	for _, url := range cases {
		q := &limitCapturingQuerier{}
		h := NewHandlerWithQuerier(q)

		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()

		h.Search(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d body=%s", url, rec.Code, rec.Body.String())
		}
		if q.firstPinsLimit != 20 {
			t.Fatalf("%s: expected default limit 20, got %d", url, q.firstPinsLimit)
		}
	}
}
