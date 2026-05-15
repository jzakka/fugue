package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// These tests pin the fixed-window invariants enforced by the spec:
//   ratelimit `HTTP 요청 빈도 제한 카운터는 단일 원자 단위로 증가·만료 설정된다`
//
// Before the EVAL refactor, INCR and EXPIRE were two round-trips and the first
// EXPIRE failure (or just missing TTL observation right after INCR) could
// produce a permanent 429 for that (IP, path) pair. We assert here that the
// counter key always has a positive TTL after the first request, that
// subsequent requests do not refresh that TTL (fixed-window, not sliding),
// and that Redis failures fail-open.

func newTestRateLimiter(t *testing.T, limit int, window time.Duration) (*RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewRateLimiter(rdb, limit, window), mr
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func doRequest(t *testing.T, h http.Handler, path, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t, 3, time.Second)
	h := rl.Middleware(okHandler())

	for i := 1; i <= 3; i++ {
		rec := doRequest(t, h, "/api/x", "1.2.3.4:1111")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, rec.Code)
		}
	}
	rec := doRequest(t, h, "/api/x", "1.2.3.4:1111")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request 4: got %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("Retry-After header missing on 429")
	}
}

func TestRateLimiter_FirstIncrSetsTTL(t *testing.T) {
	rl, mr := newTestRateLimiter(t, 10, time.Second)
	h := rl.Middleware(okHandler())

	rec := doRequest(t, h, "/api/x", "1.2.3.4:1111")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	key := fmt.Sprintf("%s/api/x:1.2.3.4", rlPrefix)
	ttl := mr.TTL(key)
	if ttl <= 0 {
		t.Fatalf("after first INCR, key %q has TTL=%s; want > 0 (spec: 같은 원자 단위에서 TTL 설정)", key, ttl)
	}
	if ttl > time.Second {
		t.Fatalf("TTL %s exceeds window 1s", ttl)
	}
}

func TestRateLimiter_SubsequentIncrPreservesFixedWindow(t *testing.T) {
	rl, mr := newTestRateLimiter(t, 10, time.Second)
	h := rl.Middleware(okHandler())
	key := fmt.Sprintf("%s/api/x:1.2.3.4", rlPrefix)

	doRequest(t, h, "/api/x", "1.2.3.4:1111")
	firstTTL := mr.TTL(key)
	if firstTTL <= 0 {
		t.Fatalf("first TTL %s not positive", firstTTL)
	}

	mr.FastForward(500 * time.Millisecond)

	doRequest(t, h, "/api/x", "1.2.3.4:1111")
	secondTTL := mr.TTL(key)
	if secondTTL > firstTTL {
		t.Fatalf("subsequent INCR refreshed TTL: first=%s second=%s (spec: TTL이 윈도우 길이로 리셋되지 않는다)", firstTTL, secondTTL)
	}
}

func TestRateLimiter_WindowResetsAfterExpiry(t *testing.T) {
	rl, mr := newTestRateLimiter(t, 2, time.Second)
	h := rl.Middleware(okHandler())

	for i := 1; i <= 2; i++ {
		rec := doRequest(t, h, "/api/x", "1.2.3.4:1111")
		if rec.Code != http.StatusOK {
			t.Fatalf("pre-expiry request %d: got %d, want 200", i, rec.Code)
		}
	}
	if rec := doRequest(t, h, "/api/x", "1.2.3.4:1111"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("pre-expiry: got %d, want 429", rec.Code)
	}

	mr.FastForward(2 * time.Second)

	if rec := doRequest(t, h, "/api/x", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("post-expiry: got %d, want 200 (window should have reset)", rec.Code)
	}
}

func TestRateLimiter_RedisFailureFailsOpen(t *testing.T) {
	rl, mr := newTestRateLimiter(t, 1, time.Second)
	h := rl.Middleware(okHandler())

	mr.Close()

	for i := 1; i <= 5; i++ {
		rec := doRequest(t, h, "/api/x", "1.2.3.4:1111")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d after redis down: got %d, want 200 (fail-open)", i, rec.Code)
		}
	}
}

func TestRateLimiter_KeyPartitionedByIPAndPath(t *testing.T) {
	rl, _ := newTestRateLimiter(t, 1, time.Second)
	h := rl.Middleware(okHandler())

	if rec := doRequest(t, h, "/api/a", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("path a ip 1: got %d, want 200", rec.Code)
	}
	if rec := doRequest(t, h, "/api/b", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("different path same ip: got %d, want 200 (different bucket)", rec.Code)
	}
	if rec := doRequest(t, h, "/api/a", "9.9.9.9:1111"); rec.Code != http.StatusOK {
		t.Fatalf("same path different ip: got %d, want 200 (different bucket)", rec.Code)
	}
	if rec := doRequest(t, h, "/api/a", "1.2.3.4:1111"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("same path same ip second hit: got %d, want 429", rec.Code)
	}
}

// The next four tests pin the per-creator keying invariants:
//   ratelimit `유저 단위 빈도 제한 surface를 노출한다`
//
// architecture.md "핀 생성: 30/분/유저" SHALL requires that /api/pins counts
// are partitioned by creator ID rather than client IP. We assert here that the
// new MiddlewareByCreatorID surface uses creator-keyed buckets when an auth
// context is present, shares a bucket across IPs for the same creator, keeps
// different creators on the same IP isolated, and falls back to IP keying
// (still counting, never fail-open) when the creator context is missing.

// withCreator wraps a handler so the request carries an authenticated creator
// ID in context, simulating what JWTMiddleware does in production.
func withCreator(creatorID uuid.UUID, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), creatorIDKey, creatorID)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestRateLimiter_MiddlewareByCreatorID_PartitionsByUser(t *testing.T) {
	rl, mr := newTestRateLimiter(t, 1, time.Second)
	alice := uuid.New()
	bob := uuid.New()
	hAlice := withCreator(alice, rl.MiddlewareByCreatorID(okHandler()))
	hBob := withCreator(bob, rl.MiddlewareByCreatorID(okHandler()))

	if rec := doRequest(t, hAlice, "/api/pins", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("alice first: got %d, want 200", rec.Code)
	}
	if rec := doRequest(t, hBob, "/api/pins", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("bob first (same IP, different creator): got %d, want 200 (different bucket)", rec.Code)
	}
	if rec := doRequest(t, hAlice, "/api/pins", "1.2.3.4:1111"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("alice second: got %d, want 429", rec.Code)
	}

	aliceKey := fmt.Sprintf("%s/api/pins:creator:%s", rlPrefix, alice.String())
	if !mr.Exists(aliceKey) {
		t.Fatalf("expected creator-keyed bucket %q to exist", aliceKey)
	}
	bobKey := fmt.Sprintf("%s/api/pins:creator:%s", rlPrefix, bob.String())
	if !mr.Exists(bobKey) {
		t.Fatalf("expected creator-keyed bucket %q to exist", bobKey)
	}
}

func TestRateLimiter_MiddlewareByCreatorID_SharesAcrossIPs(t *testing.T) {
	rl, _ := newTestRateLimiter(t, 1, time.Second)
	alice := uuid.New()
	h := withCreator(alice, rl.MiddlewareByCreatorID(okHandler()))

	if rec := doRequest(t, h, "/api/pins", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("alice from IP1: got %d, want 200", rec.Code)
	}
	if rec := doRequest(t, h, "/api/pins", "9.9.9.9:2222"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("alice from IP2 (same creator, different IP): got %d, want 429 (shared bucket)", rec.Code)
	}
}

func TestRateLimiter_MiddlewareByCreatorID_FallsBackToIPWhenUnauth(t *testing.T) {
	rl, mr := newTestRateLimiter(t, 1, time.Second)
	h := rl.MiddlewareByCreatorID(okHandler()) // no withCreator wrapper

	if rec := doRequest(t, h, "/api/pins", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("first unauth: got %d, want 200", rec.Code)
	}
	if rec := doRequest(t, h, "/api/pins", "1.2.3.4:1111"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second unauth: got %d, want 429 (must not fail-open)", rec.Code)
	}

	fallbackKey := fmt.Sprintf("%s/api/pins:ip:1.2.3.4", rlPrefix)
	if !mr.Exists(fallbackKey) {
		t.Fatalf("expected IP-fallback bucket %q to exist", fallbackKey)
	}
}

func TestRateLimiter_Middleware_IPKeyUnchanged(t *testing.T) {
	rl, mr := newTestRateLimiter(t, 10, time.Second)
	h := rl.Middleware(okHandler())

	if rec := doRequest(t, h, "/api/x", "1.2.3.4:1111"); rec.Code != http.StatusOK {
		t.Fatalf("first IP-keyed: got %d, want 200", rec.Code)
	}

	expected := fmt.Sprintf("%s/api/x:1.2.3.4", rlPrefix)
	if !mr.Exists(expected) {
		t.Fatalf("expected IP-keyed bucket %q to exist", expected)
	}
	creatorPrefixed := fmt.Sprintf("%s/api/x:creator:", rlPrefix)
	for _, k := range mr.Keys() {
		if len(k) >= len(creatorPrefixed) && k[:len(creatorPrefixed)] == creatorPrefixed {
			t.Fatalf("IP-keyed surface should not produce creator: bucket, got %q", k)
		}
	}
}
