package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	Location                   string
	RedisURL                   string
	OtelCollectorEndpoint      string
	ServiceName                string
	TasksStream                string
	ResultsStream              string
	ConsumerGroup              string
	ChaosSequentialProbability float64 // Probability (0.0-1.0) of running DNS lookups sequentially instead of concurrently
	ChaosErrorProbability      float64 // Probability (0.0-1.0) of individual DNS lookups failing with an error
}

// Load loads configuration from environment variables
func Load() *Config {
	location := os.Getenv("WORKER_LOCATION")
	if location == "" {
		location = "unknown"
	}

	// Parse chaos sequential probability (default 30%)
	chaosSequentialProb := 0.3
	if val := os.Getenv("CHAOS_SEQUENTIAL_PROBABILITY"); val != "" {
		if p, err := strconv.ParseFloat(val, 64); err == nil && p >= 0.0 && p <= 1.0 {
			chaosSequentialProb = p
		}
	}

	// Parse chaos error probability (default 10%)
	chaosErrorProb := 0.1
	if val := os.Getenv("CHAOS_ERROR_PROBABILITY"); val != "" {
		if p, err := strconv.ParseFloat(val, 64); err == nil && p >= 0.0 && p <= 1.0 {
			chaosErrorProb = p
		}
	}

	return &Config{
		Location:                   location,
		RedisURL:                   getEnv("REDIS_URL", "redis://redis:6379"),
		OtelCollectorEndpoint:      getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "worker-collector:4317"),
		ServiceName:                getEnv("OTEL_SERVICE_NAME", "dns-worker-"+location),
		TasksStream:                "dns:tasks",
		ResultsStream:              "dns:results",
		ConsumerGroup:              "workers-" + location,  // Each location has its own consumer group for fan-out
		ChaosSequentialProbability: chaosSequentialProb,
		ChaosErrorProbability:      chaosErrorProb,
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
