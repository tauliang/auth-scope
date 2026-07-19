CREATE TABLE salesforce_org_bindings (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    instance_url TEXT NOT NULL,
    org_id TEXT,
    mission_ref TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    binding_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_resolved_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_salesforce_org_bindings_instance_unique ON salesforce_org_bindings(COALESCE(tenant_id, ''), instance_url, mission_ref);
CREATE UNIQUE INDEX idx_salesforce_org_bindings_org_unique ON salesforce_org_bindings(COALESCE(tenant_id, ''), org_id, mission_ref) WHERE org_id IS NOT NULL;
CREATE INDEX idx_salesforce_org_bindings_tenant ON salesforce_org_bindings(tenant_id, status);
CREATE INDEX idx_salesforce_org_bindings_instance ON salesforce_org_bindings(instance_url, status);
CREATE INDEX idx_salesforce_org_bindings_org ON salesforce_org_bindings(org_id, status);
CREATE INDEX idx_salesforce_org_bindings_mission ON salesforce_org_bindings(mission_ref);
