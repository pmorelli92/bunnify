// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: queries.sql

package sqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const deleteOutboxEvents = `-- name: DeleteOutboxEvents :exec
DELETE FROM outbox_events WHERE event_id = ANY($1::text[])
`

func (q *Queries) DeleteOutboxEvents(ctx context.Context, ids []string) error {
	_, err := q.db.Exec(ctx, deleteOutboxEvents, ids)
	return err
}

const getOutboxEventsForPublish = `-- name: GetOutboxEventsForPublish :many
SELECT event_id, exchange, routing_key, payload, trace_id, span_id, created_at, published FROM outbox_events
WHERE published IS FALSE
ORDER BY created_at ASC
LIMIT 200 FOR UPDATE
`

func (q *Queries) GetOutboxEventsForPublish(ctx context.Context) ([]OutboxEvent, error) {
	rows, err := q.db.Query(ctx, getOutboxEventsForPublish)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OutboxEvent
	for rows.Next() {
		var i OutboxEvent
		if err := rows.Scan(
			&i.EventID,
			&i.Exchange,
			&i.RoutingKey,
			&i.Payload,
			&i.TraceID,
			&i.SpanID,
			&i.CreatedAt,
			&i.Published,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const insertOutboxEvents = `-- name: InsertOutboxEvents :exec
INSERT INTO outbox_events(event_id, exchange, routing_key, payload, trace_id, span_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

type InsertOutboxEventsParams struct {
	EventID    string
	Exchange   string
	RoutingKey string
	Payload    []byte
	TraceID    string
	SpanID     string
	CreatedAt  pgtype.Timestamptz
}

func (q *Queries) InsertOutboxEvents(ctx context.Context, arg InsertOutboxEventsParams) error {
	_, err := q.db.Exec(ctx, insertOutboxEvents,
		arg.EventID,
		arg.Exchange,
		arg.RoutingKey,
		arg.Payload,
		arg.TraceID,
		arg.SpanID,
		arg.CreatedAt,
	)
	return err
}

const markOutboxEventAsPublished = `-- name: MarkOutboxEventAsPublished :exec
UPDATE outbox_events SET published = true
WHERE event_id = ANY($1::text[])
`

func (q *Queries) MarkOutboxEventAsPublished(ctx context.Context, ids []string) error {
	_, err := q.db.Exec(ctx, markOutboxEventAsPublished, ids)
	return err
}
