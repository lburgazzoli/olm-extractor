package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lburgazzoli/olm-extractor/internal/version"
	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/catalog"
	"github.com/lburgazzoli/olm-extractor/pkg/certmanager"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/render"
)

// Config holds all configuration for the application.
type Config struct {
	Namespace   string                `mapstructure:"namespace"`
	Include     []string              `mapstructure:"include"`
	Exclude     []string              `mapstructure:"exclude"`
	TempDir     string                `mapstructure:"temp-dir"`
	Catalog     string                `mapstructure:"catalog"`
	Channel     string                `mapstructure:"channel"`
	CertManager certmanager.Config    `mapstructure:",squash"`
	Registry    bundle.RegistryConfig `mapstructure:",squash"`
}

const longDescription = `Extract Kubernetes manifests from an OLM bundle and output installation-ready YAML.

This tool extracts all necessary Kubernetes resources from an OLM (Operator Lifecycle Manager) 
bundle and transforms them for standalone installation without OLM. It handles:
  - CustomResourceDefinitions (CRDs)
  - RBAC resources (ServiceAccounts, Roles, RoleBindings, etc.)
  - Deployments with proper namespace configuration
  - Webhook configurations with automatic CA injection
  - Services for webhooks with correct port mappings

The tool supports filtering resources using jq expressions and configuring webhook CA 
injection using cert-manager.

Registry authentication uses standard Docker credentials from ~/.docker/config.json and
supports Docker credential helpers (osxkeychain on macOS, etc.) for automatic keychain integration.

All flags can be configured using environment variables with the BUNDLE_EXTRACT_ prefix.
Flag names are converted to uppercase and dashes are replaced with underscores.
For example, --namespace can be set via BUNDLE_EXTRACT_NAMESPACE.`

const exampleUsage = `  # Extract all resources from a bundle directory
  bundle-extract -n my-namespace ./path/to/bundle

  # Extract from a bundle container image
  bundle-extract -n my-namespace quay.io/example/operator-bundle:v1.0.0

  # Extract from a catalog (latest version in default channel)
  bundle-extract --catalog quay.io/catalog:latest ack-acm-controller -n my-namespace

  # Extract from a catalog (specific version)
  bundle-extract --catalog quay.io/catalog:latest ack-acm-controller:0.0.10 -n my-namespace

  # Extract from a catalog (specific channel)
  bundle-extract --catalog quay.io/catalog:latest --channel stable ack-acm-controller -n my-namespace

  # Extract without cert-manager integration
  bundle-extract -n my-namespace --cert-manager-enabled=false ./bundle

  # Configure cert-manager issuer for webhook certificates
  bundle-extract -n my-namespace --cert-manager-issuer-name my-issuer \
    --cert-manager-issuer-kind Issuer ./bundle

  # Extract from insecure registry
  bundle-extract -n my-namespace --registry-insecure localhost:5000/operator:latest

  # Extract with registry authentication
  bundle-extract -n my-namespace --registry-username user --registry-password pass \
    quay.io/private/operator:v1.0.0

  # Filter to include only Deployments and Services
  bundle-extract -n my-namespace --include '.kind == "Deployment"' \
    --include '.kind == "Service"' ./bundle

  # Using environment variables
  export BUNDLE_EXTRACT_NAMESPACE=my-namespace
  export BUNDLE_EXTRACT_CERT_MANAGER_ENABLED=false
  bundle-extract ./bundle`

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

const certManagerEnabledUsage = `Enable cert-manager integration for webhook certificates (default: true)`

const certManagerIssuerNameUsage = `Name of the cert-manager Issuer or ClusterIssuer to use for webhook certificates. If not specified, auto-generates a self-signed Issuer named "<operator>-selfsigned"`

const certManagerIssuerKindUsage = `Kind of cert-manager issuer to use: Issuer (namespace-scoped) or ClusterIssuer (cluster-wide). If not specified, defaults to "Issuer" with auto-generated self-signed issuer`

const registryInsecureUsage = `Allow insecure connections to registries (HTTP or self-signed certificates)`

const registryUsernameUsage = `Username for registry authentication (uses Docker config and credential helpers by default)`

const registryPasswordUsage = `Password for registry authentication (uses Docker config and credential helpers by default)`

const tempDirUsage = `Directory for temporary files and cache (defaults to system temp directory)`

const catalogUsage = `Catalog image to resolve bundle from (enables catalog mode). When specified, the first positional argument becomes <package>[:version] instead of a bundle image.`

const channelUsage = `Channel to use when resolving from catalog (defaults to package's defaultChannel)`

const tempDirPerms = 0750 // Directory permissions for temp directory

func main() {
	// Initialize viper for environment variable support
	viper.SetEnvPrefix("BUNDLE_EXTRACT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	rootCmd := &cobra.Command{
		Use:          "bundle-extract <bundle-path-or-image>",
		Short:        "Extract Kubernetes manifests from an OLM bundle",
		Long:         longDescription,
		Example:      exampleUsage,
		Version:      fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			input := args[0]

			// Unmarshal configuration from viper (supports both flags and env vars)
			var cfg Config
			if err := viper.Unmarshal(&cfg); err != nil {
				return fmt.Errorf("failed to parse configuration: %w", err)
			}

			if err := kube.ValidateNamespace(cfg.Namespace); err != nil {
				return fmt.Errorf("invalid namespace: %w", err)
			}

			// Create temp directory if specified and doesn't exist
			if cfg.TempDir != "" {
				if err := os.MkdirAll(cfg.TempDir, tempDirPerms); err != nil {
					return fmt.Errorf("failed to create temp-dir: %w", err)
				}
			}

			return extractAndRender(ctx, input, cfg)
		},
	}

	// Define flags
	rootCmd.Flags().StringP("namespace", "n", "", "Target namespace for installation (required)")
	rootCmd.Flags().StringArray("include", []string{}, includeFlagUsage)
	rootCmd.Flags().StringArray("exclude", []string{}, excludeFlagUsage)
	rootCmd.Flags().String("temp-dir", "", tempDirUsage)
	rootCmd.Flags().String("catalog", "", catalogUsage)
	rootCmd.Flags().String("channel", "", channelUsage)
	rootCmd.Flags().Bool("cert-manager-enabled", true, certManagerEnabledUsage)
	rootCmd.Flags().String("cert-manager-issuer-name", "", certManagerIssuerNameUsage)
	rootCmd.Flags().String("cert-manager-issuer-kind", "", certManagerIssuerKindUsage)
	rootCmd.Flags().Bool("registry-insecure", false, registryInsecureUsage)
	rootCmd.Flags().String("registry-username", "", registryUsernameUsage)
	rootCmd.Flags().String("registry-password", "", registryPasswordUsage)

	// Bind flags to viper (environment variables are automatically bound via AutomaticEnv)
	_ = viper.BindPFlags(rootCmd.Flags())

	if err := rootCmd.MarkFlagRequired("namespace"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// extractAndRender orchestrates the extraction and rendering pipeline.
func extractAndRender(ctx context.Context, input string, cfg Config) error {
	// Phase 1: Resolve bundle source
	bundleImageOrDir, err := catalog.ResolveBundleSource(
		ctx,
		input,
		cfg.Catalog,
		cfg.Channel,
		cfg.Registry,
		cfg.TempDir,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve bundle source: %w", err)
	}

	// Phase 2: Load bundle
	b, err := bundle.Load(ctx, bundleImageOrDir, cfg.Registry, cfg.TempDir)
	if err != nil {
		return fmt.Errorf("failed to load bundle: %w", err)
	}

	// Phase 3: Extract manifests
	objects, err := extract.Manifests(b, cfg.Namespace)
	if err != nil {
		return fmt.Errorf("failed to extract manifests: %w", err)
	}

	// Phase 4: Convert to unstructured
	unstructuredObjects, err := kube.ConvertToUnstructured(objects)
	if err != nil {
		return fmt.Errorf("failed to convert objects: %w", err)
	}

	// Phase 5: Apply transformations
	unstructuredObjects, err = extract.ApplyTransformations(
		unstructuredObjects,
		cfg.Namespace,
		cfg.Include,
		cfg.Exclude,
		cfg.CertManager,
	)
	if err != nil {
		return fmt.Errorf("failed to apply transformations: %w", err)
	}

	// Phase 6: Render output
	if err := render.YAMLFromUnstructured(os.Stdout, unstructuredObjects); err != nil {
		return fmt.Errorf("failed to render YAML: %w", err)
	}

	return nil
}
