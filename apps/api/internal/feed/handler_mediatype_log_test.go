package feed

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Pins the additive-logging contract for buildPersonalizedFeed's media-type
// fallback branch. The branch is fail-open by design (the caller already has
// a valid tag-based recRows slice from RecommendByTags, and L243 in the
// handler falls back to latest if recRows ends up empty), so a DB error from
// GetUserMediaTypeFrequency or RecommendByMediaType MUST NOT break the
// response — but it also MUST emit a timestamped operator log line so the
// degradation is visible before recommended-vs-latest mix collapses to
// all-latest.
//
// Mirrors the contract from handler_cache_log_test.go (cache get / set) and
// the auth package silent-error series (cycles C/F/G/I).

func TestGetFeed_LogsOnGetUserMediaTypeFrequencyError(t *testing.T) {
	tagID := uuid.New()
	q := &recordingQuerier{
		// pinCount >= 10 forces the personalized branch.
		pinCount: 15,
		// At least one tag row so RecommendByTags executes (and returns
		// empty), pushing the handler into the media-type fallback.
		tagFreq: []db.GetUserTagFrequencyRow{{TagID: tagID, Freq: 1}},
		// Empty allRec → RecommendByTags returns nil → len(recRows) < recLimit
		// → media-type fallback runs.
		allRec:    nil,
		mtFreqErr: errors.New("simulated GetUserMediaTypeFrequency failure"),
		// Provide a non-empty latest universe so the L243 latest fallback
		// produces a 200 with a valid pin set.
		allLatest: []db.ListPinsWithCreatorRow{makeLatestRow(0), makeLatestRow(1)},
	}
	h, _ := newTestHandler(t, q)

	userID := uuid.New()
	req := authenticatedRequest(t, "/api/feed?limit=20", userID)
	rec := httptest.NewRecorder()

	buf, restore := captureFeedLog(t)
	defer restore()

	h.GetFeed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (media-type fallback failure must remain fail-open); body=%s", rec.Code, rec.Body.String())
	}

	out := buf.String()
	if !strings.Contains(out, "feed.buildPersonalizedFeed: GetUserMediaTypeFrequency error:") {
		t.Fatalf("GetUserMediaTypeFrequency failure must emit operator log; got: %q", out)
	}
	wantUser := "user=" + userID.String()
	if !strings.Contains(out, wantUser) {
		t.Fatalf("log line must include the creator id %q so operator can scope the failure; got: %q", wantUser, out)
	}

	// RecommendByMediaType MUST NOT be called when the upstream
	// GetUserMediaTypeFrequency errored (branch is gated on err==nil &&
	// len(mtRows)>0). The injected mtRecsErr would surface if the gate were
	// broken.
	if len(q.mtCalls) != 0 {
		t.Fatalf("RecommendByMediaType must not run when GetUserMediaTypeFrequency errors; got %d calls", len(q.mtCalls))
	}
}

func TestGetFeed_LogsOnRecommendByMediaTypeError(t *testing.T) {
	tagID := uuid.New()
	q := &recordingQuerier{
		pinCount: 15,
		tagFreq:  []db.GetUserTagFrequencyRow{{TagID: tagID, Freq: 1}},
		allRec:   nil,
		// Non-empty mtFreq so the inner RecommendByMediaType branch runs.
		mtFreq: []db.GetUserMediaTypeFrequencyRow{
			{MediaType: "image", Freq: 5},
		},
		mtRecsErr: errors.New("simulated RecommendByMediaType failure"),
		allLatest: []db.ListPinsWithCreatorRow{makeLatestRow(0), makeLatestRow(1)},
	}
	h, _ := newTestHandler(t, q)

	userID := uuid.New()
	req := authenticatedRequest(t, "/api/feed?limit=20", userID)
	rec := httptest.NewRecorder()

	buf, restore := captureFeedLog(t)
	defer restore()

	h.GetFeed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (RecommendByMediaType failure must remain fail-open); body=%s", rec.Code, rec.Body.String())
	}

	out := buf.String()
	if !strings.Contains(out, "feed.buildPersonalizedFeed: RecommendByMediaType error:") {
		t.Fatalf("RecommendByMediaType failure must emit operator log; got: %q", out)
	}
	wantUser := "user=" + userID.String()
	if !strings.Contains(out, wantUser) {
		t.Fatalf("log line must include the creator id %q so operator can scope the failure; got: %q", wantUser, out)
	}

	// The RecommendByMediaType call MUST have been attempted exactly once —
	// the log line above is the operator-visible evidence of that single
	// failed attempt.
	if len(q.mtCalls) != 1 {
		t.Fatalf("RecommendByMediaType must be called exactly once when GetUserMediaTypeFrequency succeeds; got %d calls", len(q.mtCalls))
	}
}

func TestGetFeed_NoLogOnHealthyMediaTypeFallback(t *testing.T) {
	// Happy-path regression: when both media-type queries succeed (even if
	// they return empty rows so the fallback path doesn't actually
	// contribute), the handler MUST NOT emit any `feed.buildPersonalizedFeed:`
	// lines. Mirrors the cache-success no-log contract from
	// TestGetFeed_NoLogOnSuccessfulCacheSet.
	tagID := uuid.New()
	q := &recordingQuerier{
		pinCount:  15,
		tagFreq:   []db.GetUserTagFrequencyRow{{TagID: tagID, Freq: 1}},
		allRec:    nil,
		mtFreq:    nil, // empty → inner branch skipped, no RecommendByMediaType call
		allLatest: []db.ListPinsWithCreatorRow{makeLatestRow(0), makeLatestRow(1)},
	}
	h, _ := newTestHandler(t, q)

	userID := uuid.New()
	req := authenticatedRequest(t, "/api/feed?limit=20", userID)
	rec := httptest.NewRecorder()

	buf, restore := captureFeedLog(t)
	defer restore()

	h.GetFeed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if out := buf.String(); strings.Contains(out, "feed.buildPersonalizedFeed:") {
		t.Fatalf("happy path must not emit feed.buildPersonalizedFeed log lines, got: %q", out)
	}
}
