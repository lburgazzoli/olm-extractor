package main

import (
	"fmt"
	"os"

	"github.com/lburgazzoli/olm-extractor/internal/version"
	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/certmanager"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/filter"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/render"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const longDescription = `Extract Kubernetes manifests from an OLM bundle and output installation-ready YAML.

This tool extracts all necessary Kubernetes resources from an OLM (Operator Lifecycle Manager) 
bundle and transforms them for standalone installation without OLM. It handles:
  - CustomResourceDefinitions (CRDs)
  - RBAC resources (ServiceAccounts, Roles, RoleBindings, etc.)
  - Deployments with proper namespace configuration
  - Webhook configurations with automatic CA injection
  - Services for webhooks with correct port mappings

The tool supports filtering resources using jq expressions and configuring webhook CA 
injection using cert-manager.`

const exampleUsage = `  # Extract all resources from a bundle directory
  bundle-extract -n my-namespace ./path/to/bundle

  # Extract from a container image
  bundle-extract -n my-namespace quay.io/example/operator-bundle:v1.0.0

  # Filter to include only Deployments and Services
  bundle-extract -n my-namespace --include '.kind == "Deployment"' \
    --include '.kind == "Service"' ./bundle

  # Exclude Secrets from output
  bundle-extract -n my-namespace --exclude '.kind == "Secret"' ./bundle

  # Configure cert-manager issuer for webhook certificates
  bundle-extract -n my-namespace --issuer-name my-issuer --issuer-kind Issuer ./bundle

  # Complex filtering: include high-replica Deployments
  bundle-extract -n my-namespace \
    --include '.kind == "Deployment" and .spec.replicas > 1' ./bundle`

const includeFlagUsage = `jq expression to include resources (repeatable, acts as OR)
Examples:
  --include '.kind == "Deployment"'
  --include '.kind == "Deployment" and .spec.replicas > 1'
  --include '.metadata.name == "my-operator"'`

const excludeFlagUsage = `jq expression to exclude resources (repeatable, acts as OR, takes priority over include)
Examples:
  --exclude '.kind == "Secret"'
  --exclude '.metadata.name == "unused-resource"'
  --exclude '.kind == "ConfigMap" and (.metadata.name | startswith("test-"))'`

const caProviderFlagUsage = `CA provider for webhook certificate injection (currently only cert-manager is supported)`

const issuerNameFlagUsage = `Name of the cert-manager Issuer or ClusterIssuer to use for webhook certificates`

const issuerKindFlagUsage = `Kind of cert-manager issuer to use: Issuer (namespace-scoped) or ClusterIssuer (cluster-wide)`

func main() {
	var namespace string
	var includeExprs []string
	var excludeExprs []string
	var caProvider string
	var issuerName string
	var issuerKind string
	var insecure bool

	rootCmd := &cobra.Command{
		Use:     "bundle-extract <bundle-path-or-image>",
		Short:   "Extract Kubernetes manifests from an OLM bundle",
		Long:    longDescription,
		Example: exampleUsage,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			input := args[0]

			if err := kube.ValidateNamespace(namespace); err != nil {
				return err
			}

			return extractAndRender(input, namespace, includeExprs, excludeExprs, caProvider, issuerName, issuerKind, insecure)
		},
	}

	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace for installation (required)")
	rootCmd.Flags().StringArrayVar(&includeExprs, "include", []string{}, includeFlagUsage)
	rootCmd.Flags().StringArrayVar(&excludeExprs, "exclude", []string{}, excludeFlagUsage)
	rootCmd.Flags().StringVar(&caProvider, "ca-provider", "cert-manager", caProviderFlagUsage)
	rootCmd.Flags().StringVar(&issuerName, "issuer-name", "selfsigned-cluster-issuer", issuerNameFlagUsage)
	rootCmd.Flags().StringVar(&issuerKind, "issuer-kind", "ClusterIssuer", issuerKindFlagUsage)
	rootCmd.Flags().BoolVar(&insecure, "insecure", false, "Allow insecure connections to registries (HTTP or self-signed certificates)")

	if err := rootCmd.MarkFlagRequired("namespace"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func extractAndRender(input string, namespace string, includeExprs []string, excludeExprs []string, caProviderName string, issuerName string, issuerKind string, insecure bool) error {
	b, cleanup, err := bundle.Load(input, insecure)
	if cleanup != nil {
		defer cleanup()
	}

	if err != nil {
		return fmt.Errorf("failed to load bundle: %w", err)
	}

	objects, err := extract.Manifests(b, namespace)
	if err != nil {
		return fmt.Errorf("failed to extract manifests: %w", err)
	}

	unstructuredObjects, err := kube.ConvertToUnstructured(objects)
	if err != nil {
		return fmt.Errorf("failed to convert objects: %w", err)
	}

	if len(includeExprs) > 0 || len(excludeExprs) > 0 {
		f, err := filter.New(includeExprs, excludeExprs)
		if err != nil {
			return fmt.Errorf("failed to create filter: %w", err)
		}

		filtered := make([]*unstructured.Unstructured, 0, len(unstructuredObjects))
		for _, obj := range unstructuredObjects {
			if f.Matches(obj) {
				filtered = append(filtered, obj)
			}
		}
		unstructuredObjects = filtered
	}

	// Validate CA provider
	if caProviderName != "cert-manager" {
		return fmt.Errorf("unknown CA provider: %s (only cert-manager is supported)", caProviderName)
	}

	// Configure cert-manager CA injection for webhooks
	unstructuredObjects, err = certmanager.Configure(unstructuredObjects, namespace, issuerName, issuerKind)
	if err != nil {
		return fmt.Errorf("failed to configure cert-manager: %w", err)
	}

	if err := render.YAMLFromUnstructured(os.Stdout, unstructuredObjects); err != nil {
		return fmt.Errorf("failed to render YAML: %w", err)
	}

	return nil
}
