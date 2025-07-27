# Vivid Actor Framework Makefile

.PHONY: help test bench deps fmt lint

# Default target
help:
	@echo "Available targets:"
	@echo "  test         - Run all tests"
	@echo "  bench        - Run benchmarks"
	@echo "  build-tools  - Build development tools"
	@echo "  deps		  - Install dependencies"
	@echo "  fmt		  - Format code"
	@echo "  lint		  - Lint code"q

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run
