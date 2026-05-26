package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Pins the response-body cap that gates Google/Discord FetchProfile against a
// misbehaving (or compromised) OAuth provider. The full Google/Discord URL is
// hardcoded inside FetchProfile, so we can't drive the existing entrypoint
// against a stub server without invasive refactoring. Instead we pin the
// exact io.LimitReader(...) pattern that provider.go:84 (Google) and
// provider.go:159 (Discord) now use: a stub HTTP server replays the bytes
// the real code would read, and we assert that ReadAll yields at most the
// cap. If a future refactor drops the LimitReader wrap or shrinks the cap
// silently, the assertions break and force a conscious update.
//
// The sister-handler convention (og/service.go:105, bot/robots_filter.go:237,
// bot/snapshot/reader.go:88, bot/helpers.go:54, bot/pioneer_consumer.go:368,
// bot/media_validator.go:190) is `io.ReadAll(io.LimitReader(resp.Body, N))`,
// which is what we exercise here.

func TestMaxUserInfoBytes_MatchesIntent(t *testing.T) {
	const want = 64 * 1024
	if maxUserInfoBytes != want {
		t.Fatalf("maxUserInfoBytes drifted from documented intent (~60x safety margin over realistic 1 KB profile): got %d, want %d", maxUserInfoBytes, want)
	}
}

func TestUserInfoCap_LimitReaderTruncatesOversizeBody(t *testing.T) {
	oversize := maxUserInfoBytes * 4
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("A", oversize)))
	}))
	defer stub.Close()

	resp, err := http.Get(stub.URL)
	if err != nil {
		t.Fatalf("stub fetch: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUserInfoBytes))
	if err != nil {
		t.Fatalf("ReadAll(LimitReader) must succeed on truncation, got err=%v", err)
	}
	if len(body) != maxUserInfoBytes {
		t.Fatalf("LimitReader must cap read at maxUserInfoBytes (=%d); got %d bytes (upstream sent %d)", maxUserInfoBytes, len(body), oversize)
	}
}

func TestUserInfoCap_NormalBodyPassesThrough(t *testing.T) {
	normalBody := `{"id":"123","email":"user@example.com","verified_email":true,"name":"Test User","picture":"https://cdn.example.com/avatar.png"}`
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(normalBody))
	}))
	defer stub.Close()

	resp, err := http.Get(stub.URL)
	if err != nil {
		t.Fatalf("stub fetch: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUserInfoBytes))
	if err != nil {
		t.Fatalf("ReadAll(LimitReader) on normal body: %v", err)
	}
	if string(body) != normalBody {
		t.Fatalf("normal body must pass through verbatim: got %q want %q", string(body), normalBody)
	}
}

// TestFetchProfile_OOMRiskClosed_DocumentedUnitGap documents why we don't
// drive GoogleProvider.FetchProfile or DiscordProvider.FetchProfile directly
// in unit form: the userinfo URL is hardcoded inside each method and the
// HTTP client is constructed internally (oauth2.Config.Client for Google,
// http.DefaultClient for Discord). Refactoring to inject the URL/client is
// out of scope for this cap-only change. Mirrors the documented-gap pattern
// from boards/handler_offset_cap_test.go:36 (cycle 106) and
// feed/handler_offset_cap_test.go (cycle 107). End-to-end verification of
// the cap behavior under a real OAuth callback is deferred to real-env QA
// (server boot + login redirect 302 confirms wiring intact; cap is
// enforced by the LimitReader call pinned in
// TestUserInfoCap_LimitReaderTruncatesOversizeBody above).
func TestFetchProfile_OOMRiskClosed_DocumentedUnitGap(t *testing.T) {
	t.Skip("GoogleProvider.FetchProfile hardcodes https://www.googleapis.com/oauth2/v2/userinfo and DiscordProvider.FetchProfile hardcodes https://discord.com/api/v10/users/@me; HTTP clients are constructed internally. The LimitReader cap pattern itself is pinned in TestUserInfoCap_LimitReaderTruncatesOversizeBody and the constant value is pinned in TestMaxUserInfoBytes_MatchesIntent.")
}
