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

// ResolveBundle resolves the input to a bundle directory path.
// If input is a directory, returns it directly.
// If input is a container image reference, pulls and extracts it to a temp directory.
// Returns: directory path, cleanup function (may be nil), error.
func ResolveBundle(input string, config RegistryConfig, tempDir string) (string, func(), error) {
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		// Input is already a directory, no cleanup needed
		return input, nil, nil
	}

	// Input is an image reference, extract it
	return extractImage(input, config, tempDir)
}

// Load loads an OLM bundle from a directory path or container image reference.
// Returns the bundle, a cleanup function (may be nil), and any error.
// tempDir specifies where temporary files should be created (empty string uses system default).
func Load(input string, config RegistryConfig, tempDir string) (*manifests.Bundle, func(), error) {
	dir, cleanup, err := ResolveBundle(input, config, tempDir)
	if err != nil {
		return nil, cleanup, err
	}

	bundle, err := manifests.GetBundleFromDir(dir)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}

		return nil, nil, fmt.Errorf("failed to load bundle from directory: %w", err)
	}

	return bundle, cleanup, nil
}

// extractImage pulls a container image and extracts it to a temporary directory.
// Returns: directory path, cleanup function, error.
func extractImage(imageRef string, config RegistryConfig, tempDir string) (string, func(), error) {
	ctx := context.Background()

	// If tempDir is empty, use system default (os.TempDir())
	tmpDir, err := os.MkdirTemp(tempDir, "bundle-extract-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
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

		return "", nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	defer func() { _ = reg.Destroy() }()

	ref := image.SimpleReference(imageRef)
	if err := reg.Pull(ctx, ref); err != nil {
		cleanup()

		if config.Username == "" && config.Password == "" {
			return "", nil, fmt.Errorf("failed to pull image %s: %w\nEnsure you have authenticated with 'docker login' or 'podman login'", imageRef, err)
		}

		return "", nil, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	if err := reg.Unpack(ctx, ref, tmpDir); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("failed to unpack image: %w", err)
	}

	return tmpDir, cleanup, nil
}
