package auth

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// These tests pin the additive-logging contract for RevokeRefreshToken:
// - Redis Del/SRem failures MUST be log.Printf'd (operator visibility into the
//   "logout returned 200 OK but rt:{JTI} kept living at status=active" window).
// - The happy path MUST NOT log (zero log volume during normal operation).
// - Invalid/unparseable refresh tokens MUST early-return without touching Redis
//   (avoid log noise for non-actionable input).

func newRevokeTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	jwtSvc := NewJWTService(make([]byte, 32))
	return &Service{rdb: rdb, jwtSvc: jwtSvc}, mr
}

func captureLog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	return &buf, func() { log.SetOutput(prev) }
}

func TestRevokeRefreshToken_LogsOnRedisDelAndSRemFailure(t *testing.T) {
	s, mr := newRevokeTestService(t)

	creatorID := uuid.New()
	refresh, jti, err := s.jwtSvc.SignRefreshToken(creatorID)
	if err != nil {
		t.Fatalf("sign refresh: %v", err)
	}
	if err := s.StoreRefreshToken(context.Background(), jti, creatorID); err != nil {
		t.Fatalf("store refresh: %v", err)
	}

	mr.Close() // force every subsequent Del/SRem to fail

	buf, restore := captureLog(t)
	defer restore()

	s.RevokeRefreshToken(context.Background(), refresh)

	out := buf.String()
	if !strings.Contains(out, "auth.RevokeRefreshToken: Del error") {
		t.Fatalf("expected Del error log line, got: %q", out)
	}
	if !strings.Contains(out, "auth.RevokeRefreshToken: SRem error") {
		t.Fatalf("expected SRem error log line, got: %q", out)
	}
	if !strings.Contains(out, "jti="+jti) {
		t.Fatalf("expected jti=%s in log lines for grep-ability, got: %q", jti, out)
	}
	if !strings.Contains(out, "sub="+creatorID.String()) {
		t.Fatalf("expected sub=%s in SRem log line, got: %q", creatorID.String(), out)
	}
}

func TestRevokeRefreshToken_NoLogOnSuccess(t *testing.T) {
	s, mr := newRevokeTestService(t)

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

	s.RevokeRefreshToken(context.Background(), refresh)

	if out := buf.String(); strings.Contains(out, "auth.RevokeRefreshToken:") {
		t.Fatalf("happy path must not emit RevokeRefreshToken log lines, got: %q", out)
	}

	if mr.Exists("rt:" + jti) {
		t.Fatalf("rt:%s must be deleted after successful revoke", jti)
	}
	members, _ := mr.SMembers("rt_index:" + creatorID.String())
	if len(members) != 0 {
		t.Fatalf("rt_index must be empty after successful revoke, got %v", members)
	}
}

func TestRevokeRefreshToken_InvalidTokenEarlyReturnNoLog(t *testing.T) {
	s, _ := newRevokeTestService(t)

	buf, restore := captureLog(t)
	defer restore()

	s.RevokeRefreshToken(context.Background(), "not-a-jwt")

	if out := buf.String(); strings.Contains(out, "auth.RevokeRefreshToken:") {
		t.Fatalf("invalid token must early-return without touching Redis, got log: %q", out)
	}
}
