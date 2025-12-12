package main

import (
	"fmt"
	"os"

	"github.com/lburgazzoli/olm-extractor/internal/version"
	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/filter"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/render"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func main() {
	var namespace string
	var includeExprs []string
	var excludeExprs []string

	rootCmd := &cobra.Command{
		Use:     "bundle-extract <bundle-path-or-image>",
		Short:   "Extract Kubernetes manifests from an OLM bundle",
		Long:    "Extract Kubernetes manifests from an OLM bundle and output installation-ready YAML to stdout.",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			input := args[0]

			if err := kube.ValidateNamespace(namespace); err != nil {
				return err
			}

			return extractAndRender(input, namespace, includeExprs, excludeExprs)
		},
	}

	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace for installation (required)")
	rootCmd.Flags().StringArrayVar(&includeExprs, "include", []string{}, "jq expression to include resources (can be repeated, acts as OR)")
	rootCmd.Flags().StringArrayVar(&excludeExprs, "exclude", []string{}, "jq expression to exclude resources (can be repeated, acts as OR, takes priority over include)")

	if err := rootCmd.MarkFlagRequired("namespace"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func extractAndRender(input string, namespace string, includeExprs []string, excludeExprs []string) error {
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

	if err := render.YAMLFromUnstructured(os.Stdout, unstructuredObjects); err != nil {
		return fmt.Errorf("failed to render YAML: %w", err)
	}

	return nil
}
