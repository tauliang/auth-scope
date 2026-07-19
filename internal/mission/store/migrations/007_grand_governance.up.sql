CREATE TABLE authority_negotiations (
    id TEXT PRIMARY KEY,
    mission_ref TEXT NOT NULL REFERENCES missions(ref) ON DELETE CASCADE,
    tenant_id TEXT,
    status VARCHAR(100) NOT NULL,
    negotiation_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_authority_negotiations_mission ON authority_negotiations(mission_ref, created_at);
CREATE INDEX idx_authority_negotiations_tenant_status ON authority_negotiations(tenant_id, status);

CREATE TABLE containment_rules (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    target_type VARCHAR(100) NOT NULL,
    target_id TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    rule_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    lifted_at TIMESTAMPTZ
);

CREATE INDEX idx_containment_rules_status_target ON containment_rules(status, target_type, target_id);
CREATE INDEX idx_containment_rules_tenant_status ON containment_rules(tenant_id, status);
CREATE INDEX idx_containment_rules_expires_at ON containment_rules(expires_at);
