CREATE TABLE slack_workspace_bindings (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    workspace_id TEXT NOT NULL,
    mission_ref TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    binding_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_resolved_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_slack_workspace_bindings_unique ON slack_workspace_bindings(COALESCE(tenant_id, ''), workspace_id, mission_ref);
CREATE INDEX idx_slack_workspace_bindings_tenant ON slack_workspace_bindings(tenant_id, status);
CREATE INDEX idx_slack_workspace_bindings_workspace ON slack_workspace_bindings(workspace_id, status);
CREATE INDEX idx_slack_workspace_bindings_mission ON slack_workspace_bindings(mission_ref);
