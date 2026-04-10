CREATE TABLE bot_graph_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    
    url TEXT NOT NULL,
    url_hash TEXT NOT NULL,
    depth INT NOT NULL,
    node_type TEXT,
    parent_url TEXT,
    
    script_id UUID,
    
    visit_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    fail_count INT DEFAULT 0,
    last_visited_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    UNIQUE(site_id, url_hash)
);
