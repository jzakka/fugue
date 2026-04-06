ALTER TABLE creators ADD COLUMN bio VARCHAR(200);
ALTER TABLE creators ADD COLUMN roles TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE creators ADD COLUMN contacts JSONB NOT NULL DEFAULT '{}';

CREATE INDEX idx_creators_roles ON creators USING GIN(roles);
