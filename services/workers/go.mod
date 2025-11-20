module github.com/oteldemo/workers

go 1.25

require (
	github.com/redis/go-redis/v9 v9.7.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.8.0
	go.opentelemetry.io/otel/log v0.8.0
	go.opentelemetry.io/otel/sdk v1.35.0
	go.opentelemetry.io/otel/sdk/log v0.8.0
	go.opentelemetry.io/contrib/bridges/otelslog v0.8.0
)
