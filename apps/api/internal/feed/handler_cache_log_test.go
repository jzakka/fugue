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
// Set:
//   - Happy path MUST NOT emit `feed.GetFeed:` lines, so successful production
//     traffic does not flood logs.
//   - Set failure MUST emit a timestamped line including the cache key so the
//     operator can detect Redis write degradation (OOM / MISCONF / network /
//     read-only replica failover) before DB load amplification surfaces.
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
