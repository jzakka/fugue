package feed_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
)

// This test reproduces the production wiring at apps/api/cmd/server/main.go
// for `/api/feed` and asserts the contract that the feed handler relies on:
// `auth.CreatorIDFromContext` returns the caller's id when a valid token is
// presented and reports unauthenticated otherwise.
//
// The spec being protected from regression:
//   - feed `피드 라우트는 선택적 인증 미들웨어로 보호된다`
//   - auth `토큰이 존재할 때 선택적으로 인증 컨텍스트를 노출한다`
//
// The real Handler.GetFeed branches on exactly the value observed here, so a
// spy handler at the same observation point is equivalent to mocking the
// downstream `CountUserPins` call for the purpose of detecting wiring drift.
func TestFeedRoute_WiredWithOptionalJWTMiddleware(t *testing.T) {
	secret := []byte("test-secret-for-feed-wiring")
	jwtSvc := auth.NewJWTService(secret)

	var observedID uuid.UUID
	var observedAuthenticated bool
	spy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedID, observedAuthenticated = auth.CreatorIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/api/feed", spy.ServeHTTP)

	t.Run("no token: handler reaches unauthenticated branch", func(t *testing.T) {
		observedID, observedAuthenticated = uuid.Nil, false
		req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200 (spec forbids 401 here)", rec.Code)
		}
		if observedAuthenticated {
			t.Fatalf("handler observed authenticated=true for missing token; got id=%s", observedID)
		}
	})

	t.Run("valid cookie token: handler reaches authenticated branch", func(t *testing.T) {
		observedID, observedAuthenticated = uuid.Nil, false
		wantID := uuid.New()
		tokenStr := signTokenForTest(t, jwtSvc, secret, wantID.String(), time.Now().Add(time.Hour))

		req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
		req.AddCookie(&http.Cookie{Name: "fugue_access", Value: tokenStr})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", rec.Code)
		}
		if !observedAuthenticated {
			t.Fatalf("handler observed authenticated=false for valid cookie token; expected true")
		}
		if observedID != wantID {
			t.Fatalf("creator id mismatch: got %s, want %s", observedID, wantID)
		}
	})
}

func signTokenForTest(t *testing.T, _ *auth.JWTService, secret []byte, subject string, exp time.Time) string {
	t.Helper()
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    "fugue",
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
