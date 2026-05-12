-- scheduler-frontier-table change:
-- pioneer_frontier / harvester_frontier / harvester_frontier_pins 를
-- 한 up 마이그레이션에서 함께 생성한다. down 스크립트는 FK 순서상
-- 조인 → harvester_frontier → pioneer_frontier 순으로 DROP 한다.

CREATE TABLE pioneer_frontier (
    id                  BIGSERIAL PRIMARY KEY,
    normalized_url      TEXT NOT NULL,
    url                 TEXT NOT NULL,
    url_hash            BYTEA NOT NULL,
    host                TEXT NOT NULL,
    depth               INTEGER NOT NULL DEFAULT 0,
    score               DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_fetched_at     TIMESTAMPTZ,
    next_fetch_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    fetch_error_count   INTEGER NOT NULL DEFAULT 0,
    last_updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT pioneer_frontier_url_hash_key UNIQUE (url_hash),
    CONSTRAINT pioneer_frontier_url_hash_len_chk CHECK (octet_length(url_hash) = 32)
);

CREATE INDEX pioneer_frontier_claimable_idx
    ON pioneer_frontier (score DESC, next_fetch_at ASC)
    WHERE fetch_error_count < 5;

CREATE TABLE harvester_frontier (
    id                    BIGSERIAL PRIMARY KEY,
    normalized_url        TEXT NOT NULL,
    url                   TEXT NOT NULL,
    url_hash              BYTEA NOT NULL,
    host                  TEXT NOT NULL,
    snapshot_key          TEXT,
    score                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    harvested_at          TIMESTAMPTZ,
    next_harvest_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    harvest_error_count   INTEGER NOT NULL DEFAULT 0,
    last_updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT harvester_frontier_url_hash_key UNIQUE (url_hash),
    CONSTRAINT harvester_frontier_url_hash_len_chk CHECK (octet_length(url_hash) = 32)
);

CREATE INDEX harvester_frontier_claimable_idx
    ON harvester_frontier (score DESC, next_harvest_at ASC)
    WHERE harvested_at IS NULL AND harvest_error_count < 5;

CREATE TABLE harvester_frontier_pins (
    frontier_id  BIGINT NOT NULL REFERENCES harvester_frontier(id) ON DELETE CASCADE,
    pin_id       UUID   NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    PRIMARY KEY (frontier_id, pin_id)
);
