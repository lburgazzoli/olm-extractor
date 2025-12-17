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
	// Phase 1: Read ResourceList from stdin
	rl, err := ReadResourceList(reader)
	if err != nil {
		return fmt.Errorf("failed to read ResourceList: %w", err)
	}

	// Phase 2: Extract functionConfig
	fc, err := ExtractFunctionConfig(rl)
	if err != nil {
		return fmt.Errorf("failed to extract functionConfig: %w", err)
	}

	// Phase 3: Convert functionConfig to internal config
	cfg, input, err := fc.ExtractorConfig.ToConfig(os.TempDir())
	if err != nil {
		rl.AddError(fmt.Sprintf("invalid configuration: %v", err))

		return WriteResourceList(writer, rl)
	}

	// Phase 4: Validate namespace
	if err := kube.ValidateNamespace(cfg.Namespace); err != nil {
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
		rl.AddError(fmt.Sprintf("failed to resolve bundle source: %v", err))

		return WriteResourceList(writer, rl)
	}

	// Phase 6: Load bundle
	b, err := bundle.Load(ctx, bundleImageOrDir, cfg.Registry, cfg.TempDir)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to load bundle: %v", err))

		return WriteResourceList(writer, rl)
	}

	// Phase 7: Extract manifests
	objects, err := extract.Manifests(b, cfg.Namespace)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to extract manifests: %v", err))

		return WriteResourceList(writer, rl)
	}

	// Phase 8: Convert to unstructured
	unstructuredObjects, err := kube.ConvertToUnstructured(objects)
	if err != nil {
		rl.AddError(fmt.Sprintf("failed to convert objects: %v", err))

		return WriteResourceList(writer, rl)
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

		return WriteResourceList(writer, rl)
	}

	// Phase 10: Convert to ResourceList and write output
	outputRL := ToResourceList(unstructuredObjects)
	if err := WriteResourceList(writer, outputRL); err != nil {
		return fmt.Errorf("failed to write ResourceList: %w", err)
	}

	return nil
}
