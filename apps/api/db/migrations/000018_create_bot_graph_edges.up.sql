CREATE TABLE bot_graph_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_node_id UUID NOT NULL REFERENCES bot_graph_nodes(id) ON DELETE CASCADE,
    to_node_id UUID NOT NULL REFERENCES bot_graph_nodes(id) ON DELETE CASCADE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(from_node_id, to_node_id)
);
