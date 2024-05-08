<p align="center">
    <img src="logo.png" width="320px">
</p>

<div align="center">

[![Go Report Card](https://goreportcard.com/badge/github.com/pmorelli92/bunnify)](https://goreportcard.com/report/github.com/pmorelli92/bunnify)
[![GitHub license](https://img.shields.io/github/license/pmorelli92/bunnify)](LICENSE)
[![Tests](https://github.com/pmorelli92/bunnify/actions/workflows/main.yaml/badge.svg?branch=main)](https://github.com/pmorelli92/bunnify/actions/workflows/main.yaml)
[![Coverage Status](https://coveralls.io/repos/github/pmorelli92/bunnify/badge.svg?branch=main&kill_cache=1)](https://coveralls.io/github/pmorelli92/bunnify?branch=main)

Bunnify is a library for publishing and consuming events for AMQP.

</div>

> [!IMPORTANT]
> While from my perspective the library is working just fine, I am still changing things here and there. Even tough the library is tagged as semver, I will start respecting it after the `v1.0.0`, and I won't guarantee backwards compatibility before that.

## Features

**Easy setup:** Bunnify is designed to be easy to set up and use. Simply reference the library and start publishing and consuming events.

**Automatic payload marshaling and unmarshaling:** You can consume the same payload you published, without worrying about the details of marshaling and unmarshaling. Bunnify handles these actions for you, abstracting them away from the developer.

**Automatic reconnection:** If your connection to the AMQP server is interrupted, Bunnify will automatically handle the reconnection for you. This ensures that your events are published and consumed without interruption.

**Built-in event metadata handling:** The library automatically handles event metadata, including correlation IDs and other important details.

**Retries and dead lettering:** You can configure how many times an event can be retried and to send the event to a dead letter queue when the processing fails.

**Tracing out of the box**: Automatically injects and extracts traces when publishing and consuming. Minimal setup required is shown on the tracer test.

**Prometheus metrics**: Prometheus gatherer will collect automatically the following metrics:

- `amqp_events_received`
- `amqp_events_without_handler`
- `amqp_events_not_parsable`
- `amqp_events_nack`
- `amqp_events_processed_duration`
- `amqp_events_publish_succeed`
- `amqp_events_publish_failed`

**Only dependencies needed:** The intention of the library is to avoid having lots of unneeded dependencies. I will always try to triple check the dependencies and use the least quantity of libraries to achieve the functionality required.

- `github.com/rabbitmq/amqp091-go`: Handles the connection with AMQP protocol.
- `github.com/google/uuid`: Generates UUID for events ID and correlation ID.
- `go.uber.org/goleak`: Used on tests to verify that there are no leaks of routines on the handling of channels.
- `go.opentelemetry.io/otel`: Handles the injection and extraction of the traces on the events.
- `github.com/prometheus/client_golang`: Used in order to export metrics to Prometheus.

**Outbox publisher:** There is a submodule that you can refer with `go get github.com/pmorelli92/bunnify/outbox`. This publisher is wrapping the default bunnify publisher and stores all events in a database table which will be looped in an async way to be published to AMQP. You can read more [here].(./outbox/README.md)

## Motivation

Every workplace I have been had their own AMQP library. Most of the time the problems that they try to solve are reconnection, logging, correlation, handling the correct body type for events and dead letter. Most of this libraries are good but also built upon some other internal libraries and with some company's specifics that makes them impossible to open source.

Some developers are often spoiled with these as they provide a good dev experience and that is great; but if you cannot use it in side projects or if you start your own company, what is the point?

Bunnify aims to provide a flexible and adaptable solution that can be used in a variety of environments and scenarios. By abstracting away many of the technical details of AMQP publishing and consumption, Bunnify makes it easy to get started with event-driven architecture without needing to be an AMQP expert.

## Installation

```
go get github.com/pmorelli92/bunnify
```

## Examples

You can find all the working examples under the `tests` folder.

**Consumer**

https://github.com/pmorelli92/bunnify/blob/f356a80625d9dcdaec12d05953447ebcc24a1b13/tests/consumer_publish_test.go#L38-L61

**Dead letter consumer**

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/dead_letter_receives_event_test.go#L34-L67

**Using a default handler**

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/consumer_publish_test.go#L133-L170

**Publisher**

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/consumer_publish_test.go#L64-L78

**Enable Prometheus metrics**

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/consumer_publish_metrics_test.go#L30-L34

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/consumer_publish_metrics_test.go#L70-L76

**Enable tracing**

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/consumer_publish_tracer_test.go#L18-L20

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/consumer_publish_tracer_test.go#L49-L58

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/tests/consumer_publish_tracer_test.go#L33-L37

**Retries**

https://github.com/pmorelli92/bunnify/blob/53c83127f94da86377ae38630e010b9693f376ef/tests/consumer_retries_test.go#L66-L87

https://github.com/pmorelli92/bunnify/blob/53c83127f94da86377ae38630e010b9693f376ef/tests/consumer_retries_test.go#L66-L87

**Configuration**

Both the connection and consumer structs can be configured with the typical functional options. You can find the options below:

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/bunnify/connection.go#L15-L37

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/bunnify/consumerOption.go#L18-L65

When publishing an event, you can override the event or the correlation ID if you need. This is also achievable with options:

https://github.com/pmorelli92/bunnify/blob/76f7495ef660fd4c802af8e610ffbc9cca0e39ba/bunnify/publishableEvent.go#L22-L36
