CREATE TABLE agent_identities (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    client_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    key_thumbprint TEXT NOT NULL,
    public_key TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    identity_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE TABLE agent_nonces (
    agent_id TEXT NOT NULL REFERENCES agent_identities(id),
    nonce TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, nonce)
);

CREATE INDEX idx_agent_identities_tenant_id ON agent_identities(tenant_id);
CREATE INDEX idx_agent_identities_client_instance ON agent_identities(client_id, instance_id);
CREATE INDEX idx_agent_identities_key_thumbprint ON agent_identities(key_thumbprint);
CREATE INDEX idx_agent_nonces_seen_at ON agent_nonces(seen_at);
