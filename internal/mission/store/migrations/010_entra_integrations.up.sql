CREATE TABLE entra_app_registrations (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    issuer TEXT NOT NULL,
    client_id TEXT NOT NULL,
    mission_ref TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    registration_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_resolved_at TIMESTAMPTZ,
    UNIQUE (issuer, client_id, mission_ref)
);

CREATE INDEX idx_entra_app_registrations_tenant ON entra_app_registrations(tenant_id, status);
CREATE INDEX idx_entra_app_registrations_issuer_client ON entra_app_registrations(issuer, client_id);
CREATE INDEX idx_entra_app_registrations_mission ON entra_app_registrations(mission_ref);
