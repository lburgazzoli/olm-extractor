# Basic Kustomize Example

This example demonstrates how to use `olm-extractor` as a Kustomize generator to extract Kubernetes manifests from an OLM bundle.

## What This Example Does

- Extracts manifests from the OpenDataHub operator bundle
- Installs resources into the `opendatahub` namespace
- Enables cert-manager integration for webhook certificates
- Applies common labels to all generated resources (small customization)

## Usage

Generate the manifests:

```bash
kustomize build --enable-alpha-plugins docs/examples/kustomize/basic
```

Apply to a cluster:

```bash
kustomize build --enable-alpha-plugins docs/examples/kustomize/basic | kubectl apply -f -
```

## What Resources Are Generated

The extractor will generate:
- CustomResourceDefinitions (CRDs)
- Namespace
- ServiceAccount
- RBAC resources (Roles, RoleBindings, ClusterRoles, ClusterRoleBindings)
- Deployment
- Services (if webhooks are present)
- cert-manager resources (Issuer, Certificate) for webhook CA injection

All resources will have the common labels applied:
- `app.kubernetes.io/managed-by: kustomize`
- `app.kubernetes.io/part-of: opendatahub`

## More Options

See the [main Kustomize README](../README.md) for more advanced usage:
- Using catalog mode
- Private registry authentication
- Additional customizations (patches, transformers)
- Environment variables for credentials
