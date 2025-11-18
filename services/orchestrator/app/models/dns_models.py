from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field


class DnsOrchestrateRequest(BaseModel):
    """Request model for DNS orchestration"""
    domain: str = Field(..., description="Domain name to lookup")
    locations: List[str] = Field(default=["us-east-1", "eu-west-1", "asia-south-1"],
                                 description="Geo locations for DNS lookup")
    record_types: List[str] = Field(default=["A", "AAAA", "MX", "TXT", "NS"],
                                    description="DNS record types to query")


class DnsTaskMessage(BaseModel):
    """Message format for DNS tasks sent to workers via Redis Streams"""
    trace_id: str
    task_id: str
    domain: str
    location: str  # Optional, not used in fan-out pattern
    record_types: List[str]
    timestamp: str


class DnsWorkerResult(BaseModel):
    """Result from a DNS worker"""
    task_id: str
    trace_id: str  # OpenTelemetry trace ID for correlation
    location: str
    domain: str
    status: str
    records: Optional[Dict[str, Any]] = None
    error: Optional[str] = None
    processing_time_ms: float


class DnsOrchestrateResponse(BaseModel):
    """Response model for DNS orchestration"""
    domain: str
    status: str
    results: Optional[Dict[str, Any]] = None
    message: str
