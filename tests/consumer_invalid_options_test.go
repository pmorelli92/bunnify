package tests

import (
	"testing"

	"github.com/pmorelli92/bunnify/bunnify"
	"go.uber.org/goleak"
)

func TestConsumerShouldReturnErrorWhenNoHandlersSpecified(t *testing.T) {
	// Setup
	connection := bunnify.NewConnection()
	connection.Start()
	consumer := connection.NewConsumer("queueName")

	// Exercise
	err := consumer.Consume()

	// Assert
	if err == nil {
		t.Fatal(err)
	}

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	goleak.VerifyNone(t)
}
