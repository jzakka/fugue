ALTER INDEX idx_interactions_pin RENAME TO idx_interactions_work;

ALTER TABLE interactions RENAME COLUMN pin_id TO work_id;
ALTER TABLE board_pins RENAME COLUMN pin_id TO work_id;

ALTER INDEX idx_pins_creator RENAME TO idx_works_creator;
ALTER INDEX idx_pins_tags RENAME TO idx_works_tags;
ALTER INDEX idx_pins_field RENAME TO idx_works_field;

ALTER TABLE pins RENAME TO works;
