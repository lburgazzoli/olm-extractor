// Package run implements the CLI extraction mode for bundle-extract.
package run

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/catalog"
	"github.com/lburgazzoli/olm-extractor/pkg/certmanager"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/render"
)

// Config holds all configuration for the run subcommand.
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

This command extracts all necessary Kubernetes resources from an OLM (Operator Lifecycle Manager) 
bundle and transforms them for standalone installation without OLM. It handles:
  - CustomResourceDefinitions (CRDs)
  - RBAC resources (ServiceAccounts, Roles, RoleBindings, etc.)
  - Deployments with proper namespace configuration
  - Webhook configurations with automatic CA injection
  - Services for webhooks with correct port mappings

The tool supports filtering resources using jq expressions and configuring webhook CA 
injection using cert-manager.

All flags can be configured using environment variables with the BUNDLE_EXTRACT_ prefix.
Flag names are converted to uppercase and dashes are replaced with underscores.`

const exampleUsage = `  # Extract all resources from a bundle directory
  bundle-extract run -n my-namespace ./path/to/bundle

  # Extract from a bundle container image
  bundle-extract run -n my-namespace quay.io/example/operator-bundle:v1.0.0

  # Extract from a catalog (latest version in default channel)
  bundle-extract run --catalog quay.io/catalog:latest ack-acm-controller -n my-namespace

  # Extract from a catalog (specific version)
  bundle-extract run --catalog quay.io/catalog:latest ack-acm-controller:0.0.10 -n my-namespace

  # Extract from a catalog (specific channel)
  bundle-extract run --catalog quay.io/catalog:latest --channel stable ack-acm-controller -n my-namespace

  # Extract without cert-manager integration
  bundle-extract run -n my-namespace --cert-manager-enabled=false ./bundle

  # Configure cert-manager issuer for webhook certificates
  bundle-extract run -n my-namespace --cert-manager-issuer-name my-issuer \
    --cert-manager-issuer-kind Issuer ./bundle

  # Extract from insecure registry
  bundle-extract run -n my-namespace --registry-insecure localhost:5000/operator:latest

  # Extract with registry authentication
  bundle-extract run -n my-namespace --registry-username user --registry-password pass \
    quay.io/private/operator:v1.0.0

  # Filter to include only Deployments and Services
  bundle-extract run -n my-namespace --include '.kind == "Deployment"' \
    --include '.kind == "Service"' ./bundle

  # Pipe directly to kubectl
  bundle-extract run -n operators quay.io/example/operator:v1.0.0 | kubectl apply -f -`

const tempDirPerms = 0750

// NewCommand creates the run subcommand.
func NewCommand() *cobra.Command {
	// Initialize viper for environment variable support
	viper.SetEnvPrefix("BUNDLE_EXTRACT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	cmd := &cobra.Command{
		Use:          "run <bundle-path-or-image>",
		Short:        "Extract and output manifests as YAML (CLI mode)",
		Long:         longDescription,
		Example:      exampleUsage,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return execute(cmd.Context(), args[0])
		},
	}

	// Define flags
	cmd.Flags().StringP("namespace", "n", "", "Target namespace for installation (required)")
	cmd.Flags().StringArray("include", []string{}, "jq expression to include resources (repeatable, acts as OR)")
	cmd.Flags().StringArray("exclude", []string{}, "jq expression to exclude resources (repeatable, acts as OR)")
	cmd.Flags().String("temp-dir", "", "Directory for temporary files and cache (defaults to system temp directory)")
	cmd.Flags().String("catalog", "", "Catalog image to resolve bundle from (enables catalog mode)")
	cmd.Flags().String("channel", "", "Channel to use when resolving from catalog (defaults to package's defaultChannel)")
	cmd.Flags().Bool("cert-manager-enabled", true, "Enable cert-manager integration for webhook certificates")
	cmd.Flags().String("cert-manager-issuer-name", "", "Name of the cert-manager Issuer or ClusterIssuer")
	cmd.Flags().String("cert-manager-issuer-kind", "", "Kind of cert-manager issuer: Issuer or ClusterIssuer")
	cmd.Flags().Bool("registry-insecure", false, "Allow insecure connections to registries")
	cmd.Flags().String("registry-username", "", "Username for registry authentication")
	cmd.Flags().String("registry-password", "", "Password for registry authentication")

	// Bind flags to viper for environment variable support
	_ = viper.BindPFlags(cmd.Flags())

	_ = cmd.MarkFlagRequired("namespace")

	return cmd
}

// execute runs the extraction and rendering pipeline for CLI mode.
func execute(ctx context.Context, input string) error {
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

	// Phase 6: Render output as YAML
	if err := render.YAML(os.Stdout, unstructuredObjects); err != nil {
		return fmt.Errorf("failed to render YAML: %w", err)
	}

	return nil
}
