# OpenTelemetry Distributed Tracing Demo

A comprehensive demonstration of OpenTelemetry distributed tracing across **Java**, **Python**, and **Go** services, showcasing REST and event-based communication patterns, concurrent spans, and multi-tier collector architecture.

## Architecture Overview

This demo simulates a geo-distributed DNS lookup system that demonstrates:

- **Multi-language instrumentation**: Java (Spring Boot), Python (FastAPI), and Go
- **Communication patterns**: Synchronous (REST) and asynchronous (Redis Streams)
- **Multi-tier OTEL collectors**: 6 sidecar collectors + 1 central aggregator with authentication
- **Full observability**: Distributed tracing with multi-tier collectors and multiple exporters

See [map.md](map.md) for detailed architecture diagrams.

### What Gets Demonstrated

**Instrumentation across languages:**
- Java: Auto-instrumentation with OTEL Java agent
- Python: FastAPI and Redis automatic instrumentation
- Go: Manual span creation with concurrent goroutines

**Communication patterns:**
- Synchronous: HTTP REST calls with automatic trace propagation
- Asynchronous: Redis Streams with manual trace context injection/extraction

**Multi-tier OTEL collectors:**
- Sidecar collectors for each service with resource attribute enrichment
- Central collector with authentication and aggregation
- Multiple telemetry data consumers: Graylog, Grafana, Jaeger

**Chaos engineering (Go workers):**
- 30% probability of sequential vs concurrent DNS lookups
- 10% probability of simulated lookup failures

## Prerequisites

- **Podman** and **Podman Compose** installed
- Ports available: 8080 (Gateway API), 3000 (Grafana), 9000 (Graylog), 16686 (Jaeger UI)

## Quick Start

### 1. Start All Services

```bash
podman compose up -d
```

### 2. Send a DNS Lookup Request

```bash
curl -X POST http://localhost:8080/api/v1/dns/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "google.com",
    "locations": ["us-east-1", "eu-west-1", "asia-south-1"],
    "record_types": ["A", "AAAA", "MX", "TXT", "NS"]
  }'
```

### 3. View Observability Data

**Traces:**
- **Jaeger UI**: http://localhost:16686
- **Grafana**: http://localhost:3000 (login: admin/admin)

**Logs:**
- **Grafana LGTM**: http://localhost:3000 (login: admin/admin)
- **Graylog**: http://localhost:9000 (login: admin/admin)

**Initial Graylog Setup** (first time only):
1. Navigate to http://localhost:9000
2. Login with admin/admin
3. Complete the DataNode preflight setup
4. Go to System → Inputs
5. Select "OpenTelemetry (gRPC)" and launch new input
6. Configure: Bind address `0.0.0.0`, Port `4317`, TLS disabled
7. Save and start receiving logs