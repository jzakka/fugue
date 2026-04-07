-- Add media columns
ALTER TABLE pins ADD COLUMN media_url VARCHAR(500);
ALTER TABLE pins ADD COLUMN media_type VARCHAR(10);

-- Backfill existing rows with placeholder so NOT NULL can be applied
UPDATE pins SET media_url = 'migrated://' || id::text, media_type = 'image' WHERE media_url IS NULL;

ALTER TABLE pins ALTER COLUMN media_url SET NOT NULL;
ALTER TABLE pins ALTER COLUMN media_type SET NOT NULL;
ALTER TABLE pins ADD CONSTRAINT chk_pins_media_type CHECK (media_type IN ('image', 'audio', 'video'));

-- URL becomes optional
ALTER TABLE pins ALTER COLUMN url DROP NOT NULL;

-- Remove field, tags, pin_count (replaced by media_type, pin_tags, future metrics)
ALTER TABLE pins DROP COLUMN IF EXISTS field;
ALTER TABLE pins DROP COLUMN IF EXISTS tags;
ALTER TABLE pins DROP COLUMN IF EXISTS pin_count;

-- Drop old field index (tags GIN index was already on the dropped column)
DROP INDEX IF EXISTS idx_pins_field;
DROP INDEX IF EXISTS idx_pins_tags;

-- New index for media_type filtering
CREATE INDEX idx_pins_media_type ON pins(media_type);
