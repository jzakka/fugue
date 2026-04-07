CREATE TABLE bot_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    seed_urls TEXT[] NOT NULL,
    interval_hours INT NOT NULL DEFAULT 24,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_crawled_at TIMESTAMPTZ,
    stats JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
