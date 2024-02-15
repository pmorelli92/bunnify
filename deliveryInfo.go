package bunnify

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

func getDeliveryInfo(queueName string, delivery amqp.Delivery) DeliveryInfo {
	deliveryInfo := DeliveryInfo{
		Queue:      queueName,
		Exchange:   delivery.Exchange,
		RoutingKey: delivery.RoutingKey,
	}

	// If routing key is empty, it is mostly due to the event being dead lettered.
	// Check for the original delivery information in the headers
	if delivery.RoutingKey == "" {
		deaths, ok := delivery.Headers["x-death"].([]interface{})
		if !ok || len(deaths) == 0 {
			return deliveryInfo
		}

		death, ok := deaths[0].(amqp.Table)
		if !ok {
			return deliveryInfo
		}

		queue, ok := death["queue"].(string)
		if !ok {
			return deliveryInfo
		}
		deliveryInfo.Queue = queue

		exchange, ok := death["exchange"].(string)
		if !ok {
			return deliveryInfo
		}
		deliveryInfo.Exchange = exchange

		routingKeys, ok := death["routing-keys"].([]interface{})
		if !ok || len(routingKeys) == 0 {
			return deliveryInfo
		}
		key, ok := routingKeys[0].(string)
		if !ok {
			return deliveryInfo
		}
		deliveryInfo.RoutingKey = key
	}

	return deliveryInfo
}
