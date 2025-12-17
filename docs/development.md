# Development Guide

## Prerequisites

- Go 1.21+
- Make

## Definition of Done

A feature, function, or code change is considered **complete** only when:

1. **All tests pass**: `make test` exits with status 0
2. **All checks pass**: `make check` (linting + vulnerability scanning) exits with status 0
3. Code is committed with a clear, descriptive commit message

**Never consider work finished until both `make test` and `make check` pass successfully.**

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

## Coding Guidelines

### Kubernetes Resource Types

When working with Kubernetes resource types (GroupVersionKind):

- **Always use constants from `pkg/kube/gvks/gvks.go`** instead of hardcoded strings
- **Compare full GVK structs** rather than just the `.Kind` field for accuracy
- Add new resource types to the `gvks` package when needed

**Good:**
```go
switch gvk {
case gvks.Deployment:
    // handle deployment
case gvks.Service:
    // handle service
}
```

**Bad:**
```go
switch gvk.Kind {
case "Deployment":  // Don't hardcode strings
    // handle deployment
}
```

### Context Usage

When working with `context.Context`:

- **Always pass `context.Context` as the first parameter** to functions that:
  - Make network calls
  - Access external services (container registries, catalogs)
  - Perform I/O operations
  - Call other functions that need context
- **Never use `context.Background()` or `context.TODO()`** in package code (only acceptable at entry points like `main()` or test functions)
- **Thread context from Cobra command**: Use `cmd.Context()` to get the root context with signal handling
- **Enable context linters**: The following linters enforce proper context usage:
  - `contextcheck` - Detects non-inherited context usage
  - `noctx` - Detects HTTP requests without context
  - `containedctx` - Prevents storing context in structs
  - `fatcontext` - Detects nested contexts in loops

**Good:**
```go
func Load(ctx context.Context, input string, config RegistryConfig) error {
    // Use the passed context
    return someOperation(ctx, input)
}
```

**Bad:**
```go
func Load(input string, config RegistryConfig) error {
    ctx := context.Background()  // ❌ Don't create new contexts
    return someOperation(ctx, input)
}
```

**Note:** Third-party libraries may have their own context APIs (e.g., `remote.WithContext(ctx)`). Adapt to their patterns while keeping our functions following Go conventions.

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
