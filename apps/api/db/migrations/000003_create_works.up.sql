CREATE TABLE works (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id  UUID NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
    url         VARCHAR(1000) NOT NULL,
    title       VARCHAR(200) NOT NULL,
    description VARCHAR(500),
    field       VARCHAR(50) NOT NULL,
    tags        TEXT[] NOT NULL,
    og_image    VARCHAR(1000),
    og_data     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_works_field ON works(field);
CREATE INDEX idx_works_tags ON works USING GIN(tags);
CREATE INDEX idx_works_creator ON works(creator_id);
