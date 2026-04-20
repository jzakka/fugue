-- name: CountPioneerFrontier :one
-- Placeholder query for scheduler-frontier-table change.
-- Real claim/enqueue/fanout queries land in scheduler-claim-api.
SELECT count(*) FROM pioneer_frontier;

-- name: CountHarvesterFrontier :one
-- Placeholder query for scheduler-frontier-table change.
-- Real claim/enqueue/fanout queries land in scheduler-claim-api.
SELECT count(*) FROM harvester_frontier;

-- name: CountHarvesterFrontierPins :one
-- Placeholder query for scheduler-frontier-table change.
-- Real claim/enqueue/fanout queries land in scheduler-claim-api.
SELECT count(*) FROM harvester_frontier_pins;

-- scheduler-retry-backoff change:
-- Failure reporting queries. spec.md requires that the count increment and
-- the next_*_at update be visible as a single atomic write (SHALL NOT tear
-- across readers), so each RecordFetchError / RecordHarvestError is expressed
-- as ONE UPDATE statement. The caller pre-computes five candidate
-- T_report + delay(n) + jitter(n) timestamps (one per error_count_after
-- ∈ {1..5}) and the CASE clause in the UPDATE picks the one matching the
-- post-increment count. This keeps the jitter computation in Go (per design
-- decision "공식 계산은 Go app, time.Now() 기준") while still writing both
-- columns in a single statement.

-- name: UpdateFetchErrorDead :execrows
-- http_4xx path: set fetch_error_count to the dead threshold immediately.
-- next_fetch_at is intentionally NOT updated; the row is excluded from the
-- partial index (fetch_error_count < 5), so its backoff timestamp is unused.
-- :execrows lets the caller detect an unknown key (rows affected = 0) and
-- log a warning without synthesizing a row.
UPDATE pioneer_frontier
SET fetch_error_count = 5,
    last_updated_at = now()
WHERE url_hash = $1;

-- name: UpdateFetchErrorBackoff :execrows
-- Non-4xx path. Caller passes ts1..ts5 = T_report + delay(n) + jitter(n)
-- for n = 1..5 (all pre-jittered in Go). LEAST(count+1, 5) keeps index
-- lookup safe even if a race managed to bump count past 5 (not expected in
-- practice because dead rows are excluded from claim). The LEAST expression
-- is evaluated twice per UPDATE — once to set fetch_error_count, once as
-- the CASE selector — on the same row snapshot, so both evaluations see the
-- same fetch_error_count value (PostgreSQL single-statement semantics).
UPDATE pioneer_frontier
SET fetch_error_count = LEAST(fetch_error_count + 1, 5),
    next_fetch_at = CASE LEAST(fetch_error_count + 1, 5)
        WHEN 1 THEN sqlc.arg(next_at1)::timestamptz
        WHEN 2 THEN sqlc.arg(next_at2)::timestamptz
        WHEN 3 THEN sqlc.arg(next_at3)::timestamptz
        WHEN 4 THEN sqlc.arg(next_at4)::timestamptz
        ELSE       sqlc.arg(next_at5)::timestamptz
    END,
    last_updated_at = now()
WHERE url_hash = sqlc.arg(url_hash);

-- name: UpdateHarvestErrorDead :execrows
-- http_4xx path for the harvester frontier.
UPDATE harvester_frontier
SET harvest_error_count = 5,
    last_updated_at = now()
WHERE url_hash = $1;

-- name: UpdateHarvestErrorBackoff :execrows
-- Harvester counterpart of UpdateFetchErrorBackoff. See that query for the
-- LEAST-evaluated-twice semantics note.
UPDATE harvester_frontier
SET harvest_error_count = LEAST(harvest_error_count + 1, 5),
    next_harvest_at = CASE LEAST(harvest_error_count + 1, 5)
        WHEN 1 THEN sqlc.arg(next_at1)::timestamptz
        WHEN 2 THEN sqlc.arg(next_at2)::timestamptz
        WHEN 3 THEN sqlc.arg(next_at3)::timestamptz
        WHEN 4 THEN sqlc.arg(next_at4)::timestamptz
        ELSE       sqlc.arg(next_at5)::timestamptz
    END,
    last_updated_at = now()
WHERE url_hash = sqlc.arg(url_hash);

-- scheduler-claim-api change:
-- Enqueue / claim / status / record-error queries implementing the
-- URLScheduler interface. Row lookup uses url_hash (sha256(normalized_url))
-- exclusively because the BYTEA index is more selective than the TEXT
-- normalized_url column.

-- name: EnqueuePioneer :exec
-- Pioneer enqueue. Batch upsert via UNNEST-parallel arrays. ON CONFLICT on
-- url_hash keeps enqueue idempotent: a re-enqueue of a URL already in the
-- frontier does not touch score, depth, or next_fetch_at. depth=0 and
-- score=0.0 are spec-mandated defaults for URL-only Enqueue (proposal §
-- Enqueue; structured enqueue lands in a future change).
INSERT INTO pioneer_frontier (normalized_url, url, url_hash, host, depth, score)
SELECT
    UNNEST(sqlc.arg(normalized_urls)::text[]),
    UNNEST(sqlc.arg(urls)::text[]),
    UNNEST(sqlc.arg(url_hashes)::bytea[]),
    UNNEST(sqlc.arg(hosts)::text[]),
    0,
    0.0
ON CONFLICT (url_hash) DO NOTHING;

-- name: EnqueueHarvester :exec
-- Harvester enqueue. Conditional UPSERT per DECISIONS §8: re-enqueue of a
-- URL that is still awaiting harvest (harvested_at IS NULL) resets the
-- backoff schedule so the claim picks it up immediately; re-enqueue of an
-- already-harvested URL is a no-op. snapshot_key is deliberately not set
-- here — structured enqueue (harvester-scheduler-consumer) owns that.
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, score)
SELECT
    UNNEST(sqlc.arg(normalized_urls)::text[]),
    UNNEST(sqlc.arg(urls)::text[]),
    UNNEST(sqlc.arg(url_hashes)::bytea[]),
    UNNEST(sqlc.arg(hosts)::text[]),
    0.0
ON CONFLICT (url_hash) DO UPDATE
SET next_harvest_at = now(),
    harvest_error_count = 0,
    last_updated_at = now()
WHERE harvester_frontier.harvested_at IS NULL;

-- name: UpsertHarvesterWithSnapshot :exec
-- pioneer-scheduler-consumer change: singular UPSERT used by Pioneer
-- consumer's fanout-B path. Writes snapshot_key alongside the frontier row
-- so Harvester can pick the exact snapshot to re-hydrate. Guarded by
-- `WHERE harvester_frontier.harvested_at IS NULL` so an already-harvested
-- row is a no-op (spec: scheduler EnqueueHarvester ADDED Requirement,
-- "이미 harvest된 URL은 no-op"). For un-harvested rows snapshot_key is
-- overwritten with EXCLUDED.snapshot_key (= caller's argument), and both
-- next_harvest_at and harvest_error_count are reset so the row becomes
-- immediately claimable.
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, snapshot_key, score)
VALUES (sqlc.arg(normalized_url), sqlc.arg(url), sqlc.arg(url_hash), sqlc.arg(host), sqlc.arg(snapshot_key), 0.0)
ON CONFLICT (url_hash) DO UPDATE
SET snapshot_key = EXCLUDED.snapshot_key,
    next_harvest_at = now(),
    harvest_error_count = 0,
    last_updated_at = now()
WHERE harvester_frontier.harvested_at IS NULL;

-- name: ClaimPioneerCandidates :many
-- Top-N claim candidates from the pioneer partial index. SKIP LOCKED ensures
-- that two workers running the same query in parallel see disjoint rows.
-- The outer transaction (opened by the Go caller) keeps the lock until
-- mark_pioneer_in_flight or ROLLBACK; the claim protocol (scheduler-claim-api
-- Decision: single transaction) forbids splitting SELECT and UPDATE.
SELECT id, url, host
FROM pioneer_frontier
WHERE fetch_error_count < 5
  AND next_fetch_at <= now()
ORDER BY score DESC, next_fetch_at ASC
LIMIT sqlc.arg(n)
FOR UPDATE SKIP LOCKED;

-- name: ClaimHarvesterCandidates :many
-- Harvester counterpart of ClaimPioneerCandidates.
SELECT id, url, host
FROM harvester_frontier
WHERE harvested_at IS NULL
  AND harvest_error_count < 5
  AND next_harvest_at <= now()
ORDER BY score DESC, next_harvest_at ASC
LIMIT sqlc.arg(n)
FOR UPDATE SKIP LOCKED;

-- name: MarkPioneerInFlight :exec
-- In-flight marker. next_fetch_at is passed in from Go (clock.Now() + lease)
-- so tests can inject a fakeClock to simulate lease expiry without sleeping.
-- No separate in_flight column; pushing next_fetch_at forward removes the
-- row from the claim partial index, and an unclean worker crash will
-- naturally restore visibility once the lease expires.
UPDATE pioneer_frontier
SET next_fetch_at = sqlc.arg(next_fetch_at)::timestamptz,
    last_updated_at = now()
WHERE id = sqlc.arg(id);

-- name: MarkHarvesterInFlight :exec
UPDATE harvester_frontier
SET next_harvest_at = sqlc.arg(next_harvest_at)::timestamptz,
    last_updated_at = now()
WHERE id = sqlc.arg(id);

-- name: SetStatusFetched :execrows
-- Pioneer success. 365-day re-crawl schedule per DECISIONS §8. Error count
-- reset here rather than in a separate RecordFetchSuccess because success
-- is reported via SetStatus per consumer contract.
UPDATE pioneer_frontier
SET last_fetched_at = now(),
    next_fetch_at = now() + interval '365 days',
    fetch_error_count = 0,
    last_updated_at = now()
WHERE url_hash = sqlc.arg(url_hash);

-- name: SetStatusFetchFailed :execrows
-- Failure marking only. Error-count bump and next_fetch_at backoff are
-- RecordFetchError's responsibility (consumer contract: failure → both
-- SetStatus + RecordFetchError are invoked).
UPDATE pioneer_frontier
SET last_updated_at = now()
WHERE url_hash = sqlc.arg(url_hash);

-- name: SetStatusHarvested :one
-- Harvester success. Returns the frontier_id so the caller can insert the
-- pin-id mapping in the same transaction.
UPDATE harvester_frontier
SET harvested_at = now(),
    harvest_error_count = 0,
    last_updated_at = now()
WHERE url_hash = sqlc.arg(url_hash)
RETURNING id;

-- name: SetStatusHarvestFailed :execrows
UPDATE harvester_frontier
SET last_updated_at = now()
WHERE url_hash = sqlc.arg(url_hash);

-- name: InsertHarvesterFrontierPins :exec
-- Batch insert of (frontier_id, pin_id) pairs. pin_id is UUID — pins.id and
-- harvester_frontier_pins.pin_id are both UUID (migration 000003, 000026).
INSERT INTO harvester_frontier_pins (frontier_id, pin_id)
SELECT sqlc.arg(frontier_id), UNNEST(sqlc.arg(pin_ids)::uuid[]);

-- The record-fetch-error / record-harvest-error SQL entry points are
-- already provided by scheduler-retry-backoff (UpdateFetchErrorDead /
-- UpdateFetchErrorBackoff / UpdateHarvestErrorDead / UpdateHarvestErrorBackoff
-- above). scheduler-claim-api reuses them unchanged — the URLScheduler
-- implementation's RecordFetchError/RecordHarvestError dispatch through the
-- recordErrorOps table defined in url_scheduler.go. See tasks.md §2.12-2.15
-- notes: those entries are satisfied by the existing Update* queries, and
-- duplicating them under new names would fork the CASE-based backoff
-- selector (which needs the five pre-jittered candidates to remain atomic).
