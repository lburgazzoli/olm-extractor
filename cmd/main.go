package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lburgazzoli/olm-extractor/internal/version"
	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/render"
	"github.com/spf13/cobra"
)

func main() {
	var namespace string

	rootCmd := &cobra.Command{
		Use:     "bundle-extract <bundle-path-or-image>",
		Short:   "Extract Kubernetes manifests from an OLM bundle",
		Long:    "Extract Kubernetes manifests from an OLM bundle and output installation-ready YAML to stdout.",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			input := args[0]

			if err := validateNamespace(namespace); err != nil {
				return err
			}

			return extractAndRender(input, namespace)
		},
	}

	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace for installation (required)")
	if err := rootCmd.MarkFlagRequired("namespace"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func validateNamespace(ns string) error {
	if ns == "" {
		return errors.New("namespace cannot be empty")
	}

	if len(ns) > 63 {
		return errors.New("namespace name too long (max 63 characters)")
	}

	for i, c := range ns {
		isLowerAlpha := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isDash := c == '-'

		if !isLowerAlpha && !isDigit && !isDash {
			return errors.New("invalid namespace name: must consist of lowercase alphanumeric characters or '-'")
		}

		if i == 0 && (isDash || isDigit) {
			return errors.New("invalid namespace name: must start with a lowercase letter")
		}

		if i == len(ns)-1 && isDash {
			return errors.New("invalid namespace name: must end with an alphanumeric character")
		}
	}

	return nil
}

func extractAndRender(input string, namespace string) error {
	b, cleanup, err := bundle.Load(input)
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

	if err := render.YAML(os.Stdout, objects); err != nil {
		return fmt.Errorf("failed to render YAML: %w", err)
	}

	return nil
}
