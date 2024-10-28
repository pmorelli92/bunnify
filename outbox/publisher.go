package outbox

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pmorelli92/bunnify"
	gen_sql "github.com/pmorelli92/bunnify/outbox/internal/generated"
	"go.opentelemetry.io/otel/trace"
)

// Publisher is used for publishing events.
type Publisher struct {
	db      *pgxpool.Pool
	inner   bunnify.Publisher
	options publisherOption
	closed  bool
}

//go:embed schema/schema.sql
var outboxSchema string

// NewPublisher creates a publisher using a database
// connection and acts as a wrapper for bunnify publisher.
func NewPublisher(
	ctx context.Context,
	db *pgxpool.Pool,
	inner bunnify.Publisher,
	opts ...func(*publisherOption)) (*Publisher, error) {

	options := publisherOption{
		deleteAfterPublish: false,
		loopInterval:       5 * time.Second,
	}
	for _, opt := range opts {
		opt(&options)
	}

	_, err := db.Exec(ctx, outboxSchema)
	if err != nil {
		return nil, err
	}

	p := &Publisher{
		db:      db,
		inner:   inner,
		options: options,
	}

	go p.loop()
	return p, nil
}

// Publish publishes an event to the outbox database table.
// Then the loop will pick this up and try to use the inner bunnify Publisher do an AQMP publish.
func (p *Publisher) Publish(
	ctx context.Context,
	tx pgx.Tx,
	exchange string,
	routingKey string,
	event bunnify.PublishableEvent) error {

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("could not marshal event: %w", err)
	}

	// If telemetry is not used, this will be just default zeroes
	spanContext := trace.SpanFromContext(ctx).SpanContext()

	return gen_sql.New(tx).InsertOutboxEvents(ctx, gen_sql.InsertOutboxEventsParams{
		EventID:    event.ID,
		Exchange:   exchange,
		RoutingKey: routingKey,
		Payload:    payload,
		TraceID:    spanContext.TraceID().String(),
		SpanID:     spanContext.SpanID().String(),
		CreatedAt:  pgtype.Timestamptz{Valid: true, Time: event.Timestamp},
	})
}

func (p *Publisher) Close() {
	p.closed = true
}
