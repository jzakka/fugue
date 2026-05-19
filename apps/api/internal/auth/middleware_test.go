package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newTestJWTService(t *testing.T) *JWTService {
	t.Helper()
	return NewJWTService([]byte("test-secret-for-optional-middleware"))
}

func signTokenWithSubject(t *testing.T, svc *JWTService, subject string, expiresAt time.Time) string {
	t.Helper()
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(svc.secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// runOptionalMiddleware invokes the OptionalJWTMiddleware against a request and
// records whether the handler ran, plus what CreatorIDFromContext observed.
func runOptionalMiddleware(t *testing.T, svc *JWTService, req *http.Request) (called bool, gotID uuid.UUID, gotOK bool, status int) {
	t.Helper()
	handler := OptionalJWTMiddleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotID, gotOK = CreatorIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return called, gotID, gotOK, rec.Code
}

func TestOptionalJWTMiddleware_NoToken_PassesThroughUnauthenticated(t *testing.T) {
	svc := newTestJWTService(t)
	req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)

	called, gotID, gotOK, status := runOptionalMiddleware(t, svc, req)

	if !called {
		t.Fatalf("handler not called for missing token")
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", status, http.StatusOK)
	}
	if gotOK {
		t.Fatalf("expected CreatorIDFromContext ok=false, got true (id=%s)", gotID)
	}
	if gotID != uuid.Nil {
		t.Fatalf("expected uuid.Nil, got %s", gotID)
	}
}

func TestOptionalJWTMiddleware_ValidTokenInCookie_ExposesCreatorID(t *testing.T) {
	svc := newTestJWTService(t)
	wantID := uuid.New()
	tokenStr := signTokenWithSubject(t, svc, wantID.String(), time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
	req.AddCookie(&http.Cookie{Name: "fugue_access", Value: tokenStr})

	called, gotID, gotOK, status := runOptionalMiddleware(t, svc, req)

	if !called {
		t.Fatalf("handler not called for valid cookie token")
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", status, http.StatusOK)
	}
	if !gotOK {
		t.Fatalf("expected CreatorIDFromContext ok=true, got false")
	}
	if gotID != wantID {
		t.Fatalf("creator id mismatch: got %s, want %s", gotID, wantID)
	}
}

func TestOptionalJWTMiddleware_ValidTokenInBearerHeader_ExposesCreatorID(t *testing.T) {
	svc := newTestJWTService(t)
	wantID := uuid.New()
	tokenStr := signTokenWithSubject(t, svc, wantID.String(), time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	called, gotID, gotOK, status := runOptionalMiddleware(t, svc, req)

	if !called {
		t.Fatalf("handler not called for valid bearer token")
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", status, http.StatusOK)
	}
	if !gotOK {
		t.Fatalf("expected CreatorIDFromContext ok=true, got false")
	}
	if gotID != wantID {
		t.Fatalf("creator id mismatch: got %s, want %s", gotID, wantID)
	}
}

func TestOptionalJWTMiddleware_InvalidToken_ExpiredPassesThroughUnauthenticated(t *testing.T) {
	svc := newTestJWTService(t)
	expiredToken := signTokenWithSubject(t, svc, uuid.New().String(), time.Now().Add(-time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
	req.AddCookie(&http.Cookie{Name: "fugue_access", Value: expiredToken})

	called, gotID, gotOK, status := runOptionalMiddleware(t, svc, req)

	if !called {
		t.Fatalf("handler not called for expired token")
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200 for expired token (spec says no 401), got %d", status)
	}
	if gotOK {
		t.Fatalf("expected CreatorIDFromContext ok=false for expired token, got true (id=%s)", gotID)
	}
	if gotID != uuid.Nil {
		t.Fatalf("expected uuid.Nil for expired token, got %s", gotID)
	}
}

func TestOptionalJWTMiddleware_InvalidToken_BadSignaturePassesThroughUnauthenticated(t *testing.T) {
	signer := NewJWTService([]byte("attacker-secret"))
	verifier := newTestJWTService(t)
	badSigToken := signTokenWithSubject(t, signer, uuid.New().String(), time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
	req.AddCookie(&http.Cookie{Name: "fugue_access", Value: badSigToken})

	called, _, gotOK, status := runOptionalMiddleware(t, verifier, req)

	if !called {
		t.Fatalf("handler not called for bad-signature token")
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200 for bad-signature token (spec says no 401), got %d", status)
	}
	if gotOK {
		t.Fatalf("expected CreatorIDFromContext ok=false for bad-signature token, got true")
	}
}

func TestOptionalJWTMiddleware_InvalidToken_UnparseableSubjectPassesThroughUnauthenticated(t *testing.T) {
	svc := newTestJWTService(t)
	tokenStr := signTokenWithSubject(t, svc, "not-a-uuid", time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
	req.AddCookie(&http.Cookie{Name: "fugue_access", Value: tokenStr})

	called, _, gotOK, status := runOptionalMiddleware(t, svc, req)

	if !called {
		t.Fatalf("handler not called for unparseable-subject token")
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200 for unparseable-subject token (spec says no 401), got %d", status)
	}
	if gotOK {
		t.Fatalf("expected CreatorIDFromContext ok=false for unparseable-subject token, got true")
	}
}
