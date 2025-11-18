# OpenTelemetry Distributed Tracing Demo - Architecture

## Overview

This project demonstrates OpenTelemetry distributed tracing across Java, Python, and Go services using both REST and event-based communication patterns.

## Architecture Principles

### 1. **Trace ID as Correlation ID**

We use **OpenTelemetry's trace_id** as the universal correlation identifier instead of creating separate request IDs. This eliminates redundancy and leverages the built-in distributed tracing infrastructure.

**Benefits:**
- No need for manual ID generation (UUID, etc.)
- Automatic propagation through HTTP headers (W3C TraceContext)
- Direct correlation with traces in Jaeger
- Reduces complexity and potential for ID mismatches

**Implementation:**
```java
// Java: Extract trace_id from current span
Span currentSpan = Span.current();
String traceId = currentSpan.getSpanContext().getTraceId();
```

```python
# Python: Extract trace_id from current span
trace_id = format(span.get_span_context().trace_id, '032x')
```

```go
// Go: trace_id received via trace context propagation
// Task and Result structs use TraceID field for correlation
```

### 2. **Geographic Fan-Out Pattern**

The orchestrator publishes **ONE task** to Redis Streams, and **ALL workers** receive it via separate consumer groups. Each worker performs the same DNS lookup from their geographic location.

**Why Fan-Out?**
- Demonstrates how DNS results differ by geography (CDNs, geo-routing)
- Shows proper use of Redis Streams consumer groups for broadcasting
- Realistic use case for distributed systems

**Architecture:**
```
Orchestrator
    ↓
    Publishes 1 message to "dns:tasks"
    ↓ ↓ ↓ (Fan-out via consumer groups)

workers-us-east-1    workers-eu-west-1    workers-asia-south-1
       ↓                      ↓                      ↓
Worker US-East         Worker EU-West         Worker Asia-South
       ↓                      ↓                      ↓
DNS from US            DNS from EU            DNS from Asia
```

**Consumer Groups:**
- `workers-us-east-1` → Worker in US East region
- `workers-eu-west-1` → Worker in EU West region
- `workers-asia-south-1` → Worker in Asia South region

Each consumer group receives ALL messages (broadcast pattern), not load-balanced.

### 3. **Trace Context Propagation**

**HTTP (Gateway → Orchestrator):**
- Automatic via W3C TraceContext headers
- OpenTelemetry agents/SDKs handle injection/extraction
- No manual work needed

**Redis Streams (Orchestrator → Workers):**
- Manual injection required (no automatic propagation)
- Orchestrator embeds trace context in message:
  ```python
  trace_context = {}
  inject(trace_context)  # W3C TraceContext format
  task_dict["trace_context"] = trace_context
  ```
- Workers extract and restore context:
  ```go
  carrier := propagation.MapCarrier{}
  for k, v := range task.TraceContext {
      carrier.Set(k, v)
  }
  ctx = prop.Extract(ctx, carrier)
  ```

## Service Communication Flow

```
Client Request
    ↓
Gateway (Java) - Extracts trace_id from span
    ↓ (HTTP POST with W3C TraceContext headers)
Orchestrator (Python) - Extracts trace_id from span
    ↓ (Publishes to Redis with trace context)
Redis Streams "dns:tasks"
    ↓ ↓ ↓ (Fan-out to 3 consumer groups)
Workers (Go) - Extract trace context, perform DNS lookups
    ↓ ↓ ↓ (Publish results to Redis)
Redis Streams "dns:results"
    ↓ (Filter by trace_id)
Orchestrator - Aggregates results
    ↓ (HTTP Response)
Gateway - Returns to client
```

## Key Data Models

### Task Message (Orchestrator → Workers)
```json
{
  "trace_id": "b270a680b063957d2a986300d59e7163",
  "task_id": "b270a680b063957d2a986300d59e7163",
  "domain": "google.com",
  "record_types": ["A", "AAAA"],
  "timestamp": "2025-11-18T08:03:00Z",
  "trace_context": {
    "traceparent": "00-b270a680b063957d2a986300d59e7163-e9bad991e730350b-01"
  }
}
```

### Result Message (Workers → Orchestrator)
```json
{
  "task_id": "b270a680b063957d2a986300d59e7163",
  "trace_id": "b270a680b063957d2a986300d59e7163",
  "location": "us-east-1",
  "domain": "google.com",
  "status": "success",
  "records": { ... },
  "processing_time_ms": 32
}
```

## Observability

### Trace Structure
```
dns-gateway (Java)
└── POST /api/v1/dns/lookup
    └── HTTP call to orchestrator
        └── dns-orchestrator (Python)
            └── orchestrate_dns_lookup
                ├── publish_dns_task (Redis XADD)
                ├── wait_for_results (Redis XREADGROUP)
                └── 3 worker results in parallel:
                    ├── dns-worker-us-east-1 (Go)
                    │   └── process_dns_task
                    │       └── lookup_all_records
                    │           ├── lookup_a_record (concurrent)
                    │           └── lookup_aaaa_record (concurrent)
                    ├── dns-worker-eu-west-1 (Go)
                    │   └── [same structure]
                    └── dns-worker-asia-south-1 (Go)
                        └── [same structure]
```

### Span Attributes
- **trace.id**: OpenTelemetry trace ID (correlation)
- **task.id**: Same as trace_id for this demo
- **dns.domain**: Domain being queried
- **worker.location**: Geographic location of worker
- **result.status**: Success/failure status
- **processing_time_ms**: Processing duration

## Redis Streams Architecture

### Streams
- **dns:tasks**: Task distribution stream
- **dns:results**: Worker results stream

### Consumer Groups
**For dns:tasks (fan-out):**
- `workers-us-east-1`: US East worker group
- `workers-eu-west-1`: EU West worker group
- `workers-asia-south-1`: Asia South worker group

**For dns:results (per-orchestrator):**
- `orchestrator-{trace_id}`: Temporary group created per request
- Filters results by trace_id
- Cleaned up after collecting all results

## OpenTelemetry Collector Hierarchy

```
┌─────────────┐       ┌──────────────────┐       ┌─────────────────┐
│   Gateway   │──────>│ Gateway Collector│       │                 │
└─────────────┘       └──────────────────┘       │                 │
                               │                 │                 │
                               ↓                 │                 │
┌─────────────┐       ┌──────────────────┐      │    Central      │      ┌─────────┐
│Orchestrator │──────>│Orchestrator Coll.│─────>│   Collector     │─────>│ Jaeger  │
└─────────────┘       └──────────────────┘      │                 │      └─────────┘
                                                 │                 │
┌─────────────┐                                 │                 │
│ Workers x3  │────────────────────────────────>│                 │
└─────────────┘                                 └─────────────────┘
```

**Architecture:**
- **Sidecar collectors**: Gateway and Orchestrator have dedicated collectors
- **Direct connection**: Workers send directly to central collector (simplified)
- **Central collector**: Aggregates and forwards to Jaeger
- **No authentication**: Simplified for demo purposes

## Concurrency Patterns

### Go Workers - Concurrent DNS Lookups
Each worker spawns **5 concurrent goroutines** for different DNS record types:

```go
var wg sync.WaitGroup
for _, recordType := range recordTypes {
    wg.Add(1)
    go func(rt string) {
        defer wg.Done()
        _, span := tracer.Start(ctx, fmt.Sprintf("lookup_%s_record", rt))
        defer span.End()
        // Perform DNS lookup
    }(recordType)
}
wg.Wait()
```

This creates **parallel child spans** visible in Jaeger, demonstrating Go's concurrency model in distributed tracing.

## Testing

### Basic Request
```bash
curl -X POST http://localhost:8080/api/v1/dns/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "google.com",
    "locations": ["us-east-1", "eu-west-1", "asia-south-1"],
    "record_types": ["A", "AAAA"]
  }'
```

### View Traces
1. Open Jaeger: http://localhost:16686
2. Select **dns-gateway** service
3. Click "Find Traces"
4. Examine trace structure and spans

### Expected Results
- **1 task** published to Redis
- **3 workers** process the same task
- **3 results** aggregated by orchestrator
- **Single trace** showing all operations
- **Concurrent spans** for DNS lookups in each worker

## Key Learnings

1. **Use trace_id for correlation** - Don't create redundant request IDs
2. **Consumer groups for fan-out** - Each group gets ALL messages
3. **Manual propagation for events** - Redis/Kafka require explicit trace context embedding
4. **Automatic propagation for HTTP** - OpenTelemetry handles it transparently
5. **Geographic distribution** - Real use case for DNS/CDN systems
