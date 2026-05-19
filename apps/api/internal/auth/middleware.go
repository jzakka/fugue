package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const creatorIDKey contextKey = "creator_id"

// CreatorIDFromContext extracts the authenticated creator ID from context.
func CreatorIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(creatorIDKey).(uuid.UUID)
	return id, ok
}

// WithCreatorID returns a context that carries the given creator id under the
// same key the middleware uses. Tests and internal services use this to inject
// an authenticated principal without routing through middleware.
func WithCreatorID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, creatorIDKey, id)
}

// JWTMiddleware validates the JWT from cookie or Authorization header.
func JWTMiddleware(jwtSvc *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := jwtSvc.ValidateToken(tokenString)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					w.Header().Set("X-Token-Expired", "true")
				}
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			creatorID, err := uuid.Parse(claims.Subject)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), creatorIDKey, creatorID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalJWTMiddleware exposes an authentication context when a valid token is
// present, and otherwise lets the handler observe the caller as unauthenticated.
//
// spec: auth `토큰이 존재할 때 선택적으로 인증 컨텍스트를 노출한다` (design Decision 1)
// spec: feed `피드 라우트는 선택적 인증 미들웨어로 보호된다`
//
// Unlike JWTMiddleware, missing/expired/invalid tokens never produce 401.
// Per design Decision 2 token-expiry signalling (X-Token-Expired) is intentionally
// not surfaced here; clients learn about expiry from auth-required endpoints.
func OptionalJWTMiddleware(jwtSvc *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := jwtSvc.ValidateToken(tokenString)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			creatorID, err := uuid.Parse(claims.Subject)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), creatorIDKey, creatorID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	// 1. Check cookie first
	if cookie, err := r.Cookie("fugue_access"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// 2. Fall back to Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}
