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
