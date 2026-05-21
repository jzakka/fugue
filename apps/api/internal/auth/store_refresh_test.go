package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Pins the additive-logging contract for StoreRefreshToken's Expire call:
//   - Happy path MUST NOT emit `auth.StoreRefreshToken:` log lines.
//   - Happy path MUST set rt_index:{sub} TTL to rtTTL (7d) so the set
//     lifetime stays aligned with the underlying rt:{JTI} body.
// The failure path (Expire errors) is covered by real-env QA + code
// inspection against cycles C / F / G; miniredis cannot fail Expire
// independently without a test seam that the cycle's additive-logging
// scope does not justify.

func newStoreTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	jwtSvc := NewJWTService(make([]byte, 32))
	return &Service{rdb: rdb, jwtSvc: jwtSvc}, mr
}

func TestStoreRefreshToken_NoLogOnSuccess(t *testing.T) {
	s, mr := newStoreTestService(t)

	creatorID := uuid.New()
	jti := uuid.New().String()

	buf, restore := captureLog(t)
	defer restore()

	if err := s.StoreRefreshToken(context.Background(), jti, creatorID); err != nil {
		t.Fatalf("store refresh: %v", err)
	}

	if out := buf.String(); strings.Contains(out, "auth.StoreRefreshToken:") {
		t.Fatalf("happy path must not emit StoreRefreshToken log lines, got: %q", out)
	}

	idxKey := rtIdxPrefix + creatorID.String()
	ttl := mr.TTL(idxKey)
	if ttl <= 0 {
		t.Fatalf("rt_index:{sub} TTL after Expire must be > 0 (got %s); a silent Expire failure would leave it at -1", ttl)
	}
	// Allow a small slack (miniredis returns the nominal TTL we set, but be
	// defensive against jitter in case the harness ever introduces any).
	if ttl > rtTTL+5*time.Second || ttl < rtTTL-5*time.Second {
		t.Fatalf("rt_index:{sub} TTL=%s; want ≈ rtTTL=%s", ttl, rtTTL)
	}

	members, _ := mr.SMembers(idxKey)
	if len(members) != 1 || members[0] != jti {
		t.Fatalf("rt_index:{sub} must contain exactly the new jti (%s), got %v", jti, members)
	}
}
