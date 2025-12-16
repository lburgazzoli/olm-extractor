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
KO_DOCKER_REPO ?= $(CONTAINER_REGISTRY)/lburgazzoli/olm-extractor
KO_PLATFORMS ?= linux/amd64,linux/arm64
KO_TAGS ?= $(VERSION)

## Tools
GOLANGCI_VERSION ?= v2.7.2
GOLANGCI ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
GOVULNCHECK_VERSION ?= latest
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
KO_VERSION ?= latest
KO ?= go run github.com/google/ko@$(KO_VERSION)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Build the binary
.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) cmd/main.go

# Build and push container image
.PHONY: publish
publish:
	@echo "Building and pushing container image to $(KO_DOCKER_REPO):$(KO_TAGS)"
	@KO_DOCKER_REPO=$(KO_DOCKER_REPO) GOFLAGS="-ldflags=$(LDFLAGS)" $(KO) build ./cmd \
		--bare \
		--tags=$(KO_TAGS) \
		--platform=$(KO_PLATFORMS)

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
	@echo "  publish     - Build and push container image using ko"
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