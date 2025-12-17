# Kustomize Integration Guide

This guide explains how to use olm-extractor as a Kustomize KRM function to generate Kubernetes manifests from OLM bundles.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Registry Authentication](#registry-authentication)
- [Advanced Usage](#advanced-usage)
- [Troubleshooting](#troubleshooting)

## Overview

olm-extractor can be used as a Kustomize generator to extract manifests from OLM bundles at build time. This enables:

- **Declarative Operator Management**: Define operator installations in `kustomization.yaml`
- **Version Control**: Track operator configurations alongside application manifests
- **Kustomize Transformations**: Apply patches, labels, and overlays to generated manifests
- **GitOps Integration**: Seamlessly integrate with ArgoCD, Flux, and other GitOps tools

### Comparison: CLI vs KRM Function

| Feature | CLI Mode (`run`) | KRM Function Mode (`krm`) |
|---------|------------------|----------------------------|
| **Usage** | Direct command line | Kustomize generator |
| **Input** | Command arguments | ResourceList (stdin) |
| **Output** | YAML to stdout | ResourceList (stdout) |
| **Config** | Flags & env vars | functionConfig |
| **Integration** | Shell scripts, CI/CD | Kustomize, GitOps |

## Prerequisites

- **Kustomize** v4.0.0 or later
- **Docker** or compatible container runtime
- **Network Access** for pulling images
- **Alpha Plugins** must be enabled (`--enable-alpha-plugins`)

## Quick Start

### 1. Create a Generator Config

Create `operator-bundle.yaml`:

```yaml
apiVersion: olm.lburgazzoli.github.io/v1alpha1
kind: Extractor
metadata:
  name: my-operator
  annotations:
    config.kubernetes.io/function: |
      container:
        image: quay.io/lburgazzoli/olm-extractor:latest
        command: ["bundle-extract", "krm"]
spec:
  source: quay.io/example/operator:v1.0.0
  namespace: operators
  certManager:
    enabled: true
```

### 2. Reference in Kustomization

Create or update `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - operator-bundle.yaml
```

### 3. Build and Apply

```bash
# Preview generated manifests
kustomize build --enable-alpha-plugins .

# Apply to cluster
kustomize build --enable-alpha-plugins . | kubectl apply -f -
```

## Configuration

### Extractor API

The `Extractor` type provides a unified API for both bundle and catalog extraction modes.

#### Bundle Mode

Extract from a specific bundle image (no `catalog` field):

```yaml
apiVersion: olm.lburgazzoli.github.io/v1alpha1
kind: Extractor
metadata:
  name: my-operator
  annotations:
    config.kubernetes.io/function: |
      container:
        image: quay.io/lburgazzoli/olm-extractor:latest
        command: ["bundle-extract", "krm"]
spec:
  # Source is the bundle image
  source: quay.io/example/operator:v1.0.0
  
  # Required: Target namespace
  namespace: operators
  
  # Optional: Resource filtering
  include:
    - '.kind == "Deployment"'
    - '.kind == "Service"'
  exclude:
    - '.kind == "Secret"'
  
  # Optional: Cert-manager configuration
  certManager:
    enabled: true
    issuerName: ""  # Empty = auto-generate
    issuerKind: ""  # Empty = auto-generate
  
  # Optional: Registry authentication
  registry:
    insecure: false
    username: ""
    password: ""
```

#### Catalog Mode

Extract from a catalog index (with `catalog` field):

```yaml
apiVersion: olm.lburgazzoli.github.io/v1alpha1
kind: Extractor
metadata:
  name: prometheus-operator
  annotations:
    config.kubernetes.io/function: |
      container:
        image: quay.io/lburgazzoli/olm-extractor:latest
        command: ["bundle-extract", "krm"]
spec:
  # Source is the package name (optionally with version)
  source: prometheus:0.56.0
  
  # Catalog configuration enables catalog mode
  catalog:
    source: quay.io/operatorhubio/catalog:latest
    channel: stable  # Optional: defaults to defaultChannel
  
  # Required: Target namespace
  namespace: monitoring
  
  # Optional: Same filtering and cert-manager config as bundle mode
  certManager:
    enabled: true
```

### Container Configuration

The `config.kubernetes.io/function` annotation configures how Kustomize runs the container:

```yaml
annotations:
  config.kubernetes.io/function: |
    container:
      # Container image to use
      image: quay.io/lburgazzoli/olm-extractor:latest
      
      # Command to execute in the container
      command: ["bundle-extract", "krm"]
      
      # Optional: Environment variables
      env:
        - BUNDLE_EXTRACT_REGISTRY_INSECURE=true
      
      # Optional: Volume mounts
      mounts:
        - type: bind
          src: ~/.docker/config.json
          dst: /root/.docker/config.json
          readOnly: true
      
      # Optional: Network access (default: true)
      network: true
```

## Registry Authentication

### Method 1: Docker Config Mount (Recommended)

Mount your Docker config file to provide credentials:

```yaml
annotations:
  config.kubernetes.io/function: |
    container:
      image: quay.io/lburgazzoli/olm-extractor:latest
      command: ["bundle-extract", "krm"]
      mounts:
        - type: bind
          src: ~/.docker/config.json
          dst: /root/.docker/config.json
          readOnly: true
```

**Benefits:**
- Works with credential helpers (osxkeychain, wincred)
- Supports multiple registries
- No passwords in YAML files
- Uses existing `docker login` credentials

**Setup:**

```bash
# Login to your registry
docker login registry.example.com

# Credentials are now available
kustomize build --enable-alpha-plugins .
```

### Method 2: Environment Variables

Pass credentials via environment variables:

```yaml
annotations:
  config.kubernetes.io/function: |
    container:
      image: quay.io/lburgazzoli/olm-extractor:latest
      command: ["bundle-extract", "krm"]
      env:
        - BUNDLE_EXTRACT_REGISTRY_USERNAME=myuser
        - BUNDLE_EXTRACT_REGISTRY_PASSWORD=mypassword
```

**Note:** Credentials are visible in the YAML file. Use only for testing or CI environments.

### Method 3: Inline in Spec (Not Recommended)

```yaml
spec:
  source: registry.example.com/private/operator:v1.0.0
  namespace: operators
  registry:
    username: myuser
    password: mypassword
```

**Warning:** Exposes credentials in plain text. Avoid in production.

### Insecure Registries

For self-signed certificates or HTTP-only registries:

```yaml
spec:
  source: localhost:5000/operator:latest
  namespace: operators
  registry:
    insecure: true
```

Or via environment variable:

```yaml
annotations:
  config.kubernetes.io/function: |
    container:
      image: quay.io/lburgazzoli/olm-extractor:latest
      command: ["bundle-extract", "krm"]
      env:
        - BUNDLE_EXTRACT_REGISTRY_INSECURE=true
```

## Advanced Usage

### Resource Filtering

Include only specific resources using jq expressions:

```yaml
spec:
  source: quay.io/example/operator:v1.0.0
  namespace: operators
  include:
    - '.kind == "Deployment"'
    - '.kind == "Service"'
    - '.kind == "ConfigMap"'
```

Exclude resources:

```yaml
spec:
  source: quay.io/example/operator:v1.0.0
  namespace: operators
  exclude:
    - '.kind == "Secret"'
    - '.metadata.name | startswith("test-")'
    - '.kind == "ConfigMap" and (.metadata.labels.environment == "dev")'
```

### Cert-Manager Integration

#### Auto-Generate Self-Signed Issuer (Default)

```yaml
spec:
  certManager:
    enabled: true
    # Leave issuerName empty to auto-generate
```

Generates an Issuer named `<operator-name>-selfsigned` in the target namespace.

#### Use Existing ClusterIssuer

```yaml
spec:
  certManager:
    enabled: true
    issuerName: letsencrypt-prod
    issuerKind: ClusterIssuer
```

#### Use Existing Namespace-Scoped Issuer

```yaml
spec:
  certManager:
    enabled: true
    issuerName: my-issuer
    issuerKind: Issuer
```

#### Disable Cert-Manager

```yaml
spec:
  certManager:
    enabled: false
```

**Note:** You must manually manage webhook certificates if disabled.

### Combining with Kustomize Transformations

Apply patches to generated resources:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - operator-bundle.yaml

# Add common labels to all resources
commonLabels:
  app.kubernetes.io/managed-by: kustomize
  environment: production

# Patch specific resources
patches:
  - target:
      kind: Deployment
    patch: |-
      - op: replace
        path: /spec/replicas
        value: 3
  
  - target:
      kind: Service
      name: operator-service
    patch: |-
      - op: add
        path: /metadata/annotations/external-dns.alpha.kubernetes.io~1hostname
        value: operator.example.com

# Add additional resources
resources:
  - namespace.yaml
  - monitoring-config.yaml
```

### Multiple Operators

Generate manifests from multiple bundles:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - operators/prometheus.yaml
  - operators/grafana.yaml
  - operators/vault.yaml
```

### Overlays

Use overlays for environment-specific configurations:

```
.
├── base/
│   ├── kustomization.yaml
│   └── operator-bundle.yaml
├── overlays/
│   ├── dev/
│   │   └── kustomization.yaml
│   └── prod/
│       └── kustomization.yaml
```

**base/kustomization.yaml:**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - operator-bundle.yaml
```

**overlays/prod/kustomization.yaml:**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

patches:
  - target:
      kind: Deployment
    patch: |-
      - op: replace
        path: /spec/replicas
        value: 3
      - op: add
        path: /spec/template/spec/containers/0/resources
        value:
          limits:
            cpu: "2"
            memory: "4Gi"
          requests:
            cpu: "1"
            memory: "2Gi"
```

## Troubleshooting

### Generator Not Found

**Error:**
```
Error: couldn't execute function: ...
```

**Solution:** Use the `--enable-alpha-plugins` flag:

```bash
kustomize build --enable-alpha-plugins .
```

### Image Pull Failures

**Error:**
```
Error: failed to pull image...
```

**Solutions:**

1. **Verify image exists:**
   ```bash
   docker pull quay.io/example/operator:v1.0.0
   ```

2. **Check authentication:**
   ```bash
   docker login registry.example.com
   ```

3. **Use insecure flag for self-signed certs:**
   ```yaml
   spec:
     registry:
       insecure: true
   ```

### Empty Output

If `kustomize build` produces no resources:

1. **Check YAML syntax:**
   ```bash
   kustomize build --enable-alpha-plugins . 2>&1 | grep -i error
   ```

2. **Verify function annotation is correct:**
   ```yaml
   annotations:
     config.kubernetes.io/function: |  # Note the | for multiline
       container:
         image: quay.io/lburgazzoli/olm-extractor:latest
         command: ["bundle-extract", "krm"]
   ```

3. **Ensure spec fields are present:**
   ```yaml
   spec:
     source: <bundle-image-or-package>  # Required
     namespace: <namespace>              # Required
   ```

### Permission Denied

**Error:**
```
Error: permission denied...
```

**Solutions:**

1. **Ensure Docker is running:**
   ```bash
   docker ps
   ```

2. **Check Docker permissions:**
   ```bash
   docker run --rm hello-world
   ```

3. **Verify mount paths exist:**
   ```bash
   ls -la ~/.docker/config.json
   ```

### Namespace Validation Errors

**Error:**
```
Error: invalid namespace: ...
```

**Solution:** Ensure namespace follows DNS-1123 conventions:
- Lowercase alphanumeric characters or `-`
- Start and end with alphanumeric
- Max 63 characters

### Function Config Errors

**Error:**
```
Error: failed to extract functionConfig: ...
```

**Solutions:**

1. **Check API version:**
   ```yaml
   apiVersion: olm.lburgazzoli.github.io/v1alpha1  # Must be exact
   ```

2. **Check kind:**
   ```yaml
   kind: Extractor
   ```

3. **Verify required fields:**
   ```yaml
   spec:
     source: quay.io/example/operator:v1.0.0  # Required
     namespace: operators                      # Required
   ```

### Debug Mode

Enable verbose output:

```bash
# See what Kustomize is executing
kustomize build --enable-alpha-plugins . 2>&1 | tee build.log

# Check container execution
docker ps -a | grep olm-extractor
```

## Best Practices

1. **Use Docker Config Mount**: Prefer mounting `~/.docker/config.json` over inline credentials
2. **Pin Image Versions**: Use specific tags instead of `:latest` for reproducibility
3. **Version Control**: Commit generator configs to Git alongside applications
4. **Test Locally**: Run `kustomize build` before pushing to verify output
5. **Use Overlays**: Separate environment-specific configurations
6. **Resource Filtering**: Use `include`/`exclude` to minimize generated resources
7. **Cert-Manager**: Let olm-extractor manage webhook certificates automatically

## Integration with GitOps

### ArgoCD

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-operator
spec:
  source:
    repoURL: https://github.com/org/repo
    path: operators/my-operator
    targetRevision: main
    kustomize:
      # Enable alpha plugins
      buildOptions: --enable-alpha-plugins
```

### Flux

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-operator
spec:
  sourceRef:
    kind: GitRepository
    name: fleet-infra
  path: ./operators/my-operator
  prune: true
  # Flux doesn't support alpha plugins - use pre-rendered manifests
```

**Note:** Flux doesn't support Kustomize alpha plugins. Pre-render manifests in CI:

```bash
kustomize build --enable-alpha-plugins operators/my-operator > operators/my-operator/manifests.yaml
```

## Additional Resources

- [Examples Directory](examples/kustomize/) - Ready-to-use examples
- [Kustomize Documentation](https://kustomize.io/)
- [KRM Functions Specification](https://github.com/kubernetes-sigs/kustomize/blob/master/cmd/config/docs/api-conventions/functions-spec.md)
- [olm-extractor CLI Documentation](spec.md)
- [Webhook Certificates Guide](webhook-certificates.md)
