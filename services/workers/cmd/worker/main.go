package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oteldemo/workers/internal/config"
	"github.com/oteldemo/workers/internal/dns"
	"github.com/oteldemo/workers/internal/redis"
	"github.com/oteldemo/workers/internal/server"
	"github.com/oteldemo/workers/internal/telemetry"
	"github.com/oteldemo/workers/internal/worker"
)

func main() {
	// Load configuration
	cfg := config.Load()

	log.Printf("Starting DNS Worker for location: %s", cfg.Location)

	// Initialize OpenTelemetry
	shutdown, err := telemetry.InitTracer(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	// Initialize Redis client
	redisClient := redis.NewClient(cfg.RedisURL)
	defer redisClient.Close()

	// Initialize DNS resolver
	dnsResolver := dns.NewResolver(cfg)

	// Create worker
	w := worker.NewWorker(cfg, redisClient, dnsResolver)

	// Start HTTP server for health checks
	srv := server.NewServer(cfg, redisClient)
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Start worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := w.Start(ctx); err != nil {
			log.Printf("Worker error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down worker...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down server: %v", err)
	}

	cancel() // Stop worker
	time.Sleep(2 * time.Second)

	log.Println("Worker stopped")
}
