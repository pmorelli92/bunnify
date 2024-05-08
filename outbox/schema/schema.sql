CREATE TABLE IF NOT EXISTS outbox_events(
    event_id VARCHAR NOT NULL PRIMARY KEY,
    exchange VARCHAR NOT NULL,
    routing_key VARCHAR NOT NULL,
    payload JSONB NOT NULL,
    trace_id VARCHAR NOT NULL,
    span_id VARCHAR NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    published BOOLEAN NOT NULL DEFAULT FALSE);

CREATE INDEX IF NOT EXISTS ix_outbox_event_publish ON outbox_events(published);
