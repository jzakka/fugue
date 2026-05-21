package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Pins the additive-logging contract for RevokeAllTokens's three Redis steps
// (SMembers → per-jti Del → rt_index Del):
//   - Happy path MUST NOT emit `auth.RevokeAllTokens:` lines so a future
//     compromise-detection wiring does not flood logs during normal sweeps.
//   - SMembers failure MUST emit a timestamped operator log with the sub
//     identifier so the operator can detect Redis read degradation before
//     the orphan-key window lets a compromised refresh token keep rotating.
//
// Per-jti Del failure and rt_index Del failure log lines (lines 343 and 346
// of service.go) are covered by code inspection against the cycle C / F /
// G / I / K additive-logging pattern + real-env QA (PR closeout), because
// miniredis cannot fail individual Del commands while leaving SMembers
// successful without a test seam that the cycle's additive-logging scope
// does not justify (same precedent as StoreRefreshToken Expire cycle I and
// feed.GetFeed cache Set cycle K).

func newRevokeAllTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	jwtSvc := NewJWTService(make([]byte, 32))
	return &Service{rdb: rdb, jwtSvc: jwtSvc}, mr
}

func TestRevokeAllTokens_NoLogOnSuccess(t *testing.T) {
	s, mr := newRevokeAllTestService(t)

	creatorID := uuid.New()
	jti1 := uuid.New().String()
	jti2 := uuid.New().String()

	// Seed the same shape StoreRefreshToken would produce: two rt:{JTI}
	// bodies and an rt_index:{sub} SET pointing to both.
	idxKey := rtIdxPrefix + creatorID.String()
	if err := mr.Set(rtPrefix+jti1, `{"creator_id":"`+creatorID.String()+`","status":"active"}`); err != nil {
		t.Fatalf("seed rt:%s: %v", jti1, err)
	}
	if err := mr.Set(rtPrefix+jti2, `{"creator_id":"`+creatorID.String()+`","status":"active"}`); err != nil {
		t.Fatalf("seed rt:%s: %v", jti2, err)
	}
	if _, err := mr.SAdd(idxKey, jti1, jti2); err != nil {
		t.Fatalf("seed SAdd: %v", err)
	}

	buf, restore := captureLog(t)
	defer restore()

	s.RevokeAllTokens(context.Background(), creatorID)

	if out := buf.String(); strings.Contains(out, "auth.RevokeAllTokens:") {
		t.Fatalf("happy path must not emit RevokeAllTokens log lines, got: %q", out)
	}

	// Verify the sweep actually completed: all three keys gone. A silent
	// per-jti Del or rt_index Del failure would leave them alive at the
	// original 7d TTL, which a future compromise-detection wiring would
	// mistakenly trust as "sessions revoked".
	if mr.Exists(rtPrefix + jti1) {
		t.Fatalf("rt:%s must be deleted after RevokeAllTokens", jti1)
	}
	if mr.Exists(rtPrefix + jti2) {
		t.Fatalf("rt:%s must be deleted after RevokeAllTokens", jti2)
	}
	if mr.Exists(idxKey) {
		t.Fatalf("rt_index:{sub} (%s) must be deleted after RevokeAllTokens", idxKey)
	}
}

func TestRevokeAllTokens_LogsOnSMembersFailure(t *testing.T) {
	s, mr := newRevokeAllTestService(t)

	creatorID := uuid.New()

	// Close miniredis to force every subsequent Redis command (including
	// SMembers) to fail. The function must early-return after logging the
	// SMembers error so that the per-jti Del loop and rt_index Del never
	// run (which would otherwise emit a second wave of misleading logs
	// against a known-down Redis).
	mr.Close()

	buf, restore := captureLog(t)
	defer restore()

	s.RevokeAllTokens(context.Background(), creatorID)

	out := buf.String()
	if !strings.Contains(out, "auth.RevokeAllTokens: SMembers error:") {
		t.Fatalf("SMembers failure must emit operator log; got: %q", out)
	}
	wantSub := "sub=" + creatorID.String()
	if !strings.Contains(out, wantSub) {
		t.Fatalf("log line must include the sub identifier %q so operator can map the outage to a creator; got: %q", wantSub, out)
	}
	// Early-return invariant: per-jti Del and rt_index Del log lines must
	// NOT appear (they would, if the function continued past the SMembers
	// failure and naively called Del against the closed connection).
	if strings.Contains(out, "auth.RevokeAllTokens: Del rt error") {
		t.Fatalf("per-jti Del must not run after SMembers failure (early return required); got: %q", out)
	}
	if strings.Contains(out, "auth.RevokeAllTokens: Del rt_index error") {
		t.Fatalf("rt_index Del must not run after SMembers failure (early return required); got: %q", out)
	}
}
