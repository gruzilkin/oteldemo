package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oteldemo/workers/internal/config"
	"github.com/oteldemo/workers/internal/dns"
	"github.com/oteldemo/workers/internal/redis"
	"github.com/oteldemo/workers/internal/telemetry"
	"github.com/oteldemo/workers/internal/worker"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize OpenTelemetry Tracing
	shutdownTracer, err := telemetry.InitTracer(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := shutdownTracer(context.Background()); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	// Initialize OpenTelemetry Logging
	shutdownLogger, _, err := telemetry.InitLogger(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := shutdownLogger(context.Background()); err != nil {
			log.Printf("Error shutting down logger: %v", err)
		}
	}()

	slog.Info("Starting DNS Worker",
		"location", cfg.Location,
	)

	// Initialize Redis client
	redisClient := redis.NewClient(cfg.RedisURL)
	defer redisClient.Close()

	// Initialize DNS resolver
	dnsResolver := dns.NewResolver(cfg)

	// Create worker
	w := worker.NewWorker(cfg, redisClient, dnsResolver)

	// Start worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := w.Start(ctx); err != nil {
			slog.Error("Worker error", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down worker...")

	// Graceful shutdown
	cancel() // Stop worker
	time.Sleep(2 * time.Second)

	slog.Info("Worker stopped")
}
