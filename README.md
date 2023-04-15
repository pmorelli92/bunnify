# Bunnify (experimental stage)

Bunnify is a library for publishing and consuming events with AMQP. It presents the following features:

- Easy setup.
- Consume the same Payload you published, marshal and unmarshal actions are handled by the library and abstracted to the developer.
- Reconnection handled by default.
- Correlation ID and other event metadata is handled by default.
- Does not add extra dependencies more than `github.com/rabbitmq/amqp091-go` on which all the libraries are built upon.

## Experimental

This library is on an early stage which means the API may change substantially in the next few weeks. Beware before using this in a real work environment.

## Motivation

Every workplace I have been had their own AMQP library. Most of the time the problems that they try to solve are reconnection, logging, correlation, handling the correct body type for events and dead letter. Most of this libraries are good but also built upon some other internal libraries and with some company's specifics that makes them impossible to open source.

Some developers are often spoiled with these as they provide a good dev experience and that is great; but you cannot use it in side projects, or if you start your own company.

Then, it is clear for me that there is a need for a library that is simple to use, to understand and that provides most of the mentioned functionalities out of the box.

## Example

You can find working examples under the `tests` folder.

### Consumer

```go
package main

import (
	"context"
	"time"
    "fmt"

	"github.com/pmorelli92/bunnify/bunnify"
)

func main() {
	c := bunnify.NewConnection()
	c.Start()

	c.NewQueueListener(
		"exchange1",
		"queue1",
		bunnify.NewHandlerFor("catCreated", HandleCatCreated),
		bunnify.NewHandlerFor("personCreated", HandlePersonCreated)).Listen()

	wg := sync.WaitGroup{}
	wg.Wait()
}

type personCreated struct {
	Name string `json:"name"`
}

func HandlePersonCreated(ctx context.Context, event bunnify.ConsumableEvent[personCreated]) error {
	fmt.Println("Event ID: " + event.ID)
	fmt.Println("Correlation ID: " + event.CorrelationID)
	fmt.Println("Timestamp: " + event.Timestamp.Format(time.DateTime))
	fmt.Println("Person created with name: " + event.Payload.Name)
	return nil
}

type catCreated struct {
	Years int `json:"years"`
}

func HandleCatCreated(ctx context.Context, event bunnify.ConsumableEvent[catCreated]) error {
	fmt.Println("Event ID: " + event.ID)
	fmt.Println("Correlation ID: " + event.CorrelationID)
	fmt.Println("Timestamp: " + event.Timestamp.Format(time.DateTime))
	fmt.Println("Cat created with years: " + event.Payload.Years)
	return nil
}
```

### Publisher

```go
package main

import (
	"context"
	"time"
    "fmt"

	"github.com/pmorelli92/bunnify/bunnify"
)

func main() {
	c := bunnify.NewConnection()
	c.Start()

    publisher := c.NewPublisher()

	event := bunnify.PublishableEvent{
		Metadata: bunnify.Metadata{
			ID:            "12345",
			CorrelationID: "6789",
			Timestamp:     time.Now(),
		},
		Payload: catCreated{
            Years: 22
        },
	}

    if err := publisher.Publish(context.TODO(), "exchange1", "catCreated", publishedEvent); err != nil {
		fmt.Println(err)
	}
}

type catCreated struct {
	Years int `json:"years"`
}
```
