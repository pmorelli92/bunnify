## Outbox

> [!IMPORTANT]
> This submodule is couple with `github.com/jackc/pgx/v5` and therefore works for Postgres databases only. This is the reason why this is shipped in a submodule instead of being included in the base module.

This submodule implements the [outbox pattern](https://microservices.io/patterns/data/transactional-outbox.html) on top of the Bunnify publisher.

## How does it work

Whenever you call the `Publish` function the event is stored in a database table created only if not exists on the creation of the `Publisher`. This function takes a transaction, which makes your entities + creation of the event atomic.

There is a go routine that every certain duration will try to fetch the outbox events pending for publishing if any. Each one of them will be published in the same way that the bunnify Publisher does. All published events will be marked as published in the database table (or deleted if configured).

## Installation

`go get github.com/pmorelli92/bunnify/outbox`

## Examples

**Setup Publisher**

https://github.com/pmorelli92/bunnify/blob/985913450b7b9a21219b96f23843288bef7eac74/outbox/tests/consumer_publish_test.go#L87-L104

**Publish**

https://github.com/pmorelli92/bunnify/blob/985913450b7b9a21219b96f23843288bef7eac74/outbox/tests/consumer_publish_test.go#L107-L127

**Configuration**

https://github.com/pmorelli92/bunnify/blob/985913450b7b9a21219b96f23843288bef7eac74/outbox/publisherOption.go#L17-L38
