package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/oteldemo/workers/internal/config"
	"github.com/oteldemo/workers/internal/dns"
	"github.com/oteldemo/workers/internal/redis"
)

var tracer = otel.Tracer("dns-worker")

// Worker processes DNS lookup tasks
type Worker struct {
	cfg         *config.Config
	redis       *redis.Client
	dnsResolver *dns.Resolver
}

// NewWorker creates a new worker
func NewWorker(cfg *config.Config, redisClient *redis.Client, dnsResolver *dns.Resolver) *Worker {
	return &Worker{
		cfg:         cfg,
		redis:       redisClient,
		dnsResolver: dnsResolver,
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	log.Printf("Worker starting for location: %s", w.cfg.Location)

	// Create consumer group
	if err := w.redis.CreateConsumerGroup(ctx, w.cfg.TasksStream, w.cfg.ConsumerGroup); err != nil {
		return err
	}

	consumerName := "consumer-" + w.cfg.Location

	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Worker stopping...")
			return nil
		default:
			// First, check for pending messages (delivered but not acknowledged)
			// This ensures no messages are lost if worker crashes/restarts
			pendingMessages, err := w.redis.ReadPendingMessages(
				ctx,
				w.cfg.TasksStream,
				w.cfg.ConsumerGroup,
				consumerName,
			)

			if err != nil && !strings.Contains(err.Error(), "i/o timeout") {
				log.Printf("Error reading pending messages: %v", err)
			}

			// Process pending messages first
			for _, msg := range pendingMessages {
				w.processMessage(ctx, msg)
				if err := w.redis.AckMessage(ctx, w.cfg.TasksStream, w.cfg.ConsumerGroup, msg.ID); err != nil {
					log.Printf("Error acknowledging pending message: %v", err)
				}
			}

			// Then read new messages from stream
			messages, err := w.redis.ReadFromStream(
				ctx,
				w.cfg.TasksStream,
				w.cfg.ConsumerGroup,
				consumerName,
			)

			if err != nil {
				// Timeout errors are expected during idle periods, don't log them
				if !strings.Contains(err.Error(), "i/o timeout") {
					log.Printf("Error reading from stream: %v", err)
				}
				time.Sleep(1 * time.Second)
				continue
			}

			// Process each new message
			for _, msg := range messages {
				w.processMessage(ctx, msg)

				// Acknowledge message
				if err := w.redis.AckMessage(ctx, w.cfg.TasksStream, w.cfg.ConsumerGroup, msg.ID); err != nil {
					log.Printf("Error acknowledging message: %v", err)
				}
			}
		}
	}
}

// processMessage processes a single DNS lookup task
func (w *Worker) processMessage(ctx context.Context, msg redis.StreamMessage) {
	start := time.Now()

	// Extract task data
	dataJSON, ok := msg.Data["data"].(string)
	if !ok {
		log.Printf("Invalid message format: %v", msg.Data)
		return
	}

	var task Task
	if err := json.Unmarshal([]byte(dataJSON), &task); err != nil {
		log.Printf("Error parsing task: %v", err)
		return
	}

	// No filtering needed - each worker receives messages via its own consumer group
	log.Printf("Processing task %s for domain %s at worker location %s", task.TaskID, task.Domain, w.cfg.Location)

	// Extract trace context from task metadata if present
	carrier := propagation.MapCarrier{}
	if task.TraceContext != nil {
		log.Printf("Received trace context: %v", task.TraceContext)
		for k, v := range task.TraceContext {
			carrier.Set(k, v)
		}
	} else {
		log.Printf("WARNING: No trace context received for task %s", task.TaskID)
	}

	// Extract context from carrier
	prop := otel.GetTextMapPropagator()
	ctx = prop.Extract(ctx, carrier)

	// Start span for processing this task
	ctx, span := tracer.Start(ctx, "process_dns_task",
		trace.WithAttributes(
			attribute.String("task.id", task.TaskID),
			attribute.String("trace.id", task.TraceID),
			attribute.String("dns.domain", task.Domain),
			attribute.String("worker.location", w.cfg.Location),  // Use worker's configured location
		),
	)
	defer span.End()

	// Perform DNS lookups (concurrent!)
	results := w.dnsResolver.LookupAllRecords(ctx, task.Domain, task.RecordTypes)

	processingTime := time.Since(start)

	// Prepare result
	result := Result{
		TaskID:           task.TaskID,
		TraceID:          task.TraceID,
		Location:         w.cfg.Location,  // Use worker's configured location
		Domain:           task.Domain,
		Status:           "success",
		Records:          results,
		ProcessingTimeMs: float64(processingTime.Milliseconds()),
	}

	// Check if all lookups failed
	allFailed := true
	for _, r := range results {
		if r.Error == "" {
			allFailed = false
			break
		}
	}

	if allFailed {
		result.Status = "failed"
		result.Error = "All DNS lookups failed"
		err := errors.New("all DNS lookups failed")
		span.RecordError(err)
		span.SetStatus(codes.Error, "All DNS lookups failed")
	}

	span.SetAttributes(
		attribute.String("result.status", result.Status),
		attribute.Float64("processing_time_ms", result.ProcessingTimeMs),
	)

	// Publish result
	if _, err := w.redis.PublishResult(ctx, w.cfg.ResultsStream, result); err != nil {
		log.Printf("Error publishing result: %v", err)
		span.SetAttributes(
			attribute.Bool("error", true),
			attribute.String("error.message", err.Error()),
		)
	} else {
		log.Printf("Published result for task %s (processing time: %dms)", task.TaskID, processingTime.Milliseconds())
	}
}

// Task represents a DNS lookup task from Redis stream
// Note: All workers receive the same task via separate consumer groups (fan-out pattern)
type Task struct {
	TraceID      string            `json:"trace_id"`        // OpenTelemetry trace ID for correlation
	TaskID       string            `json:"task_id"`
	Domain       string            `json:"domain"`
	Location     string            `json:"location,omitempty"` // Not used - each worker uses its own configured location
	RecordTypes  []string          `json:"record_types"`
	Timestamp    string            `json:"timestamp"`
	TraceContext map[string]string `json:"trace_context,omitempty"`
}

// Result represents the result of a DNS lookup task
type Result struct {
	TaskID           string                      `json:"task_id"`
	TraceID          string                      `json:"trace_id"` // OpenTelemetry trace ID for correlation
	Location         string                      `json:"location"`
	Domain           string                      `json:"domain"`
	Status           string                      `json:"status"`
	Records          map[string]dns.LookupResult `json:"records"`
	Error            string                      `json:"error,omitempty"`
	ProcessingTimeMs float64                     `json:"processing_time_ms"`
}
