package feed

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Pins the offset cap embedded in parsePagination's cursor decode path.
// Sister-handler convention from pin/handler.go:568 (`o > 0 && o <= 100000`),
// search/handler.go (cycle 105 PR #295, `maxSearchOffset`), and
// boards/handler.go (cycle 106 PR #301, `maxBoardsOffset`).
//
// Unlike the boards sister (cycle 106) whose offset parse is unreachable
// without a working DB, feed's cap lives inside the pure helper
// `parsePagination(r *http.Request)`. Testing it directly is sufficient
// because the helper is the single funnel for cursor→offset translation
// (GetFeed at handler.go:72 is the only caller and forwards the value
// verbatim to every querier branch).

func cursorFor(offset int) string {
	raw := fmt.Sprintf("offset:%d", offset)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

func TestParsePagination_OffsetSilentlyClampedAboveCap(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/feed?cursor="+cursorFor(maxFeedOffset+1), nil)
	_, offset := parsePagination(req)
	if offset != 0 {
		t.Fatalf("cursor offset=%d (above cap=%d) must silently clamp to 0; got %d",
			maxFeedOffset+1, maxFeedOffset, offset)
	}
}

func TestParsePagination_OffsetForgedHugeValueClamped(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/feed?cursor="+cursorFor(999999999), nil)
	_, offset := parsePagination(req)
	if offset != 0 {
		t.Fatalf("forged cursor offset=999999999 must silently clamp to 0; got %d", offset)
	}
}

func TestParsePagination_OffsetAtCapPassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/feed?cursor="+cursorFor(maxFeedOffset), nil)
	_, offset := parsePagination(req)
	if offset != maxFeedOffset {
		t.Fatalf("cursor offset=%d (= cap inclusive) must pass through; got %d",
			maxFeedOffset, offset)
	}
}

func TestParsePagination_OffsetNormalRangePassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/feed?cursor="+cursorFor(40), nil)
	_, offset := parsePagination(req)
	if offset != 40 {
		t.Fatalf("normal cursor offset=40 must pass through; got %d", offset)
	}
}

func TestParsePagination_NoCursorReturnsZero(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
	_, offset := parsePagination(req)
	if offset != 0 {
		t.Fatalf("missing cursor must yield offset=0 (regression guard); got %d", offset)
	}
}

func TestParsePagination_MalformedCursorReturnsZero(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/feed?cursor=not-base64-!!!", nil)
	_, offset := parsePagination(req)
	if offset != 0 {
		t.Fatalf("malformed base64 cursor must silently fall back to offset=0 (regression guard); got %d", offset)
	}
}

func TestMaxFeedOffset_ValueMatchesSisterConvention(t *testing.T) {
	const wantSisterCap = 100000 // pin/handler.go:568, search.maxSearchOffset, boards.maxBoardsOffset
	if maxFeedOffset != wantSisterCap {
		t.Fatalf("maxFeedOffset drifted from sister cap: got %d, want %d", maxFeedOffset, wantSisterCap)
	}
}
