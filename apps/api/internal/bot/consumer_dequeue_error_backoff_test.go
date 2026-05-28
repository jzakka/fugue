package bot

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Pins the dequeue-error backoff for PioneerConsumer.Run and
// HarvesterConsumer.Run. Before this fix, a permanent DequeueCtx error
// (the realistic shape: PGURLScheduler.DequeueCtx → tryClaim → BeginTx
// failing in microseconds when the DB connection pool is down) caused the
// Run loops to log+continue with no sleep — initial probing showed ~50k
// iterations within a single 200ms test window per worker, fully saturating
// one CPU core and drowning the log-shipping pipeline. The fix wires a
// `select { ctx.Done : time.After(backoff) }` in the err branch that mirrors
// PGURLScheduler.DequeueCtx's empty-queue sleep (postgres_scheduler.go:236-243).
//
// These tests pin three properties:
//   1. With a known backoff and a known timeout, the dequeue call count is
//      bounded by ~timeout/backoff (not "thousands per second").
//   2. ctx cancellation during the backoff sleep still unblocks the loop
//      promptly (SIGTERM responsiveness preserved).
//   3. The (empty queue / no error) regression path is untouched — that
//      sleep lives in DequeueCtx itself, not the Run loop, so an in-process
//      stub that returns ("", nil) immediately must NOT be slowed by the
//      err-branch sleep.

func TestPioneerConsumer_Run_DequeueError_BackoffBoundsRetryRate(t *testing.T) {
	// With a 10ms backoff and a 50ms ctx window, the Run loop should issue
	// roughly 50/10 = 5 dequeue attempts, plus the first one before the
	// first sleep — call it 3 to 12. The pre-fix code hit ~50,000 in
	// 200ms (≈25× the post-fix rate per ms). Asserting an upper bound of
	// 50 catches any future regression that drops the sleep, without being
	// flaky on slow CI runners.
	const backoff = 10 * time.Millisecond
	const window = 50 * time.Millisecond

	sched := &fakeScheduler{
		dequeueScript: makePermanentErrScript("transient: connection reset", 200),
	}
	fetcher := &fakeFetcher{body: []byte("<html></html>"), finalURL: "https://a.example/x", statusCode: 200}
	store := &fakeSnapshotStore{}
	chain := NewFilterChain()
	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.budget = 1
	c.errorBackoff = backoff

	ctx, cancel := context.WithTimeout(context.Background(), window)
	defer cancel()

	start := time.Now()
	err := c.Run(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run must return ctx.DeadlineExceeded after the window expires under permanent dequeue err, got %v", err)
	}
	if elapsed > window+50*time.Millisecond {
		t.Fatalf("Run overshot the ctx window by more than 50ms (elapsed=%v window=%v) — backoff sleep is not honoring ctx cancellation", elapsed, window)
	}
	if sched.dequeueCalls > 50 {
		t.Fatalf("dequeue err branch must back off (got %d calls in %v with backoff=%v) — hot-spin regression", sched.dequeueCalls, window, backoff)
	}
	if sched.dequeueCalls < 1 {
		t.Fatalf("expected at least one dequeue attempt before the first sleep, got %d", sched.dequeueCalls)
	}
}

func TestPioneerConsumer_Run_DequeueError_CtxCancelDuringBackoffReturnsPromptly(t *testing.T) {
	// SIGTERM regression: an external cancel during the backoff sleep must
	// unblock Run within roughly one ctx-propagation tick, not wait out the
	// full backoff. We pick a backoff (500ms) that is large compared to the
	// cancel deadline (20ms) so a missing ctx.Done arm would force a wait
	// 25× longer than acceptable.
	const backoff = 500 * time.Millisecond
	const cancelAfter = 20 * time.Millisecond

	sched := &fakeScheduler{
		dequeueScript: makePermanentErrScript("transient: connection reset", 200),
	}
	fetcher := &fakeFetcher{body: []byte("<html></html>"), finalURL: "https://a.example/x", statusCode: 200}
	store := &fakeSnapshotStore{}
	chain := NewFilterChain()
	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.budget = 1
	c.errorBackoff = backoff

	ctx, cancel := context.WithTimeout(context.Background(), cancelAfter)
	defer cancel()

	start := time.Now()
	err := c.Run(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run must surface ctx.DeadlineExceeded on cancel during backoff, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("Run did not unblock promptly on ctx cancel during backoff (elapsed=%v, backoff=%v) — ctx.Done arm missing from the err-branch select", elapsed, backoff)
	}
}

func TestPioneerConsumer_Run_EmptyDequeue_NotSlowedByErrorBackoff(t *testing.T) {
	// Regression: the err-branch backoff must not affect the (empty queue /
	// no error) path. That path's sleep lives inside DequeueCtx itself —
	// PGURLScheduler.pollIntervalOrDefault on (claimed=false, no-err) —
	// not in the Run loop. With our in-process fakeScheduler returning
	// ("", nil) immediately and then a success, the Run loop should reach
	// the success within microseconds; setting errorBackoff to a huge value
	// must NOT delay it (because the empty path never enters the err branch).
	sched := &fakeScheduler{
		dequeueScript: []dequeueResult{
			{url: ""},                     // empty (no err) — must skip with no extra sleep
			{url: "https://a.example/u1"}, // success — finishes budget
		},
	}
	c := newBudgetConsumer(t, sched, []byte("<html></html>"), 1)
	// Production-sized backoff. If the regression breaks the err/empty
	// discrimination, this would make the test sit for 1s.
	c.errorBackoff = 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run on (empty, success) script must return nil at budget exhaustion, got %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("empty-dequeue regression: Run waited %v with errorBackoff=1s — err-branch sleep leaked into the empty path", elapsed)
	}
	if sched.dequeueCalls != 2 {
		t.Fatalf("expected 2 dequeue calls (1 empty + 1 success), got %d", sched.dequeueCalls)
	}
}

func TestHarvesterConsumer_Run_DequeueError_BackoffBoundsRetryRate(t *testing.T) {
	// Symmetric pin for HarvesterConsumer.Run: same upper-bound argument,
	// same hot-spin regression risk if the dequeue-err select disappears.
	const backoff = 10 * time.Millisecond
	const window = 50 * time.Millisecond

	sched := &fakeHarvestScheduler{dequeueErr: errors.New("permanent failure")}
	fetcher := newMapFetcher()
	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.errorBackoff = backoff

	ctx, cancel := context.WithTimeout(context.Background(), window)
	defer cancel()

	start := time.Now()
	err := c.Run(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run must return ctx.DeadlineExceeded under permanent dequeue err, got %v", err)
	}
	if elapsed > window+50*time.Millisecond {
		t.Fatalf("Run overshot the ctx window by more than 50ms (elapsed=%v window=%v) — backoff sleep is not honoring ctx cancellation", elapsed, window)
	}
	if sched.dequeueCalls > 50 {
		t.Fatalf("dequeue err branch must back off (got %d calls in %v with backoff=%v) — hot-spin regression", sched.dequeueCalls, window, backoff)
	}
	if sched.dequeueCalls < 1 {
		t.Fatalf("expected at least one dequeue attempt before the first sleep, got %d", sched.dequeueCalls)
	}
}

func TestHarvesterConsumer_Run_DequeueError_CtxCancelDuringBackoffReturnsPromptly(t *testing.T) {
	// Symmetric SIGTERM regression: same shape as Pioneer's variant.
	const backoff = 500 * time.Millisecond
	const cancelAfter = 20 * time.Millisecond

	sched := &fakeHarvestScheduler{dequeueErr: errors.New("permanent failure")}
	fetcher := newMapFetcher()
	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.errorBackoff = backoff

	ctx, cancel := context.WithTimeout(context.Background(), cancelAfter)
	defer cancel()

	start := time.Now()
	err := c.Run(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run must surface ctx.DeadlineExceeded on cancel during backoff, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("Run did not unblock promptly on ctx cancel during backoff (elapsed=%v, backoff=%v) — ctx.Done arm missing from the err-branch select", elapsed, backoff)
	}
}

func TestDequeueErrorBackoff_MatchesSchedulerPollIntervalIntent(t *testing.T) {
	// Drift guard: the constant must equal scheduler.defaultPollInterval
	// (1s, postgres_scheduler.go:27). The two are deliberately equal so
	// operators reason about quiet-because-healthy and quiet-because-broken
	// backoffs with one number. If either is bumped without the other, this
	// fires and forces a conscious decision.
	const want = 1 * time.Second
	if dequeueErrorBackoff != want {
		t.Fatalf("dequeueErrorBackoff drifted from scheduler.defaultPollInterval intent (1s): got %v, want %v", dequeueErrorBackoff, want)
	}
}

// makePermanentErrScript builds a dequeueScript that returns the same error
// for `n` calls. Used by hot-spin pin tests that need a script big enough
// that the budget loop never drains it within the ctx window.
func makePermanentErrScript(msg string, n int) []dequeueResult {
	out := make([]dequeueResult, n)
	for i := range out {
		out[i] = dequeueResult{err: errors.New(msg)}
	}
	return out
}
