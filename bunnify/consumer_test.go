package bunnify

import (
	"testing"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

func TestGetDeliveryInfo(t *testing.T) {
	t.Run("When event gets processed first time", func(t *testing.T) {
		// Setup
		queueName := uuid.NewString()
		exchange := uuid.NewString()
		routingKey := uuid.NewString()

		// Exercise
		info := getDeliveryInfo(queueName, amqp091.Delivery{
			Exchange:   exchange,
			RoutingKey: routingKey,
		})

		// Assert
		if queueName != info.Queue {
			t.Fatalf("expected queue %s, got %s", queueName, info.Queue)
		}
		if exchange != info.Exchange {
			t.Fatalf("expected exchange %s, got %s", exchange, info.Exchange)
		}
		if routingKey != info.RoutingKey {
			t.Fatalf("expected routing key %s, got %s", routingKey, info.RoutingKey)
		}
	})

	t.Run("When event has been sent to dead queue", func(t *testing.T) {
		// Setup
		queueName := uuid.NewString()
		exchange := uuid.NewString()
		routingKey := uuid.NewString()

		// Exercise
		info := getDeliveryInfo(queueName, amqp091.Delivery{
			Headers: map[string]interface{}{
				"x-death": []interface{}{
					amqp091.Table{
						"queue":    queueName,
						"exchange": exchange,
						"routing-keys": []interface{}{
							routingKey,
						},
					},
				},
			},
			Exchange:   "dead-exchange",
			RoutingKey: "",
		})

		// Assert
		if queueName != info.Queue {
			t.Fatalf("expected queue %s, got %s", queueName, info.Queue)
		}
		if exchange != info.Exchange {
			t.Fatalf("expected exchange %s, got %s", exchange, info.Exchange)
		}
		if routingKey != info.RoutingKey {
			t.Fatalf("expected routing key %s, got %s", routingKey, info.RoutingKey)
		}
	})
}
