-- Rename works table to pins
ALTER TABLE works RENAME TO pins;

-- Rename indexes
ALTER INDEX idx_works_field RENAME TO idx_pins_field;
ALTER INDEX idx_works_tags RENAME TO idx_pins_tags;
ALTER INDEX idx_works_creator RENAME TO idx_pins_creator;

-- Rename FK columns in related tables
ALTER TABLE board_pins RENAME COLUMN work_id TO pin_id;
ALTER TABLE interactions RENAME COLUMN work_id TO pin_id;

-- Rename indexes on interactions
ALTER INDEX idx_interactions_work RENAME TO idx_interactions_pin;
