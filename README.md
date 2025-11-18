# OpenTelemetry Distributed Tracing Demo

A comprehensive demonstration of OpenTelemetry distributed tracing across **Java**, **Python**, and **Go** services, showcasing REST and event-based communication patterns, concurrent spans, and multi-tier collector architecture.

## Architecture Overview

This demo simulates a geo-distributed DNS lookup system that demonstrates:

- **Multi-language tracing**: Java (Spring Boot), Python (FastAPI), and Go (Gin)
- **Multiple communication patterns**: REST API calls and Redis Streams (event-based)
- **Go concurrency**: Parallel DNS lookups with concurrent child spans
- **Multi-tier OTEL Collectors**: Sidecar collectors with authentication forwarding to central collector
- **Full observability**: End-to-end trace visualization in Jaeger

### Service Architecture

```
┌──────────┐
│  Client  │
└────┬─────┘
     │ REST
     ▼
┌─────────────────┐      ┌──────────────────┐
│ API Gateway     │◄────►│ Gateway Collector│──┐
│ (Java/Spring)   │      └──────────────────┘  │
└────┬────────────┘                            │
     │ REST                                    │ OTLP + Auth
     ▼                                         │
┌─────────────────┐      ┌──────────────────┐ │
│ Orchestrator    │◄────►│ Orch Collector   │─┤
│ (Python/FastAPI)│      └──────────────────┘ │
└────┬────────────┘                            │
     │ Redis Streams                           │
     ├────────────┬────────────┬───────────┐   │
     ▼            ▼            ▼           ▼   │
┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
│Worker-US│  │Worker-EU│  │Worker   │  │ Central │
│(Go/Gin) │  │(Go/Gin) │  │Asia     │  │Collector│
│         │  │         │  │(Go/Gin) │  │         │
└────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘
     │            │            │             │
     │ Concurrent DNS Lookups  │             │ OTLP
     │   (A, AAAA, MX, TXT,    │             ▼
     │    NS records)          │        ┌─────────┐
     │                         │        │ Jaeger  │
     └─────────┬───────────────┘        │   UI    │
               │ Results                └─────────┘
               ▼
          Redis Streams
```

### What Gets Demonstrated

#### 1. **Java Auto-Instrumentation** (Gateway Service)
- Automatic HTTP server/client instrumentation
- Spring Boot integration with OTEL Java agent
- Span attribute enrichment

#### 2. **Python Async Patterns** (Orchestrator Service)
- FastAPI automatic instrumentation
- Redis Streams publisher with tracing
- Async/await context propagation
- Result aggregation from multiple workers

#### 3. **Go Concurrent Spans** (Worker Services)
- **Manual span creation** for fine-grained control
- **Goroutines with child spans** - each DNS record type lookup creates a child span
- Context propagation across goroutines
- Demonstrates parallel processing in a single trace

#### 4. **Communication Patterns**
- **REST**: Client → Gateway → Orchestrator
- **Event-based**: Orchestrator → Workers via Redis Streams
- **Result collection**: Workers → Orchestrator via Redis Streams

#### 5. **Multi-Tier OTEL Collectors**
- **Sidecar collectors**: One per service
- **API Key authentication**: Sidecars authenticate to central collector
- **Central collector**: Aggregates all traces and exports to Jaeger
- **Resource attributes**: Each collector adds identifying metadata

## Prerequisites

- **Podman** and **Podman Compose** installed
- At least **4GB RAM** available for containers
- Ports available: 8080, 8001, 8082-8084, 6379, 16686, 4317-4319

## Quick Start

### 1. Build and Start All Services

```bash
podman-compose build
podman-compose up -d
```

This will start:
- 1 Redis instance
- 1 Jaeger instance
- 1 Central OTEL Collector
- 6 Sidecar OTEL Collectors (one per service)
- 1 Java Gateway
- 1 Python Orchestrator
- 3 Go Workers (simulating US, EU, and Asia locations)

### 2. Wait for Services to Be Healthy

```bash
# Check all services are running
podman-compose ps

# Check gateway health
curl http://localhost:8080/api/v1/health

# Check orchestrator health
curl http://localhost:8001/health

# Check workers health
curl http://localhost:8082/health  # US worker
curl http://localhost:8083/health  # EU worker
curl http://localhost:8084/health  # Asia worker
```

### 3. Send a DNS Lookup Request

```bash
curl -X POST http://localhost:8080/api/v1/dns/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "google.com",
    "locations": ["us-east-1", "eu-west-1", "asia-south-1"],
    "record_types": ["A", "AAAA", "MX", "TXT", "NS"]
  }'
```

### 4. View Traces in Jaeger

Open your browser and navigate to:
```
http://localhost:16686
```

1. Select service: **dns-gateway**, **dns-orchestrator**, or any worker
2. Click **Find Traces**
3. Click on a trace to see the full distributed trace

## What to Look For in Jaeger

### Trace Structure

A complete trace will show:

1. **Gateway Span** (`dns-gateway`)
   - HTTP POST /api/v1/dns/lookup
   - Attributes: domain, locations, request_id

2. **Gateway → Orchestrator HTTP Call** (`dns-gateway`)
   - HTTP client span to orchestrator

3. **Orchestrator Receive** (`dns-orchestrator`)
   - HTTP server span receiving request

4. **Orchestrator Process** (`dns-orchestrator`)
   - Redis Stream publish operations (one per location)

5. **Worker Spans** (3 parallel branches - one per location)
   - **Parent span**: `process_dns_task`
   - **Child spans** (5 concurrent goroutines per worker):
     - `lookup_a_record` - IPv4 addresses
     - `lookup_aaaa_record` - IPv6 addresses
     - `lookup_mx_record` - Mail servers
     - `lookup_txt_record` - Text records
     - `lookup_ns_record` - Nameservers

### Concurrent Spans in Action

Each worker will show **5 parallel child spans** running concurrently:

```
process_dns_task (us-east-1)
├── lookup_a_record     ─────────┐
├── lookup_aaaa_record  ────────┤
├── lookup_mx_record    ────────┤ Running
├── lookup_txt_record   ────────┤ in parallel
└── lookup_ns_record    ─────────┘ via goroutines
```

### Span Attributes to Explore

- **dns.domain**: Domain being queried
- **dns.record_type**: Type of DNS record
- **dns.records.count**: Number of records found
- **dns.duration_ms**: Time taken for lookup
- **worker.location**: Geographic location of worker
- **collector.name**: Which collector processed this span
- **processing_time_ms**: Total processing time

## Architecture Deep Dive

### OpenTelemetry Collector Flow

```
Service → Sidecar Collector → Central Collector → Jaeger
           (OTLP/gRPC)         (OTLP + Auth)      (OTLP)
```

**Sidecar Collectors**:
- Receive traces from their associated service
- Add resource attributes (collector name, type)
- Batch and forward to central collector
- Authenticate using bearer token

**Central Collector**:
- Validates API key from sidecars
- Aggregates traces from all services
- Exports to Jaeger backend
- Provides health check and debugging endpoints

### DNS Lookup Concurrency (Go Workers)

The workers demonstrate Go's concurrency model:

```go
// From internal/dns/resolver.go
func (r *Resolver) LookupAllRecords(ctx context.Context, domain string, recordTypes []string) {
    var wg sync.WaitGroup

    for _, recordType := range recordTypes {
        wg.Add(1)

        // Each goroutine gets its own child span
        go func(rt string) {
            defer wg.Done()

            // Create child span for this concurrent lookup
            _, span := tracer.Start(ctx, fmt.Sprintf("lookup_%s_record", rt))
            defer span.End()

            // Perform DNS lookup using dig
            result := r.lookupRecord(domain, rt)

            // Add attributes to span
            span.SetAttributes(...)
        }(recordType)
    }

    wg.Wait() // Wait for all concurrent lookups
}
```

## Configuration

### Environment Variables

Edit `.env` to customize:

```bash
# OTEL Collector authentication key
COLLECTOR_API_KEY=demo-secret-key-12345

# Redis connection
REDIS_URL=redis://redis:6379

# Jaeger endpoint
JAEGER_ENDPOINT=jaeger:4317
```

### Collector Configuration

**Sidecar**: `otel-collectors/sidecar-config.yaml`
- OTLP receivers (gRPC and HTTP)
- Batch processor
- Resource processor (adds metadata)
- OTLP exporter with authentication

**Central**: `otel-collectors/central-config.yaml`
- OTLP receivers with auth
- Bearer token authenticator
- Batch and sampling processors
- Jaeger exporter
- Health check, pprof, and zpages extensions

## Testing Different Scenarios

### Test with Different Domains

```bash
# Test with cloudflare.com
curl -X POST http://localhost:8080/api/v1/dns/lookup \
  -H "Content-Type: application/json" \
  -d '{"domain": "cloudflare.com", "locations": ["us-east-1", "eu-west-1"]}'

# Test with github.com
curl -X POST http://localhost:8080/api/v1/dns/lookup \
  -H "Content-Type: application/json" \
  -d '{"domain": "github.com", "locations": ["asia-south-1"]}'
```

### Test with Specific Record Types

```bash
# Only A and AAAA records
curl -X POST http://localhost:8080/api/v1/dns/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "locations": ["us-east-1"],
    "record_types": ["A", "AAAA"]
  }'
```

### Load Testing

Generate multiple concurrent requests:

```bash
for i in {1..10}; do
  curl -X POST http://localhost:8080/api/v1/dns/lookup \
    -H "Content-Type: application/json" \
    -d "{\"domain\": \"test${i}.com\", \"locations\": [\"us-east-1\", \"eu-west-1\", \"asia-south-1\"]}" &
done
wait
```

## Monitoring

### Service Health Endpoints

- Gateway: http://localhost:8080/api/v1/health
- Orchestrator: http://localhost:8001/health
- Worker (US): http://localhost:8082/health
- Worker (EU): http://localhost:8083/health
- Worker (Asia): http://localhost:8084/health

### Collector Endpoints

- Central Collector Health: http://localhost:13133
- Central Collector zpages: http://localhost:55679/debug/tracez
- Central Collector Metrics: http://localhost:8888/metrics

### Jaeger UI

- Main UI: http://localhost:16686
- Health Check: http://localhost:16686/

## Troubleshooting

### No traces appearing in Jaeger

1. Check all services are running:
   ```bash
   podman-compose ps
   ```

2. Check collector logs:
   ```bash
   podman logs oteldemo-central-collector
   podman logs oteldemo-gateway-collector
   ```

3. Verify API key matches in `.env` and collector configs

### Workers not processing tasks

1. Check Redis connection:
   ```bash
   podman exec -it oteldemo-redis redis-cli ping
   ```

2. Check Redis streams:
   ```bash
   podman exec -it oteldemo-redis redis-cli XINFO STREAM dns:tasks
   ```

3. Check worker logs:
   ```bash
   podman logs oteldemo-worker-us-east
   ```

### DNS lookups failing

Workers need `dig` command, which is installed in the container. If lookups fail:

```bash
# Test dig inside worker container
podman exec -it oteldemo-worker-us-east dig google.com A +short
```

## Cleanup

Stop and remove all containers:

```bash
podman-compose down
```

Remove built images:

```bash
podman-compose down --rmi all
```

Clean up volumes:

```bash
podman volume prune
```

## Project Structure

```
oteldemo/
├── services/
│   ├── gateway/              # Java Spring Boot API Gateway
│   │   ├── src/main/java/com/oteldemo/gateway/
│   │   ├── pom.xml
│   │   └── Containerfile
│   ├── orchestrator/         # Python FastAPI Orchestrator
│   │   ├── app/
│   │   │   ├── main.py
│   │   │   ├── models/
│   │   │   ├── routes/
│   │   │   └── services/
│   │   ├── requirements.txt
│   │   └── Containerfile
│   └── workers/              # Go Gin DNS Workers
│       ├── cmd/worker/
│       ├── internal/
│       │   ├── config/
│       │   ├── dns/         # Concurrent DNS resolver
│       │   ├── redis/
│       │   ├── server/
│       │   ├── telemetry/
│       │   └── worker/
│       ├── go.mod
│       └── Containerfile
├── otel-collectors/
│   ├── sidecar-config.yaml   # Sidecar collector config
│   └── central-config.yaml   # Central collector config
├── podman-compose.yml        # Container orchestration
├── .env                      # Environment variables
└── README.md                 # This file
```

## Learning Objectives

After running this demo, you will understand:

1. **How to instrument applications** in Java, Python, and Go with OpenTelemetry
2. **Context propagation** across HTTP and message queue boundaries
3. **Concurrent span creation** in Go using goroutines
4. **Multi-tier collector architecture** with authentication
5. **Different communication patterns** and their tracing implications
6. **How to visualize** distributed traces in Jaeger
7. **Resource attributes** and span enrichment best practices

## Next Steps

- Modify the code to add custom span attributes
- Implement error scenarios and see how they appear in traces
- Add metrics and logs to the observability stack
- Experiment with different sampling strategies
- Add more complex business logic and trace it

## License

This is a demonstration project for learning OpenTelemetry.

## Contributing

Feel free to extend this demo with additional features like:
- Additional programming languages (Rust, .NET, etc.)
- More complex event patterns
- Metrics and logging integration
- Custom instrumentation examples
- Performance testing scenarios
