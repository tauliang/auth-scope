CREATE TABLE okta_app_bindings (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    issuer TEXT NOT NULL,
    client_id TEXT NOT NULL,
    mission_ref TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    binding_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_resolved_at TIMESTAMPTZ,
    UNIQUE (issuer, client_id, mission_ref)
);

CREATE INDEX idx_okta_app_bindings_tenant ON okta_app_bindings(tenant_id, status);
CREATE INDEX idx_okta_app_bindings_issuer_client ON okta_app_bindings(issuer, client_id);
CREATE INDEX idx_okta_app_bindings_mission ON okta_app_bindings(mission_ref);
