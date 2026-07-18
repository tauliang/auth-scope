-- Outbox Events Table (for transactional event publishing)
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(100) NOT NULL,
    mission_ref UUID REFERENCES missions(ref),
    payload JSONB NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for unprocessed events (partial index)
CREATE INDEX idx_outbox_events_unprocessed ON outbox_events(processed_at) WHERE processed_at IS NULL;
