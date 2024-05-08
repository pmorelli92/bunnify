-- name: InsertOutboxEvents :exec
INSERT INTO outbox_events(event_id, exchange, routing_key, payload, trace_id, span_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: MarkOutboxEventAsPublished :exec
UPDATE outbox_events SET published = true
WHERE event_id = ANY(@ids::text[]);

-- name: DeleteOutboxEvents :exec
DELETE FROM outbox_events WHERE event_id = ANY(@ids::text[]);

-- name: GetOutboxEventsForPublish :many
SELECT * FROM outbox_events
WHERE published IS FALSE
ORDER BY created_at ASC
LIMIT 200 FOR UPDATE;
