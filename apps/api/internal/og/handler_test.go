package og

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFetch_BodyTooLarge verifies http.MaxBytesReader pre-empts the JSON
// decoder before any large body is buffered. The body is valid-prefix JSON
// — `{"url":"<padding>"}` where padding pushes the total past
// ogRequestBodyCap — so the decoder is forced to read past the cap (an
// all-garbage body would fail syntax-first and never exercise the cap).
// Handler is reachable with a nil svc because the body cap rejects before
// the decoder completes — svc.Fetch is never invoked.
func TestFetch_BodyTooLarge(t *testing.T) {
	h := &Handler{}
	prefix := []byte(`{"url":"`)
	suffix := []byte(`"}`)
	padding := bytes.Repeat([]byte("a"), ogRequestBodyCap+1-len(prefix)-len(suffix))
	body := append(append(append([]byte{}, prefix...), padding...), suffix...)
	if len(body) <= ogRequestBodyCap {
		t.Fatalf("test setup: body must exceed cap (%d), got %d", ogRequestBodyCap, len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/api/og/fetch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Fetch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for body %d bytes (cap=%d), got %d; body=%s",
			len(body), ogRequestBodyCap, rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "본문") {
		t.Errorf("error message should reference body size, got %q", resp["error"])
	}
}

// TestFetch_URLTooLong_ASCII verifies the URL rune cap pre-empts further
// processing before service.Fetch or http.NewRequest is touched. 501 ASCII
// runes (cap=500) must produce 400. Handler is reachable with a nil svc
// because the rune-length guard returns before dispatching to h.svc.Fetch.
func TestFetch_URLTooLong_ASCII(t *testing.T) {
	h := &Handler{}
	longURL := "https://example.com/" + strings.Repeat("a", maxOGURLRunes-19)
	if r := []rune(longURL); len(r) != maxOGURLRunes+1 {
		t.Fatalf("test setup: longURL must be %d runes, got %d", maxOGURLRunes+1, len(r))
	}
	payload, _ := json.Marshal(map[string]string{"url": longURL})
	req := httptest.NewRequest(http.MethodPost, "/api/og/fetch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Fetch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 501-rune ASCII URL (cap=500), got %d; body=%s",
			rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "500") {
		t.Errorf("error message should reference the 500 cap, got %q", resp["error"])
	}
}

// TestFetch_URLTooLong_Korean verifies the cap is rune-based, not byte-based.
// 501 Korean runes is 1503 UTF-8 bytes — a byte-counted check would reject
// it at the wrong threshold. The rune cap must reject it.
// The Korean payload also stays under ogRequestBodyCap so the URL guard is
// what triggers (1503 bytes URL + JSON envelope ≈ 1.5KB << 8KB body cap).
func TestFetch_URLTooLong_Korean(t *testing.T) {
	h := &Handler{}
	// Build a URL with a Korean fragment that pushes total runes to 501.
	prefix := "https://example.com/"
	koreanCount := maxOGURLRunes + 1 - len([]rune(prefix))
	longURL := prefix + strings.Repeat("가", koreanCount)
	if r := []rune(longURL); len(r) != maxOGURLRunes+1 {
		t.Fatalf("test setup: longURL must be %d runes, got %d", maxOGURLRunes+1, len(r))
	}
	payload, _ := json.Marshal(map[string]string{"url": longURL})
	req := httptest.NewRequest(http.MethodPost, "/api/og/fetch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Fetch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 501-rune Korean URL, got %d; body=%s",
			rec.Code, rec.Body.String())
	}
}

// TestFetch_URLAtMaxLength_Boundary verifies 500 runes is the inclusive
// upper bound — exactly cap-length input must pass the length guard and
// reach svc.Fetch. Since this handler uses a nil svc, the dispatch will
// panic, which is the expected signal that the guard let the request
// through. A 400 here would mean the cap is off-by-one.
func TestFetch_URLAtMaxLength_Boundary(t *testing.T) {
	h := &Handler{}
	prefix := "https://example.com/"
	asciiCount := maxOGURLRunes - len([]rune(prefix))
	url := prefix + strings.Repeat("a", asciiCount)
	if r := []rune(url); len(r) != maxOGURLRunes {
		t.Fatalf("test setup: url must be %d runes, got %d", maxOGURLRunes, len(r))
	}
	payload, _ := json.Marshal(map[string]string{"url": url})
	req := httptest.NewRequest(http.MethodPost, "/api/og/fetch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		// Nil svc dispatch panics — that proves the length guard let the
		// request through. A 400 here would mean the cap is off-by-one.
		if r := recover(); r == nil && rec.Code == http.StatusBadRequest {
			t.Fatalf("500-rune URL at boundary was rejected as 400; guard is off-by-one. body=%s",
				rec.Body.String())
		}
	}()
	h.Fetch(rec, req)
}
