package bundle

import (
	"fmt"
	"os"

	"github.com/operator-framework/api/pkg/manifests"

	"github.com/lburgazzoli/olm-extractor/pkg/registry"
)

// RegistryConfig contains registry authentication and connection options.
type RegistryConfig struct {
	Insecure bool   `mapstructure:"registry-insecure"`
	Username string `mapstructure:"registry-username"`
	Password string `mapstructure:"registry-password"`
}

// BundleResource encapsulates all resources associated with a loaded bundle.
// It manages temporary directories, providing a single cleanup method that is
// safe to call even on partially initialized resources.
type BundleResource struct {
	dir    string
	tmpDir string
}

// Dir returns the directory path containing the unpacked bundle.
func (br *BundleResource) Dir() string {
	return br.dir
}

// Cleanup releases all resources held by the BundleResource.
// It is idempotent and safe to call on zero-value or partially initialized resources.
func (br *BundleResource) Cleanup() {
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

// getBundlePathPrefixes returns the default path prefixes for OLM bundles.
func getBundlePathPrefixes() []string {
	return []string{"/manifests/", "/metadata/"}
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

	// Input is an image reference, extract it with bundle-specific path prefixes
	return ExtractImage(input, config, tempDir, getBundlePathPrefixes())
}

// ExtractImage pulls a container image and extracts it to a temporary directory.
// Returns a BundleResource containing all created resources.
// On error, returns a partial BundleResource that is safe to clean up.
// This is exported for use by the catalog package.
func ExtractImage(
	imageRef string,
	config RegistryConfig,
	tempDir string,
	pathPrefixes []string,
) (BundleResource, error) {
	// Build registry options
	opts := []registry.Option{
		registry.WithTempDir(tempDir),
		registry.WithPathPrefixes(pathPrefixes),
	}

	if config.Insecure {
		opts = append(opts, registry.WithInsecure(true))
	}

	if config.Username != "" && config.Password != "" {
		opts = append(opts, registry.WithAuth(config.Username, config.Password))
	}

	// Extract image using registry package
	resource, err := registry.ExtractImage(imageRef, opts...)
	if err != nil {
		return BundleResource{}, fmt.Errorf("failed to extract image: %w", err)
	}

	// Convert registry.Resource to BundleResource
	return BundleResource{
		dir:    resource.Dir(),
		tmpDir: resource.Dir(), // Both point to the same temp dir
	}, nil
}
