package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
)

// stubProvider is a no-op Provider so Callback's pre-state-check branches
// can be exercised without real OAuth wiring. Only Name + presence in the
// providers map is needed for the error-reflection branch (L77).
type stubProvider struct{}

func (stubProvider) Name() string                    { return "google" }
func (stubProvider) AuthCodeURL(state string) string { return "" }
func (stubProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return nil, nil
}
func (stubProvider) FetchProfile(ctx context.Context, token *oauth2.Token) (*UserProfile, error) {
	return nil, nil
}

func newCallbackTestHandler() http.Handler {
	h := &Handler{
		providers: map[string]Provider{"google": stubProvider{}},
		frontend:  "https://app.example.com",
	}
	r := chi.NewRouter()
	r.Get("/api/auth/{provider}/callback", h.Callback)
	return r
}

// TestCallback_ErrorQueryReflectionIsEscaped covers the open-redirect-adjacent
// reflected query injection: an attacker-supplied `error` value that decodes to
// `foo&next=https://attacker.example` must NOT be re-emitted into the Location
// header as a smuggled `next` query parameter. url.QueryEscape on the reflected
// value is the required defense.
func TestCallback_ErrorQueryReflectionIsEscaped(t *testing.T) {
	handler := newCallbackTestHandler()

	raw := "foo&next=https://attacker.example"
	req := httptest.NewRequest(http.MethodGet,
		"/api/auth/google/callback?error="+url.QueryEscape(raw), nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 Found, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("Location header missing")
	}

	// Vulnerability signature: smuggled `next=...` reaches the redirect URL.
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location is not a parseable URL: %q err=%v", loc, err)
	}
	q := parsed.Query()
	if q.Get("next") != "" {
		t.Fatalf("attacker-injected `next` must NOT appear as a separate query param after fix: Location=%q next=%q", loc, q.Get("next"))
	}
	if got := q.Get("error"); got != raw {
		t.Fatalf("error param round-trip must preserve the original decoded value: got %q want %q", got, raw)
	}
	if len(q) != 1 {
		t.Fatalf("Location must carry exactly one query param (`error`), got %d: %v", len(q), q)
	}
}

// TestCallback_ErrorQueryNormalCaseIsLossless guards against over-encoding
// regression: a plain alnum/underscore error code that the frontend already
// understands (e.g. `access_denied`) must remain byte-identical so the
// frontend's error-message switch keeps matching.
func TestCallback_ErrorQueryNormalCaseIsLossless(t *testing.T) {
	handler := newCallbackTestHandler()

	req := httptest.NewRequest(http.MethodGet,
		"/api/auth/google/callback?error=access_denied", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	loc := rec.Header().Get("Location")
	want := "https://app.example.com/login?error=access_denied"
	if loc != want {
		t.Fatalf("alnum/underscore error must be lossless: got %q want %q", loc, want)
	}
}

// TestCallback_UnknownProviderRedirectUnchanged is a regression guard for the
// adjacent literal-constant branch (L72) which must not be affected by the
// QueryEscape change to the user-input branch.
func TestCallback_UnknownProviderRedirectUnchanged(t *testing.T) {
	handler := newCallbackTestHandler()

	req := httptest.NewRequest(http.MethodGet,
		"/api/auth/facebook/callback?error=irrelevant", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	loc := rec.Header().Get("Location")
	want := "https://app.example.com/login?error=unknown_provider"
	if loc != want {
		t.Fatalf("unknown-provider branch must redirect to the literal-constant error code: got %q want %q", loc, want)
	}
}
