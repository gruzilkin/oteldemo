package dns

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/oteldemo/workers/internal/config"
)

var tracer = otel.Tracer("dns-resolver")

// Resolver handles DNS lookups
type Resolver struct {
	cfg *config.Config
}

// NewResolver creates a new DNS resolver
func NewResolver(cfg *config.Config) *Resolver {
	return &Resolver{cfg: cfg}
}

// LookupResult represents the result of a DNS lookup
type LookupResult struct {
	RecordType string        `json:"record_type"`
	Records    []string      `json:"records"`
	Duration   time.Duration `json:"duration_ms"`
	Error      string        `json:"error,omitempty"`
}

// LookupAllRecords performs DNS lookups for multiple record types
// Randomly chooses between concurrent and sequential execution based on chaos probability
func (r *Resolver) LookupAllRecords(ctx context.Context, domain string, recordTypes []string) map[string]LookupResult {
	ctx, span := tracer.Start(ctx, "lookup_all_records",
		trace.WithAttributes(
			attribute.String("dns.domain", domain),
			attribute.Int("dns.record_types.count", len(recordTypes)),
		),
	)
	defer span.End()

	// Randomly decide execution mode based on chaos probability
	useSequential := rand.Float64() < r.cfg.ChaosSequentialProbability
	executionMode := "concurrent"
	if useSequential {
		executionMode = "sequential"
	}

	span.SetAttributes(
		attribute.String("execution.mode", executionMode),
		attribute.Float64("chaos.sequential_probability", r.cfg.ChaosSequentialProbability),
	)

	var results map[string]LookupResult
	if useSequential {
		results = r.lookupSequential(ctx, domain, recordTypes)
	} else {
		results = r.lookupConcurrent(ctx, domain, recordTypes)
	}

	span.SetAttributes(
		attribute.Int("dns.results.count", len(results)),
	)

	return results
}

// lookupConcurrent performs concurrent DNS lookups using goroutines
func (r *Resolver) lookupConcurrent(ctx context.Context, domain string, recordTypes []string) map[string]LookupResult {
	// Create a map to store results
	results := make(map[string]LookupResult)
	var mu sync.Mutex // Protect results map

	// Use WaitGroup to wait for all goroutines
	var wg sync.WaitGroup

	// Launch concurrent goroutines for each record type
	for _, recordType := range recordTypes {
		wg.Add(1)

		// Launch goroutine with its own span
		go func(rt string) {
			defer wg.Done()

			// Create child span for this lookup
			_, lookupSpan := tracer.Start(ctx, fmt.Sprintf("lookup_%s_record", strings.ToLower(rt)),
				trace.WithAttributes(
					attribute.String("dns.record_type", rt),
					attribute.String("dns.domain", domain),
				),
			)
			defer lookupSpan.End()

			// Perform the actual DNS lookup
			result := r.lookupRecord(domain, rt)

			// Add span attributes based on result
			lookupSpan.SetAttributes(
				attribute.Int("dns.records.count", len(result.Records)),
				attribute.Int64("dns.duration_ms", result.Duration.Milliseconds()),
			)

			if result.Error != "" {
				lookupSpan.SetAttributes(
					attribute.Bool("error", true),
					attribute.String("error.message", result.Error),
				)
				// Mark chaos-injected errors
				if strings.HasPrefix(result.Error, "chaos engineering:") {
					lookupSpan.SetAttributes(attribute.Bool("chaos.injected_error", true))
				}
			}

			// Store result (thread-safe)
			mu.Lock()
			results[rt] = result
			mu.Unlock()
		}(recordType)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return results
}

// lookupSequential performs DNS lookups sequentially (chaos mode - simulates poor performance)
func (r *Resolver) lookupSequential(ctx context.Context, domain string, recordTypes []string) map[string]LookupResult {
	results := make(map[string]LookupResult)

	// Process each record type sequentially (no concurrency)
	for _, rt := range recordTypes {
		// Create child span for this lookup
		_, lookupSpan := tracer.Start(ctx, fmt.Sprintf("lookup_%s_record", strings.ToLower(rt)),
			trace.WithAttributes(
				attribute.String("dns.record_type", rt),
				attribute.String("dns.domain", domain),
			),
		)

		// Perform the actual DNS lookup
		result := r.lookupRecord(domain, rt)

		// Add span attributes based on result
		lookupSpan.SetAttributes(
			attribute.Int("dns.records.count", len(result.Records)),
			attribute.Int64("dns.duration_ms", result.Duration.Milliseconds()),
		)

		if result.Error != "" {
			lookupSpan.SetAttributes(
				attribute.Bool("error", true),
				attribute.String("error.message", result.Error),
			)
			// Mark chaos-injected errors
			if strings.HasPrefix(result.Error, "chaos engineering:") {
				lookupSpan.SetAttributes(attribute.Bool("chaos.injected_error", true))
			}
		}

		lookupSpan.End()

		// Store result
		results[rt] = result
	}

	return results
}

// lookupRecord performs a DNS lookup for a specific record type using dig
func (r *Resolver) lookupRecord(domain, recordType string) LookupResult {
	start := time.Now()

	result := LookupResult{
		RecordType: recordType,
		Records:    []string{},
	}

	// Chaos engineering: randomly inject DNS lookup failure
	if rand.Float64() < r.cfg.ChaosErrorProbability {
		result.Duration = time.Since(start)
		result.Error = "chaos engineering: simulated DNS lookup failure"
		return result
	}

	// Execute dig command
	cmd := exec.Command("dig", "+short", domain, recordType)
	output, err := cmd.CombinedOutput()

	result.Duration = time.Since(start)

	if err != nil {
		result.Error = fmt.Sprintf("dig command failed: %v", err)
		return result
	}

	// Parse output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result.Records = append(result.Records, line)
		}
	}

	return result
}
