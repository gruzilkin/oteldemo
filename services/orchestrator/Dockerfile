FROM docker.io/python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY app ./app

# Expose port
EXPOSE 8001

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV OTEL_SERVICE_NAME=dns-orchestrator
ENV OTEL_EXPORTER_OTLP_ENDPOINT=http://orchestrator-collector:4317

# Run application
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8001"]
