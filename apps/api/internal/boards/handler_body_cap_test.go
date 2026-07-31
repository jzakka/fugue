package boards_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	"github.com/chungsanghwa/fugue/apps/api/internal/boards"
)

// boardsRequestBodyCap (8 KB) is the value defined in boards/handler.go.
// Mirror it here so test setup mathematics is local-readable. If the cap
// value changes the test setup will mismatch and fail loudly.
const boardsBodyCap = 8 * 1024

// newCappedRequest builds an authenticated request body of size cap+1
// using valid-prefix JSON so json.NewDecoder is forced to read past the
// cap (an all-garbage body would fail syntax-first and never exercise
// the cap).
func newCappedRequest(t *testing.T, method, path, jsonPrefix, jsonSuffix string) (*http.Request, int) {
	t.Helper()
	prefix := []byte(jsonPrefix)
	suffix := []byte(jsonSuffix)
	padding := bytes.Repeat([]byte("a"), boardsBodyCap+1-len(prefix)-len(suffix))
	body := append(append(append([]byte{}, prefix...), padding...), suffix...)
	if len(body) <= boardsBodyCap {
		t.Fatalf("test setup: body must exceed cap (%d), got %d", boardsBodyCap, len(body))
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.WithCreatorID(req.Context(), uuid.New())
	return req.WithContext(ctx), len(body)
}

func decodeBodyCapError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v (raw=%q)", err, rec.Body.String())
	}
	return body["error"]
}

// TestCreate_BodyTooLarge verifies http.MaxBytesReader pre-empts the JSON
// decoder before any large body is buffered. Handler is reachable with a
// nil database because the body cap rejects before the decoder completes
// — no query method on the handler's querier is ever invoked.
func TestCreate_BodyTooLarge(t *testing.T) {
	h := boards.NewHandler(nil)
	req, n := newCappedRequest(t, http.MethodPost, "/api/boards", `{"name":"`, `"}`)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for body %d bytes (cap=%d), got %d; body=%s",
			n, boardsBodyCap, rec.Code, rec.Body.String())
	}
	if msg := decodeBodyCapError(t, rec); !strings.Contains(msg, "본문") {
		t.Errorf("error message should reference body size, got %q", msg)
	}
}

// TestUpdate_BodyTooLarge — Update's MaxBytesReader is installed AFTER the
// auth + URL parse + GetBoard ownership check, so we cannot easily reach
// it with a nil DB. Verify the body-cap branch is the first error
// encountered by routing through the real chi mux with a generated board
// id; the ownership check will fail before db access only if we keep the
// request body large enough that the body wrap fires during DECODE. Since
// boards.Update reads the existing board from DB before decode, this test
// is intentionally skipped at the unit-test layer — real-env QA in cycle
// 103 exercises Update body cap end-to-end via curl.
func TestUpdate_BodyTooLarge_DocumentedUnitGap(t *testing.T) {
	t.Skip("Update body cap reaches MaxBytesReader only after a successful GetBoard DB read; covered by real-env QA, not by nil-DB unit harness.")
}

// TestAddPin_BodyTooLarge — same gap as Update: body decode happens after
// boardID parse but BEFORE the ownership GetBoard call, so with nil DB
// the handler returns from MaxBytesReader path before touching DB. Wire
// the chi URL param so chi.URLParam(r, "id") returns a valid UUID.
func TestAddPin_BodyTooLarge(t *testing.T) {
	h := boards.NewHandler(nil)
	req, n := newCappedRequest(t, http.MethodPost, "/api/boards/00000000-0000-0000-0000-000000000000/pins", `{"pin_id":"`, `"}`)

	// Inject chi URL param `id` so chi.URLParam(r, "id") works without a router.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "00000000-0000-0000-0000-000000000000")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// Re-attach creator id (WithContext above replaced the context).
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))
	// Re-attach chi context after the auth context replace.
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.AddPin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for body %d bytes (cap=%d), got %d; body=%s",
			n, boardsBodyCap, rec.Code, rec.Body.String())
	}
	if msg := decodeBodyCapError(t, rec); !strings.Contains(msg, "본문") {
		t.Errorf("error message should reference body size, got %q", msg)
	}
}

// TestCreate_BodyAtCap verifies that a body within the cap passes
// MaxBytesReader and reaches the existing post-decode validation. Use an
// over-length `name` (101 runes) so the post-decode rune cap fires —
// proving the body cap did not pre-empt validation.
func TestCreate_BodyAtCap(t *testing.T) {
	h := boards.NewHandler(nil)

	body, err := json.Marshal(map[string]any{
		"name": strings.Repeat("A", 101),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(body) > boardsBodyCap {
		t.Fatalf("test setup: body must fit cap (%d), got %d", boardsBodyCap, len(body))
	}

	req := httptest.NewRequest(http.MethodPost, "/api/boards", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from name rune cap, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if msg := decodeBodyCapError(t, rec); !strings.Contains(msg, "보드 이름은 100자") {
		t.Errorf("body cap let request through but did not reach name-rune-cap branch; got %q", msg)
	}
}
