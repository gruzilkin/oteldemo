import logging
import uuid
from datetime import datetime
from typing import Dict, Any

from fastapi import APIRouter, HTTPException
from opentelemetry import trace
from opentelemetry.trace import Status, StatusCode

from app.models.dns_models import (
    DnsOrchestrateRequest,
    DnsOrchestrateResponse,
    DnsTaskMessage
)
from app.services.redis_service import redis_service

logger = logging.getLogger(__name__)
tracer = trace.get_tracer(__name__)

router = APIRouter()


@router.post("/dns/orchestrate", response_model=DnsOrchestrateResponse)
async def orchestrate_dns_lookup(request: DnsOrchestrateRequest):
    """
    Orchestrate distributed DNS lookups across multiple locations

    This endpoint:
    1. Receives a DNS lookup request
    2. Extracts trace_id from current span for correlation
    3. Creates a task and publishes to Redis Streams
    4. Waits for worker results (correlated by trace_id)
    5. Aggregates and returns results
    """
    with tracer.start_as_current_span("orchestrate_dns_lookup") as span:
        # Extract trace_id from current span - this is our correlation ID
        trace_id = format(span.get_span_context().trace_id, '032x')

        span.set_attribute("dns.domain", request.domain)
        span.set_attribute("locations.count", len(request.locations))

        logger.info(f"Orchestrating DNS lookup for domain: {request.domain}, "
                   f"locations: {request.locations}")

        try:
            # Create ONE task that will be consumed by all worker locations (fan-out)
            task_message = DnsTaskMessage(
                trace_id=trace_id,
                task_id=trace_id,  # Use trace_id as task_id for simplicity
                domain=request.domain,
                location="",  # No specific location - all workers process it
                record_types=request.record_types,
                timestamp=datetime.utcnow().isoformat()
            )

            # Publish to Redis Stream ONCE - all workers will receive it via their consumer groups
            # Trace context injection happens inside publish_dns_task
            await redis_service.publish_dns_task(task_message.model_dump())

            logger.info(f"Published task - expecting {len(request.locations)} worker responses")
            span.set_attribute("tasks.published", 1)
            span.set_attribute("expected_workers", len(request.locations))

            # Wait for results from ALL workers (each worker's consumer group gets the same message)
            results = await redis_service.wait_for_results(
                trace_id=trace_id,
                expected_count=len(request.locations)  # Expect one result per location
            )

            # Aggregate results
            aggregated_results = aggregate_results(results)

            # Determine status
            expected_results = len(request.locations)
            if len(results) == 0:
                status = "timeout"
                message = "No results received from workers"
            elif len(results) < expected_results:
                status = "partial"
                message = f"Received {len(results)}/{expected_results} results from worker locations"
            else:
                status = "success"
                message = f"Successfully received results from all {len(results)} worker locations"

            span.set_attribute("results.count", len(results))
            span.set_attribute("response.status", status)

            return DnsOrchestrateResponse(
                domain=request.domain,
                status=status,
                results=aggregated_results,
                message=message
            )

        except Exception as e:
            logger.error(f"Error orchestrating DNS lookup: {e}", exc_info=True)
            span.record_exception(e)
            span.set_status(Status(StatusCode.ERROR, "Error orchestrating DNS lookup"))

            raise HTTPException(
                status_code=500,
                detail=f"Error orchestrating DNS lookup: {str(e)}"
            )


def aggregate_results(results: list) -> Dict[str, Any]:
    """Aggregate results from multiple workers"""
    aggregated = {
        "by_location": {},
        "summary": {
            "total_locations": len(results),
            "successful": 0,
            "failed": 0
        }
    }

    for result in results:
        location = result.get("location", "unknown")
        status = result.get("status", "unknown")

        aggregated["by_location"][location] = {
            "status": status,
            "records": result.get("records", {}),
            "error": result.get("error"),
            "processing_time_ms": result.get("processing_time_ms", 0)
        }

        if status == "success":
            aggregated["summary"]["successful"] += 1
        else:
            aggregated["summary"]["failed"] += 1

    return aggregated
