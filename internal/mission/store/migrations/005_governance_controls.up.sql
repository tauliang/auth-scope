CREATE TABLE expansion_requests (
    id TEXT PRIMARY KEY,
    mission_ref TEXT NOT NULL REFERENCES missions(ref) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    expansion_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ
);

CREATE INDEX idx_expansion_requests_mission_status ON expansion_requests(mission_ref, status);
CREATE INDEX idx_expansion_requests_tenant_created ON expansion_requests(tenant_id, created_at);

CREATE TABLE evaluation_evidence (
    id TEXT PRIMARY KEY,
    mission_ref TEXT NOT NULL REFERENCES missions(ref) ON DELETE CASCADE,
    tenant_id TEXT,
    artifact TEXT NOT NULL,
    evidence_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_evaluation_evidence_mission_created ON evaluation_evidence(mission_ref, created_at);
CREATE INDEX idx_evaluation_evidence_tenant_created ON evaluation_evidence(tenant_id, created_at);

CREATE TABLE tool_contracts (
    tool_name TEXT PRIMARY KEY,
    contract_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
