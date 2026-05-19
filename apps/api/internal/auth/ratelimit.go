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

// rateLimitScript atomically increments a counter and sets the window TTL on
// its first INCR. Splitting INCR and EXPIRE across separate round-trips lets a
// transient failure between them strand the key at TTL=-1 forever, after which
// the counter would accumulate past the limit and produce permanent 429s for
// that (IP, path) pair. EVAL keeps both commands in one server-side step so a
// partial failure is impossible.
//
// spec: ratelimit `HTTP 요청 빈도 제한 카운터는 단일 원자 단위로 증가·만료 설정된다`
var rateLimitScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return n
`)

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
//
// The counter is fixed-window: the first INCR for a key seeds the window TTL,
// and subsequent INCRs within the same window only increment without resetting
// the TTL. The INCR + EXPIRE pair is run via Lua EVAL so a network or process
// failure cannot leave the key at TTL=-1.
//
// Use this surface for routes whose `docs/architecture.md` Rate Limit entry
// reads "…/IP" (e.g., OG fetch: 20/분/IP). For routes whose entry reads
// "…/유저" (e.g., 핀 생성: 30/분/유저), use MiddlewareByCreatorID instead.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return rl.middleware(next, func(r *http.Request) string {
		return extractIP(r)
	})
}

// MiddlewareByCreatorID returns a Chi middleware that rate-limits by the
// authenticated creator ID from request context. Must be wired AFTER a
// middleware that populates the creator ID (e.g., JWTMiddleware), otherwise
// the counter falls back to client IP so that an unauthenticated request
// cannot bypass the limit by sheer wiring error.
//
// spec: ratelimit `유저 단위 빈도 제한 surface를 노출한다`
// spec anchor: `docs/architecture.md` 의 Rate Limit 섹션 "핀 생성: 30/분/유저"
func (rl *RateLimiter) MiddlewareByCreatorID(next http.Handler) http.Handler {
	return rl.middleware(next, func(r *http.Request) string {
		if id, ok := CreatorIDFromContext(r.Context()); ok {
			return "creator:" + id.String()
		}
		return "ip:" + extractIP(r)
	})
}

// middleware is the shared implementation that both surfaces delegate to.
// Centralizing the Lua EVAL call keeps the fixed-window atomicity invariant
// (Requirement: 카운터는 단일 원자 단위로 증가·만료 설정된다) identical across
// both keying strategies.
func (rl *RateLimiter) middleware(next http.Handler, bucketKey func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := fmt.Sprintf("%s%s:%s", rlPrefix, r.URL.Path, bucketKey(r))

		ctx := r.Context()
		count, err := rateLimitScript.Run(ctx, rl.rdb, []string{key}, int(rl.window.Seconds())).Int64()
		if err != nil {
			// Redis down: fail-open
			next.ServeHTTP(w, r)
			return
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
