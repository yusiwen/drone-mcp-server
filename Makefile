# Makefile for drone-mcp-server

# Variables
BINARY_NAME = drone-mcp-server
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GO_VERSION ?= $(shell go version | awk '{print $$3}')

# Go build flags
LDFLAGS = -s -w \
	-X main.buildVersion=$(VERSION) \
	-X main.buildCommit=$(COMMIT) \
	-X main.buildDate=$(BUILD_DATE)

# Platforms to build for
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

# Install to GOPATH/bin
.PHONY: install
install: build
	@echo "Installing to $(GOPATH)/bin..."
	cp $(BINARY_NAME) $(GOPATH)/bin/

# Run in development mode (stdio)
.PHONY: run
run: build
	@echo "Running in stdio mode..."
	./$(BINARY_NAME)

# Run in Streamable HTTP mode for testing (requires MCP_AUTH_TOKEN)
.PHONY: run-http
run-http: build
	@echo "Running in Streamable HTTP mode on http://localhost:8080..."
	@test -n "$(MCP_AUTH_TOKEN)" || (echo "MCP_AUTH_TOKEN must be set for HTTP mode" && exit 1)
	./$(BINARY_NAME) --http --host localhost --port 8080

# Build for all platforms
.PHONY: build-all
build-all:
	@echo "Building for all platforms..."
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		OUTPUT="dist/$(BINARY_NAME)_$(VERSION)_$${OS}_$${ARCH}/$(BINARY_NAME)"; \
		if [ "$$OS" = "windows" ]; then \
			OUTPUT="dist/$(BINARY_NAME)_$(VERSION)_$${OS}_$${ARCH}/$(BINARY_NAME).exe"; \
		fi; \
		echo "Building for $$OS/$$ARCH..."; \
		GOOS=$$OS GOARCH=$$ARCH CGO_ENABLED=0 go build \
			-ldflags="$(LDFLAGS)" \
			-o $$OUTPUT .; \
	done

# Create release archives
.PHONY: release
release: clean build-all
	@echo "Creating release archives..."
	@mkdir -p releases
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		DIR="dist/$(BINARY_NAME)_$(VERSION)_$${OS}_$${ARCH}"; \
		if [ "$$OS" = "windows" ]; then \
			echo "Creating zip for $$OS/$$ARCH..."; \
			cd $$DIR && zip -r "../../releases/$(BINARY_NAME)_$(VERSION)_$${OS}_$${ARCH}.zip" . && cd ../..; \
		else \
			echo "Creating tar.gz for $$OS/$$ARCH..."; \
			cd $$DIR && tar czf "../../releases/$(BINARY_NAME)_$(VERSION)_$${OS}_$${ARCH}.tar.gz" . && cd ../..; \
		fi; \
	done
	@echo "Release files created in releases/ directory:"
	@ls -lh releases/

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test ./... -v

# Run tests with the race detector
.PHONY: test-race
test-race:
	@echo "Running tests with the race detector..."
	go test ./... -race

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run the end-to-end security smoke test (offline, no Drone instance required)
.PHONY: smoke
smoke: build
	@echo "Running the security smoke test..."
	BIN=./$(BINARY_NAME) ./scripts/security-smoke-test.sh

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe
	rm -rf dist/ releases/
	rm -f coverage.out coverage.html

# Show version information
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build date: $(BUILD_DATE)"
	@echo "Go version: $(GO_VERSION)"

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build for current platform (default)"
	@echo "  install      - Build and install to GOPATH/bin"
	@echo "  run          - Build and run in stdio mode"
	@echo "  run-http     - Build and run in Streamable HTTP mode on localhost:8080"
	@echo "  build-all    - Build for all platforms (linux, darwin, windows)"
	@echo "  release      - Build release archives for all platforms"
	@echo "  test         - Run tests"
	@echo "  test-race    - Run tests with the race detector"
	@echo "  smoke        - Run the end-to-end security smoke test"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean        - Clean build artifacts"
	@echo "  version      - Show version information"
	@echo "  help         - Show this help message"

# Docker targets
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build \
		--build-arg BUILD_VERSION=$(VERSION) \
		--build-arg BUILD_COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t drone-mcp-server:$(VERSION) .

.PHONY: docker-run
docker-run: docker-build
	@echo "Running Docker container..."
	@test -n "$(MCP_AUTH_TOKEN)" || (echo "MCP_AUTH_TOKEN must be set for HTTP mode" && exit 1)
	@# Secrets are forwarded from the host environment by name so that they do
	@# not appear in the container command line or in the shell history.
	docker run --rm -it \
		-e DRONE_SERVER \
		-e DRONE_TOKEN \
		-e MCP_AUTH_TOKEN \
		-p 127.0.0.1:8080:8080 \
		drone-mcp-server:$(VERSION) --http --host 0.0.0.0

.PHONY: docker-clean
docker-clean:
	@echo "Cleaning Docker images..."
	docker rmi drone-mcp-server:$(VERSION) 2>/dev/null || true

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	go vet ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Check code quality
.PHONY: check
check: fmt vet lint

# Default target
.DEFAULT_GOAL := help