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
