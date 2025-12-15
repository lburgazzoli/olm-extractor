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
   - Bundle container image reference (e.g., `quay.io/example/operator-bundle:v1.0.0`)
   - Catalog container image with package name (requires `--catalog` flag)

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

**Bundle Mode (Direct):**
```bash
bundle-extract <bundle-path-or-image> --namespace <namespace>
```

**Catalog Mode:**
```bash
bundle-extract --catalog <catalog-image> <package>[:version] --namespace <namespace>
```

### Required Arguments

| Argument | Short | Description | Required |
|----------|-------|-------------|----------|
| `--namespace` | `-n` | Target namespace for installation | Yes |

### Optional Arguments

| Argument | Short | Description | Default |
|----------|-------|-------------|---------|
| `--include` | | jq expression to include resources (repeatable, acts as OR) | None |
| `--exclude` | | jq expression to exclude resources (repeatable, acts as OR) | None |
| `--temp-dir` | | Directory for temporary files and cache | System temp directory |
| `--catalog` | | Catalog image to resolve bundle from (enables catalog mode) | None |
| `--channel` | | Channel to use when resolving from catalog | Package's defaultChannel |
| `--cert-manager-enabled` | | Enable cert-manager integration for webhook certificates | `true` |
| `--cert-manager-issuer-name` | | Name of the cert-manager Issuer or ClusterIssuer for webhook certificates | `selfsigned-cluster-issuer` |
| `--cert-manager-issuer-kind` | | Kind of cert-manager issuer: Issuer or ClusterIssuer | `ClusterIssuer` |
| `--registry-insecure` | | Allow insecure connections to registries (HTTP or self-signed certificates) | `false` |
| `--registry-auth-file` | | Path to registry authentication file | `~/.docker/config.json` |
| `--registry-username` | | Username for registry authentication | None |
| `--registry-password` | | Password for registry authentication | None |

### Environment Variables

All flags can be configured using environment variables with the `BUNDLE_EXTRACT_` prefix. Flag names are converted to uppercase and dashes are replaced with underscores.

| Flag | Environment Variable | Example |
|------|---------------------|---------|
| `--namespace` | `BUNDLE_EXTRACT_NAMESPACE` | `export BUNDLE_EXTRACT_NAMESPACE=operators` |
| `--temp-dir` | `BUNDLE_EXTRACT_TEMP_DIR` | `export BUNDLE_EXTRACT_TEMP_DIR=/mnt/fast-storage` |
| `--cert-manager-enabled` | `BUNDLE_EXTRACT_CERT_MANAGER_ENABLED` | `export BUNDLE_EXTRACT_CERT_MANAGER_ENABLED=false` |
| `--cert-manager-issuer-name` | `BUNDLE_EXTRACT_CERT_MANAGER_ISSUER_NAME` | `export BUNDLE_EXTRACT_CERT_MANAGER_ISSUER_NAME=my-issuer` |
| `--cert-manager-issuer-kind` | `BUNDLE_EXTRACT_CERT_MANAGER_ISSUER_KIND` | `export BUNDLE_EXTRACT_CERT_MANAGER_ISSUER_KIND=Issuer` |
| `--registry-insecure` | `BUNDLE_EXTRACT_REGISTRY_INSECURE` | `export BUNDLE_EXTRACT_REGISTRY_INSECURE=true` |
| `--registry-auth-file` | `BUNDLE_EXTRACT_REGISTRY_AUTH_FILE` | `export BUNDLE_EXTRACT_REGISTRY_AUTH_FILE=/path/to/config.json` |
| `--registry-username` | `BUNDLE_EXTRACT_REGISTRY_USERNAME` | `export BUNDLE_EXTRACT_REGISTRY_USERNAME=myuser` |
| `--registry-password` | `BUNDLE_EXTRACT_REGISTRY_PASSWORD` | `export BUNDLE_EXTRACT_REGISTRY_PASSWORD=mypass` |
| `--include` | `BUNDLE_EXTRACT_INCLUDE` | `export BUNDLE_EXTRACT_INCLUDE='.kind == "Deployment"'` |
| `--exclude` | `BUNDLE_EXTRACT_EXCLUDE` | `export BUNDLE_EXTRACT_EXCLUDE='.kind == "Secret"'` |

Command-line flags take precedence over environment variables.

### Container Registry Authentication

The tool automatically uses credentials from `~/.docker/config.json` when pulling bundle images from container registries. 

**Option 1: Using existing Docker/Podman credentials**

```bash
# Using Docker
docker login registry.example.com

# Using Podman
podman login registry.example.com

# Then extract the bundle
bundle-extract registry.example.com/my-operator:v1.0.0 -n operators | kubectl apply -f -
```

**Option 2: Using command-line flags**

```bash
# Provide credentials directly
bundle-extract --registry-username myuser --registry-password mypass \
  registry.example.com/my-operator:v1.0.0 -n operators | kubectl apply -f -

# Use custom auth file
bundle-extract --registry-auth-file /path/to/config.json \
  registry.example.com/my-operator:v1.0.0 -n operators | kubectl apply -f -
```

**Option 3: Using environment variables**

```bash
export BUNDLE_EXTRACT_REGISTRY_USERNAME=myuser
export BUNDLE_EXTRACT_REGISTRY_PASSWORD=mypass
bundle-extract registry.example.com/my-operator:v1.0.0 -n operators | kubectl apply -f -
```

For registries with self-signed certificates or HTTP-only registries (development/testing), use the `--registry-insecure` flag:

```bash
bundle-extract --registry-insecure localhost:5000/my-operator:latest -n operators | kubectl apply -f -
```

### File-Based Catalog (FBC) Support

The tool supports extracting bundles from OLM catalog images by automatically resolving package references to bundle images. This allows you to extract operators from catalog indices without manually finding the bundle image reference.

#### How Catalog Mode Works

1. **Pull the catalog image** specified by `--catalog`
2. **Parse the File-Based Catalog (FBC)** declarative config
3. **Find the package** by name (first positional argument)
4. **Resolve to a bundle image** using the specified version or latest in channel
5. **Extract and process** the bundle normally

#### Usage Examples

**Extract latest version of a package:**

```bash
# Uses the package's defaultChannel and latest version in that channel
bundle-extract --catalog quay.io/operatorhubio/catalog:latest prometheus -n monitoring | kubectl apply -f -
```

**Extract specific version:**

```bash
# Specify version after package name with colon separator
bundle-extract --catalog registry.redhat.io/redhat/certified-operator-index:v4.14 \
  ack-acm-controller:0.0.10 -n operators | kubectl apply -f -
```

**Extract from specific channel:**

```bash
# Override the defaultChannel with --channel flag
bundle-extract --catalog quay.io/operatorhubio/catalog:latest \
  --channel stable postgresql-operator -n databases | kubectl apply -f -
```

**Combine with registry authentication:**

```bash
# Catalog authentication uses the same registry flags
bundle-extract --catalog registry.example.com/private-catalog:latest \
  --registry-username myuser --registry-password mypass \
  my-operator:1.2.3 -n operators | kubectl apply -f -
```

#### Package Reference Format

The first positional argument in catalog mode accepts:
- **Package name only:** `my-operator` - resolves to latest version in defaultChannel
- **Package with version:** `my-operator:1.2.3` - resolves to specific version

#### Error Handling

The tool provides helpful error messages when resolution fails:

- **Package not found:** Lists available packages in the catalog
- **Version not found:** Lists available versions for the package in the channel
- **Channel not found:** Lists available channels for the package
- **No defaultChannel:** Requires explicit `--channel` flag

**Example error output:**

```
Error: version "1.0.0" not found for package "prometheus" in channel "stable" (available versions: ["1.1.0", "1.2.0", "1.2.1"])
```

### Webhook Certificate Management

When extracting operators with admission webhooks (ValidatingWebhookConfiguration, MutatingWebhookConfiguration), the tool automatically configures cert-manager to manage TLS certificates. This eliminates the need for manual certificate management or OLM's certificate rotation mechanisms.

#### Overview

The cert-manager integration:
- **Discovers webhook certificate secrets** from deployment volumes (no guessing)
- **Creates cert-manager Certificate resources** with the correct secret names
- **Injects CA bundles** into webhook configurations automatically
- **Ensures services exist** for webhooks

This allows operators with webhooks to be installed directly via `kubectl` without OLM.

#### Prerequisites

**1. cert-manager must be installed:**

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
```

Verify installation:
```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=120s
```

**2. A ClusterIssuer or Issuer must exist:**

Create a self-signed ClusterIssuer (suitable for development and most production scenarios):

```bash
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-cluster-issuer
spec:
  selfSigned: {}
EOF
```

Verify the ClusterIssuer is ready:
```bash
kubectl get clusterissuer selfsigned-cluster-issuer
```

#### Configuration

**Default behavior** (cert-manager enabled):
```bash
# Uses cert-manager with default ClusterIssuer
bundle-extract quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

**Custom ClusterIssuer or Issuer:**
```bash
# Use a custom ClusterIssuer
bundle-extract --cert-manager-issuer-name my-issuer \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Use a namespace-scoped Issuer
bundle-extract --cert-manager-issuer-name my-issuer \
  --cert-manager-issuer-kind Issuer \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

**Disable cert-manager integration:**
```bash
# Manual certificate management (you must create secrets and inject CA bundles manually)
bundle-extract --cert-manager-enabled=false \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

#### How It Works

When processing webhooks, the tool:

1. **Extracts service information** from webhook configurations
2. **Derives the deployment name** from the service name (removes `-service` suffix)
3. **Inspects deployment volumes** to find the actual webhook certificate secret name
4. **Creates a Certificate resource** with the discovered secret name
5. **Adds annotations** to webhook configurations for CA injection
6. **Ensures services exist** with correct selectors and ports

This approach works generically across all OLM bundles regardless of naming conventions.

#### Secret Name Discovery

The tool automatically discovers webhook certificate secret names by:

**Primary method:** Inspecting deployment volumes
```yaml
# Example: Tool finds "operator-webhook-cert" from deployment
spec:
  template:
    spec:
      volumes:
        - name: cert
          secret:
            secretName: operator-webhook-cert  # Extracted
```

**Keyword matching:** When multiple secrets exist, selects the most likely webhook cert using keywords: `webhook`, `cert`, `tls`, `serving`

**Fallback:** If deployment not found, generates name as `<service-name>-tls`

#### What Gets Generated

For an operator with webhooks, the tool generates:

**Certificate resource:**
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: operator-service-cert
  namespace: operators
spec:
  secretName: operator-webhook-cert  # Discovered from deployment
  dnsNames:
    - operator-service.operators.svc
    - operator-service.operators.svc.cluster.local
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned-cluster-issuer
```

**Annotated webhook:**
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: operator-webhook
  annotations:
    cert-manager.io/inject-ca-from: operators/operator-service-cert  # Added
```

**Service (if missing):**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: operator-service
  namespace: operators
spec:
  selector:
    # Extracted from deployment labels
  ports:
    - port: 443
      targetPort: 9443  # From deployment container ports
```

#### Verification

After applying the manifests, verify cert-manager is working:

**1. Check Certificate status:**
```bash
kubectl get certificate -n operators
```

Expected output:
```
NAME                    READY   SECRET                  AGE
operator-service-cert   True    operator-webhook-cert   30s
```

**2. Check Secret was created:**
```bash
kubectl get secret operator-webhook-cert -n operators
```

**3. Check webhook is ready:**
```bash
kubectl get validatingwebhookconfiguration
```

The webhook should have `caBundle` populated by cert-manager.

**4. Check deployment logs:**
```bash
kubectl logs -n operators deployment/operator-controller-manager
```

Should not show certificate-related errors.

#### Troubleshooting

**Certificate stays in "Pending" state:**
- Check if cert-manager is running: `kubectl get pods -n cert-manager`
- Check Certificate events: `kubectl describe certificate <name> -n <namespace>`
- Verify ClusterIssuer exists: `kubectl get clusterissuer`

**"secret not found" error in deployment:**
- Secret name mismatch between Certificate and Deployment
- Check what secret deployment expects: `kubectl get deployment <name> -o yaml | grep -A5 volumes:`
- Check what Certificate creates: `kubectl get certificate <name> -o yaml | grep secretName`
- If they don't match, the deployment may not have been in the original bundle

**Webhook fails with "connection refused":**
- Service selector doesn't match deployment pods
- Check service: `kubectl get service <name> -o yaml | grep -A5 selector:`
- Check deployment: `kubectl get deployment <name> -o yaml | grep -A5 matchLabels:`

**CA bundle not injected:**
- Check annotation exists: `kubectl get validatingwebhookconfiguration <name> -o yaml | grep inject-ca-from`
- Check cert-manager CA injector is running: `kubectl get pods -n cert-manager -l app=cainjector`

For detailed documentation on the webhook certificate resolution and configuration process, see [docs/webhook-certificates.md](webhook-certificates.md).

### Examples

```bash
# Extract from bundle image
bundle-extract quay.io/example/operator:v1.0.0 -n my-system | kubectl apply -f -

# Extract from local directory
bundle-extract ./bundle --namespace operators | kubectl apply -f -

# Extract without cert-manager integration
bundle-extract --cert-manager-enabled=false ./bundle -n operators | kubectl apply -f -

# Configure custom cert-manager issuer
bundle-extract --cert-manager-issuer-name my-issuer --cert-manager-issuer-kind Issuer \
  ./bundle -n operators | kubectl apply -f -

# Extract from private registry (after docker login)
bundle-extract registry.example.com/private/operator:v1.0.0 -n operators | kubectl apply -f -

# Extract from private registry with inline credentials
bundle-extract --registry-username myuser --registry-password mypass \
  registry.example.com/private/operator:v1.0.0 -n operators | kubectl apply -f -

# Extract from insecure registry
bundle-extract --registry-insecure localhost:5000/operator:latest -n dev | kubectl apply -f -

# Using environment variables
export BUNDLE_EXTRACT_NAMESPACE=operators
export BUNDLE_EXTRACT_CERT_MANAGER_ENABLED=false
bundle-extract ./bundle | kubectl apply -f -

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
- Image pull failures (suggests authenticating with `docker login` or `podman login`)
- Invalid namespace name (DNS-1123 validation)
- Unsupported install strategy
- YAML serialization errors
- TLS verification failures (suggests using `--insecure` for development)

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
- ✅ Supports authentication via Docker config file
- ✅ Supports insecure registries via `--insecure` flag
