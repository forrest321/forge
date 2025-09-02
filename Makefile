# Makefile for Forge Backend

.PHONY: build run test lint clean tidy kill-8080 help

# Default target
help:
	@echo "Available targets:"
	@echo "  build      - Build the backend"
	@echo "  run        - Run the backend"
	@echo "  test       - Run tests"
	@echo "  lint       - Lint the code"
	@echo "  clean      - Clean build artifacts"
	@echo "  tidy       - Tidy Go modules"
	@echo "  kill-8080  - Kill any server running on port 8080"
	@echo "  help       - Show this help"

# Build the backend
build:
	go build ./...

# Run the backend
run:
	go run ./cmd

# Run tests
test:
	go test ./...

# Lint the code (requires golangci-lint)
lint:
	golangci-lint run

# Clean build artifacts
clean:
	go clean ./...
	rm -f main

# Tidy Go modules
tidy:
	go mod tidy

# Kill server running on port 8080
kill-8080:
	@echo "Killing any server running on port 8080..."
	@lsof -ti :8080 | xargs kill -9 2>/dev/null || echo "No server found on port 8080"

# Development workflow
dev: tidy lint test build
