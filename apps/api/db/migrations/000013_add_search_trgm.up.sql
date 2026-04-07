CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_pins_title_trgm ON pins USING gin (title gin_trgm_ops);
CREATE INDEX idx_creators_nickname_trgm ON creators USING gin (nickname gin_trgm_ops);
CREATE INDEX idx_boards_name_trgm ON boards USING gin (name gin_trgm_ops);
