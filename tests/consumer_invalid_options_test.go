package tests

import (
	"testing"

	"github.com/pmorelli92/bunnify"
	"go.uber.org/goleak"
)

func TestConnectionReturnErrorWhenNotValidURI(t *testing.T) {
	// Setup
	connection := bunnify.NewConnection(bunnify.WithURI("13123"))

	// Exercise
	err := connection.Start()

	// Assert
	if err == nil {
		t.Fatal(err)
	}

	goleak.VerifyNone(t)
}

func TestConsumerShouldReturnErrorWhenNoHandlersSpecified(t *testing.T) {
	// Setup
	connection := bunnify.NewConnection()
	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	// Exercise
	consumer := connection.NewConsumer("queueName")
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
