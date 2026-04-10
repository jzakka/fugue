CREATE TABLE bot_pioneer_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running',
    
    nodes_discovered INT DEFAULT 0,
    nodes_updated INT DEFAULT 0,
    scripts_generated INT DEFAULT 0,
    scripts_reused INT DEFAULT 0,
    
    ai_api_calls INT DEFAULT 0,
    ai_cost_usd DECIMAL(10, 6) DEFAULT 0,
    
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
