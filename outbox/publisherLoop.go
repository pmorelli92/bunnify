package outbox

import (
	"context"
	_ "embed"
	"encoding/json"
	"time"

	"github.com/pmorelli92/bunnify"
	gen_sql "github.com/pmorelli92/bunnify/outbox/generated"
	"go.opentelemetry.io/otel/trace"
)

// loop every certain duration to check if there are
// events to be published with the inner bunnify publisher to AMQP.
func (p *Publisher) loop() {
	ticker := time.NewTicker(p.options.loopInterval)
	for {
		if p.closed {
			notifyEndingLoop(p.options.notificationChannel)
			return
		}

		<-ticker.C

		ctx := context.TODO()
		tx, err := p.db.Begin(ctx)
		if err != nil {
			notifyCannotCreateTx(p.options.notificationChannel)
			continue
		}

		q := gen_sql.New(tx)
		evts, err := q.GetOutboxEventsForPublish(ctx)
		if err != nil {
			_ = tx.Rollback(ctx)
			notifyCannotQueryOutboxTable(p.options.notificationChannel)
			continue
		}

		if len(evts) == 0 {
			_ = tx.Rollback(ctx)
			continue
		}

		ids := make([]string, 0, len(evts))
		for _, e := range evts {
			var publishableEvt bunnify.PublishableEvent
			err = json.Unmarshal(e.Payload, &publishableEvt)
			if err != nil {
				continue
			}

			// Recreates a context with trace id and span id stored in the database, if any
			sid, _ := trace.SpanIDFromHex(e.SpanID)
			tid, _ := trace.TraceIDFromHex(e.TraceID)
			newSpanContext := trace.NewSpanContext(trace.SpanContextConfig{
				SpanID:  sid,
				TraceID: tid,
			})

			traceCtx := trace.ContextWithSpanContext(context.TODO(), newSpanContext)
			err = p.inner.Publish(traceCtx, e.Exchange, e.RoutingKey, publishableEvt)
			if err != nil {
				notifyCannotPublishAMQP(p.options.notificationChannel)
				continue
			}

			ids = append(ids, e.EventID)
		}

		if p.options.deleteAfterPublish {
			if err := q.DeleteOutboxEvents(ctx, ids); err != nil {
				notifyCannotDeleteOutboxEvents(p.options.notificationChannel)
				continue
			}
		} else {
			if err := q.MarkOutboxEventAsPublished(ctx, ids); err != nil {
				notifyCannotMarkOutboxEventsAsPublished(p.options.notificationChannel)
				continue
			}
		}

		if err = tx.Commit(ctx); err != nil {
			notifyCannotCommitTransaction(p.options.notificationChannel)
			_ = tx.Rollback(ctx)
			continue
		}

		notifyOutboxEventsPublished(p.options.notificationChannel, len(ids))
	}
}
