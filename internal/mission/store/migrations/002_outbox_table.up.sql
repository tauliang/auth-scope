CREATE TABLE outbox_events (
    id TEXT PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    mission_ref TEXT REFERENCES missions(ref),
    event_json JSONB NOT NULL,
    payload JSONB NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_events_unprocessed ON outbox_events(processed_at) WHERE processed_at IS NULL;
