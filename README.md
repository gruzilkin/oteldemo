# OpenTelemetry Distributed Tracing Demo

A comprehensive demonstration of OpenTelemetry distributed tracing across **Java**, **Python**, and **Go** services, showcasing REST and event-based communication patterns, concurrent spans, and multi-tier collector architecture.

## Architecture Overview

This demo simulates a geo-distributed DNS lookup system that demonstrates:

- **Multi-language instrumentation**: Java (Spring Boot), Python (FastAPI), and Go (Gin)
- **Communication patterns**: Synchronous (REST) and asynchronous (Redis Streams)
- **Multi-tier OTEL collectors**: 6 sidecar collectors + 1 central aggregator with authentication
- **Full observability**: End-to-end trace visualization in Jaeger

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
- Complete trace export pipeline to Jaeger

## Prerequisites

- **Podman** and **Podman Compose** installed
- Ports available: 8080 (Gateway API), 16686 (Jaeger UI)

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

### 3. View Traces in Jaeger

Open your browser and navigate to:
```
http://localhost:16686
```