CREATE TABLE projections (
    id TEXT PRIMARY KEY,
    mission_ref TEXT NOT NULL REFERENCES missions(ref) ON DELETE CASCADE,
    tenant_id TEXT,
    status VARCHAR(50) NOT NULL,
    projection_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_projections_mission_status ON projections(mission_ref, status);
CREATE INDEX idx_projections_expires_at ON projections(expires_at);

CREATE TABLE mission_leases (
    id TEXT PRIMARY KEY,
    mission_ref TEXT NOT NULL REFERENCES missions(ref) ON DELETE CASCADE,
    tenant_id TEXT,
    mission_version INTEGER NOT NULL,
    lease_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_mission_leases_mission_version ON mission_leases(mission_ref, mission_version);
CREATE INDEX idx_mission_leases_expires_at ON mission_leases(expires_at);

CREATE TABLE approval_rules (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    applies_to VARCHAR(100) NOT NULL,
    rule_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approval_rules_tenant_applies ON approval_rules(tenant_id, applies_to);

CREATE TABLE approval_records (
    id TEXT PRIMARY KEY,
    target_type VARCHAR(100) NOT NULL,
    target_id TEXT NOT NULL,
    tenant_id TEXT,
    approver_subject TEXT NOT NULL,
    approver_issuer TEXT NOT NULL,
    record_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (target_type, target_id, approver_subject, approver_issuer)
);

CREATE INDEX idx_approval_records_target ON approval_records(target_type, target_id, created_at);
