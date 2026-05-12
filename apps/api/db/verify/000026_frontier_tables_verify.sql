-- Verification script for scheduler-frontier-table change (migration 000026).
-- Run after `make migrate` against a dev DB. Non-destructive (cleans up its own rows).
-- Covers tasks 5.1~5.8 in openspec/changes/scheduler-frontier-table/tasks.md.
--
-- Usage:
--   docker exec -i fugue-postgres-1 psql -U fugue -d fugue \
--     < apps/api/db/verify/000026_frontier_tables_verify.sql

\echo '== 5.1 schema existence =='
\d pioneer_frontier
\d harvester_frontier
\d harvester_frontier_pins

\echo '== 5.2 pioneer claim uses partial index =='
SET enable_seqscan = off;
EXPLAIN SELECT *
FROM pioneer_frontier
WHERE fetch_error_count < 5 AND next_fetch_at <= now()
ORDER BY score DESC, next_fetch_at ASC
LIMIT 10;

\echo '== 5.3 harvester claim uses partial index =='
EXPLAIN SELECT *
FROM harvester_frontier
WHERE harvested_at IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()
ORDER BY score DESC, next_harvest_at ASC
LIMIT 10;
RESET enable_seqscan;

\echo '== 5.4 unique violation (pioneer) =='
BEGIN;
INSERT INTO pioneer_frontier (normalized_url, url, url_hash, host)
VALUES ('https://verify.test/', 'https://verify.test/',
        decode('00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test');
-- expected: duplicate key violation
INSERT INTO pioneer_frontier (normalized_url, url, url_hash, host)
VALUES ('https://verify.test/', 'https://verify.test/',
        decode('00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test');
ROLLBACK;

\echo '== 5.4 unique violation (harvester) =='
BEGIN;
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host)
VALUES ('https://verify.test/', 'https://verify.test/',
        decode('aa112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test');
-- expected: duplicate key violation
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host)
VALUES ('https://verify.test/', 'https://verify.test/',
        decode('aa112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test');
ROLLBACK;

\echo '== 5.5 CASCADE on harvester_frontier delete =='
DO $$
DECLARE
  cid UUID;
  pid UUID;
  fid BIGINT;
  cnt_before INT;
  cnt_after INT;
BEGIN
  INSERT INTO creators (nickname) VALUES ('verify_frontier_tmp') RETURNING id INTO cid;
  INSERT INTO pins (creator_id, url, title, media_url, media_type)
  VALUES (cid, 'https://verify.test/pin', 'tmp', 'https://verify.test/img.jpg', 'image')
  RETURNING id INTO pid;
  INSERT INTO harvester_frontier (normalized_url, url, url_hash, host)
  VALUES ('https://verify.test/cascade', 'https://verify.test/cascade',
          decode('cc112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
          'verify.test')
  RETURNING id INTO fid;
  INSERT INTO harvester_frontier_pins (frontier_id, pin_id) VALUES (fid, pid);
  SELECT count(*) INTO cnt_before FROM harvester_frontier_pins WHERE frontier_id = fid;
  DELETE FROM harvester_frontier WHERE id = fid;
  SELECT count(*) INTO cnt_after FROM harvester_frontier_pins WHERE frontier_id = fid;
  RAISE NOTICE 'cascade: before=%, after=% (expected before=1, after=0)', cnt_before, cnt_after;
  DELETE FROM pins WHERE id = pid;
  DELETE FROM creators WHERE id = cid;
END $$;

\echo '== 5.6 UPSERT no-op when already harvested =='
BEGIN;
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, snapshot_key, harvested_at)
VALUES ('https://verify.test/done', 'https://verify.test/done',
        decode('dd112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test', 'snap-old', now());
-- guarded UPSERT must NOT overwrite
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, snapshot_key)
VALUES ('https://verify.test/done', 'https://verify.test/done',
        decode('dd112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test', 'snap-NEW')
ON CONFLICT (url_hash) DO UPDATE
  SET snapshot_key = EXCLUDED.snapshot_key,
      next_harvest_at = now(),
      harvest_error_count = 0,
      last_updated_at = now()
  WHERE harvester_frontier.harvested_at IS NULL;
-- expected: snap-old
SELECT snapshot_key FROM harvester_frontier
WHERE url_hash = decode('dd112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex');
ROLLBACK;

\echo '== 5.7 UPSERT updates when harvested_at IS NULL =='
BEGIN;
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, snapshot_key, harvest_error_count)
VALUES ('https://verify.test/open', 'https://verify.test/open',
        decode('ee112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test', 'snap-old', 2);
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, snapshot_key)
VALUES ('https://verify.test/open', 'https://verify.test/open',
        decode('ee112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex'),
        'verify.test', 'snap-NEW')
ON CONFLICT (url_hash) DO UPDATE
  SET snapshot_key = EXCLUDED.snapshot_key,
      next_harvest_at = now(),
      harvest_error_count = 0,
      last_updated_at = now()
  WHERE harvester_frontier.harvested_at IS NULL;
-- expected: snap-NEW, harvest_error_count = 0
SELECT snapshot_key, harvest_error_count FROM harvester_frontier
WHERE url_hash = decode('ee112233445566778899aabbccddeeff00112233445566778899aabbccddeeff', 'hex');
ROLLBACK;

\echo '== 5.8 url_hash length CHECK =='
BEGIN;
-- expected: CHECK constraint violation
INSERT INTO pioneer_frontier (normalized_url, url, url_hash, host)
VALUES ('https://verify.test/short', 'https://verify.test/short',
        decode('0011', 'hex'), 'verify.test');
ROLLBACK;
