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
// For image references, temporary files are automatically cleaned up after loading.
// tempDir specifies where temporary files should be created (empty string uses system default).
func Load(input string, config RegistryConfig, tempDir string) (*manifests.Bundle, error) {
	dir, cleanup, err := resolve(input, config, tempDir)
	if err != nil {
		return nil, err
	}

	// Ensure cleanup runs regardless of success or failure
	defer cleanup()

	bundle, err := manifests.GetBundleFromDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bundle from directory: %w", err)
	}

	return bundle, nil
}

// resolve resolves the input to a bundle directory path.
// If input is a directory, returns it directly.
// If input is a container image reference, pulls and extracts it to a temp directory.
// Returns: directory path, cleanup function (always non-nil), error.
func resolve(input string, config RegistryConfig, tempDir string) (string, func(), error) {
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		// Input is already a directory, return no-op cleanup
		return input, func() {}, nil
	}

	// Input is an image reference, extract it
	return extractImage(input, config, tempDir)
}

// extractImage pulls a container image and extracts it to a temporary directory.
// Returns: directory path, cleanup function, error.
func extractImage(imageRef string, config RegistryConfig, tempDir string) (string, func(), error) {
	ctx := context.Background()

	// Create temporary directory for unpacked bundle
	tmpDir, err := os.MkdirTemp(tempDir, "bundle-extract-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create separate cache directory for containerd registry
	// This keeps registry cache separate from bundle files
	cacheDir, err := os.MkdirTemp(tempDir, "bundle-cache-*")
	if err != nil {
		_ = os.RemoveAll(tmpDir)

		return "", func() {}, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Cleanup function removes both directories
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
		_ = os.RemoveAll(cacheDir)
	}

	// Build registry options
	registryOpts := []containerdregistry.RegistryOption{
		containerdregistry.SkipTLSVerify(config.Insecure),
		containerdregistry.WithCacheDir(cacheDir),
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
		return "", cleanup, fmt.Errorf("failed to create registry client: %w", err)
	}

	// Ensure registry is destroyed on function exit
	defer func() { _ = reg.Destroy() }()

	ref := image.SimpleReference(imageRef)
	if err := reg.Pull(ctx, ref); err != nil {
		if config.Username == "" && config.Password == "" {
			return "", cleanup, fmt.Errorf("failed to pull image %s: %w\nEnsure you have authenticated with 'docker login' or 'podman login'", imageRef, err)
		}

		return "", cleanup, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	if err := reg.Unpack(ctx, ref, tmpDir); err != nil {
		return "", cleanup, fmt.Errorf("failed to unpack image: %w", err)
	}

	return tmpDir, cleanup, nil
}
