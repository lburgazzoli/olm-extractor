package krm

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/catalog"
	"github.com/lburgazzoli/olm-extractor/pkg/extract"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
)

// Execute implements the KRM function interface for Kustomize.
// It reads a ResourceList from the reader, processes it, and writes the result to the writer.
func Execute(ctx context.Context, reader io.Reader, writer io.Writer) error {
	fmt.Fprintf(os.Stderr, "[KRM] Starting execution\n")

	// Phase 1: Read ResourceList from stdin
	rl, err := ReadResourceList(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error reading ResourceList: %v\n", err)
		return fmt.Errorf("failed to read ResourceList: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Read ResourceList with %d items\n", len(rl.Items))

	// Phase 2: Extract functionConfig
	fc, err := ExtractFunctionConfig(rl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error extracting functionConfig: %v\n", err)
		return fmt.Errorf("failed to extract functionConfig: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Extracted functionConfig: kind=%s\n", fc.Kind)

	// Phase 3: Convert functionConfig to internal config
	cfg, input, err := fc.ExtractorConfig.ToConfig(os.TempDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error converting config: %v\n", err)
		rl.AddError(fmt.Sprintf("invalid configuration: %v", err))

		return WriteResourceList(writer, rl)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Config: namespace=%s, source=%s\n", cfg.Namespace, input)

	// Phase 4: Validate namespace
	if err := kube.ValidateNamespace(cfg.Namespace); err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Invalid namespace: %v\n", err)
		rl.AddError(fmt.Sprintf("invalid namespace: %v", err))

		return WriteResourceList(writer, rl)
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
		fmt.Fprintf(os.Stderr, "[KRM] Error resolving bundle source: %v\n", err)
		rl.AddError(fmt.Sprintf("failed to resolve bundle source: %v", err))

		return WriteResourceList(writer, rl)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Bundle source: %s\n", bundleImageOrDir)

	// Phase 6: Load bundle
	b, err := bundle.Load(ctx, bundleImageOrDir, cfg.Registry, cfg.TempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error loading bundle: %v\n", err)
		rl.AddError(fmt.Sprintf("failed to load bundle: %v", err))

		return WriteResourceList(writer, rl)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Bundle loaded successfully\n")

	// Phase 7: Extract manifests
	objects, err := extract.Manifests(b, cfg.Namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error extracting manifests: %v\n", err)
		rl.AddError(fmt.Sprintf("failed to extract manifests: %v", err))

		return WriteResourceList(writer, rl)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Extracted %d manifest objects\n", len(objects))

	// Phase 8: Convert to unstructured
	unstructuredObjects, err := kube.ConvertToUnstructured(objects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error converting to unstructured: %v\n", err)
		rl.AddError(fmt.Sprintf("failed to convert objects: %v", err))

		return WriteResourceList(writer, rl)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Converted to %d unstructured objects\n", len(unstructuredObjects))

	// Phase 9: Apply transformations
	unstructuredObjects, err = extract.ApplyTransformations(
		unstructuredObjects,
		cfg.Namespace,
		cfg.Include,
		cfg.Exclude,
		cfg.CertManager,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error applying transformations: %v\n", err)
		rl.AddError(fmt.Sprintf("failed to apply transformations: %v", err))

		return WriteResourceList(writer, rl)
	}
	fmt.Fprintf(os.Stderr, "[KRM] Applied transformations: %d objects\n", len(unstructuredObjects))

	// Phase 10: Convert to ResourceList and write output
	outputRL := ToResourceList(unstructuredObjects)
	if err := WriteResourceList(writer, outputRL); err != nil {
		fmt.Fprintf(os.Stderr, "[KRM] Error writing ResourceList: %v\n", err)
		return fmt.Errorf("failed to write ResourceList: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[KRM] Successfully completed with %d items\n", len(outputRL.Items))
	return nil
}
