CREATE TABLE github_repository_bindings (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    repository TEXT NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL,
    binding_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_webhook_at TIMESTAMPTZ
);

CREATE INDEX idx_github_repository_bindings_tenant ON github_repository_bindings(tenant_id, status);
CREATE INDEX idx_github_repository_bindings_repository ON github_repository_bindings(repository);

CREATE TABLE github_webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,
    event VARCHAR(100) NOT NULL,
    repository TEXT,
    binding_id TEXT REFERENCES github_repository_bindings(id),
    tenant_id TEXT,
    mission_ref TEXT,
    status VARCHAR(50) NOT NULL,
    delivery_json JSONB NOT NULL,
    payload_summary JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_github_webhook_deliveries_repository ON github_webhook_deliveries(repository, received_at);
CREATE INDEX idx_github_webhook_deliveries_binding ON github_webhook_deliveries(binding_id, received_at);
