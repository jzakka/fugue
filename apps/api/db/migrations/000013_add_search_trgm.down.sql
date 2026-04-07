DROP INDEX IF EXISTS idx_pins_title_trgm;
DROP INDEX IF EXISTS idx_creators_nickname_trgm;
DROP INDEX IF EXISTS idx_boards_name_trgm;

DROP EXTENSION IF EXISTS pg_trgm;
