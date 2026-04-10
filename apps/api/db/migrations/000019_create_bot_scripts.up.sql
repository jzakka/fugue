CREATE TABLE bot_scripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    node_type TEXT NOT NULL,
    
    script_lang TEXT DEFAULT 'js',
    script_code TEXT NOT NULL,
    
    ai_model TEXT DEFAULT 'claude-3-5-haiku',
    generation_cost_usd DECIMAL(10, 6),
    
    validation_success_count INT DEFAULT 0,
    validation_fail_count INT DEFAULT 0,
    last_validated_at TIMESTAMPTZ,
    
    success_count INT DEFAULT 0,
    fail_count INT DEFAULT 0,
    avg_execution_ms INT,
    avg_items_extracted FLOAT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    UNIQUE(site_id, node_type)
);
