CREATE INDEX idx_graph_nodes_site ON bot_graph_nodes(site_id);
CREATE INDEX idx_graph_nodes_hash ON bot_graph_nodes(site_id, url_hash);
CREATE INDEX idx_graph_nodes_type ON bot_graph_nodes(site_id, node_type);
