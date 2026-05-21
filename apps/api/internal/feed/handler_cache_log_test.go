package feed

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Pins the additive-logging contract for feed.GetFeed's opportunistic cache
// reads and writes:
//   - Happy path MUST NOT emit `feed.GetFeed:` lines, so successful production
//     traffic does not flood logs.
//   - Set failure MUST emit a timestamped line including the cache key so the
//     operator can detect Redis write degradation (OOM / MISCONF / network /
//     read-only replica failover) before DB load amplification surfaces.
//   - Get failure (other than redis.Nil) MUST emit a timestamped line including
//     the cache key. redis.Nil (normal cache miss) MUST stay silent because
//     that is the documented signal that the compute path should run.
//
// Mirrors cycle C / F / G / I in the auth package
// (RevokeRefreshToken / RotateRefreshToken / RateLimiter / StoreRefreshToken).

func captureFeedLog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	return &buf, func() { log.SetOutput(prev) }
}

func TestGetFeed_NoLogOnSuccessfulCacheSet(t *testing.T) {
	q := &recordingQuerier{
		pinCount:  5, // cold-start: buildLatestFeed path
		allLatest: []db.ListPinsWithCreatorRow{makeLatestRow(0)},
	}
	h, mr := newTestHandler(t, q)

	userID := uuid.New()
	req := authenticatedRequest(t, "/api/feed?limit=20", userID)
	rec := httptest.NewRecorder()

	buf, restore := captureFeedLog(t)
	defer restore()

	h.GetFeed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if out := buf.String(); strings.Contains(out, "feed.GetFeed:") {
		t.Fatalf("happy path must not emit feed.GetFeed log lines, got: %q", out)
	}

	cacheKey := fmt.Sprintf("feed:%s:%d:%d", userID.String(), 20, 0)
	val, err := mr.Get(cacheKey)
	if err != nil {
		t.Fatalf("cache key %q must exist after successful GetFeed (silent Set failure would leave it empty); got err=%v", cacheKey, err)
	}
	if val == "" {
		t.Fatalf("cache value at %q must be the serialized FeedResponse, got empty", cacheKey)
	}
}

func TestGetFeed_LogsOnCacheSetFailure(t *testing.T) {
	q := &recordingQuerier{
		pinCount:  5,
		allLatest: []db.ListPinsWithCreatorRow{makeLatestRow(0)},
	}
	h, mr := newTestHandler(t, q)

	// Close miniredis to force every subsequent Redis command to fail. The
	// cache Set (and the preceding Get on the same request) will both error
	// out; the handler must still respond 200 via fail-open and the Set
	// failure must produce a timestamped operator log line.
	mr.Close()

	userID := uuid.New()
	req := authenticatedRequest(t, "/api/feed?limit=20", userID)
	rec := httptest.NewRecorder()

	buf, restore := captureFeedLog(t)
	defer restore()

	h.GetFeed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (fail-open is mandatory regardless of cache); body=%s", rec.Code, rec.Body.String())
	}

	out := buf.String()
	if !strings.Contains(out, "feed.GetFeed: cache set error:") {
		t.Fatalf("cache Set failure must emit operator log; got: %q", out)
	}
	wantKey := fmt.Sprintf("key=feed:%s:%d:%d", userID.String(), 20, 0)
	if !strings.Contains(out, wantKey) {
		t.Fatalf("log line must include the cache key %q so operator can map the outage to (sub, limit, offset); got: %q", wantKey, out)
	}
}

func TestGetFeed_NoLogOnRedisNilCacheMiss(t *testing.T) {
	// Cold-start path with an empty miniredis: the cache key does not exist,
	// so Redis returns redis.Nil. Compute path runs successfully, populates
	// the cache, and returns. No `feed.GetFeed:` line must appear because
	// redis.Nil is the normal cache-miss signal — only true degradation
	// should produce operator noise.
	q := &recordingQuerier{
		pinCount:  5, // cold-start: buildLatestFeed path
		allLatest: []db.ListPinsWithCreatorRow{makeLatestRow(0)},
	}
	h, _ := newTestHandler(t, q)

	userID := uuid.New()
	req := authenticatedRequest(t, "/api/feed?limit=20", userID)
	rec := httptest.NewRecorder()

	buf, restore := captureFeedLog(t)
	defer restore()

	h.GetFeed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (cache miss must fall through to compute path); body=%s", rec.Code, rec.Body.String())
	}
	if out := buf.String(); strings.Contains(out, "feed.GetFeed: cache get error:") {
		t.Fatalf("redis.Nil cache miss must stay silent (documented signal, not degradation); got: %q", out)
	}
}

func TestGetFeed_LogsOnCacheGetFailure(t *testing.T) {
	// Close miniredis to force every subsequent Redis command — including
	// the initial GET — to fail with a connection error (not redis.Nil).
	// The handler must still respond 200 via fail-open and the GET failure
	// must produce a timestamped operator log line so Redis degradation is
	// visible before DB load amplification surfaces. Mirrors cycle K's
	// SET failure contract on the same function (inverse code path).
	q := &recordingQuerier{
		pinCount:  5,
		allLatest: []db.ListPinsWithCreatorRow{makeLatestRow(0)},
	}
	h, mr := newTestHandler(t, q)
	mr.Close()

	userID := uuid.New()
	req := authenticatedRequest(t, "/api/feed?limit=20", userID)
	rec := httptest.NewRecorder()

	buf, restore := captureFeedLog(t)
	defer restore()

	h.GetFeed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (cache GET failure must fall through to compute path); body=%s", rec.Code, rec.Body.String())
	}

	out := buf.String()
	if !strings.Contains(out, "feed.GetFeed: cache get error:") {
		t.Fatalf("cache GET failure (not redis.Nil) must emit operator log; got: %q", out)
	}
	wantKey := fmt.Sprintf("key=feed:%s:%d:%d", userID.String(), 20, 0)
	if !strings.Contains(out, wantKey) {
		t.Fatalf("log line must include the cache key %q so operator can map the outage to (sub, limit, offset); got: %q", wantKey, out)
	}
}
