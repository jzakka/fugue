package scheduler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// URLScheduler is the scheduler boundary between crawler workers (Pioneer /
// Harvester) and the Postgres frontier tables.
//
// The full interface (Dequeue / Enqueue / SetStatus / RecordFetchError /
// RecordHarvestError) is specified by OpenSpec change `scheduler-claim-api`.
// This change (`scheduler-retry-backoff`) ships only the two failure-reporting
// methods; `scheduler-claim-api` will extend this interface with the remaining
// three methods. Keeping the name stable now lets downstream code type-assert
// against URLScheduler without a subsequent rename.
//
// TODO(scheduler-claim-api): re-evaluate whether RecordFetchError /
// RecordHarvestError should accept a context.Context. The current signature
// is fixed by scheduler-claim-api spec, but a follow-up refactor may want to
// propagate cancellation from the worker loop.
type URLScheduler interface {
	// RecordFetchError reports a Pioneer fetch failure for the given key
	// (= normalized_url) with one of the four errorKind enum values.
	RecordFetchError(key string, errorKind string) error
	// RecordHarvestError reports a Harvester harvest failure for the given
	// key with one of the four errorKind enum values.
	RecordHarvestError(key string, errorKind string) error
}

// Error-kind enum values accepted by the failure-reporting API.
const (
	ErrorKindHTTP4xx = "http_4xx"
	ErrorKindHTTP5xx = "http_5xx"
	ErrorKindNetwork = "network"
	ErrorKindTimeout = "timeout"
)

// ErrUnknownErrorKind is returned when a caller passes an errorKind outside
// the allowed enum. The row is not modified. Callers MUST NOT map their own
// internal error types to this return; the errorKind string comes from
// fetcher-level classification which is the source of truth.
var ErrUnknownErrorKind = errors.New("scheduler: unknown errorKind")

// validateErrorKind returns ErrUnknownErrorKind (wrapped with the offending
// value) when k is outside the enum, and nil otherwise. Extracted so unit
// tests can exercise enum rejection without constructing a PGURLScheduler.
func validateErrorKind(k string) error {
	switch k {
	case ErrorKindHTTP4xx, ErrorKindHTTP5xx, ErrorKindNetwork, ErrorKindTimeout:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownErrorKind, k)
	}
}

// PGURLScheduler is the Postgres-backed URLScheduler implementation. The
// current change only wires up the failure-reporting methods; dequeue /
// enqueue / setStatus are added by `scheduler-claim-api`.
type PGURLScheduler struct {
	queries *db.Queries
	clock   Clock
	jitter  Jitterer
}

// NewPGURLScheduler constructs the scheduler with operational defaults:
// a real wall-clock and the process-wide uniform PRNG jitterer. Tests that
// need deterministic timestamps should use NewPGURLSchedulerWithDeps.
func NewPGURLScheduler(sqlDB *sql.DB) *PGURLScheduler {
	return NewPGURLSchedulerWithDeps(sqlDB, RealClock(), defaultJitterer())
}

// NewPGURLSchedulerWithDeps is a constructor variant for tests and advanced
// callers that need to substitute the clock and jitter source. Passing a nil
// Clock or Jitterer falls back to the operational defaults.
func NewPGURLSchedulerWithDeps(sqlDB *sql.DB, clock Clock, jitter Jitterer) *PGURLScheduler {
	if clock == nil {
		clock = RealClock()
	}
	if jitter == nil {
		jitter = defaultJitterer()
	}
	return &PGURLScheduler{
		queries: db.New(sqlDB),
		clock:   clock,
		jitter:  jitter,
	}
}

// RecordFetchError implements URLScheduler.
func (s *PGURLScheduler) RecordFetchError(key string, errorKind string) error {
	return s.recordError(key, errorKind, recordErrorOpsFetch)
}

// RecordHarvestError implements URLScheduler.
func (s *PGURLScheduler) RecordHarvestError(key string, errorKind string) error {
	return s.recordError(key, errorKind, recordErrorOpsHarvest)
}

// recordErrorOps bundles the SQL entry points for one side of the frontier
// (pioneer vs harvester) so RecordFetchError and RecordHarvestError share the
// classification / logging code. Each entry point runs exactly one UPDATE
// statement; the backoff variant uses a CASE clause over five pre-jittered
// Go-computed timestamps to write fetch_error_count and next_*_at atomically.
type recordErrorOps struct {
	side string // "fetch" or "harvest" — used only in log lines.
	// dead sets the error count to 5 without touching next_*_at and returns
	// rows affected so the caller can detect unknown-key warnings.
	dead func(ctx context.Context, q *db.Queries, hash []byte) (int64, error)
	// backoff bumps the error count and picks next_*_at from the five
	// pre-jittered candidates corresponding to error_count_after = 1..5.
	backoff func(ctx context.Context, q *db.Queries, hash []byte, candidates [5]time.Time) (int64, error)
}

// recordErrorOpsFetch / recordErrorOpsHarvest are read-only dispatch tables
// binding each URLScheduler failure path to its sqlc entry points. They are
// package-level `var` (not `const`) only because Go's `const` cannot hold
// function values; nothing mutates them after init.
var recordErrorOpsFetch = recordErrorOps{
	side: "fetch",
	dead: func(ctx context.Context, q *db.Queries, hash []byte) (int64, error) {
		return q.UpdateFetchErrorDead(ctx, hash)
	},
	backoff: func(ctx context.Context, q *db.Queries, hash []byte, c [5]time.Time) (int64, error) {
		return q.UpdateFetchErrorBackoff(ctx, db.UpdateFetchErrorBackoffParams{
			UrlHash: hash,
			NextAt1: c[0], NextAt2: c[1], NextAt3: c[2], NextAt4: c[3], NextAt5: c[4],
		})
	},
}

var recordErrorOpsHarvest = recordErrorOps{
	side: "harvest",
	dead: func(ctx context.Context, q *db.Queries, hash []byte) (int64, error) {
		return q.UpdateHarvestErrorDead(ctx, hash)
	},
	backoff: func(ctx context.Context, q *db.Queries, hash []byte, c [5]time.Time) (int64, error) {
		return q.UpdateHarvestErrorBackoff(ctx, db.UpdateHarvestErrorBackoffParams{
			UrlHash: hash,
			NextAt1: c[0], NextAt2: c[1], NextAt3: c[2], NextAt4: c[3], NextAt5: c[4],
		})
	},
}

// hashKey returns sha256(key) as the BYTEA argument expected by the frontier
// queries. `key` is the normalized_url.
func hashKey(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}

// recordError is the shared implementation for RecordFetchError and
// RecordHarvestError. It (1) validates errorKind, (2) for http_4xx runs a
// single-statement dead UPDATE, (3) for the other three enum values runs a
// single UPDATE that increments the error count and selects next_*_at from
// five pre-computed candidate timestamps via a CASE clause. The caller never
// has to SELECT the current count, so the write is a true single statement.
//
// The ctx is created internally because the URLScheduler interface signature
// (defined by scheduler-claim-api) does not accept one. See the TODO note on
// the interface declaration for the follow-up migration.
func (s *PGURLScheduler) recordError(key, errorKind string, ops recordErrorOps) error {
	if err := validateErrorKind(errorKind); err != nil {
		return err
	}
	ctx := context.Background()
	hash := hashKey(key)

	if errorKind == ErrorKindHTTP4xx {
		rows, err := ops.dead(ctx, s.queries, hash)
		if err != nil {
			return fmt.Errorf("scheduler: %s dead update: %w", ops.side, err)
		}
		warnIfUnknownKey(rows, ops.side, key)
		return nil
	}

	// Non-4xx: pre-compute all five candidate next_*_at values in Go. The
	// UPDATE's CASE clause picks the one matching LEAST(count+1, 5). Each
	// candidate has an independent jitter sample so callers that retry
	// different failure kinds on the same row still observe a fresh jitter.
	//
	// Edge: if a row is already dead (count=5), LEAST(count+1, 5) = 5 keeps the
	// count idempotent but candidates[4] overwrites next_*_at with a fresh
	// 480s-offset timestamp. This is harmless because dead rows are excluded
	// from the claim partial index (count < 5), so the stale next_*_at is
	// never observed. The spec's "4xx does not change next_*_at" invariant
	// applies only to the 4xx path; the non-4xx path makes no such guarantee.
	now := s.clock.Now()
	var candidates [5]time.Time
	for i := 0; i < 5; i++ {
		n := i + 1
		delay := computeBackoff(n)
		candidates[i] = now.Add(delay + s.jitter(delay))
	}

	rows, err := ops.backoff(ctx, s.queries, hash, candidates)
	if err != nil {
		return fmt.Errorf("scheduler: update %s backoff: %w", ops.side, err)
	}
	warnIfUnknownKey(rows, ops.side, key)
	return nil
}

// warnIfUnknownKey emits the shared "row not in frontier" warning both the
// 4xx dead path and the non-4xx backoff path use. Extracted so the log format
// stays identical between the two call sites (operators grep for the prefix).
func warnIfUnknownKey(rows int64, side, key string) {
	if rows == 0 {
		log.Printf("WARN scheduler.record_%s_error: unknown key (row not in frontier); key=%q", side, key)
	}
}
