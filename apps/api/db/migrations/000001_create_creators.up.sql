CREATE TABLE creators (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nickname    VARCHAR(50) NOT NULL,
    bio         VARCHAR(200),
    roles       TEXT[] NOT NULL,
    contacts    JSONB NOT NULL,
    avatar_url  VARCHAR(500),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_creators_roles ON creators USING GIN(roles);
