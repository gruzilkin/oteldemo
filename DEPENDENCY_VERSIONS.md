# OpenTelemetry Dependency Versions

This document tracks the OpenTelemetry and related dependency versions used in this project.

**Last Updated:** November 2025

## Java (Gateway Service)

### Framework
- **Spring Boot:** 3.4.1 (latest stable)
- **Java Version:** 17

### OpenTelemetry
- **OpenTelemetry API:** 1.56.0
- **OpenTelemetry Java Agent:** 2.20.1

### Configuration
All OpenTelemetry versions are managed via the `${opentelemetry.version}` property in `pom.xml`:

```xml
<properties>
    <opentelemetry.version>1.56.0</opentelemetry.version>
</properties>
```

### Agent Download
The Java agent is downloaded during Docker build from:
```
https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v2.20.1/opentelemetry-javaagent.jar
```

## Python (Orchestrator Service)

### Framework
- **FastAPI:** >=0.115.0
- **Uvicorn:** >=0.34.0
- **Pydantic:** >=2.10.0

### OpenTelemetry
- **opentelemetry-api:** >=1.38.0
- **opentelemetry-sdk:** >=1.38.0
- **opentelemetry-instrumentation-fastapi:** >=0.59b0
- **opentelemetry-instrumentation-redis:** >=0.59b0
- **opentelemetry-exporter-otlp-proto-grpc:** >=1.38.0

### Other Dependencies
- **redis:** >=5.2.0
- **python-dotenv:** >=1.0.0

### Version Strategy
Using `>=` allows pip to install the latest compatible versions while ensuring minimum version requirements.

## Go (Worker Services)

### Language
- **Go Version:** 1.23 (latest stable)

### Framework
- **Gin:** 1.10.0
- **Redis Client:** github.com/redis/go-redis/v9 v9.7.0

### OpenTelemetry
- **go.opentelemetry.io/otel:** v1.35.0
- **go.opentelemetry.io/otel/sdk:** v1.35.0
- **go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc:** v1.35.0
- **go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin:** v0.57.0

### Configuration
Versions are specified in `go.mod`:

```go
go 1.23

require (
    go.opentelemetry.io/otel v1.35.0
    go.opentelemetry.io/otel/sdk v1.35.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0
    go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.57.0
)
```

## Update Strategy

### Why Latest Versions?
1. **Security:** Latest versions include important security patches
2. **Features:** Access to newest OpenTelemetry capabilities
3. **Performance:** Improvements in trace collection and export
4. **Compatibility:** Better integration with modern observability platforms
5. **Standards:** Alignment with latest W3C TraceContext specifications

### How to Update

**Java:**
1. Update `<opentelemetry.version>` in `pom.xml`
2. Update Java agent version in `Containerfile`
3. Update Spring Boot version if needed

**Python:**
1. Update minimum versions in `requirements.txt`
2. Run `pip install --upgrade` to get latest compatible versions

**Go:**
1. Update version numbers in `go.mod`
2. Delete `go.sum`
3. Run `go mod tidy && go mod download`

### Testing After Updates
```bash
# Rebuild all services
podman compose build

# Restart services
podman compose up -d --force-recreate

# Test DNS lookup
curl -X POST http://localhost:8080/api/v1/dns/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "locations": ["us-east-1", "eu-west-1"],
    "record_types": ["A"]
  }'

# Verify traces in Jaeger
open http://localhost:16686
```

## Version Compatibility

### OpenTelemetry SDK Compatibility
- **Java SDK 1.56.0** ↔ **Java Agent 2.20.1** ✅ Compatible
- **Python SDK 1.38.0** ↔ **Instrumentation 0.59b0** ✅ Compatible
- **Go SDK 1.35.0** ↔ **Contrib 0.57.0** ✅ Compatible

### Service Communication
All services use **OTLP (OpenTelemetry Protocol)** over gRPC, which is version-compatible across:
- Java → Collector
- Python → Collector
- Go → Collector

### W3C TraceContext
All versions support the **W3C TraceContext** specification for trace propagation:
- HTTP headers: `traceparent`, `tracestate`
- Format: `00-{trace-id}-{span-id}-{trace-flags}`

## Maintenance Schedule

**Recommended Update Frequency:**
- **Security patches:** Immediately
- **Minor versions:** Monthly check
- **Major versions:** Quarterly review

**Resources:**
- Java: https://github.com/open-telemetry/opentelemetry-java/releases
- Python: https://pypi.org/project/opentelemetry-api/
- Go: https://github.com/open-telemetry/opentelemetry-go/releases
