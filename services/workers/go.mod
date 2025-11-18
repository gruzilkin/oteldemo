module github.com/oteldemo/workers

go 1.23

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/redis/go-redis/v9 v9.7.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0
	go.opentelemetry.io/otel/sdk v1.35.0
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.57.0
)
