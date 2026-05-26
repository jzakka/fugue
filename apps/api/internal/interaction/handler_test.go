package interaction

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
)

// TestCreate_BodyTooLarge verifies http.MaxBytesReader pre-empts the JSON
// decoder before any large body is buffered. The body is valid-prefix
// JSON — `{"pin_id":"<padding>"...` where padding pushes the total past
// createRequestBodyCap — so the decoder is forced to read past the cap
// (an all-garbage body would fail syntax-first and never exercise the
// cap). Handler is reachable with a nil database because the body cap
// rejects before the decoder completes — db.New(h.database) is never
// invoked.
func TestCreate_BodyTooLarge(t *testing.T) {
	h := &Handler{}
	prefix := []byte(`{"pin_id":"`)
	suffix := []byte(`","type":"view"}`)
	padding := bytes.Repeat([]byte("a"), createRequestBodyCap+1-len(prefix)-len(suffix))
	body := append(append(append([]byte{}, prefix...), padding...), suffix...)
	if len(body) <= createRequestBodyCap {
		t.Fatalf("test setup: body must exceed cap (%d), got %d", createRequestBodyCap, len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/api/interactions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for body %d bytes (cap=%d), got %d; body=%s",
			len(body), createRequestBodyCap, rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "본문") {
		t.Errorf("error message should reference body size, got %q", resp["error"])
	}
}

// TestCreate_BodyAtCap verifies that a body within the cap passes
// MaxBytesReader and reaches the existing validation. We use an invalid
// `type` ("like" is not in the whitelist) so the request fails the
// post-decode whitelist check — proving the body cap did not pre-empt
// validation. Handler is reachable with a nil database because the
// whitelist check returns before db.New(h.database) is called.
func TestCreate_BodyAtCap(t *testing.T) {
	h := &Handler{}
	body := []byte(`{"pin_id":"` + uuid.New().String() + `","type":"like"}`)
	if len(body) > createRequestBodyCap {
		t.Fatalf("test setup: body must fit cap (%d), got %d", createRequestBodyCap, len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/api/interactions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from type whitelist, got %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "인터랙션 타입") {
		t.Errorf("body cap let request through but did not reach type-whitelist branch; got %q",
			resp["error"])
	}
}
