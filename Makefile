.PHONY: help build up down logs clean test

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build all container images
	podman-compose build

up: ## Start all services
	podman-compose up -d

down: ## Stop all services
	podman-compose down

logs: ## Show logs from all services
	podman-compose logs -f

logs-gateway: ## Show logs from gateway service
	podman logs -f oteldemo-gateway

logs-orchestrator: ## Show logs from orchestrator service
	podman logs -f oteldemo-orchestrator

logs-worker-us: ## Show logs from US worker
	podman logs -f oteldemo-worker-us-east

logs-worker-eu: ## Show logs from EU worker
	podman logs -f oteldemo-worker-eu-west

logs-worker-asia: ## Show logs from Asia worker
	podman logs -f oteldemo-worker-asia-south

logs-central: ## Show logs from central collector
	podman logs -f oteldemo-central-collector

ps: ## Show running containers
	podman-compose ps

restart: down up ## Restart all services

clean: ## Remove all containers and volumes
	podman-compose down -v --rmi all

test: ## Send a test DNS lookup request
	@echo "Sending DNS lookup request for google.com..."
	@curl -X POST http://localhost:8080/api/v1/dns/lookup \
		-H "Content-Type: application/json" \
		-d '{"domain": "google.com", "locations": ["us-east-1", "eu-west-1", "asia-south-1"], "record_types": ["A", "AAAA", "MX", "NS"]}' \
		| python3 -m json.tool

health: ## Check health of all services
	@echo "Gateway:"; curl -s http://localhost:8080/api/v1/health; echo ""
	@echo "Orchestrator:"; curl -s http://localhost:8001/health | python3 -m json.tool
	@echo "Worker US:"; curl -s http://localhost:8082/health | python3 -m json.tool
	@echo "Worker EU:"; curl -s http://localhost:8083/health | python3 -m json.tool
	@echo "Worker Asia:"; curl -s http://localhost:8084/health | python3 -m json.tool

jaeger: ## Open Jaeger UI in browser
	@echo "Opening Jaeger UI at http://localhost:16686"
	@xdg-open http://localhost:16686 2>/dev/null || open http://localhost:16686 2>/dev/null || echo "Please open http://localhost:16686 in your browser"
