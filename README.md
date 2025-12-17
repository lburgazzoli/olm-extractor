# olm-extractor

A CLI tool that extracts Kubernetes manifests from OLM bundles for direct installation via kubectl.

## Usage

**Using Container Image:**

```bash
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

**Using Go:**

```bash
go run github.com/lburgazzoli/olm-extractor/cmd@latest \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

**Using Catalog:**

```bash
# Extract latest version from catalog
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  --catalog quay.io/operatorhubio/catalog:latest \
  prometheus -n monitoring | kubectl apply -f -

# Extract specific version
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  --catalog quay.io/operatorhubio/catalog:latest \
  prometheus:1.2.3 -n monitoring | kubectl apply -f -
```

**Filtering Resources:**

```bash
# Include only Deployments and Services
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  --include '.kind == "Deployment"' \
  --include '.kind == "Service"' \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Exclude Secrets
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  --exclude '.kind == "Secret"' \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

**Cert-Manager Configuration:**

```bash
# Default: Auto-generates self-signed Issuer (e.g., "my-operator-selfsigned")
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Use custom ClusterIssuer
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  --cert-manager-issuer-name my-cluster-issuer \
  --cert-manager-issuer-kind ClusterIssuer \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Use custom namespace-scoped Issuer
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  --cert-manager-issuer-name my-issuer \
  --cert-manager-issuer-kind Issuer \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -

# Disable cert-manager integration
docker run --rm quay.io/lburgazzoli/olm-extractor:main \
  --cert-manager-enabled=false \
  quay.io/example/operator:v1.0.0 -n operators | kubectl apply -f -
```

## Documentation

See [`docs/spec.md`](docs/spec.md) for detailed usage, options, and examples.

