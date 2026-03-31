CREATE TABLE auth_accounts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id  UUID NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
    provider    VARCHAR(20) NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    email       VARCHAR(255),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_id)
);

CREATE INDEX idx_auth_accounts_creator ON auth_accounts(creator_id);
CREATE INDEX idx_auth_accounts_email ON auth_accounts(email);
