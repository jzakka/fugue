package scheduler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Claim / lease tuning constants. Spec: scheduler-claim-api.
const (
	// defaultCandidateN is used when SCHEDULER_CLAIM_CANDIDATE_N is unset or
	// unparseable. Matches the proposal's default of 1.
	defaultCandidateN = 1
	// defaultPollInterval is the fixed sleep between claim retries when the
	// queue is empty or every candidate host is throttled. Spec pins this to
	// 1 second.
	defaultPollInterval = 1 * time.Second
	// defaultLeaseDuration is how far forward tryClaim pushes next_*_at when
	// it marks a row in flight. Spec pins this to 10 minutes.
	defaultLeaseDuration = 10 * time.Minute
)

// ErrUnknownQueueType is returned by Enqueue/Dequeue when the caller passes a
// QueueType outside the defined enum. PGURLScheduler never synthesizes new
// queue types internally; this error always indicates a caller bug.
var ErrUnknownQueueType = errors.New("scheduler: unknown QueueType")

// ErrUnknownStatus is returned by SetStatus for status values outside the
// four-element enum.
var ErrUnknownStatus = errors.New("scheduler: unknown Status")

// ErrRateLimiterRequired is returned by Dequeue when WithRateLimiter has not
// been called. The claim protocol MUST call HostRateLimiter.Allow(host) for
// every candidate, so silently nil-derefing on the first claim attempt would
// hide the bootstrap bug. Returning a real error surfaces it at the first
// Dequeue instead of panicking in the hot path.
var ErrRateLimiterRequired = errors.New("scheduler: Dequeue requires a HostRateLimiter (call WithRateLimiter)")

// WithRateLimiter attaches a HostRateLimiter to the scheduler. The scheduler
// can be constructed without one for tests that exercise paths that do not
// touch the claim protocol (e.g. pure RecordFetchError tests predating this
// change). Claim paths require a non-nil limiter; the tryClaim loop will
// panic nil-deref otherwise, which is acceptable because reaching the claim
// path with a nil limiter means bootstrap forgot to call this setter.
func (s *PGURLScheduler) WithRateLimiter(r HostRateLimiterIface) *PGURLScheduler {
	s.rateLimiter = r
	return s
}

// candidateNFromEnv returns the per-tryClaim candidate window size. A missing
// env var, empty value, or non-positive integer all fall back to the default.
// We log a warning on parse failure so operators can distinguish "not set"
// (silent) from "typo" (warned).
func candidateNFromEnv() int {
	raw := os.Getenv("SCHEDULER_CLAIM_CANDIDATE_N")
	if raw == "" {
		return defaultCandidateN
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		log.Printf("WARN scheduler.claim: SCHEDULER_CLAIM_CANDIDATE_N=%q invalid; using default %d", raw, defaultCandidateN)
		return defaultCandidateN
	}
	return n
}

// Enqueue implements URLScheduler. It normalizes each URL, derives url_hash,
// and runs a single batch insert against the chosen frontier table. pioneer
// uses `ON CONFLICT DO NOTHING` (idempotent); harvester uses a conditional
// UPSERT that reactivates un-harvested rows (DECISIONS §8).
//
// Empty urls slice is a no-op and returns nil — this matches the spec's
// "at least one" phrasing (0 inputs means 0 work, not an error).
func (s *PGURLScheduler) Enqueue(queueType QueueType, urls ...string) error {
	if len(urls) == 0 {
		return nil
	}
	normalized, raw, hashes, hosts, err := prepareEnqueueBatch(urls)
	if err != nil {
		return err
	}

	ctx := context.Background()
	switch queueType {
	case QueuePioneer:
		return s.queries.EnqueuePioneer(ctx, db.EnqueuePioneerParams{
			NormalizedUrls: normalized,
			Urls:           raw,
			UrlHashes:      hashes,
			Hosts:          hosts,
		})
	case QueueHarvester:
		return s.queries.EnqueueHarvester(ctx, db.EnqueueHarvesterParams{
			NormalizedUrls: normalized,
			Urls:           raw,
			UrlHashes:      hashes,
			Hosts:          hosts,
		})
	default:
		return fmt.Errorf("%w: %q", ErrUnknownQueueType, string(queueType))
	}
}

// EnqueueHarvester implements URLScheduler. Singular-URL UPSERT that writes
// snapshot_key alongside the row. The SQL (UpsertHarvesterWithSnapshot) is
// guarded by `WHERE harvested_at IS NULL`, so an already-harvested row is a
// no-op — the caller receives nil and the row is untouched. snapshot_key is
// written via sql.NullString; this method requires a non-empty snapshotKey
// (empty violates the spec's "snapshot_key를 호출 인자 값으로 세팅" clause).
//
// Spec: pioneer-scheduler-consumer change / scheduler spec ADDED Requirement.
func (s *PGURLScheduler) EnqueueHarvester(rawURL string, snapshotKey string) error {
	if snapshotKey == "" {
		return fmt.Errorf("scheduler: EnqueueHarvester requires non-empty snapshotKey")
	}
	nu, host, parseErr := normalizeURL(rawURL)
	if parseErr != nil {
		return fmt.Errorf("scheduler: EnqueueHarvester parse %q: %w", rawURL, parseErr)
	}
	h := sha256.Sum256([]byte(nu))
	ctx := context.Background()
	return s.queries.UpsertHarvesterWithSnapshot(ctx, db.UpsertHarvesterWithSnapshotParams{
		NormalizedUrl: nu,
		Url:           rawURL,
		UrlHash:       h[:],
		Host:          host,
		SnapshotKey:   sql.NullString{String: snapshotKey, Valid: true},
	})
}

// prepareEnqueueBatch parses each URL once and returns four parallel slices
// ready for UNNEST. Parse failure for any URL fails the whole batch because
// the caller contract (enqueue N URLs) expects atomic success — partial
// insertion would leak half-initialized rows and muddle retry semantics.
func prepareEnqueueBatch(raws []string) (normalized, rawOut []string, hashes [][]byte, hosts []string, err error) {
	normalized = make([]string, len(raws))
	rawOut = make([]string, len(raws))
	hashes = make([][]byte, len(raws))
	hosts = make([]string, len(raws))
	for i, r := range raws {
		nu, host, parseErr := normalizeURL(r)
		if parseErr != nil {
			return nil, nil, nil, nil, fmt.Errorf("scheduler: enqueue parse %q: %w", r, parseErr)
		}
		normalized[i] = nu
		rawOut[i] = r
		h := sha256.Sum256([]byte(nu))
		hashes[i] = h[:]
		hosts[i] = host
	}
	return normalized, rawOut, hashes, hosts, nil
}

// normalizeURL is the scheduler's own minimal URL normalization — lowercase
// scheme+host, trim default ports, strip fragment. This is intentionally a
// thin function: the canonical normalizer lives in the crawler fetcher and
// will replace this in a follow-up change. For now, the scheduler accepts
// URLs as-is and only guarantees that two structurally-identical URLs hash
// the same. Empty or schemeless inputs error out so they cannot sneak into
// the url_hash index with ambiguous keys.
func normalizeURL(raw string) (normalized, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("url missing scheme or host: %q", raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	// Strip default ports so http://x:80/ and http://x/ hash the same.
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) ||
		(u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = strings.SplitN(u.Host, ":", 2)[0]
	}
	return u.String(), u.Hostname(), nil
}

// Dequeue implements URLScheduler. It loops, calling tryClaim until a URL
// can be claimed. Empty queue and host throttle both trigger the same
// 1-second sleep — the spec intentionally does not distinguish them, which
// keeps the loop branch-free and eliminates a whole class of "which timer
// fires first" bugs.
//
// NOTE (future context-support): the URLScheduler interface does not take a
// context.Context in this change's scope, so Dequeue blocks indefinitely
// until a claim succeeds or a DB error occurs. Graceful shutdown is the
// responsibility of a follow-up change that adds a context-taking overload;
// today, operators relying on worker restart must accept up to one
// pollInterval of quiescence before the process can exit.
func (s *PGURLScheduler) Dequeue(queueType QueueType) (string, error) {
	if queueType != QueuePioneer && queueType != QueueHarvester {
		return "", fmt.Errorf("%w: %q", ErrUnknownQueueType, string(queueType))
	}
	if s.rateLimiter == nil {
		return "", ErrRateLimiterRequired
	}
	for {
		url, claimed, err := s.tryClaim(queueType)
		if err != nil {
			return "", err
		}
		if claimed {
			return url, nil
		}
		time.Sleep(s.pollIntervalOrDefault())
	}
}

// pollIntervalOrDefault returns the configured poll interval or the spec
// default. Centralizing the zero-value fallback here keeps Dequeue free of
// branching and makes WithPollInterval the single mutation site.
func (s *PGURLScheduler) pollIntervalOrDefault() time.Duration {
	if s.pollInterval > 0 {
		return s.pollInterval
	}
	return defaultPollInterval
}

// tryClaim runs the full claim protocol in a single Postgres transaction.
// Returns (url, true, nil) on success; (_, false, nil) when the queue is
// empty OR every candidate's host is throttled; (_, false, err) on a real
// DB error. The caller loops on the (false, nil) case with a fixed sleep.
//
// IMPORTANT: the SELECT (ClaimPioneerCandidates) and the UPDATE
// (MarkPioneerInFlight) MUST run inside the same transaction so that
// FOR UPDATE SKIP LOCKED's row locks are still held when the UPDATE fires.
// Splitting them across transactions opens a window where another worker
// could re-claim the same row between SELECT commit and UPDATE begin.
func (s *PGURLScheduler) tryClaim(queueType QueueType) (string, bool, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("scheduler: begin claim tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	qtx := s.queries.WithTx(tx)
	n := s.candidateN
	if n < 1 {
		n = defaultCandidateN
	}

	var candidates []claimCandidate
	switch queueType {
	case QueuePioneer:
		rows, err := qtx.ClaimPioneerCandidates(ctx, n)
		if err != nil {
			return "", false, fmt.Errorf("scheduler: claim pioneer candidates: %w", err)
		}
		candidates = make([]claimCandidate, len(rows))
		for i, r := range rows {
			candidates[i] = claimCandidate{id: r.ID, url: r.Url, host: r.Host}
		}
	case QueueHarvester:
		rows, err := qtx.ClaimHarvesterCandidates(ctx, n)
		if err != nil {
			return "", false, fmt.Errorf("scheduler: claim harvester candidates: %w", err)
		}
		candidates = make([]claimCandidate, len(rows))
		for i, r := range rows {
			candidates[i] = claimCandidate{id: r.ID, url: r.Url, host: r.Host}
		}
	default:
		// Already validated in Dequeue; keep the switch exhaustive for future
		// QueueType values so the compiler does not silently fall through.
		return "", false, fmt.Errorf("%w: %q", ErrUnknownQueueType, string(queueType))
	}

	for _, c := range candidates {
		if !s.rateLimiter.Allow(c.host) {
			continue
		}
		leaseUntil := s.clock.Now().Add(s.leaseDuration())
		switch queueType {
		case QueuePioneer:
			if err := qtx.MarkPioneerInFlight(ctx, db.MarkPioneerInFlightParams{
				NextFetchAt: leaseUntil,
				ID:          c.id,
			}); err != nil {
				return "", false, fmt.Errorf("scheduler: mark pioneer in-flight: %w", err)
			}
		case QueueHarvester:
			if err := qtx.MarkHarvesterInFlight(ctx, db.MarkHarvesterInFlightParams{
				NextHarvestAt: leaseUntil,
				ID:            c.id,
			}); err != nil {
				return "", false, fmt.Errorf("scheduler: mark harvester in-flight: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return "", false, fmt.Errorf("scheduler: commit claim: %w", err)
		}
		committed = true
		return c.url, true, nil
	}

	// No candidates winner: rollback happens in the deferred cleanup. Empty
	// candidate list (queue empty) and all-throttled both route here.
	return "", false, nil
}

// leaseDuration is a method so future changes can make it configurable per
// QueueType or via env var without widening PGURLScheduler's struct.
func (s *PGURLScheduler) leaseDuration() time.Duration {
	if s.lease != 0 {
		return s.lease
	}
	return defaultLeaseDuration
}

type claimCandidate struct {
	id   int64
	url  string
	host string
}

// SetStatus implements URLScheduler. status dispatches to one UPDATE per
// branch; StatusHarvested additionally inserts pin-id rows inside the same
// transaction as the status UPDATE so a pins INSERT failure rolls the
// harvested_at flip back.
func (s *PGURLScheduler) SetStatus(key string, status Status, pinIDs []uuid.UUID) error {
	ctx := context.Background()
	hash := hashKey(key)

	switch status {
	case StatusFetched:
		rows, err := s.queries.SetStatusFetched(ctx, hash)
		if err != nil {
			return fmt.Errorf("scheduler: set_status_fetched: %w", err)
		}
		warnIfUnknownKey(rows, "set_status_fetched", key)
		return nil

	case StatusFetchFailed:
		rows, err := s.queries.SetStatusFetchFailed(ctx, hash)
		if err != nil {
			return fmt.Errorf("scheduler: set_status_fetch_failed: %w", err)
		}
		warnIfUnknownKey(rows, "set_status_fetch_failed", key)
		return nil

	case StatusHarvested:
		return s.setStatusHarvested(ctx, key, hash, pinIDs)

	case StatusHarvestFailed:
		rows, err := s.queries.SetStatusHarvestFailed(ctx, hash)
		if err != nil {
			return fmt.Errorf("scheduler: set_status_harvest_failed: %w", err)
		}
		warnIfUnknownKey(rows, "set_status_harvest_failed", key)
		return nil

	default:
		return fmt.Errorf("%w: %q", ErrUnknownStatus, string(status))
	}
}

// setStatusHarvested runs the UPDATE-then-INSERT pair in one transaction.
// Empty pinIDs skips the INSERT entirely — both to avoid a no-op roundtrip
// and because UNNEST of an empty uuid[] is a portability hazard.
// Unknown-key handling: the UPDATE RETURNING id query returns sql.ErrNoRows
// when no frontier row matches; we swallow that and warn, matching the
// fetch side's warnIfUnknownKey convention.
func (s *PGURLScheduler) setStatusHarvested(ctx context.Context, key string, hash []byte, pinIDs []uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("scheduler: begin harvested tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	qtx := s.queries.WithTx(tx)
	frontierID, err := qtx.SetStatusHarvested(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Unknown key: warn and commit the empty tx. Rollback also works
			// (tx saw no writes) but committing keeps log lines symmetric
			// with the fetch side.
			warnIfUnknownKey(0, "set_status_harvested", key)
			if commitErr := tx.Commit(); commitErr != nil {
				return fmt.Errorf("scheduler: commit empty harvested tx: %w", commitErr)
			}
			committed = true
			return nil
		}
		return fmt.Errorf("scheduler: set_status_harvested: %w", err)
	}

	if len(pinIDs) > 0 {
		if err := qtx.InsertHarvesterFrontierPins(ctx, db.InsertHarvesterFrontierPinsParams{
			FrontierID: frontierID,
			PinIds:     pinIDs,
		}); err != nil {
			return fmt.Errorf("scheduler: insert_harvester_frontier_pins: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("scheduler: commit harvested tx: %w", err)
	}
	committed = true
	return nil
}
