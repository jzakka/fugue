-- backfill_placeholder_media.sql
--
-- Identifies bot-created Pins whose `media_url` points to a placeholder
-- payload (e.g. the 1×1 GIF observed in the 2026-04-27 QA report,
-- b2136cc2-...gif, 37 bytes) so the operator can re-queue them through
-- the Harvester re-crawl path. design.md D6 keeps "재크롤 큐 재투입" as
-- the recovery contract; this query is the identification step.
--
-- This file ships with TWO queries — the application picks the right one
-- depending on whether ObjectStorage exposes a `bot_media_objects` view
-- or whether the operator has populated a temporary helper table.
--
-- ============================================================
-- Q1. Filename-pattern heuristic (no helper table required)
-- ============================================================
-- Bot-created Pins persist `media_url` under one of these legacy or
-- current shapes:
--   - `bot/<uuid>.<ext>`             (legacy harvest_pipeline.downloadAndUpload)
--   - `image/<uuid>.gif`             (legacy single-segment prefix referenced in QA)
--   - `images/<sha256>/<unix>.<ext>` (current cacheImage; sha-prefixed)
-- The 2026-04-27 QA bug surfaced placeholder objects ~37 bytes long with
-- a `.gif` extension. The current implementation rejects 1×1 placeholders
-- in cacheImage AND in downloadAndUpload before they hit the canonical key,
-- so this query targets the pre-deployment legacy shapes (`bot/` and
-- `/image/`). The operator combines this with an out-of-band MinIO size
-- probe (HEAD <object>, threshold per design.md D6) before re-queue.
SELECT
    p.id            AS pin_id,
    p.url           AS canonical_url,
    p.media_url     AS placeholder_media_url,
    p.created_at
FROM   pins p
WHERE  p.creator_id   = '00000000-0000-0000-0000-00000000f096' -- BotCreatorID
  AND  (p.media_url ILIKE '%/bot/%' OR p.media_url ILIKE '%/image/%')
  AND  p.media_url ILIKE '%.gif'
ORDER  BY p.created_at DESC;

-- ============================================================
-- Q2. Operator-side join after a `placeholder_keys` helper has been
--     populated by an out-of-band MinIO scan (bytes ≤ threshold).
-- ============================================================
-- Recommended flow when the operator has already produced a list of
-- ObjectStorage keys whose bytes are ≤ design.md D6 threshold:
--   1. Run `mc find --smaller 256B fugue-media/` (or equivalent) to
--      produce the full key list.
--   2. Bulk-load into a temporary table:
--        CREATE TEMP TABLE placeholder_keys(media_url text PRIMARY KEY);
--        \COPY placeholder_keys FROM '/tmp/placeholder_keys.txt';
--   3. Run the query below to obtain the Pin rows that need re-queue.
SELECT
    p.id            AS pin_id,
    p.url           AS canonical_url,
    p.media_url     AS placeholder_media_url,
    p.created_at
FROM   pins p
JOIN   placeholder_keys k ON k.media_url = p.media_url
WHERE  p.creator_id = '00000000-0000-0000-0000-00000000f096'
ORDER  BY p.created_at DESC;
