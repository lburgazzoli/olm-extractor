# Bundle Extract CLI Tool Specification

## Overview

A command-line tool that extracts Kubernetes manifests from an OLM (Operator Lifecycle Manager) bundle and outputs installation-ready YAML to stdout. The tool leverages existing libraries from the operator-framework repositories for bundle handling and RBAC generation.

## Purpose

Enable direct installation of OLM operators using `kubectl` without requiring OLM to be installed on the cluster:

```bash
bundle-extract quay.io/example/operator-bundle:v1.0.0 -n operators | kubectl apply -f -
```

## Requirements

### Functional Requirements

1. **Input Sources**
   - Local bundle directory path (e.g., `./bundle`)
   - Container image reference (e.g., `quay.io/example/operator-bundle:v1.0.0`)

2. **Output Format**
   - Valid Kubernetes YAML manifests
   - Multiple documents separated by `---` (using yaml.v3 Encoder)
   - Ready for direct piping to `kubectl apply -f -`

3. **Extracted Manifests** (in order)
   - Namespace (if not "default")
   - CustomResourceDefinitions (CRDs, with conversion webhook config if applicable)
   - ServiceAccounts
   - Roles and RoleBindings
   - ClusterRoles and ClusterRoleBindings
   - Deployments
   - Webhook Services (backing services for webhook deployments)
   - ValidatingWebhookConfigurations
   - MutatingWebhookConfigurations
   - Other bundle resources (Services, ConfigMaps, etc.)

4. **Excluded from Output**
   - ClusterServiceVersion (CSV) itself - OLM-specific, not needed for direct installation

### Non-Functional Requirements

- Modular package structure under `pkg/`
- Uses official operator-framework libraries (no custom bundle parsing)
- Uses OLM's `RBACForClusterServiceVersion` for RBAC generation
- Clear error messages
- Exit with appropriate status codes

## CLI Interface

### Command Syntax

```bash
bundle-extract <bundle-path-or-image> --namespace <namespace>
```

### Required Arguments

| Argument | Short | Description | Required |
|----------|-------|-------------|----------|
| `--namespace` | `-n` | Target namespace for installation | Yes |

### Examples

```bash
# Extract from bundle image
bundle-extract quay.io/example/operator:v1.0.0 -n my-system | kubectl apply -f -

# Extract from local directory
bundle-extract ./bundle --namespace operators | kubectl apply -f -

# Save to file
bundle-extract ./bundle -n default > install.yaml

# Check version
bundle-extract --version
```

## Project Structure

```
olm-extractor/
├── cmd/
│   ├── main.go              # CLI entry point
│   └── main_test.go         # Namespace validation tests
├── pkg/
│   ├── bundle/
│   │   └── bundle.go        # Bundle loading from directory/image
│   ├── extract/
│   │   └── extract.go       # Manifest extraction from bundle
│   ├── kube/
│   │   ├── kube.go          # Kubernetes resource helpers
│   │   └── kube_test.go     # Helper function tests
│   └── render/
│       ├── render.go        # YAML output utilities
│       └── render_test.go   # Unstructured cleaning tests
├── internal/
│   └── version/
│       └── version.go       # Version info (set via ldflags, internal only)
├── docs/
│   ├── spec.md              # This file
│   └── development.md       # Build commands
├── go.mod
├── go.sum
├── Makefile
└── .golangci.yml
```

## Package Responsibilities

| Package | Exports | Purpose |
|---------|---------|---------|
| `pkg/bundle` | `Load`, `LoadFromImage` | Load OLM bundles from directory or container image |
| `pkg/extract` | `Manifests`, `CRDs`, `InstallStrategy`, `Webhooks`, `WebhookServices`, `OtherResources` | Extract K8s resources from bundle |
| `pkg/kube` | `CreateNamespace`, `CreateDeployment`, `CreateWebhookService`, `IsNamespaced`, `SetNamespace` | Kubernetes resource helpers |
| `pkg/render` | `YAML`, `ToUnstructured`, `CleanUnstructured` | YAML output and object cleaning |
| `internal/version` | `Version`, `Commit`, `Date` | Build version info (internal only) |

## Technical Implementation

### Key Dependencies

```go
import (
    // Bundle and manifest handling
    "github.com/operator-framework/api/pkg/manifests"
    "github.com/operator-framework/api/pkg/operators/v1alpha1"
    
    // RBAC generation from CSV (uses OLM's battle-tested implementation)
    "github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver"
    
    // Image registry handling
    "github.com/operator-framework/operator-registry/pkg/image"
    "github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
    
    // CLI framework
    "github.com/spf13/cobra"
    
    // YAML serialization (with multi-document support)
    "gopkg.in/yaml.v3"
    
    // Kubernetes types
    "k8s.io/api/apps/v1"
    "k8s.io/api/core/v1"
    "k8s.io/api/rbac/v1"
    "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
    "k8s.io/apimachinery/pkg/runtime"
)
```

### RBAC Generation

The tool uses OLM's `resolver.RBACForClusterServiceVersion()` function to generate RBAC resources from the CSV's install strategy. This ensures:

- Hash-based unique names for Roles/ClusterRoles (avoids duplicates)
- Proper handling of multiple permissions per ServiceAccount
- Consistent behavior with OLM's own resource generation

```go
permissions, err := resolver.RBACForClusterServiceVersion(csv)
// Returns map[string]*OperatorPermissions with:
// - ServiceAccount
// - Roles []
// - RoleBindings []
// - ClusterRoles []
// - ClusterRoleBindings []
```

### YAML Output

Uses `gopkg.in/yaml.v3` Encoder for automatic document separation:

```go
encoder := yaml.NewEncoder(os.Stdout)
encoder.SetIndent(2)
for _, obj := range objects {
    encoder.Encode(cleanedMap)  // Automatically adds ---
}
```

### Unstructured Cleaning

Before serialization, objects are converted to unstructured maps and cleaned of nil/empty values:

```go
func CleanUnstructured(obj map[string]any) map[string]any
// Recursively removes:
// - nil values
// - empty maps {}
// - empty slices []
// - empty strings ""
// Preserves zero values (0, false)
```

### Webhook Extraction

The tool extracts webhook configurations from `csv.Spec.WebhookDefinitions`:

- **ValidatingAdmissionWebhook** - Creates `ValidatingWebhookConfiguration`
- **MutatingAdmissionWebhook** - Creates `MutatingWebhookConfiguration`
- **ConversionWebhook** - Patches CRD `spec.conversion` field

For each webhook deployment, a backing Service is generated with the naming convention `<deployment-name>-webhook-service`.

**Important: CA Bundle Injection Required**

Webhooks are extracted with **empty CA bundles**. Users must inject TLS certificates post-extraction using one of:

1. **cert-manager** - Add annotations to trigger automatic certificate injection
2. **Manual certificates** - Generate and inject CA bundles into webhook configurations
3. **Service mesh** - Let the mesh handle mTLS

Example with cert-manager:
```yaml
# Add annotation to webhook configuration
metadata:
  annotations:
    cert-manager.io/inject-ca-from: <namespace>/<certificate-name>
```

## Error Handling

Clear error messages for:
- Invalid bundle path or image reference
- Missing CSV in bundle
- Image pull failures (suggests checking authentication)
- Invalid namespace name (DNS-1123 validation)
- Unsupported install strategy
- YAML serialization errors

Exit with non-zero status code on any error.

## Success Criteria

- ✅ Modular package structure
- ✅ Uses official operator-framework libraries for bundle handling
- ✅ Uses OLM's `RBACForClusterServiceVersion` for RBAC generation
- ✅ Accepts both bundle directories and images
- ✅ Requires `--namespace` flag
- ✅ Outputs valid, installation-ready Kubernetes YAML
- ✅ Output works with: `bundle-extract <input> -n <ns> | kubectl apply -f -`
- ✅ Handles both v1 and v1beta1 CRDs
- ✅ Generates proper RBAC with hash-based naming
- ✅ Sets namespaces correctly on all resources
- ✅ Excludes CSV from output
- ✅ Cleans nil/empty fields from output
- ✅ Version info injectable via ldflags
- ✅ Extracts ValidatingWebhookConfigurations and MutatingWebhookConfigurations
- ✅ Generates webhook backing Services
- ✅ Patches CRDs with conversion webhook configuration
