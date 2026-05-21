package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSearch_EmptyQuery(t *testing.T) {
	h := &Handler{} // q="" triggers early return before DB access
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] == "" {
		t.Error("expected error message")
	}
}

func TestSearch_WhitespaceQuery(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=+++", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for whitespace-only query, got %d", rec.Code)
	}
}

func TestSearch_InvalidType(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&type=invalid", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSearch_TooManyTags(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&tag_ids="+
		"00000000-0000-0000-0000-000000000001,"+
		"00000000-0000-0000-0000-000000000002,"+
		"00000000-0000-0000-0000-000000000003,"+
		"00000000-0000-0000-0000-000000000004,"+
		"00000000-0000-0000-0000-000000000005,"+
		"00000000-0000-0000-0000-000000000006", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for >5 tags, got %d", rec.Code)
	}
}

func TestSearch_InvalidTagID(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&tag_ids=not-a-uuid", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// Pins the input-length cap on the `q` parameter. The endpoint is
// unauthenticated and not rate-limited and the query feeds into pg_trgm
// `similarity($1)` + `ILIKE '%' || $1 || '%'` across pins+creators+boards.
// 200 runes mirrors `pins.title` VARCHAR(200) — see search/handler.go
// L65-75 (maxSearchQueryRunes doc comment).

func TestSearch_QueryTooLong_ASCII(t *testing.T) {
	h := &Handler{}
	q := strings.Repeat("x", 201)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape(q), nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 201-rune ASCII q (cap=200), got %d; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if !strings.Contains(body["error"], "200") {
		t.Errorf("error message should reference the 200 cap, got %q", body["error"])
	}
}

func TestSearch_QueryTooLong_Korean(t *testing.T) {
	// Verifies the cap is enforced on rune count, not byte count.
	// 201 Korean runes = 603 UTF-8 bytes. A byte-count cap would still
	// admit this, defeating the DoS guard for multi-byte input.
	h := &Handler{}
	q := strings.Repeat("가", 201)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape(q), nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 201-rune Korean q (rune-count cap, not byte-count), got %d; body=%s", rec.Code, rec.Body.String())
	}
}

func TestSearch_QueryAtMaxLength_KoreanBoundary(t *testing.T) {
	// Boundary check: exactly 200 Korean runes must NOT be rejected by the
	// length cap. The handler will proceed past the length check; absent a
	// db.Queries it will nil-panic — we recover and assert the panic point
	// to confirm the length guard let the request through.
	h := &Handler{}
	q := strings.Repeat("가", 200)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape(q), nil)
	rec := httptest.NewRecorder()

	defer func() {
		_ = recover() // expected: nil-deref past the length-cap branch
		if rec.Code == http.StatusBadRequest {
			t.Fatalf("200 Korean runes (= cap) must NOT trip the length-cap 400; got 400")
		}
	}()
	h.Search(rec, req)
}
