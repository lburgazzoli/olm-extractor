package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lburgazzoli/olm-extractor/cmd/krm"
	"github.com/lburgazzoli/olm-extractor/cmd/run"
	"github.com/lburgazzoli/olm-extractor/internal/version"
)

const rootLongDescription = `Extract Kubernetes manifests from OLM bundles for direct installation via kubectl.

This tool can operate in two modes:

1. CLI Mode (run subcommand):
   Extract manifests and output YAML to stdout for direct kubectl pipelines.
   Supports all configuration via flags and environment variables.

2. KRM Function Mode (krm subcommand):
   Operate as a Kustomize generator, reading ResourceList from stdin
   and writing generated manifests to stdout. Configuration comes from
   the functionConfig in the ResourceList.

Registry authentication uses standard Docker credentials from ~/.docker/config.json and
supports Docker credential helpers (osxkeychain on macOS, etc.) for automatic keychain integration.

Environment variables can be used to configure the 'run' subcommand. All flags can be
set using the BUNDLE_EXTRACT_ prefix. Flag names are converted to uppercase and dashes
are replaced with underscores. For example, --namespace can be set via BUNDLE_EXTRACT_NAMESPACE.`

func main() {
	rootCmd := &cobra.Command{
		Use:     "bundle-extract",
		Short:   "Extract Kubernetes manifests from OLM bundles",
		Long:    rootLongDescription,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
	}

	// Add subcommands
	rootCmd.AddCommand(run.NewCommand())
	rootCmd.AddCommand(krm.NewCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
