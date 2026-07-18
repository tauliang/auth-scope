-- Outbox Events Processed Tracking Table (for tracking processed events)
CREATE TABLE outbox_events_processed (
    event_id UUID PRIMARY KEY REFERENCES outbox_events(id),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
