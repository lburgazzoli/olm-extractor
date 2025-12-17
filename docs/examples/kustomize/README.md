# Kustomize Integration Examples

This directory contains examples of using `olm-extractor` as a Kustomize generator to extract Kubernetes manifests from OLM bundles.

## Quick Start

The [basic](./basic/) example demonstrates extracting manifests from a bundle image:

```bash
kubectl kustomize docs/examples/kustomize/basic
```

## API Reference

The `olm-extractor` Kustomize generator uses the `Extractor` API type:

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
  # Source: bundle image OR package[:version] (depending on mode)
  source: quay.io/example/operator-bundle:v1.0.0
  
  # Namespace: target namespace for installation (required)
  namespace: operators
  
  # Catalog: enables catalog mode (optional)
  catalog:
    source: quay.io/catalog:latest  # Catalog image
    channel: stable                  # Channel (optional)
  
  # Include: jq expressions to filter resources (optional)
  include:
    - '.kind == "Deployment"'
  
  # Exclude: jq expressions to exclude resources (optional)
  exclude:
    - '.kind == "Secret"'
  
  # CertManager: webhook certificate configuration (optional)
  certManager:
    enabled: true                    # Default: true
    issuerName: my-issuer            # Default: auto-generated
    issuerKind: ClusterIssuer        # Default: Issuer
  
  # Registry: registry authentication (optional)
  registry:
    insecure: false
    username: myuser
    password: mypass
```

## Usage Modes

### Bundle Mode (Default)

Extract manifests directly from a bundle image:

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
  source: quay.io/example/operator-bundle:v1.0.0
  namespace: operators
```

### Catalog Mode

Extract manifests by resolving a package from a catalog:

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
  source: prometheus:0.56.0  # package:version
  catalog:
    source: quay.io/operatorhubio/catalog:latest
    channel: stable
  namespace: monitoring
```

## Registry Authentication

### Method 1: Mount Docker Config (Recommended)

Mount your `~/.docker/config.json` to use existing credentials:

```yaml
apiVersion: olm.lburgazzoli.github.io/v1alpha1
kind: Extractor
metadata:
  name: private-operator
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
spec:
  source: registry.example.com/private/operator:v1.0.0
  namespace: operators
```

This method works with Docker credential helpers (e.g., `osxkeychain` on macOS).

### Method 2: Environment Variables

Pass credentials via environment variables:

```yaml
apiVersion: olm.lburgazzoli.github.io/v1alpha1
kind: Extractor
metadata:
  name: private-operator
  annotations:
    config.kubernetes.io/function: |
      container:
        image: quay.io/lburgazzoli/olm-extractor:latest
        command: ["bundle-extract", "krm"]
        env:
          - BUNDLE_EXTRACT_REGISTRY_USERNAME=myuser
          - BUNDLE_EXTRACT_REGISTRY_PASSWORD=mypass
spec:
  source: registry.example.com/private/operator:v1.0.0
  namespace: operators
```

### Method 3: Inline Credentials (Not Recommended)

Credentials can be specified inline but this is not recommended for production:

```yaml
spec:
  source: registry.example.com/private/operator:v1.0.0
  namespace: operators
  registry:
    username: myuser
    password: mypass
```

## Customizing Generated Resources

Kustomize provides powerful features to customize the generated manifests.

### Adding Labels

Add common labels to all generated resources:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - generator.yaml

commonLabels:
  app.kubernetes.io/managed-by: kustomize
  app.kubernetes.io/part-of: my-app
  environment: production
```

### Adding Annotations

Add common annotations:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - generator.yaml

commonAnnotations:
  company.com/team: platform
  company.com/owner: ops-team
```

### Patching Resources

Apply strategic merge patches to generated resources:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - generator.yaml

patches:
  # Increase Deployment replicas
  - target:
      kind: Deployment
      name: operator-controller-manager
    patch: |-
      - op: replace
        path: /spec/replicas
        value: 3

  # Add resource limits
  - target:
      kind: Deployment
    patch: |-
      - op: add
        path: /spec/template/spec/containers/0/resources
        value:
          limits:
            cpu: "1"
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
```

### Namespace Transformation

Override the namespace for all resources:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - generator.yaml

namespace: production-operators
```

### Image Replacement

Replace container images in generated Deployments:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - generator.yaml

images:
  - name: quay.io/example/operator
    newName: registry.example.com/operator
    newTag: v2.0.0
```

## Advanced Configuration

### Filtering Resources

Use jq expressions to include or exclude specific resources:

```yaml
spec:
  source: quay.io/example/operator:v1.0.0
  namespace: operators
  include:
    - '.kind == "Deployment"'
    - '.kind == "Service"'
    - '.kind == "CustomResourceDefinition"'
  exclude:
    - '.metadata.name | startswith("test-")'
```

### Custom Cert-Manager Issuer

Use a specific cert-manager Issuer for webhook certificates:

```yaml
spec:
  source: quay.io/example/operator:v1.0.0
  namespace: operators
  certManager:
    enabled: true
    issuerName: letsencrypt-prod
    issuerKind: ClusterIssuer
```

### Disable Cert-Manager Integration

If cert-manager is not available:

```yaml
spec:
  source: quay.io/example/operator:v1.0.0
  namespace: operators
  certManager:
    enabled: false
```

## Complete Example

Here's a production-ready example with multiple customizations:

**kustomization.yaml:**
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generators:
  - generator.yaml

namespace: production-operators

commonLabels:
  app.kubernetes.io/managed-by: kustomize
  app.kubernetes.io/part-of: my-platform
  environment: production

commonAnnotations:
  company.com/team: platform
  company.com/contact: platform@example.com

patches:
  - target:
      kind: Deployment
    patch: |-
      - op: add
        path: /spec/template/spec/containers/0/resources
        value:
          limits:
            cpu: "2"
            memory: 1Gi
          requests:
            cpu: 200m
            memory: 256Mi

images:
  - name: quay.io/example/operator
    newName: registry.example.com/operators/example-operator
    newTag: v2.1.0-stable
```

**generator.yaml:**
```yaml
apiVersion: olm.lburgazzoli.github.io/v1alpha1
kind: Extractor
metadata:
  name: example-operator
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
spec:
  source: registry.example.com/operators/example-operator-bundle:v2.1.0
  namespace: production-operators
  certManager:
    enabled: true
    issuerName: letsencrypt-prod
    issuerKind: ClusterIssuer
```

## Troubleshooting

### Authentication Errors

If you encounter authentication errors:

1. Verify credentials: `docker login registry.example.com`
2. Check credential helper: `docker-credential-osxkeychain list`
3. Ensure Docker config is mounted correctly
4. Try inline credentials for testing (not recommended for production)

### Resource Generation Errors

Check the Kustomize logs for error messages:

```bash
kubectl kustomize docs/examples/kustomize/basic --enable-alpha-plugins 2>&1
```

Common issues:
- Invalid bundle image reference
- Network connectivity to registry
- Invalid namespace name
- Missing required spec fields

### Cert-Manager Issues

If cert-manager resources aren't generated:

1. Verify `certManager.enabled` is `true` (default)
2. Check if the operator bundle includes webhooks
3. Ensure cert-manager CRDs are installed in your cluster

## Further Reading

- [Main Documentation](../../kustomize-integration.md) - Complete Kustomize integration guide
- [CLI Documentation](../../spec.md) - Full specification and CLI reference
- [Kustomize Documentation](https://kubectl.docs.kubernetes.io/references/kustomize/) - Official Kustomize docs
