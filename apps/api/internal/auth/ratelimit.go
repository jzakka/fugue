package auth

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const rlPrefix = "rl:"

// RateLimiter is a Redis-based fixed-window rate limiter.
type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{rdb: rdb, limit: limit, window: window}
}

// Middleware returns a Chi middleware that rate-limits by client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		key := fmt.Sprintf("%s%s:%s", rlPrefix, r.URL.Path, ip)

		ctx := r.Context()
		count, err := rl.rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis down: fail-open
			next.ServeHTTP(w, r)
			return
		}

		if count == 1 {
			rl.rdb.Expire(ctx, key, rl.window)
		}

		if count > int64(rl.limit) {
			w.Header().Set("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	// Chi's middleware.RealIP sets RemoteAddr to the real client IP
	// from X-Forwarded-For / X-Real-IP headers. Safe behind Next.js
	// rewrite proxy and production ingress.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
