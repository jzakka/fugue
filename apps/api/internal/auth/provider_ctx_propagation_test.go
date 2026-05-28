package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// Pins the ctx-propagation contract for GoogleProvider.FetchProfile.
//
// Before this fix the method was:
//
//	client := p.config.Client(ctx, token)
//	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
//
// `oauth2.Config.Client(ctx, token)` only uses ctx for the token-refresh
// path inside the TokenSource. The outbound *http.Client returned does NOT
// thread the parent ctx onto its requests — oauth2 v0.36.0 transport.go:50
// does `req2 := req.Clone(req.Context())`, i.e. it sees only the request's
// own context. `client.Get(URL)` builds its request via `http.NewRequest`,
// which attaches `context.Background()` — so a SIGTERM cancel on the
// callback handler's ctx (or an `http.Server.Shutdown` mid-flight) never
// unblocked the userinfo round-trip, leaving the goroutine stuck on the
// upstream socket until the OS-level timeout fired.
//
// DiscordProvider.FetchProfile already used the sister pattern
// `http.NewRequestWithContext(ctx, ...) + client.Do(req)` (provider.go),
// which DOES thread ctx through `req.Context()` into the Transport. The
// fix aligns Google with Discord. These tests pin the contract so any
// future refactor that drops `NewRequestWithContext` (or reverts to
// `client.Get`) breaks loudly.
//
// Test seam: `GoogleProvider.userinfoURL` is overridable so we can point
// the real production code path at an `httptest.NewServer` that hangs
// or replies on cue. The seam is documented at provider.go (`const
// googleUserinfoURL` + field comment).

func TestGoogleProvider_FetchProfile_CtxCancelPropagatesToOutboundRequest(t *testing.T) {
	// Stub server hangs forever (or until ctx cancel kicks the client side).
	// Without ctx propagation, http.DefaultTransport would sit on this read
	// for the full 30s + OS socket timeout. With propagation, the Transport's
	// req.Context().Done() arm fires on cancel and we get back ~immediately
	// with a wrapped ctx error.
	serverDone := make(chan struct{})
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // server-side: hang until client disconnects (or test ends)
		close(serverDone)
	}))
	defer stub.Close()

	p := &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		},
		userinfoURL: stub.URL,
	}
	token := &oauth2.Token{AccessToken: "stub-token", TokenType: "Bearer"}

	const cancelAfter = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), cancelAfter)
	defer cancel()

	start := time.Now()
	profile, err := p.FetchProfile(ctx, token)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("FetchProfile must return error on ctx cancel, got profile=%+v", profile)
	}
	// The error path wraps `client.Do` failure as "google userinfo request: %w".
	// The underlying cause must be a ctx error so callers (and operators
	// staring at logs) can tell shutdown-induced aborts apart from real
	// upstream failures.
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchProfile err must wrap ctx error (DeadlineExceeded or Canceled), got %v", err)
	}
	// Generous upper bound: cancel deadline + ~1 ctx-propagation tick.
	// Pre-fix this would sit on the stub server until *its* shutdown,
	// which Close() triggers on test teardown — easily >1s.
	if elapsed > 2*time.Second {
		t.Fatalf("FetchProfile did not unblock promptly on ctx cancel (elapsed=%v, cancelAfter=%v) — ctx not propagated to outbound HTTP request", elapsed, cancelAfter)
	}
}

func TestGoogleProvider_FetchProfile_HappyPathRegression(t *testing.T) {
	// Regression guard: the NewRequestWithContext + client.Do refactor must
	// preserve the existing parse/normalize behavior. Stub replies with the
	// minimal Google userinfo shape; we assert the UserProfile fields map
	// exactly as before.
	const respBody = `{"id":"google-sub-12345","email":"user@example.com","verified_email":true,"name":"Test User","picture":"https://cdn.example.com/avatar.png"}`
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The Authorization header must still be injected by the oauth2
		// Transport even when we drive the request via NewRequestWithContext.
		// If this assertion fires, the fix broke auth-header injection.
		got := r.Header.Get("Authorization")
		if !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("upstream must receive Authorization: Bearer <token>, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	}))
	defer stub.Close()

	p := &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		},
		userinfoURL: stub.URL,
	}
	token := &oauth2.Token{AccessToken: "stub-token", TokenType: "Bearer"}

	profile, err := p.FetchProfile(context.Background(), token)
	if err != nil {
		t.Fatalf("FetchProfile on happy-path stub: %v", err)
	}

	if profile.ProviderID != "google-sub-12345" {
		t.Errorf("ProviderID: got %q, want %q", profile.ProviderID, "google-sub-12345")
	}
	if profile.Email != "user@example.com" {
		t.Errorf("Email: got %q, want %q", profile.Email, "user@example.com")
	}
	if !profile.EmailVerified {
		t.Errorf("EmailVerified: got false, want true")
	}
	if profile.Nickname != "Test User" {
		t.Errorf("Nickname: got %q, want %q", profile.Nickname, "Test User")
	}
	if profile.AvatarURL != "https://cdn.example.com/avatar.png" {
		t.Errorf("AvatarURL: got %q, want %q", profile.AvatarURL, "https://cdn.example.com/avatar.png")
	}
	// RawProfile must be the verbatim response body so downstream
	// observability (UserProfile.RawProfile is persisted to the OAuth row)
	// keeps full fidelity. json.Compact would lose insignificant whitespace
	// — we accept either since the upstream chose this exact byte sequence.
	var roundtrip map[string]any
	if err := json.Unmarshal(profile.RawProfile, &roundtrip); err != nil {
		t.Fatalf("RawProfile must be parseable JSON: %v (raw=%q)", err, string(profile.RawProfile))
	}
}

func TestGoogleProvider_DefaultUserinfoURL_PinsProductionEndpoint(t *testing.T) {
	// The `userinfoURL` field exists purely as a test seam — production must
	// always hit the public Google endpoint. If `NewGoogleProvider` ever
	// stops initializing the field (or the constant drifts), this catches it
	// before a release ships a provider that requests "" or some half-built
	// URL.
	p := NewGoogleProvider("cid", "csecret", "https://app.example.com/callback")
	if p.userinfoURL != "https://www.googleapis.com/oauth2/v2/userinfo" {
		t.Fatalf("NewGoogleProvider must initialize userinfoURL to the public Google endpoint, got %q", p.userinfoURL)
	}
	if googleUserinfoURL != "https://www.googleapis.com/oauth2/v2/userinfo" {
		t.Fatalf("googleUserinfoURL constant drifted: got %q", googleUserinfoURL)
	}
}
