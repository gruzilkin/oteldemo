import os
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.redis import RedisInstrumentor

from app.routes import dns_routes
from app.services.redis_service import redis_service

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def setup_telemetry():
    """Configure OpenTelemetry tracing"""
    # Create resource
    resource = Resource.create({
        "service.name": os.getenv("OTEL_SERVICE_NAME", "dns-orchestrator"),
        "service.version": "1.0.0",
    })

    # Create tracer provider
    tracer_provider = TracerProvider(resource=resource)

    # Configure OTLP exporter
    otlp_exporter = OTLPSpanExporter(
        endpoint=os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://orchestrator-collector:4317"),
        insecure=True
    )

    # Add span processor
    tracer_provider.add_span_processor(BatchSpanProcessor(otlp_exporter))

    # Set global tracer provider
    trace.set_tracer_provider(tracer_provider)

    # Instrument Redis
    RedisInstrumentor().instrument()

    logger.info("OpenTelemetry configured successfully")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifecycle manager for FastAPI application"""
    # Startup
    logger.info("Starting DNS Orchestrator service...")
    setup_telemetry()
    await redis_service.connect()
    logger.info("DNS Orchestrator service started successfully")

    yield

    # Shutdown
    logger.info("Shutting down DNS Orchestrator service...")
    await redis_service.disconnect()
    logger.info("DNS Orchestrator service shut down successfully")


# Create FastAPI app
app = FastAPI(
    title="DNS Lookup Orchestrator",
    description="Orchestrates distributed DNS lookups across multiple geo-locations",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routes
app.include_router(dns_routes.router, prefix="/api/v1", tags=["DNS"])

# Instrument FastAPI with OpenTelemetry
FastAPIInstrumentor.instrument_app(app)


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "service": "dns-orchestrator",
        "redis_connected": redis_service.is_connected()
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)
