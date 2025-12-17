# olm-extractor

A CLI tool that extracts Kubernetes manifests from OLM bundles for direct installation via kubectl or as a Kustomize generator.

## Features

- **CLI Mode**: Extract manifests and pipe directly to kubectl
- **Kustomize Integration**: Use as a KRM function generator in kustomization.yaml
- **Catalog Support**: Resolve operators from OLM catalogs by package name
- **Cert-Manager Integration**: Automatic webhook certificate management
- **Resource Filtering**: Include/exclude resources using jq expressions
- **Registry Authentication**: Support for private registries and credential helpers

## Quick Start

### CLI Mode

**Using Container Image:**

```bash
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  run quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

**Using Go:**

```bash
go run github.com/lburgazzoli/olm-extractor/cmd@latest \
  run quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

### Kustomize Mode

Create a `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - operator-bundle.yaml
```

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
        network: true
spec:
  source: quay.io/example/operator:v1.0.0
  namespace: operators
```

Then run:

```bash
kustomize build --enable-alpha-plugins --network . | kubectl apply -f -
```

## CLI Examples

**Using Catalog:**

```bash
# Extract latest version from catalog
bundle-extract run --catalog quay.io/operatorhubio/catalog:latest \
  prometheus -n monitoring | kubectl apply -f -

# Extract specific version
bundle-extract run --catalog quay.io/operatorhubio/catalog:latest \
  prometheus:1.2.3 -n monitoring | kubectl apply -f -
```

**Filtering Resources:**

```bash
# Include only Deployments and Services
bundle-extract run --include '.kind == "Deployment"' \
  --include '.kind == "Service"' \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Exclude Secrets
bundle-extract run --exclude '.kind == "Secret"' \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

**Cert-Manager Configuration:**

```bash
# Default: Auto-generates self-signed Issuer
bundle-extract run quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Use custom ClusterIssuer
bundle-extract run --cert-manager-issuer-name my-cluster-issuer \
  --cert-manager-issuer-kind ClusterIssuer \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Disable cert-manager integration
bundle-extract run --cert-manager-enabled=false \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

## Documentation

- **[Complete Specification](docs/spec.md)** - Detailed CLI usage, options, and features
- **[Kustomize Integration Guide](docs/kustomize-integration.md)** - Using as a Kustomize generator
- **[Kustomize Examples](docs/examples/kustomize/)** - Ready-to-use examples with bundle and catalog modes
  - [Basic Example](docs/examples/kustomize/basic/) - Simple bundle extraction with customization
- **[Webhook Certificates](docs/webhook-certificates.md)** - Cert-manager integration details
- **[Development Guide](docs/development.md)** - Building and contributing

