CREATE TABLE bot_graph_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    from_node_id UUID NOT NULL REFERENCES bot_graph_nodes(id) ON DELETE CASCADE,
    to_node_id UUID NOT NULL REFERENCES bot_graph_nodes(id) ON DELETE CASCADE,
    link_text TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(from_node_id, to_node_id)
);
