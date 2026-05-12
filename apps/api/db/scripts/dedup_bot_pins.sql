-- One-off dedup script for bot-owned Pins that share the same canonical URL.
--
-- Run this BEFORE applying migration 000027_add_pins_url_bot_unique.up.sql,
-- otherwise CREATE UNIQUE INDEX CONCURRENTLY will fail on duplicate rows.
--
-- Strategy:
--   (a) Pick the most recent `created_at` row per URL as the survivor.
--   (b) Re-point every join row in `harvester_frontier_pins` that references
--       a soon-to-be-deleted duplicate at the survivor pin_id. This preserves
--       the `og_data.source → frontier URL` back-reference. ON DELETE CASCADE
--       would otherwise drop those rows entirely when we delete duplicates.
--   (c) Delete the remaining duplicate bot Pins.
--
-- Tables that currently reference `pins.id`:
--   - harvester_frontier_pins.pin_id (FK w/ ON DELETE CASCADE)
--   - pin_tags.pin_id                (FK w/ ON DELETE CASCADE)
--   - interactions.target_id         (no FK, polymorphic by target_type)
--   - board_pins.pin_id              (FK w/ ON DELETE CASCADE)
--
-- interactions/board_pins are user-facing; if a bot Pin is referenced there
-- it is safer to re-point rather than delete. The UPDATE steps below cover
-- every known join. Run the counts at the end to confirm no duplicates
-- remain before applying the migration.

BEGIN;

-- Temporary table: one survivor per duplicate group.
CREATE TEMP TABLE bot_pin_survivors ON COMMIT DROP AS
SELECT DISTINCT ON (url) id AS survivor_id, url
FROM pins
WHERE creator_id = '00000000-0000-0000-0000-00000000f096'
  AND url IS NOT NULL
ORDER BY url, created_at DESC, id DESC;

-- Every bot Pin that is NOT the survivor in its URL group.
CREATE TEMP TABLE bot_pin_duplicates ON COMMIT DROP AS
SELECT p.id AS duplicate_id, s.survivor_id
FROM pins p
JOIN bot_pin_survivors s ON s.url = p.url
WHERE p.creator_id = '00000000-0000-0000-0000-00000000f096'
  AND p.id <> s.survivor_id;

-- (b) Re-point joins.
UPDATE harvester_frontier_pins
SET pin_id = d.survivor_id
FROM bot_pin_duplicates d
WHERE harvester_frontier_pins.pin_id = d.duplicate_id;

UPDATE pin_tags
SET pin_id = d.survivor_id
FROM bot_pin_duplicates d
WHERE pin_tags.pin_id = d.duplicate_id
  -- Avoid violating the (pin_id, tag_id) uniqueness by only re-pointing
  -- rows whose (survivor_id, tag_id) does not already exist.
  AND NOT EXISTS (
      SELECT 1 FROM pin_tags existing
      WHERE existing.pin_id = d.survivor_id
        AND existing.tag_id = pin_tags.tag_id
  );
-- Any pin_tags rows that could not be re-pointed due to the conflict above
-- are dropped when their duplicate Pin is deleted below.

UPDATE board_pins
SET pin_id = d.survivor_id
FROM bot_pin_duplicates d
WHERE board_pins.pin_id = d.duplicate_id
  AND NOT EXISTS (
      SELECT 1 FROM board_pins existing
      WHERE existing.board_id = board_pins.board_id
        AND existing.pin_id = d.survivor_id
  );

UPDATE interactions
SET target_id = d.survivor_id
FROM bot_pin_duplicates d
WHERE interactions.target_id = d.duplicate_id
  AND interactions.target_type = 'pin';

-- (c) Delete duplicates. CASCADE handles any leftover join rows.
DELETE FROM pins
WHERE id IN (SELECT duplicate_id FROM bot_pin_duplicates);

-- Verification: no bot URL should have more than one row now.
DO $$
DECLARE
    remaining INT;
BEGIN
    SELECT count(*) INTO remaining FROM (
        SELECT url FROM pins
        WHERE creator_id = '00000000-0000-0000-0000-00000000f096'
          AND url IS NOT NULL
        GROUP BY url HAVING count(*) > 1
    ) AS dup;
    IF remaining > 0 THEN
        RAISE EXCEPTION 'dedup left % duplicate URL(s) behind', remaining;
    END IF;
END
$$;

COMMIT;
