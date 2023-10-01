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

## Features

**Easy setup:** Bunnify is designed to be easy to set up and use. Simply reference the library and start publishing and consuming events.

**Automatic payload marshaling and unmarshaling:** You can consume the same payload you published, without worrying about the details of marshaling and unmarshaling. Bunnify handles these actions for you, abstracting them away from the developer.

**Automatic reconnection:** If your connection to the AMQP server is interrupted, Bunnify will automatically handle the reconnection for you. This ensures that your events are published and consumed without interruption.

**Built-in event metadata handling:** The library automatically handles event metadata, including correlation IDs and other important details.

**Tracing out of the box**: Automatically injects and extracts traces when publishing and consuming. Minimal setup is required and shown on the tracer test.

**Minimal dependencies:** The intention of the library is to avoid being a vector of attack due to lots of unneeded dependencies. I will always try to triple check the dependencies and use the least quantity of libraries to achieve the functionality required.

- `github.com/rabbitmq/amqp091-go`: Handles the connection with AMQP protocol.
- `github.com/google/uuid`: Generates UUID for events ID and correlation ID.
- `go.uber.org/goleak`: Used on tests to verify that there are no leaks of routines on the handling of channels.
- `go.opentelemetry.io/otel`: Handles the injection and extraction of the traces on the events.

## Motivation

Every workplace I have been had their own AMQP library. Most of the time the problems that they try to solve are reconnection, logging, correlation, handling the correct body type for events and dead letter. Most of this libraries are good but also built upon some other internal libraries and with some company's specifics that makes them impossible to open source.

Some developers are often spoiled with these as they provide a good dev experience and that is great; but you cannot use it in side projects, or if you start your own company.

Bunnify aims to provide a flexible and adaptable solution that can be used in a variety of environments and scenarios. By abstracting away many of the technical details of AMQP publishing and consumption, Bunnify makes it easy to get started with event-driven architecture without needing to be an AMQP expert.

## What is next to come

- Support for exposing Prometheus metrics.

## Examples

You can find all the working examples under the `tests` folder.

### Consumer

https://github.com/pmorelli92/bunnify/blob/aa8f63943345a8e3d092e98b9cd69cf73e7a0ec9/tests/consumer_publish_test.go#L38-L59

### Dead letter consumer

https://github.com/pmorelli92/bunnify/blob/aa8f63943345a8e3d092e98b9cd69cf73e7a0ec9/tests/dead_letter_receives_event_test.go#L46-L66

### Publisher

https://github.com/pmorelli92/bunnify/blob/aa8f63943345a8e3d092e98b9cd69cf73e7a0ec9/tests/consumer_publish_test.go#L61-L71

## Configuration

Both the connection and consumer structs can be configured with the typical functional options. You can find the options below:

https://github.com/pmorelli92/bunnify/blob/aa8f63943345a8e3d092e98b9cd69cf73e7a0ec9/bunnify/connection.go#L15-L37

https://github.com/pmorelli92/bunnify/blob/aa8f63943345a8e3d092e98b9cd69cf73e7a0ec9/bunnify/consumer.go#L24-L71

When publishing an event, you can override the event or the correlation ID if you need. This is also achievable with options:

https://github.com/pmorelli92/bunnify/blob/aa8f63943345a8e3d092e98b9cd69cf73e7a0ec9/bunnify/publishableEvent.go#L22-L36
