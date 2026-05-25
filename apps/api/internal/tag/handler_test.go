package tag

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestList_QueryTooLong_ASCII verifies the q-length cap pre-empts SearchTags
// before any DB access. 201-rune ASCII input must produce 400 with no DB
// query attempted (handler is reachable here with a nil *db.Queries because
// the length guard returns before dispatching to h.q.SearchTags).
func TestList_QueryTooLong_ASCII(t *testing.T) {
	h := &Handler{} // nil queries: guard must trigger before any DB call
	q := strings.Repeat("x", 201)
	req := httptest.NewRequest(http.MethodGet, "/api/tags?q="+url.QueryEscape(q), nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 201-rune ASCII q (cap=200), got %d; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if !strings.Contains(body["error"], "200") {
		t.Errorf("error message should reference the 200 cap, got %q", body["error"])
	}
}

// TestList_QueryTooLong_Korean verifies the cap is rune-based, not byte-based.
// 201 Korean runes is 603 UTF-8 bytes — a byte-counted check would let this
// pass (or trigger at the wrong threshold). The rune cap must reject it.
func TestList_QueryTooLong_Korean(t *testing.T) {
	h := &Handler{}
	q := strings.Repeat("가", 201)
	req := httptest.NewRequest(http.MethodGet, "/api/tags?q="+url.QueryEscape(q), nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 201-rune Korean q, got %d; body=%s", rec.Code, rec.Body.String())
	}
}

// TestList_QueryAtMaxLength_KoreanBoundary verifies 200 Korean runes is the
// inclusive upper bound — exactly cap-length input must pass the guard
// (length check returns false, dispatching to h.q.SearchTags). Since this
// handler uses a nil *db.Queries the dispatch will panic, which is the
// expected signal that the guard let the request through.
func TestList_QueryAtMaxLength_KoreanBoundary(t *testing.T) {
	h := &Handler{}
	q := strings.Repeat("가", 200)
	req := httptest.NewRequest(http.MethodGet, "/api/tags?q="+url.QueryEscape(q), nil)
	rec := httptest.NewRecorder()

	defer func() {
		// Nil *db.Queries dispatch panics — that proves the length guard let
		// the request through. A 400 here would mean the cap is off-by-one.
		if r := recover(); r == nil && rec.Code == http.StatusBadRequest {
			t.Fatalf("200-rune q at boundary was rejected as 400; guard is off-by-one. body=%s", rec.Body.String())
		}
	}()
	h.List(rec, req)
}
