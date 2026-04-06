CREATE TABLE interactions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
    work_id     UUID REFERENCES works(id) ON DELETE SET NULL,
    type        VARCHAR(20) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_interactions_user_time ON interactions(user_id, created_at DESC);
CREATE INDEX idx_interactions_work ON interactions(work_id);
CREATE INDEX idx_interactions_type ON interactions(type);

-- Denormalized pin count for N+1 prevention
ALTER TABLE works ADD COLUMN pin_count INTEGER NOT NULL DEFAULT 0;
