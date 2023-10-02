package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify/bunnify"
	"go.uber.org/goleak"
)

func TestGoRoutinesAreNotLeaked(t *testing.T) {
	// Setup
	ticker := time.NewTicker(2 * time.Second)
	connection := bunnify.NewConnection()
	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	// Exercise
	for i := 0; i < 100; i++ {
		c := connection.NewConsumer(
			uuid.NewString(),
			bunnify.WithBindingToExchange(uuid.NewString()),
			bunnify.WithDefaultHandler(func(ctx context.Context, event bunnify.ConsumableEvent[json.RawMessage]) error {
				return nil
			}))

		if err := c.Consume(); err != nil {
			t.Fatal(err)
		}
	}

	// While technically the waits are not needed I added them
	// so I can visualize on the management that channels are in fact
	// opened and then closed before the tests finishes.
	<-ticker.C
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	<-ticker.C

	// Assert
	goleak.VerifyNone(t)
}
