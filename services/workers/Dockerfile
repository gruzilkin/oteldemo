# Build stage
FROM docker.io/golang:1.23-alpine AS build

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod ./

# Copy source code first (needed for go mod tidy to work)
COPY . .

# Download dependencies and generate go.sum
RUN go mod tidy && go mod download

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker ./cmd/worker

# Runtime stage
FROM docker.io/alpine:latest

# Install dig for DNS lookups
RUN apk add --no-cache bind-tools ca-certificates

WORKDIR /app

# Copy binary from build stage
COPY --from=build /app/worker .

# Set environment variables
ENV OTEL_SERVICE_NAME=dns-worker

# Run worker
ENTRYPOINT ["./worker"]
