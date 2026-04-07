package admin

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// AdminKeyMiddleware validates the X-Admin-Key header against the
// ADMIN_API_KEY environment variable.
func AdminKeyMiddleware(next http.Handler) http.Handler {
	apiKey := os.Getenv("ADMIN_API_KEY")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			log.Println("admin: ADMIN_API_KEY not set, rejecting request")
			writeError(w, http.StatusForbidden, "Admin API not configured")
			return
		}

		provided := r.Header.Get("X-Admin-Key")
		if provided == "" {
			writeError(w, http.StatusUnauthorized, "Missing X-Admin-Key header")
			return
		}

		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(provided)) != 1 {
			writeError(w, http.StatusForbidden, "Invalid admin key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
