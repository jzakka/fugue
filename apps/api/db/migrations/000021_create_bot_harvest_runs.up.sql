CREATE TABLE bot_harvest_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running',
    
    nodes_visited INT DEFAULT 0,
    nodes_succeeded INT DEFAULT 0,
    nodes_failed INT DEFAULT 0,
    items_extracted INT DEFAULT 0,
    items_deduplicated INT DEFAULT 0,
    pins_created INT DEFAULT 0,
    
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
