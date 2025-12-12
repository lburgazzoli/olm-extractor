package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lburgazzoli/olm-extractor/internal/version"
	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/filter"
	"github.com/lburgazzoli/olm-extractor/pkg/render"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

			if err := validateNamespace(namespace); err != nil {
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

	// Convert to Unstructured once and apply filters if provided
	if len(includeExprs) > 0 || len(excludeExprs) > 0 {
		unstructuredObjects, err := convertToUnstructured(objects)
		if err != nil {
			return fmt.Errorf("failed to convert objects: %w", err)
		}

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

		if err := render.YAMLFromUnstructured(os.Stdout, filtered); err != nil {
			return fmt.Errorf("failed to render YAML: %w", err)
		}
	} else {
		if err := render.YAML(os.Stdout, objects); err != nil {
			return fmt.Errorf("failed to render YAML: %w", err)
		}
	}

	return nil
}

func convertToUnstructured(objects []runtime.Object) ([]*unstructured.Unstructured, error) {
	result := make([]*unstructured.Unstructured, 0, len(objects))

	for _, obj := range objects {
		objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
		}

		u := &unstructured.Unstructured{Object: objMap}
		result = append(result, u)
	}

	return result, nil
}
