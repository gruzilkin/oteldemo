import logging
import uuid
from datetime import datetime
from typing import Dict, Any

from fastapi import APIRouter, HTTPException
from opentelemetry import trace
from opentelemetry.propagate import inject

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
    2. Creates tasks for each requested location
    3. Publishes tasks to Redis Streams
    4. Waits for worker results
    5. Aggregates and returns results
    """
    with tracer.start_as_current_span("orchestrate_dns_lookup") as span:
        span.set_attribute("request.id", request.request_id)
        span.set_attribute("dns.domain", request.domain)
        span.set_attribute("locations.count", len(request.locations))

        logger.info(f"Orchestrating DNS lookup for domain: {request.domain}, "
                   f"locations: {request.locations}, request_id: {request.request_id}")

        try:
            # Create tasks for each location
            tasks_published = 0
            task_ids = []

            for location in request.locations:
                task_id = f"{request.request_id}-{location}"
                task_ids.append(task_id)

                task_message = DnsTaskMessage(
                    request_id=request.request_id,
                    task_id=task_id,
                    domain=request.domain,
                    location=location,
                    record_types=request.record_types,
                    timestamp=datetime.utcnow().isoformat()
                )

                # Inject trace context into the task
                task_dict = task_message.model_dump()
                trace_context = {}
                inject(trace_context)  # Inject current trace context
                task_dict["trace_context"] = trace_context

                # Debug: Log trace context
                logger.info(f"Injected trace context for {location}: {trace_context}")

                # Publish to Redis Stream
                await redis_service.publish_dns_task(task_dict)
                tasks_published += 1

            logger.info(f"Published {tasks_published} tasks for request {request.request_id}")
            span.set_attribute("tasks.published", tasks_published)

            # Wait for results from workers
            results = await redis_service.wait_for_results(
                request_id=request.request_id,
                expected_count=tasks_published,
                timeout_seconds=30
            )

            # Aggregate results
            aggregated_results = aggregate_results(results)

            # Determine status
            if len(results) == 0:
                status = "timeout"
                message = "No results received from workers"
            elif len(results) < tasks_published:
                status = "partial"
                message = f"Received {len(results)}/{tasks_published} results"
            else:
                status = "success"
                message = f"Successfully processed {len(results)} location lookups"

            span.set_attribute("results.count", len(results))
            span.set_attribute("response.status", status)

            return DnsOrchestrateResponse(
                request_id=request.request_id,
                domain=request.domain,
                status=status,
                results=aggregated_results,
                message=message
            )

        except Exception as e:
            logger.error(f"Error orchestrating DNS lookup: {e}", exc_info=True)
            span.set_attribute("error", True)
            span.set_attribute("error.message", str(e))

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
