CREATE TABLE mission_proposals (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    purpose TEXT NOT NULL,
    proposal_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ
);

CREATE TABLE missions (
    ref TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    state VARCHAR(50) NOT NULL,
    version INTEGER NOT NULL,
    principal_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    parent_mission_ref TEXT REFERENCES missions(ref),
    purpose TEXT NOT NULL,
    mission_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE TABLE events (
    id TEXT PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    mission_ref TEXT REFERENCES missions(ref),
    tenant_id TEXT,
    event_json JSONB NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_missions_tenant_id ON missions(tenant_id);
CREATE INDEX idx_missions_principal_id ON missions(principal_id);
CREATE INDEX idx_missions_agent_id ON missions(agent_id);
CREATE INDEX idx_missions_state ON missions(state);
CREATE INDEX idx_missions_parent_mission_ref ON missions(parent_mission_ref);
CREATE INDEX idx_proposals_tenant_id ON mission_proposals(tenant_id);
CREATE INDEX idx_proposals_principal_id ON mission_proposals(principal_id);
CREATE INDEX idx_events_type_created_at ON events(type, created_at);
