# Vivid Actor Framework Makefile

.PHONY: help test bench clean docs docs-ecology build-tools

# Default target
help:
	@echo "Available targets:"
	@echo "  test         - Run all tests"
	@echo "  bench        - Run benchmarks"
	@echo "  docs         - Generate all documentation"
	@echo "  docs-ecology - Generate ecology documentation"
	@echo "  build-tools  - Build development tools"
	@echo "  clean        - Clean build artifacts"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Generate all documentation
docs: docs-ecology
	@echo "Documentation generation complete"

# Generate ecology documentation
docs-ecology: build-tools
	@echo "Generating ecology documentation..."
	./tools/docgen/docgen --target=ecology --output=README.md

# Build development tools
build-tools:
	@echo "Building documentation generator..."
	cd tools/docgen && go build -o docgen .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f tools/docgen/docgen
	rm -f tools/docgen/docgen.exe

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

# Run ecology component tests
test-ecology:
	@echo "Testing ecology components..."
	cd ecology/grpc-server && go test -v ./...