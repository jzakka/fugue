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
