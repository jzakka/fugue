CREATE TABLE bot_graph_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    
    url TEXT NOT NULL,
    url_hash TEXT NOT NULL,
    node_type TEXT,
    
    script_id UUID,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    UNIQUE(site_id, url_hash)
);
