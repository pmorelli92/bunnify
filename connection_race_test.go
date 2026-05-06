package bunnify

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestReconnectIsRaceFree forces a broker-side disconnect while many
// publishers concurrently call getNewChannel. Run with -race. Without the
// connection-level synchronization fix the reconnect goroutine writes
// Connection.connection without any synchronization while every publisher
// reads it, and the race detector catches it.
func TestReconnectIsRaceFree(t *testing.T) {
	conn := NewConnection(
		WithReconnectInterval(20 * time.Millisecond),
	)
	if err := conn.Start(); err != nil {
		t.Fatal(err)
	}

	// Captured once before any reconnect could fire — the reconnect
	// goroutine is still parked on NotifyClose, so this single read is not
	// concurrent with any write.
	underlying := conn.connection

	const publishers = 16
	exchange := uuid.NewString()
	routingKey := "race.test"

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range publishers {
		wg.Add(1)
		publisher := conn.NewPublisher()
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = publisher.Publish(
					context.Background(), exchange, routingKey,
					NewPublishableEvent(struct {
						ID string `json:"id"`
					}{ID: uuid.NewString()}),
				)
			}
		}()
	}

	// Let publishers cache their channels before we yank the conn.
	time.Sleep(50 * time.Millisecond)

	// Closing the underlying conn fires NotifyClose; the reconnect
	// goroutine then writes c.connection while every publisher's cached
	// channel breaks and they all race into getNewChannel concurrently.
	if err := underlying.Close(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	close(stop)
	wg.Wait()

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}
