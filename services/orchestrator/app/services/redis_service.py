import os
import json
import logging
import asyncio
from typing import Optional, Dict, Any
from datetime import datetime

import redis.asyncio as redis
from opentelemetry import trace
from opentelemetry.propagate import inject

logger = logging.getLogger(__name__)
tracer = trace.get_tracer(__name__)


class RedisService:
    """Service for interacting with Redis Streams"""

    def __init__(self):
        self.redis_client: Optional[redis.Redis] = None
        self.redis_url = os.getenv("REDIS_URL", "redis://redis:6379")
        self.dns_tasks_stream = "dns:tasks"
        self.dns_results_stream = "dns:results"

    async def connect(self):
        """Connect to Redis"""
        try:
            self.redis_client = redis.from_url(
                self.redis_url,
                decode_responses=True,
                encoding="utf-8"
            )
            # Test connection
            await self.redis_client.ping()
            logger.info(f"Connected to Redis at {self.redis_url}")
        except Exception as e:
            logger.error(f"Failed to connect to Redis: {e}")
            raise

    async def disconnect(self):
        """Disconnect from Redis"""
        if self.redis_client:
            await self.redis_client.close()
            logger.info("Disconnected from Redis")

    def is_connected(self) -> bool:
        """Check if connected to Redis"""
        return self.redis_client is not None

    async def publish_dns_task(self, task_data: Dict[str, Any]) -> str:
        """
        Publish a DNS lookup task to Redis Stream with trace context

        Args:
            task_data: Dictionary containing task information

        Returns:
            Message ID from Redis Stream
        """
        with tracer.start_as_current_span("publish_dns_task") as span:
            try:
                span.set_attribute("task.id", task_data.get("task_id"))
                span.set_attribute("task.location", task_data.get("location"))
                span.set_attribute("task.domain", task_data.get("domain"))

                # Inject trace context into the task message
                trace_context = {}
                inject(trace_context)  # Inject current trace context using W3C TraceContext format
                task_data["trace_context"] = trace_context

                # Debug: Log trace context
                logger.info(f"Injected trace context into task {task_data.get('task_id')}: {trace_context}")

                # Convert task data to string fields for Redis Stream
                stream_data = {
                    "data": json.dumps(task_data)
                }

                message_id = await self.redis_client.xadd(
                    self.dns_tasks_stream,
                    stream_data
                )

                logger.info(f"Published DNS task {task_data.get('task_id')} to stream {self.dns_tasks_stream}")
                span.set_attribute("message.id", message_id)

                return message_id

            except Exception as e:
                logger.error(f"Error publishing DNS task: {e}")
                span.set_attribute("error", True)
                span.set_attribute("error.message", str(e))
                raise

    async def wait_for_results(self, trace_id: str, expected_count: int, timeout_seconds: int = 30) -> list:
        """
        Wait for worker results from Redis Stream

        Args:
            trace_id: The OpenTelemetry trace ID to filter results
            expected_count: Number of results to wait for
            timeout_seconds: Maximum time to wait

        Returns:
            List of result dictionaries
        """
        with tracer.start_as_current_span("wait_for_results") as span:
            span.set_attribute("expected.count", expected_count)

            results = []
            consumer_group = f"orchestrator-{trace_id}"
            consumer_name = f"consumer-{trace_id}"

            try:
                # Create consumer group (ignore error if exists)
                try:
                    await self.redis_client.xgroup_create(
                        self.dns_results_stream,
                        consumer_group,
                        id='$',
                        mkstream=True
                    )
                except redis.ResponseError as e:
                    if "BUSYGROUP" not in str(e):
                        raise

                start_time = datetime.now()
                last_id = ">"  # Only new messages

                while len(results) < expected_count:
                    # Check timeout
                    elapsed = (datetime.now() - start_time).total_seconds()
                    if elapsed > timeout_seconds:
                        logger.warning(f"Timeout waiting for results. Got {len(results)}/{expected_count}")
                        break

                    # Read from stream
                    messages = await self.redis_client.xreadgroup(
                        consumer_group,
                        consumer_name,
                        {self.dns_results_stream: last_id},
                        count=10,
                        block=1000  # Block for 1 second
                    )

                    if messages:
                        for stream_name, stream_messages in messages:
                            for message_id, message_data in stream_messages:
                                try:
                                    # Parse result data
                                    result_json = message_data.get("data", "{}")
                                    result = json.loads(result_json)

                                    # Filter by trace_id
                                    if result.get("trace_id") == trace_id:
                                        results.append(result)
                                        logger.info(f"Received result {len(results)}/{expected_count} for trace {trace_id}")

                                    # Acknowledge message
                                    await self.redis_client.xack(
                                        self.dns_results_stream,
                                        consumer_group,
                                        message_id
                                    )
                                except Exception as e:
                                    logger.error(f"Error processing message: {e}")

                # Cleanup consumer group
                try:
                    await self.redis_client.xgroup_destroy(self.dns_results_stream, consumer_group)
                except Exception:
                    pass

                span.set_attribute("results.count", len(results))
                return results

            except Exception as e:
                logger.error(f"Error waiting for results: {e}")
                span.set_attribute("error", True)
                span.set_attribute("error.message", str(e))
                return results


# Global instance
redis_service = RedisService()
