.PHONY: build test clean lint fmt vet run install help

# Variables
BINARY_NAME=autodev
BUILD_DIR=bin
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

# Default target
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Build the binary
build: ## Build the autodev binary
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/autodev

# Build for multiple platforms
build-all: ## Build for all supported platforms
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/autodev
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/autodev
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/autodev
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/autodev
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/autodev

# Run tests
test: ## Run all tests
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

# Run tests with verbose output
test-verbose: ## Run tests with verbose output
	$(GO) test -v -race ./...

# Run specific test
test-run: ## Run specific test (e.g., make test-run TEST=TestPipeline)
	$(GO) test -v -race -run $(TEST) ./...

# Run linter
lint: ## Run golangci-lint
	@golangci-lint run ./... || echo "Install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

# Run go fmt
fmt: ## Format all Go files
	$(GO) fmt ./...

# Run go vet
vet: ## Run go vet
	$(GO) vet ./...

# Run all checks
check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

# Clean build artifacts
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out
	find . -name "*.test" -delete
	find . -name "*.out" -delete

# Install the binary globally
install: ## Install the binary to GOPATH/bin
	$(GO) install $(LDFLAGS) ./cmd/autodev

# Run the tool (pass ARGS for arguments)
run: ## Run the autodev tool (e.g., make run ARGS='run "my task"')
	$(GO) run ./cmd/autodev $(ARGS)

# Update dependencies
deps: ## Update Go module dependencies
	$(GO) get -u ./...
	$(GO) mod tidy

# Verify dependencies
deps-verify: ## Verify dependencies are tidy
	$(GO) mod verify

# Show dependency graph
deps-graph: ## Show module dependency graph
	$(GO) mod graph | head -50

# Benchmark
bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

# Benchmark with detailed output
bench-verbose: ## Run benchmarks with detailed output
	$(GO) test -bench=. -benchmem -benchtime=5s ./...

# Generate coverage report
cover: ## Generate and view coverage report
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Race detector
race: ## Run tests with race detector
	$(GO) test -race ./...

# Build with debug symbols
build-debug: ## Build with debug symbols
	@mkdir -p $(BUILD_DIR)
	$(GO) build -gcflags "all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug ./cmd/autodev

# Static analysis
analyze: ## Run static analysis
	$(GO) vet ./...
	@echo "Running gosec..."
	@gosec ./... || echo "Install gosec: go install github.com/securego/gosec/v2/cmd/gosec@latest"

# Create release
release: clean build-all ## Create release binaries
	@echo "Release binaries created in $(BUILD_DIR)/"

# Docker build (if Dockerfile exists)
docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):latest .

# Docker run
docker-run: ## Run Docker container
	docker run --rm -it $(BINARY_NAME):latest

# Generate mock files
mocks: ## Generate mock files (if mockgen is installed)
	@go generate ./...

# Generate documentation
docs: ## Generate documentation
	$(GO) doc -all ./... | head -200

# Show version info
version: ## Show build version info
	@echo "Go version: $(shell go version)"
	@echo "Build date: $(shell date -u +%Y-%m-%dT%H:%M:%SZ)"
	@git log -1 --format="%h %s" 2>/dev/null || echo "No git repo"

# Watch for changes and rebuild (requires reflex)
watch: ## Watch for changes and rebuild
	@reflex -r '\.go$$' -- make build

# Initialize git hooks
init-hooks: ## Initialize git hooks
	@mkdir -p .git/hooks
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make check' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed"
