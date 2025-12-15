package bundle

import (
	"context"
	"fmt"
	"os"

	"github.com/operator-framework/api/pkg/manifests"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
)

// RegistryConfig contains registry authentication and connection options.
type RegistryConfig struct {
	Insecure bool   `mapstructure:"registry-insecure"`
	AuthFile string `mapstructure:"registry-auth-file"`
	Username string `mapstructure:"registry-username"`
	Password string `mapstructure:"registry-password"`
}

// Load loads an OLM bundle from a directory path or container image reference.
// Returns the bundle, a cleanup function (may be nil), and any error.
// tempDir specifies where temporary files should be created (empty string uses system default).
func Load(input string, config RegistryConfig, tempDir string) (*manifests.Bundle, func(), error) {
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		bundle, err := manifests.GetBundleFromDir(input)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load bundle from directory: %w", err)
		}

		return bundle, nil, nil
	}

	return LoadFromImage(input, config, tempDir)
}

// LoadFromImage pulls a container image and extracts the OLM bundle from it.
// Returns the bundle, a cleanup function to remove temp files, and any error.
// tempDir specifies where temporary files should be created (empty string uses system default).
func LoadFromImage(imageRef string, config RegistryConfig, tempDir string) (*manifests.Bundle, func(), error) {
	ctx := context.Background()

	// If tempDir is empty, use system default (os.TempDir())
	tmpDir, err := os.MkdirTemp(tempDir, "bundle-extract-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	// Build registry options
	// Use tmpDir for cache to avoid creating cache in current directory
	registryOpts := []containerdregistry.RegistryOption{
		containerdregistry.SkipTLSVerify(config.Insecure),
		containerdregistry.WithCacheDir(tmpDir),
	}

	// Enable plaintext HTTP only for insecure connections (development/testing)
	if config.Insecure {
		registryOpts = append(registryOpts, containerdregistry.WithPlainHTTP(true))
	}

	// Use custom auth file if specified
	if config.AuthFile != "" {
		_ = os.Setenv("DOCKER_CONFIG", config.AuthFile)
	}

	reg, err := containerdregistry.NewRegistry(registryOpts...)
	if err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	defer func() { _ = reg.Destroy() }()

	ref := image.SimpleReference(imageRef)
	if err := reg.Pull(ctx, ref); err != nil {
		cleanup()

		if config.Username == "" && config.Password == "" {
			return nil, nil, fmt.Errorf("failed to pull image %s: %w\nEnsure you have authenticated with 'docker login' or 'podman login'", imageRef, err)
		}

		return nil, nil, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
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
