// Package krm implements the KRM function mode for bundle-extract.
package krm

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/lburgazzoli/olm-extractor/pkg/krm"
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
			return krm.Execute(cmd.Context(), os.Stdin, os.Stdout)
		},
	}

	return cmd
}
