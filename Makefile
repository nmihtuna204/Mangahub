.PHONY: help build-cli run-cli clean test test-unit test-integration test-coverage lint load-test all

help:
	@echo "MangaHub Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build-cli         - Build the CLI executable"
	@echo "  run-cli           - Run the CLI"
	@echo "  clean             - Remove build artifacts"
	@echo "  test              - Run all tests"
	@echo "  test-unit         - Run unit tests only"
	@echo "  test-integration  - Run integration tests"
	@echo "  test-coverage     - Generate coverage report"
	@echo "  lint              - Run linters"
	@echo "  load-test         - Run load tests"
	@echo "  all               - Build everything"

build-cli:
	@echo "Building MangaHub CLI..."
	go build -o bin/mangahub.exe ./cmd/cli
	@echo "✓ CLI built: bin/mangahub.exe"

run-cli:
	go run ./cmd/cli

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "✓ Clean complete"

test:
	@echo "Running all tests..."
	go test -v ./...

test-unit:
	@echo "Running unit tests..."
	go test -v -short ./...

test-integration:
	@echo "Running integration tests..."
	@echo "Note: Requires all servers to be running"
	go test -v ./test/...

test-coverage:
	@echo "Generating coverage report..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

lint:
	@echo "Running linters..."
	go fmt ./...
	go vet ./...
	@echo "✓ Lint complete"

load-test:
	@echo "Running load tests..."
	chmod +x test/load_test.sh
	bash test/load_test.sh

all: build-cli
	@echo "✓ Build complete"
