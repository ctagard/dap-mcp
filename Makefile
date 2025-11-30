.PHONY: all build clean test lint install docker docker-push help

# Binary name
BINARY=dap-mcp
VERSION?=0.1.1
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Docker settings
DOCKER_IMAGE=dap-mcp
DOCKER_TAG=$(VERSION)

# Go settings
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

all: build

## Build the binary
build:
	@echo "Building $(BINARY)..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/dap-mcp

## Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/dap-mcp
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 ./cmd/dap-mcp

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 ./cmd/dap-mcp
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 ./cmd/dap-mcp

build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-windows-amd64.exe ./cmd/dap-mcp

## Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY)..."
	go install $(LDFLAGS) ./cmd/dap-mcp

## Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean

## Run tests
test:
	@echo "Running tests..."
	go test -v ./...

## Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

## Format code
fmt:
	@echo "Formatting code..."
	gofmt -s -w $(GOFILES)

## Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

## Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

## Push Docker image
docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

## Run the server locally
run:
	@echo "Running $(BINARY)..."
	go run ./cmd/dap-mcp

## Show help
help:
	@echo "DAP-MCP Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the binary"
	@echo "  build-all    Build for all platforms (Linux, macOS, Windows)"
	@echo "  install      Install to \$$GOPATH/bin"
	@echo "  clean        Clean build artifacts"
	@echo "  test         Run tests"
	@echo "  lint         Run linter"
	@echo "  fmt          Format code"
	@echo "  tidy         Tidy dependencies"
	@echo "  docker       Build Docker image"
	@echo "  docker-push  Push Docker image"
	@echo "  run          Run the server locally"
	@echo "  help         Show this help"
