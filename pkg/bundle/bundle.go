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

// BundleResource encapsulates all resources associated with a loaded bundle.
// It manages temporary directories and registry resources, providing a single
// cleanup method that is safe to call even on partially initialized resources.
type BundleResource struct {
	dir      string
	tmpDir   string
	cacheDir string
	registry *containerdregistry.Registry
}

// Cleanup releases all resources held by the BundleResource.
// It is idempotent and safe to call on zero-value or partially initialized resources.
func (br *BundleResource) Cleanup() {
	if br.registry != nil {
		_ = br.registry.Destroy()
	}
	if br.cacheDir != "" {
		_ = os.RemoveAll(br.cacheDir)
	}
	if br.tmpDir != "" {
		_ = os.RemoveAll(br.tmpDir)
	}
}

// Load loads an OLM bundle from a directory path or container image reference.
// For image references, temporary files are automatically cleaned up after loading.
// tempDir specifies where temporary files should be created (empty string uses system default).
func Load(input string, config RegistryConfig, tempDir string) (*manifests.Bundle, error) {
	resource, err := resolve(input, config, tempDir)
	defer resource.Cleanup()

	if err != nil {
		return nil, err
	}

	bundle, err := manifests.GetBundleFromDir(resource.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bundle from directory: %w", err)
	}

	return bundle, nil
}

// resolve resolves the input to a BundleResource.
// If input is a directory, returns a BundleResource with only dir set.
// If input is a container image reference, pulls and extracts it to a temp directory.
func resolve(input string, config RegistryConfig, tempDir string) (BundleResource, error) {
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		// Input is already a directory, return resource with only dir set
		// Zero values for other fields are safe for Cleanup()
		return BundleResource{dir: input}, nil
	}

	// Input is an image reference, extract it
	return extractImage(input, config, tempDir)
}

// extractImage pulls a container image and extracts it to a temporary directory.
// Returns a BundleResource containing all created resources.
// On error, returns a partial BundleResource that is safe to clean up.
func extractImage(imageRef string, config RegistryConfig, tempDir string) (BundleResource, error) {
	ctx := context.Background()
	resource := BundleResource{}

	// Create temporary directory for unpacked bundle
	tmpDir, err := os.MkdirTemp(tempDir, "bundle-extract-*")
	if err != nil {
		return resource, fmt.Errorf("failed to create temp directory: %w", err)
	}
	resource.tmpDir = tmpDir
	resource.dir = tmpDir

	// Create separate cache directory for containerd registry
	// This keeps registry cache separate from bundle files
	cacheDir, err := os.MkdirTemp(tempDir, "bundle-cache-*")
	if err != nil {
		return resource, fmt.Errorf("failed to create cache directory: %w", err)
	}
	resource.cacheDir = cacheDir

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
		return resource, fmt.Errorf("failed to create registry client: %w", err)
	}
	resource.registry = reg

	ref := image.SimpleReference(imageRef)
	if err := reg.Pull(ctx, ref); err != nil {
		if config.Username == "" && config.Password == "" {
			return resource, fmt.Errorf("failed to pull image %s: %w\nEnsure you have authenticated with 'docker login' or 'podman login'", imageRef, err)
		}

		return resource, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	if err := reg.Unpack(ctx, ref, tmpDir); err != nil {
		return resource, fmt.Errorf("failed to unpack image: %w", err)
	}

	return resource, nil
}
