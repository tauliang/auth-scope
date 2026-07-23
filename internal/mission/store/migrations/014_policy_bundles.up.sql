CREATE TABLE policy_bundles (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    version TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    bundle_hash TEXT NOT NULL,
    signature TEXT,
    bundle_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_policy_bundles_tenant_version ON policy_bundles(COALESCE(tenant_id, ''), version);
CREATE UNIQUE INDEX idx_policy_bundles_active_tenant ON policy_bundles(COALESCE(tenant_id, '')) WHERE status = 'active';
CREATE INDEX idx_policy_bundles_tenant_status ON policy_bundles(tenant_id, status);
CREATE INDEX idx_policy_bundles_created ON policy_bundles(created_at, id);
