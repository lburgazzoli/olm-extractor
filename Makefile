# bundle-extract CLI Makefile

# Binary name
BINARY_NAME=bin/bundle-extract

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Build flags
LDFLAGS = -X 'github.com/lburgazzoli/olm-extractor/internal/version.Version=$(VERSION)' \
          -X 'github.com/lburgazzoli/olm-extractor/internal/version.Commit=$(COMMIT)' \
          -X 'github.com/lburgazzoli/olm-extractor/internal/version.Date=$(DATE)'

# Linter configuration
LINT_TIMEOUT := 10m

# Container registry configuration
CONTAINER_REGISTRY ?= quay.io
CONTAINER_REPO ?= $(CONTAINER_REGISTRY)/lburgazzoli/olm-extractor
CONTAINER_PLATFORMS ?= linux/amd64,linux/arm64
CONTAINER_TAGS ?= $(VERSION)

## Tools
GOLANGCI_VERSION ?= v2.7.2
GOLANGCI ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
GOVULNCHECK_VERSION ?= latest
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Build the binary
.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) cmd/main.go

# Ensure buildx builder exists for multi-platform builds
.PHONY: buildx-setup
buildx-setup:
	@docker buildx inspect multiplatform >/dev/null 2>&1 || \
		(echo "Creating buildx builder for multi-platform builds..." && \
		 docker buildx create --name multiplatform --driver docker-container --bootstrap --use)

# Build and push container image using Docker buildx
.PHONY: publish
publish: buildx-setup
	@echo "Building and pushing container image to $(CONTAINER_REPO):$(CONTAINER_TAGS)"
	docker buildx build \
		--builder=multiplatform \
		--platform=$(CONTAINER_PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		--tag=$(CONTAINER_REPO):$(CONTAINER_TAGS) \
		--push \
		.

# Run the CLI
.PHONY: run
run:
	go run -ldflags "$(LDFLAGS)" cmd/main.go

# Tidy up dependencies
.PHONY: tidy
tidy:
	go mod tidy

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	go clean -x
	go clean -x -testcache

# Format code
.PHONY: fmt
fmt:
	@$(GOLANGCI) fmt --config .golangci.yml
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	@$(GOLANGCI) run --config .golangci.yml --timeout $(LINT_TIMEOUT)

# Run linter with auto-fix
.PHONY: lint/fix
lint/fix:
	@$(GOLANGCI) run --config .golangci.yml --timeout $(LINT_TIMEOUT) --fix

# Run vulnerability check
.PHONY: vulncheck
vulncheck:
	@$(GOVULNCHECK) ./...

# Run all checks
.PHONY: check
check: lint vulncheck

# Run tests
.PHONY: test
test:
	go test ./...

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Build the bundle-extract binary"
	@echo "  publish     - Build and push container image using Docker buildx"
	@echo "  run         - Run the CLI"
	@echo "  tidy        - Tidy up Go module dependencies"
	@echo "  clean       - Remove build artifacts and test cache"
	@echo "  fmt         - Format Go code"
	@echo "  lint        - Run golangci-lint"
	@echo "  lint/fix    - Run golangci-lint with auto-fix"
	@echo "  vulncheck   - Run vulnerability scanner"
	@echo "  check       - Run all checks (lint + vulncheck)"
	@echo "  test        - Run tests"
	@echo "  help        - Show this help message"