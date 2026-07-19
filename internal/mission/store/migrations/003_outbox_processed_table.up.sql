CREATE TABLE outbox_events_processed (
    event_id TEXT PRIMARY KEY REFERENCES outbox_events(id),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
