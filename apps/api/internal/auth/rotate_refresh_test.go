package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Pins the additive-logging contract for RotateRefreshToken's post-rotation
// cleanup (grace mark Set + index SRem):
//   - Happy path MUST NOT emit RotateRefreshToken log lines.
//   - Happy path MUST leave the old rt:{JTI} marked status=rotated with short
//     grace TTL and prune the old jti from rt_index:{sub}.
// The failure paths (Set fails / SRem fails) are covered by real-env QA —
// the Get + StoreRefreshToken steps run BEFORE cleanup, so a globally-down
// Redis (mr.Close) fails the upstream Get before exercising cleanup. Mid-
// function failure injection would require a test seam that the cycle's
// additive-logging scope does not justify.

func newRotateTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	jwtSvc := NewJWTService(make([]byte, 32))
	return &Service{rdb: rdb, jwtSvc: jwtSvc}, mr
}

func TestRotateRefreshToken_NoLogOnSuccess(t *testing.T) {
	s, mr := newRotateTestService(t)

	creatorID := uuid.New()
	refresh, jti, err := s.jwtSvc.SignRefreshToken(creatorID)
	if err != nil {
		t.Fatalf("sign refresh: %v", err)
	}
	if err := s.StoreRefreshToken(context.Background(), jti, creatorID); err != nil {
		t.Fatalf("store refresh: %v", err)
	}

	buf, restore := captureLog(t)
	defer restore()

	pair, err := s.RotateRefreshToken(context.Background(), refresh)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if pair == nil || pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("expected non-empty token pair, got %+v", pair)
	}

	if out := buf.String(); strings.Contains(out, "auth.RotateRefreshToken:") {
		t.Fatalf("happy path must not emit RotateRefreshToken log lines, got: %q", out)
	}

	val, err := mr.Get("rt:" + jti)
	if err != nil {
		t.Fatalf("old rt key must still exist with rotated status, got err: %v", err)
	}
	if !strings.Contains(val, `"status":"rotated"`) {
		t.Fatalf("old rt key must be marked rotated, got: %q", val)
	}
	members, _ := mr.SMembers("rt_index:" + creatorID.String())
	if len(members) != 1 || members[0] != pair.RefreshJTI {
		t.Fatalf("index must contain only new jti (%s), got %v", pair.RefreshJTI, members)
	}
}
