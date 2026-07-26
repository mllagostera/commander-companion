.PHONY: help build run test lint generate-sql migrate-up migrate-down clean

# Variables
DB_URL="postgres://postgres:postgres@localhost:5432/commander?sslmode=disable"
DOCKER_CMD=docker run --rm -v $(PWD):/app -w /app

help: ## Show this help
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m  %-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the API binary
	go build -o bin/api ./cmd/api

run: ## Run the API locally
	go run ./cmd/api

test: ## Run unit tests
	go test -v -race ./...

lint: ## Run golangci-lint for quality gates
	golangci-lint run ./...

lint-docker: ## Run golangci-lint using Docker
	$(DOCKER_CMD) golangci/golangci-lint:v2.12.2 golangci-lint run -v

generate-sql: ## Generate repository code using sqlc
	sqlc generate

generate-sql-docker: ## Generate repository code using sqlc in Docker
	$(DOCKER_CMD) sqlc/sqlc generate

migrate-up: ## Run goose migrations UP
	goose -dir migrations postgres $(DB_URL) up

migrate-down: ## Run goose migrations DOWN
	goose -dir migrations postgres $(DB_URL) down

clean: ## Clean binaries
	rm -rf bin/
