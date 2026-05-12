package scheduler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/urlcanon"
)

// QueueType selects which frontier table (pioneer vs harvester) an
// Enqueue/Dequeue call targets. Spec: OpenSpec change `scheduler-claim-api`.
type QueueType string

const (
	QueuePioneer   QueueType = "pioneer"
	QueueHarvester QueueType = "harvester"
)

// Status is the enum accepted by SetStatus. Values are the four terminal
// outcomes a crawler worker can report. Spec: `scheduler-claim-api`.
type Status string

const (
	StatusFetched       Status = "fetched"
	StatusFetchFailed   Status = "fetch_failed"
	StatusHarvested     Status = "harvested"
	StatusHarvestFailed Status = "harvest_failed"
)

// ErrorKind is the enum accepted by RecordFetchError / RecordHarvestError.
// Spec: `scheduler-claim-api`.
type ErrorKind string

// Error-kind enum values accepted by the failure-reporting API.
const (
	ErrorHTTP4xx ErrorKind = "http_4xx"
	ErrorHTTP5xx ErrorKind = "http_5xx"
	ErrorNetwork ErrorKind = "network"
	ErrorTimeout ErrorKind = "timeout"
)

// HostRateLimiterIface is the minimal surface PGURLScheduler consumes from
// scheduler.HostRateLimiter. Declared as a narrow interface (not the concrete
// struct) so that tests can substitute a deterministic mock. The production
// implementation is *HostRateLimiter (host_rate_limiter.go).
type HostRateLimiterIface interface {
	Allow(host string) bool
}

// URLScheduler is the scheduler boundary between crawler workers (Pioneer /
// Harvester) and the Postgres frontier tables.
//
// Spec: OpenSpec change `scheduler-claim-api` defines the full five-method
// interface. `Dequeue` has block-on-empty and linearizable semantics. The
// failure-reporting methods (RecordFetchError/RecordHarvestError) were
// originally delivered by `scheduler-retry-backoff` and are now typed
// (ErrorKind) here.
//
// TODO(scheduler-claim-api): re-evaluate whether the failure-reporting methods
// should accept a context.Context. The current signature omits it; a follow-up
// refactor may want to propagate cancellation from the worker loop.
type URLScheduler interface {
	// Enqueue inserts one or more URLs into the given queue's frontier table.
	// Duplicates are no-ops for pioneer and conditional UPSERTs for harvester
	// (see DECISIONS §8 / scheduler-claim-api proposal).
	Enqueue(queueType QueueType, urls ...string) error
	// EnqueueHarvester is the snapshot-aware variant of the harvester enqueue
	// path. Unlike Enqueue(QueueHarvester, urls...), which does not touch
	// snapshot_key (baseline contract), this method writes `snapshotKey` into
	// the harvester_frontier row so Harvester can re-hydrate the exact
	// snapshot Pioneer saved. Used by Pioneer consumer's fanout-B step.
	// Semantics: UPSERT guarded by `harvested_at IS NULL` — already-harvested
	// rows are a no-op (no re-harvest). Spec: pioneer-scheduler-consumer
	// change / `specs/scheduler/spec.md` ADDED Requirement.
	EnqueueHarvester(url string, snapshotKey string) error
	// Dequeue blocks until a claimable URL is returned. Empty queue and host
	// throttle both trigger a 1-second sleep before retry. Linearizable via
	// FOR UPDATE SKIP LOCKED.
	Dequeue(queueType QueueType) (url string, err error)
	// SetStatus reports a terminal outcome for `key` (= normalized_url). For
	// StatusHarvested the caller may pass pin ids (UUIDs, matching pins.id) to
	// atomically insert into harvester_frontier_pins in the same transaction.
	SetStatus(key string, status Status, pinIDs []uuid.UUID) error
	// RecordFetchError reports a Pioneer fetch failure for the given key
	// (= normalized_url) with one of the four errorKind enum values.
	RecordFetchError(key string, errorKind ErrorKind) error
	// RecordHarvestError reports a Harvester harvest failure for the given
	// key with one of the four errorKind enum values.
	RecordHarvestError(key string, errorKind ErrorKind) error
}

// ErrUnknownErrorKind is returned when a caller passes an errorKind outside
// the allowed enum. The row is not modified. Callers MUST NOT map their own
// internal error types to this return; the errorKind string comes from
// fetcher-level classification which is the source of truth.
var ErrUnknownErrorKind = errors.New("scheduler: unknown errorKind")

// validateErrorKind returns ErrUnknownErrorKind (wrapped with the offending
// value) when k is outside the enum, and nil otherwise. Extracted so unit
// tests can exercise enum rejection without constructing a PGURLScheduler.
func validateErrorKind(k ErrorKind) error {
	switch k {
	case ErrorHTTP4xx, ErrorHTTP5xx, ErrorNetwork, ErrorTimeout:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownErrorKind, string(k))
	}
}

// PGURLScheduler is the Postgres-backed URLScheduler implementation. It wires
// the sqlc Queries bundle, a Clock for testable lease arithmetic, a Jitterer
// for backoff sampling, a HostRateLimiter for the claim protocol's host
// politeness check, and the raw *sql.DB so tryClaim can open its own
// transaction (SELECT ... FOR UPDATE SKIP LOCKED + UPDATE must share a tx).
//
// The struct is intentionally mutable via WithRateLimiter/WithLease/
// WithPollInterval so that bootstrap wiring can use the basic
// NewPGURLScheduler constructor and then attach the rate limiter once it's
// been constructed from config; tests override lease/poll without env vars.
type PGURLScheduler struct {
	db           *sql.DB
	queries      *db.Queries
	clock        Clock
	jitter       Jitterer
	rateLimiter  HostRateLimiterIface
	lease        time.Duration // zero = defaultLeaseDuration
	pollInterval time.Duration // zero = defaultPollInterval
	candidateN   int32         // populated from env at construction; zero = defaultCandidateN
}

// NewPGURLScheduler constructs the scheduler with operational defaults:
// a real wall-clock and the process-wide uniform PRNG jitterer. Tests that
// need deterministic timestamps should use NewPGURLSchedulerWithDeps.
// The returned scheduler has no rate limiter; the bootstrap path is expected
// to chain WithRateLimiter before calling Dequeue.
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
		db:         sqlDB,
		queries:    db.New(sqlDB),
		clock:      clock,
		jitter:     jitter,
		candidateN: int32(candidateNFromEnv()),
	}
}

// WithLease overrides the default 10-minute lease duration. Primarily for
// tests that need to exercise lease-expiry paths without waiting.
func (s *PGURLScheduler) WithLease(d time.Duration) *PGURLScheduler {
	s.lease = d
	return s
}

// WithPollInterval overrides the default 1-second empty-queue poll interval.
// Primarily for tests that want fast block-on-empty feedback without sleeping
// a real second per iteration.
func (s *PGURLScheduler) WithPollInterval(d time.Duration) *PGURLScheduler {
	s.pollInterval = d
	return s
}

// RecordFetchError implements URLScheduler.
func (s *PGURLScheduler) RecordFetchError(key string, errorKind ErrorKind) error {
	return s.recordError(key, errorKind, recordErrorOpsFetch)
}

// RecordHarvestError implements URLScheduler.
func (s *PGURLScheduler) RecordHarvestError(key string, errorKind ErrorKind) error {
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
// queries. The caller MUST pass an already-canonicalized URL — this is the
// enqueue path's hashing primitive (see prepareEnqueueBatch / EnqueueHarvester
// where `urlcanon.CanonicalWithHost` runs immediately upstream). Lookup paths
// (SetStatus / RecordFetchError / RecordHarvestError) instead receive a raw
// URL straight from a Dequeue return value or external caller, so they MUST
// use `hashLookupKey` which canonicalizes internally before hashing.
func hashKey(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}

// hashLookupKey returns (sha256(canonical(rawKey)), true) when rawKey
// canonicalizes to a non-empty URL, and (nil, false) when canonicalization
// yields an empty string. The bool signals "skip the DB call" so callers can
// short-circuit gracefully without a DB roundtrip and without panicking.
//
// Why canonicalize here: the spec contract for SetStatus/RecordFetchError/
// RecordHarvestError says the lookup hash MUST equal the hash that Enqueue
// stored. Enqueue runs `urlcanon.CanonicalWithHost(raw)` then sha256s the
// normalized result; lookup paths receive a raw URL (typically the Dequeue
// return value) so they must apply the same canonicalization before hashing
// or hash mismatch silently misses the row.
//
// `urlcanon.Canonical` is idempotent — applying it to already-canonical input
// is a no-op — so callers may pass either raw or normalized URLs safely.
func hashLookupKey(rawKey string) ([]byte, bool) {
	canonical := urlcanon.Canonical(rawKey)
	if canonical == "" {
		return nil, false
	}
	h := sha256.Sum256([]byte(canonical))
	return h[:], true
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
func (s *PGURLScheduler) recordError(key string, errorKind ErrorKind, ops recordErrorOps) error {
	if err := validateErrorKind(errorKind); err != nil {
		return err
	}
	ctx := context.Background()
	hash, ok := hashLookupKey(key)
	if !ok {
		// Canonicalization yielded an empty URL — skip the DB call. See the
		// matching short-circuit in SetStatus for the rationale.
		return nil
	}

	if errorKind == ErrorHTTP4xx {
		rows, err := ops.dead(ctx, s.queries, hash)
		if err != nil {
			return fmt.Errorf("scheduler: %s dead update: %w", ops.side, err)
		}
		warnIfUnknownKey(rows, "record_"+ops.side+"_error", key)
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
	warnIfUnknownKey(rows, "record_"+ops.side+"_error", key)
	return nil
}

// warnIfUnknownKey emits the shared "row not in frontier" warning. op is the
// full operation label (e.g. "record_fetch_error", "set_status_fetched"); the
// caller picks it so SetStatus paths don't get a misleading "record_*_error"
// prefix. Operators grep scheduler warnings by the `scheduler.<op>` prefix.
func warnIfUnknownKey(rows int64, op, key string) {
	if rows == 0 {
		log.Printf("WARN scheduler.%s: unknown key (row not in frontier); key=%q", op, key)
	}
}
