CREATE TABLE atlassian_site_bindings (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    site_url TEXT NOT NULL,
    cloud_id TEXT,
    mission_ref TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    binding_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_resolved_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_atlassian_site_bindings_site_unique ON atlassian_site_bindings(COALESCE(tenant_id, ''), site_url, mission_ref);
CREATE UNIQUE INDEX idx_atlassian_site_bindings_cloud_unique ON atlassian_site_bindings(COALESCE(tenant_id, ''), cloud_id, mission_ref) WHERE cloud_id IS NOT NULL;
CREATE INDEX idx_atlassian_site_bindings_tenant ON atlassian_site_bindings(tenant_id, status);
CREATE INDEX idx_atlassian_site_bindings_site ON atlassian_site_bindings(site_url, status);
CREATE INDEX idx_atlassian_site_bindings_cloud ON atlassian_site_bindings(cloud_id, status);
CREATE INDEX idx_atlassian_site_bindings_mission ON atlassian_site_bindings(mission_ref);
