CREATE TABLE bot_sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain TEXT NOT NULL UNIQUE,
    root_url TEXT NOT NULL,
    
    pioneer_status TEXT DEFAULT 'pending',
    pioneer_started_at TIMESTAMPTZ,
    pioneer_completed_at TIMESTAMPTZ,
    
    last_harvest_at TIMESTAMPTZ,
    active BOOLEAN DEFAULT true,
    
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
