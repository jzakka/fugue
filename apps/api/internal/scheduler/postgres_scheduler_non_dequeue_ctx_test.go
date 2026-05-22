package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// These tests pin the pre-DB ctx.Err() fastpath added to the five non-Dequeue
// *Ctx variants (EnqueueCtx, EnqueueHarvesterCtx, SetStatusCtx,
// RecordFetchErrorCtx, RecordHarvestErrorCtx). Mirror of
// TestUnit_DequeueCtx_RejectsCancelledCtxBeforeAnyDBCall: with the
// scheduler's *sql.DB and *db.Queries left nil, any sqlc query call would
// panic on the nil receiver — so a passing test proves the ctx check ran
// first and short-circuited the function before any DB work.
//
// Why a manual ctx.Err() check matters even though the sqlc-generated
// *Context queries propagate ctx to the driver: the driver only sees ctx
// at query-send time, and reaching that path requires dereferencing
// s.queries.db (which is nil here). The pre-DB check is what guarantees
// the cancelled ctx returns immediately without touching the connection
// pool — important so callers can rely on near-zero overhead when they
// know they're cancelled, and so a misconfigured scheduler can't be
// driven into a nil-deref panic via a cancelled-ctx call.

func TestUnit_EnqueueCtx_RejectsCancelledCtxBeforeAnyDBCall(t *testing.T) {
	s := &PGURLScheduler{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := s.EnqueueCtx(ctx, QueuePioneer, "https://example.com/a")
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("EnqueueCtx err=%v, want context.Canceled (cancelled ctx must short-circuit before any DB call)", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("EnqueueCtx took %s on cancelled ctx — expected near-zero (ctx check before DB)", elapsed)
	}
}

func TestUnit_EnqueueHarvesterCtx_RejectsCancelledCtxBeforeAnyDBCall(t *testing.T) {
	s := &PGURLScheduler{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := s.EnqueueHarvesterCtx(ctx, "https://example.com/a", "snap-key")
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("EnqueueHarvesterCtx err=%v, want context.Canceled (cancelled ctx must short-circuit before any DB call)", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("EnqueueHarvesterCtx took %s on cancelled ctx — expected near-zero", elapsed)
	}
}

func TestUnit_SetStatusCtx_RejectsCancelledCtxBeforeAnyDBCall(t *testing.T) {
	s := &PGURLScheduler{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := s.SetStatusCtx(ctx, "https://example.com/a", StatusFetched, nil)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SetStatusCtx err=%v, want context.Canceled (cancelled ctx must short-circuit before any DB call)", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("SetStatusCtx took %s on cancelled ctx — expected near-zero", elapsed)
	}
}

// SetStatusCtx with StatusHarvested also short-circuits on cancelled ctx
// even though that branch goes through the multi-statement transaction
// (setStatusHarvested + BeginTx + InsertHarvesterFrontierPins). The ctx
// check at the top of SetStatusCtx runs before the switch, so the
// transaction is never started.
func TestUnit_SetStatusCtx_StatusHarvestedRejectsCancelledCtxBeforeBeginTx(t *testing.T) {
	s := &PGURLScheduler{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := s.SetStatusCtx(ctx, "https://example.com/a", StatusHarvested, []uuid.UUID{uuid.New()})
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SetStatusCtx(StatusHarvested) err=%v, want context.Canceled (cancelled ctx must short-circuit before BeginTx)", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("SetStatusCtx(StatusHarvested) took %s on cancelled ctx — expected near-zero", elapsed)
	}
}

func TestUnit_RecordFetchErrorCtx_RejectsCancelledCtxBeforeAnyDBCall(t *testing.T) {
	s := &PGURLScheduler{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := s.RecordFetchErrorCtx(ctx, "https://example.com/a", ErrorNetwork)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RecordFetchErrorCtx err=%v, want context.Canceled", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("RecordFetchErrorCtx took %s on cancelled ctx — expected near-zero", elapsed)
	}
}

func TestUnit_RecordHarvestErrorCtx_RejectsCancelledCtxBeforeAnyDBCall(t *testing.T) {
	s := &PGURLScheduler{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := s.RecordHarvestErrorCtx(ctx, "https://example.com/a", ErrorNetwork)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RecordHarvestErrorCtx err=%v, want context.Canceled", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("RecordHarvestErrorCtx took %s on cancelled ctx — expected near-zero", elapsed)
	}
}

// The five non-ctx wrappers are validated separately to lock in the
// backward-compat invariant that they delegate to *Ctx with
// context.Background() (which is never cancelled), so the cancelled-ctx
// short-circuit MUST NOT fire — instead the call proceeds past the pre-DB
// check and reaches the nil-DB panic on the first sqlc call. We use the
// known pre-DB validation branches (empty errorKind / empty snapshotKey /
// unknown QueueType) to exercise the wrapper without crossing the DB.

func TestUnit_RecordFetchError_BackwardCompatWrapper(t *testing.T) {
	s := &PGURLScheduler{}
	// Unknown ErrorKind is rejected by validateErrorKind before any DB call,
	// proving the wrapper path is alive without needing a real DB.
	err := s.RecordFetchError("https://example.com/a", ErrorKind("bogus"))
	if !errors.Is(err, ErrUnknownErrorKind) {
		t.Errorf("RecordFetchError got %v, want ErrUnknownErrorKind", err)
	}
}

func TestUnit_RecordHarvestError_BackwardCompatWrapper(t *testing.T) {
	s := &PGURLScheduler{}
	err := s.RecordHarvestError("https://example.com/a", ErrorKind("bogus"))
	if !errors.Is(err, ErrUnknownErrorKind) {
		t.Errorf("RecordHarvestError got %v, want ErrUnknownErrorKind", err)
	}
}

func TestUnit_EnqueueHarvester_BackwardCompatWrapper(t *testing.T) {
	s := &PGURLScheduler{}
	// Empty snapshotKey is rejected before any DB call.
	err := s.EnqueueHarvester("https://example.com/a", "")
	if err == nil {
		t.Fatalf("EnqueueHarvester(empty snapshotKey) want error, got nil")
	}
}

func TestUnit_Enqueue_BackwardCompatWrapper_NoOpOnEmptyUrls(t *testing.T) {
	s := &PGURLScheduler{}
	// Empty urls is a no-op and returns nil — exercises the wrapper without DB.
	if err := s.Enqueue(QueuePioneer); err != nil {
		t.Errorf("Enqueue(empty urls) got %v, want nil (no-op)", err)
	}
}

func TestUnit_SetStatus_BackwardCompatWrapper_UnknownKeySkipsDB(t *testing.T) {
	s := &PGURLScheduler{}
	// An empty/unparseable key short-circuits hashLookupKey → returns nil
	// without touching DB. Proves the wrapper path is alive.
	if err := s.SetStatus("", StatusFetched, nil); err != nil {
		t.Errorf("SetStatus(empty key) got %v, want nil (skip DB)", err)
	}
}
