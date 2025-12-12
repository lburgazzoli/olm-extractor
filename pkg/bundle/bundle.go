package bundle

import (
	"context"
	"fmt"
	"os"

	"github.com/operator-framework/api/pkg/manifests"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
)

// Load loads an OLM bundle from a directory path or container image reference.
// Returns the bundle, a cleanup function (may be nil), and any error.
func Load(input string, insecure bool) (*manifests.Bundle, func(), error) {
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		bundle, err := manifests.GetBundleFromDir(input)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load bundle from directory: %w", err)
		}

		return bundle, nil, nil
	}

	return LoadFromImage(input, insecure)
}

// LoadFromImage pulls a container image and extracts the OLM bundle from it.
// Returns the bundle, a cleanup function to remove temp files, and any error.
func LoadFromImage(imageRef string, insecure bool) (*manifests.Bundle, func(), error) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "bundle-extract-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	reg, err := containerdregistry.NewRegistry(containerdregistry.SkipTLSVerify(insecure))
	if err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	defer func() { _ = reg.Destroy() }()

	ref := image.SimpleReference(imageRef)
	if err := reg.Pull(ctx, ref); err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("failed to pull image %s: %w\nEnsure you have authenticated with 'docker login' or 'podman login'", imageRef, err)
	}

	if err := reg.Unpack(ctx, ref, tmpDir); err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("failed to unpack image: %w", err)
	}

	bundle, err := manifests.GetBundleFromDir(tmpDir)
	if err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("failed to load bundle from extracted image: %w", err)
	}

	return bundle, cleanup, nil
}
