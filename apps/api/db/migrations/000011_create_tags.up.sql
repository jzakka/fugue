CREATE TABLE tags (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(50) NOT NULL UNIQUE,
    slug          VARCHAR(50) NOT NULL UNIQUE,
    category      VARCHAR(30) NOT NULL,
    display_order INT DEFAULT 0
);

CREATE INDEX idx_tags_category ON tags(category);

CREATE TABLE pin_tags (
    pin_id UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id),
    PRIMARY KEY (pin_id, tag_id)
);

CREATE INDEX idx_pin_tags_tag ON pin_tags(tag_id);
