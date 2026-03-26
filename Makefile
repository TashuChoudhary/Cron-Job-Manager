.PHONY: help dev docker-build docker-up docker-down docker-restart docker-logs docker-clean deploy backup restore test lint

# Default target
.DEFAULT_GOAL := help

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

##@ General

help: ## Display this help message
	@echo "$(BLUE)CronJob Manager - Available Commands$(NC)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(GREEN)<target>$(NC)\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

dev: ## Run locally with hot reload (requires air: go install github.com/cosmtrek/air@latest)
	@echo "$(BLUE)Starting local development server...$(NC)"
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "$(YELLOW)Air not found. Running without hot reload...$(NC)"; \
		go run main.go; \
	fi

run: ## Run locally without hot reload
	@echo "$(BLUE)Starting local server...$(NC)"
	go run main.go

test: ## Run tests
	@echo "$(BLUE)Running tests...$(NC)"
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

lint: ## Run linter
	@echo "$(BLUE)Running linter...$(NC)"
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "$(RED)golangci-lint not found. Install: https://golangci-lint.run/usage/install/$(NC)"; \
	fi

##@ Docker

docker-build: ## Build Docker images
	@echo "$(BLUE)Building Docker images...$(NC)"
	docker-compose -f docker-compose.yml build --no-cache

docker-up: ## Start Docker containers
	@echo "$(BLUE)Starting Docker containers...$(NC)"
	docker-compose -f docker-compose.yml up -d
	@echo "$(GREEN)Containers started!$(NC)"
	@echo "App: http://localhost:5000"
	@echo "Logs: make docker-logs"

docker-down: ## Stop Docker containers (keeps data)
	@echo "$(BLUE)Stopping Docker containers...$(NC)"
	docker-compose -f docker-compose.yml down
	@echo "$(GREEN)Containers stopped. Data preserved in volumes.$(NC)"

docker-restart: ## Restart Docker containers
	@echo "$(BLUE)Restarting Docker containers...$(NC)"
	docker-compose -f docker-compose.yml restart

docker-logs: ## View Docker logs (follow mode)
	docker-compose -f docker-compose.yml logs -f

docker-clean: ## Stop containers and remove volumes (⚠️ DELETES ALL DATA!)
	@echo "$(RED)WARNING: This will delete all data in Docker volumes!$(NC)"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose -f docker-compose.yml down -v; \
		echo "$(GREEN)Containers and volumes removed.$(NC)"; \
	else \
		echo "$(YELLOW)Cancelled.$(NC)"; \
	fi

docker-shell: ## Open shell in app container
	@echo "$(BLUE)Opening shell in app container...$(NC)"
	docker exec -it cronjob-manager sh

docker-db-shell: ## Open PostgreSQL shell
	@echo "$(BLUE)Opening PostgreSQL shell...$(NC)"
	docker exec -it cronjob-postgres psql -U postgres -d cronjob_manager

##@ Database

backup: ## Backup Docker database
	@echo "$(BLUE)Backing up database...$(NC)"
	@mkdir -p backups
	docker exec cronjob-postgres pg_dump -U postgres cronjob_manager > backups/backup_$$(date +%Y%m%d_%H%M%S).sql
	@echo "$(GREEN)Backup created in backups/$(NC)"

restore: ## Restore database from backup (Usage: make restore FILE=backups/backup_20260225.sql)
	@echo "$(BLUE)Restoring database from $(FILE)...$(NC)"
	@if [ -z "$(FILE)" ]; then \
		echo "$(RED)Error: Please specify FILE=backups/backup_XXXXXX.sql$(NC)"; \
		exit 1; \
	fi
	docker exec -i cronjob-postgres psql -U postgres -d cronjob_manager < $(FILE)
	@echo "$(GREEN)Database restored!$(NC)"

db-reset: ## Reset database (⚠️ DELETES ALL DATA!)
	@echo "$(RED)WARNING: This will delete all data in the database!$(NC)"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker exec cronjob-postgres psql -U postgres -c "DROP DATABASE IF EXISTS cronjob_manager;"; \
		docker exec cronjob-postgres psql -U postgres -c "CREATE DATABASE cronjob_manager;"; \
		docker-compose -f docker-compose.yml restart postgres; \
		echo "$(GREEN)Database reset complete!$(NC)"; \
	else \
		echo "$(YELLOW)Cancelled.$(NC)"; \
	fi

##@ Deployment

deploy-render: ## Deploy to Render.com (requires render CLI)
	@echo "$(BLUE)Deploying to Render.com...$(NC)"
	@if command -v render > /dev/null; then \
		render deploy; \
	else \
		echo "$(RED)Render CLI not found. Install: https://render.com/docs/cli$(NC)"; \
	fi

##@ Monitoring

status: ## Show container status
	@echo "$(BLUE)Container Status:$(NC)"
	@docker-compose -f docker-compose.yml ps

health: ## Check health of all services
	@echo "$(BLUE)Service Health:$(NC)"
	@docker inspect cronjob-postgres --format='{{.State.Health.Status}}' 2>/dev/null || echo "postgres: $(RED)not running$(NC)"
	@docker inspect cronjob-manager --format='{{.State.Health.Status}}' 2>/dev/null || echo "app: $(RED)not running$(NC)"

stats: ## Show resource usage
	@echo "$(BLUE)Resource Usage:$(NC)"
	@docker stats --no-stream cronjob-postgres cronjob-manager 2>/dev/null || echo "$(RED)Containers not running$(NC)"

##@ Setup

setup: ## Initial project setup
	@echo "$(BLUE)Setting up project...$(NC)"
	@mkdir -p backups
	@mkdir -p docker/postgres
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "$(GREEN)Created .env from .env.example$(NC)"; \
	fi
	@if [ ! -f .env.production ]; then \
		cp .env.example .env.production; \
		echo "$(GREEN)Created .env.production from .env.example$(NC)"; \
		echo "$(YELLOW)Don't forget to update DB_HOST=postgres in .env.production!$(NC)"; \
	fi
	@echo "$(GREEN)Setup complete!$(NC)"

deps: ## Install Go dependencies
	@echo "$(BLUE)Installing Go dependencies...$(NC)"
	go mod download
	go mod tidy
	@echo "$(GREEN)Dependencies installed!$(NC)"