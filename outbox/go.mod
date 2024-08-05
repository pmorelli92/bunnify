module github.com/pmorelli92/bunnify/outbox

go 1.21

toolchain go1.22.2

replace github.com/pmorelli92/bunnify => ../

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/pmorelli92/bunnify v0.0.4
	github.com/prometheus/client_golang v1.19.1
	go.opentelemetry.io/otel v1.28.0
	go.opentelemetry.io/otel/sdk v1.27.0
	go.opentelemetry.io/otel/trace v1.28.0
	go.uber.org/goleak v1.3.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rabbitmq/amqp091-go v1.10.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	golang.org/x/crypto v0.25.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.23.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)
