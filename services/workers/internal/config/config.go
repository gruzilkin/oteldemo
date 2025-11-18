package config

import (
	"os"
)

// Config holds application configuration
type Config struct {
	Location              string
	RedisURL              string
	OtelCollectorEndpoint string
	ServiceName           string
	HTTPPort              string
	TasksStream           string
	ResultsStream         string
	ConsumerGroup         string
}

// Load loads configuration from environment variables
func Load() *Config {
	location := os.Getenv("WORKER_LOCATION")
	if location == "" {
		location = "unknown"
	}

	return &Config{
		Location:              location,
		RedisURL:              getEnv("REDIS_URL", "redis://redis:6379"),
		OtelCollectorEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "worker-collector:4317"),
		ServiceName:           getEnv("OTEL_SERVICE_NAME", "dns-worker-"+location),
		HTTPPort:              getEnv("HTTP_PORT", "8080"),
		TasksStream:           "dns:tasks",
		ResultsStream:         "dns:results",
		ConsumerGroup:         "workers-" + location,  // Each location has its own consumer group for fan-out
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
