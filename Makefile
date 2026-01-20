.PHONY: build run test clean help

BINARY_NAME=gateway
BUILD_DIR=bin
MAIN_PATH=cmd/gateway/main.go

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the gateway binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

run: ## Run the gateway (without building)
	@go run $(MAIN_PATH)

build-run: build ## Build and run the gateway
	@./$(BUILD_DIR)/$(BINARY_NAME)

test: ## Run tests
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter
	@golangci-lint run ./... || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"

fmt: ## Format code
	@go fmt ./...

vet: ## Run go vet
	@go vet ./...

tidy: ## Tidy dependencies
	@go mod tidy

clean: ## Clean build artifacts
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Cleaned build artifacts"

dev: ## Run in development mode with auto-reload (requires air)
	@air || echo "Install air: go install github.com/cosmtrek/air@latest"

docker-build: ## Build Docker image
	@docker build -t api-gateway:latest .

docker-run: ## Run Docker container
	@docker run -p 8080:8080 api-gateway:latest

install-tools: ## Install development tools
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

bench: ## Run benchmarks
	@go test -bench=. -benchmem ./...

.DEFAULT_GOAL := help