# Development Guide

## Prerequisites

- Go 1.21+
- Make

## Build Commands

```bash
# Build the binary
make build

# Run the CLI (with ldflags for version info)
make run

# Format code
make fmt

# Run linter
make lint

# Run linter with auto-fix
make lint/fix

# Run vulnerability scanner
make vulncheck

# Run all checks (lint + vulncheck)
make check

# Run tests
make test

# Tidy dependencies
make tidy

# Clean build artifacts
make clean
```

## Project Structure

```
olm-extractor/
├── cmd/
│   ├── main.go              # CLI entry point (~100 lines)
│   └── main_test.go         # Namespace validation tests
├── pkg/
│   ├── bundle/              # Bundle loading
│   │   └── bundle.go
│   ├── extract/             # Manifest extraction
│   │   └── extract.go
│   ├── kube/                # K8s resource helpers
│   │   ├── kube.go
│   │   └── kube_test.go
│   └── render/              # YAML output
│       ├── render.go
│       └── render_test.go
├── internal/
│   └── version/
│       └── version.go       # Version info (ldflags, internal)
├── docs/
├── go.mod
├── go.sum
├── Makefile
└── .golangci.yml
```

## Version Information

Version info is injected at build time via ldflags:

```makefile
LDFLAGS = -X 'github.com/lburgazzoli/olm-extractor/internal/version.Version=$(VERSION)' \
          -X 'github.com/lburgazzoli/olm-extractor/internal/version.Commit=$(COMMIT)' \
          -X 'github.com/lburgazzoli/olm-extractor/internal/version.Date=$(DATE)'
```

The variables are defined in `internal/version/version.go` and populated from:
- `VERSION`: Git tag or "dev"
- `COMMIT`: Short commit hash
- `DATE`: Build timestamp (UTC)

## Testing

Run all tests:
```bash
make test
```

Test packages:
- `cmd/` - Namespace validation
- `pkg/kube/` - IsNamespaced, CreateNamespace, SetNamespace
- `pkg/render/` - CleanUnstructured (nil/empty field removal)

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/operator-framework/api` | OLM bundle and CSV types |
| `github.com/operator-framework/operator-lifecycle-manager` | RBAC generation from CSV |
| `github.com/operator-framework/operator-registry` | Container image handling |
| `github.com/spf13/cobra` | CLI framework |
| `gopkg.in/yaml.v3` | YAML encoding with multi-doc support |
| `github.com/onsi/gomega` | Test assertions |

## Linting

Uses golangci-lint v2 with configuration in `.golangci.yml`. Key enabled linters:
- All default linters
- revive
- gci (import ordering)
- gochecknoglobals

Run with:
```bash
make lint
```
