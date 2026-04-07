-- Restore dropped columns
ALTER TABLE pins ADD COLUMN field VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE pins ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE pins ADD COLUMN pin_count INTEGER NOT NULL DEFAULT 0;

-- Make URL required again
ALTER TABLE pins ALTER COLUMN url SET NOT NULL;

-- Remove media columns
ALTER TABLE pins DROP CONSTRAINT IF EXISTS chk_pins_media_type;
ALTER TABLE pins DROP COLUMN IF EXISTS media_url;
ALTER TABLE pins DROP COLUMN IF EXISTS media_type;

-- Restore old indexes
DROP INDEX IF EXISTS idx_pins_media_type;
CREATE INDEX idx_pins_field ON pins(field);
CREATE INDEX idx_pins_tags ON pins USING GIN(tags);
