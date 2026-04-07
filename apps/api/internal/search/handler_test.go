package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
