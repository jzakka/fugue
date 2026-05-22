package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestUnit_DequeueCtx_RejectsCancelledCtxBeforeAnyDBCall is the constructor-
// level guard against future refactors that might lose the ctx.Err()
// check at the top of the loop. With an already-cancelled context, the
// fastpath ctx.Err() check inside DequeueCtx MUST fire before any DB roundtrip
// (tryClaim → BeginTx). Because the scheduler's *sql.DB is nil here, any
// BeginTx call would panic — so a passing test proves the ctx check ran
// first. This is the cancellation analog to TestUnit_UnknownQueueTypeRejected
// which exercises another no-DB rejection path.
func TestUnit_DequeueCtx_RejectsCancelledCtxBeforeAnyDBCall(t *testing.T) {
	// Build a minimal scheduler sufficient for the pre-DB rejection branches.
	// rateLimiter must be non-nil (otherwise ErrRateLimiterRequired fires
	// first); a no-op limiter satisfies the constructor without touching DB.
	s := &PGURLScheduler{rateLimiter: allowAll{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE the call

	start := time.Now()
	url, err := s.DequeueCtx(ctx, QueuePioneer)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DequeueCtx err=%v, want context.Canceled (cancelled ctx must short-circuit before any DB call)", err)
	}
	if url != "" {
		t.Errorf("url=%q, want empty when ctx cancelled", url)
	}
	// The fastpath MUST be near-instant — no DB roundtrip, no pollInterval
	// wait. 100ms is a generous ceiling against scheduler jitter on slow CI.
	if elapsed > 100*time.Millisecond {
		t.Errorf("DequeueCtx took %s on cancelled ctx — expected near-zero (ctx check before tryClaim)", elapsed)
	}
}

// TestUnit_DequeueCtx_UnknownQueueTypeRejected mirrors
// TestUnit_UnknownQueueTypeRejected for the ctx-aware overload, ensuring the
// QueueType validation happens before any ctx-dependent work so the
// behaviour matches Dequeue's pre-DB rejection ordering.
func TestUnit_DequeueCtx_UnknownQueueTypeRejected(t *testing.T) {
	s := &PGURLScheduler{}
	_, err := s.DequeueCtx(context.Background(), QueueType("bogus"))
	if !errors.Is(err, ErrUnknownQueueType) {
		t.Errorf("DequeueCtx got %v, want ErrUnknownQueueType", err)
	}
}

// TestUnit_Dequeue_BackwardCompatWrapper guards the invariant that the
// no-ctx Dequeue is a thin wrapper over DequeueCtx(context.Background(),
// ...). We exercise the same fastpath rejection (unknown QueueType) on the
// no-ctx surface so any future refactor that splits the two implementations
// trips this test.
func TestUnit_Dequeue_BackwardCompatWrapper(t *testing.T) {
	s := &PGURLScheduler{}
	_, err := s.Dequeue(QueueType("bogus"))
	if !errors.Is(err, ErrUnknownQueueType) {
		t.Errorf("Dequeue got %v, want ErrUnknownQueueType", err)
	}
}

// TestIntegration_DequeueCtx_RespectsCancelledPollSleep covers the runtime-
// behaviour invariant that complements the unit fastpath above: with a real
// DB and an empty queue, DequeueCtx loops on (false, nil) into the poll
// select. When ctx is cancelled mid-poll, the select MUST unblock within a
// few ms instead of waiting up to one full pollInterval.
//
// Gated on TEST_DATABASE_URL via openTestDB(t). The pre-fix Dequeue used
// time.Sleep(pollInterval) and would hold the worker for up to 1s on
// SIGTERM during a quiescent queue — this test would observe that 1s wait
// and fail the 200ms bound.
func TestIntegration_DequeueCtx_RespectsCancelledPollSleep(t *testing.T) {
	sqlDB := openTestDB(t)
	// Use a long pollInterval so the test failure mode (waiting out the
	// sleep) is observable and not masked by a fast iteration.
	s := NewPGURLScheduler(sqlDB).
		WithRateLimiter(allowAll{}).
		WithPollInterval(2 * time.Second)

	host := uniqHost(t)
	t.Cleanup(func() { purgeByHost(t, sqlDB, host) })
	// Queue intentionally left empty for this host. Other tests may seed
	// pioneer_frontier rows, but if the worker happens to claim one the
	// test still passes (we check err type, not the URL value) — the goal
	// is to prove the *poll wait* unblocks on cancel, not that the queue
	// is observationally empty.

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan struct {
		url string
		err error
	}, 1)
	go func() {
		u, err := s.DequeueCtx(ctx, QueuePioneer)
		result <- struct {
			url string
			err error
		}{u, err}
	}()

	// Let the worker enter the poll select.
	time.Sleep(150 * time.Millisecond)
	cancelStart := time.Now()
	cancel()

	select {
	case r := <-result:
		elapsed := time.Since(cancelStart)
		// If the worker happened to claim a row right before cancel, that's
		// fine — we only fail when the *poll sleep* refused to unblock.
		if r.err != nil && !errors.Is(r.err, context.Canceled) {
			t.Fatalf("DequeueCtx err=%v, want nil or context.Canceled", r.err)
		}
		// 500ms generous bound: select{<-ctx.Done()} fires within a few
		// goroutine schedules. Pre-fix time.Sleep(2s) would blow this.
		if elapsed > 500*time.Millisecond {
			t.Errorf("DequeueCtx returned %s after cancel — expected <500ms (poll sleep must honour ctx.Done)", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("DequeueCtx never returned after cancel — poll sleep ignored ctx")
	}
}
