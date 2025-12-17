// Package krm implements the KRM function mode for bundle-extract.
package krm

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/catalog"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/krm"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/render"
)

const longDescription = `Run as KRM function for Kustomize generators.

This command implements the Kubernetes Resource Model (KRM) function interface.
It reads a ResourceList from stdin containing functionConfig (Extractor)
and writes a ResourceList with generated manifests to stdout.

This mode is designed exclusively for use as a Kustomize generator and is not
intended for direct CLI usage. Use the 'run' subcommand for CLI operations.

The functionConfig should be an Extractor type which can work in two modes:
  - Bundle mode: Extract from a specific bundle image (when catalog is not set)
  - Catalog mode: Extract from a catalog index (when catalog is set)

Configuration is passed entirely through the functionConfig in the ResourceList.
Environment variables and command-line flags are not supported in KRM mode.`

const exampleUsage = `  # Typically invoked by Kustomize, not directly by users
  echo "<ResourceList YAML>" | bundle-extract krm

  # Example functionConfig for bundle mode in kustomization.yaml:
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

  # Example functionConfig for catalog mode:
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
    source: prometheus:1.2.3
    catalog:
      source: quay.io/operatorhubio/catalog:latest
      channel: stable
    namespace: monitoring`

// NewCommand creates the krm subcommand.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "krm",
		Short:        "Run as KRM function for Kustomize (reads ResourceList from stdin)",
		Long:         longDescription,
		Example:      exampleUsage,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return execute(cmd.Context())
		},
	}

	return cmd
}

// execute implements the KRM function interface for Kustomize.
func execute(ctx context.Context) error {
	// Phase 1: Read ResourceList from stdin
	rl, err := krm.ReadResourceList(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read ResourceList: %w", err)
	}

	// Phase 2: Extract functionConfig
	fc, err := krm.ExtractFunctionConfig(rl)
	if err != nil {
		return fmt.Errorf("failed to extract functionConfig: %w", err)
	}

	// Phase 3: Convert functionConfig to internal config
	cfg, input, err := fc.ExtractorConfig.ToConfig(os.TempDir())
	if err != nil {
		rl.AddError(fmt.Sprintf("invalid configuration: %v", err))

		return krm.WriteResourceList(os.Stdout, rl)
	}

	// Phase 4: Validate namespace
	if err := kube.ValidateNamespace(cfg.Namespace); err != nil {
		rl.AddError(fmt.Sprintf("invalid namespace: %v", err))

		return krm.WriteResourceList(os.Stdout, rl)
	}

	// Phase 5: Resolve bundle source
	bundleImageOrDir, err := catalog.ResolveBundleSource(
		ctx,
		input,
		cfg.Catalog,
		cfg.Channel,
		cfg.Registry,
		cfg.TempDir,
	)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to resolve bundle source: %v", err))

		return krm.WriteResourceList(os.Stdout, rl)
	}

	// Phase 6: Load bundle
	b, err := bundle.Load(ctx, bundleImageOrDir, cfg.Registry, cfg.TempDir)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to load bundle: %v", err))

		return krm.WriteResourceList(os.Stdout, rl)
	}

	// Phase 7: Extract manifests
	objects, err := extract.Manifests(b, cfg.Namespace)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to extract manifests: %v", err))

		return krm.WriteResourceList(os.Stdout, rl)
	}

	// Phase 8: Convert to unstructured
	unstructuredObjects, err := kube.ConvertToUnstructured(objects)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to convert objects: %v", err))

		return krm.WriteResourceList(os.Stdout, rl)
	}

	// Phase 9: Apply transformations
	unstructuredObjects, err = extract.ApplyTransformations(
		unstructuredObjects,
		cfg.Namespace,
		cfg.Include,
		cfg.Exclude,
		cfg.CertManager,
	)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to apply transformations: %v", err))

		return krm.WriteResourceList(os.Stdout, rl)
	}

	// Phase 10: Convert to ResourceList and write output
	outputRL := render.ToResourceList(unstructuredObjects)
	if err := krm.WriteResourceList(os.Stdout, outputRL); err != nil {
		return fmt.Errorf("failed to write ResourceList: %w", err)
	}

	return nil
}
