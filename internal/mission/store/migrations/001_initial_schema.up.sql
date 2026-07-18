-- Create extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Mission Proposals Table
CREATE TABLE mission_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    purpose TEXT NOT NULL,
    regions JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ
);

-- Missions Table
CREATE TABLE missions (
    ref UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id UUID REFERENCES mission_proposals(id),
    state VARCHAR(50) NOT NULL DEFAULT 'proposed',
    principal_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    purpose TEXT NOT NULL,
    regions JSONB NOT NULL,
    current_decision VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Events Table (for backward compatibility with in-memory events)
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(100) NOT NULL,
    mission_ref UUID REFERENCES missions(ref),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_missions_principal_id ON missions(principal_id);
CREATE INDEX idx_missions_agent_id ON missions(agent_id);
CREATE INDEX idx_missions_state ON missions(state);
CREATE INDEX idx_proposals_principal_id ON mission_proposals(principal_id);
CREATE INDEX idx_events_type_created_at ON events(type, created_at);
