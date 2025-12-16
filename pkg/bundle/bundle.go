package bundle

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

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

// Dir returns the directory path containing the unpacked bundle.
func (br *BundleResource) Dir() string {
	return br.dir
}

// resolveDockerConfigDir resolves the Docker config directory for DOCKER_CONFIG.
// Returns the directory path that should be set as DOCKER_CONFIG environment variable.
// Returns empty string if no auth file is specified (use default behavior).
func resolveDockerConfigDir(authFile string) (string, error) {
	if authFile == "" {
		// No auth file specified, let containerd use default
		// Try to set default to ~/.docker
		usr, err := user.Current()
		if err != nil {
			// Can't get home dir, return empty to use containerd's default
			return "", nil
		}

		return filepath.Join(usr.HomeDir, ".docker"), nil
	}

	// Expand ~ to home directory
	if len(authFile) >= 2 && authFile[:2] == "~/" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("failed to get current user: %w", err)
		}
		authFile = filepath.Join(usr.HomeDir, authFile[2:])
	}

	// Check if it's a file or directory
	info, err := os.Stat(authFile)
	if err != nil {
		return "", fmt.Errorf("auth file/directory does not exist: %w", err)
	}

	if info.IsDir() {
		// It's a directory, use it directly
		return authFile, nil
	}

	// It's a file (e.g., config.json), DOCKER_CONFIG must point to the directory
	return filepath.Dir(authFile), nil
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
	return ExtractImage(input, config, tempDir)
}

// ExtractImage pulls a container image and extracts it to a temporary directory.
// Returns a BundleResource containing all created resources.
// On error, returns a partial BundleResource that is safe to clean up.
// This is exported for use by the catalog package.
func ExtractImage(imageRef string, config RegistryConfig, tempDir string) (BundleResource, error) {
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

	// Set DOCKER_CONFIG environment variable for authentication
	// It must point to a directory containing config.json, not the file itself
	dockerConfigDir, err := resolveDockerConfigDir(config.AuthFile)
	if err != nil {
		return resource, fmt.Errorf("failed to resolve Docker config directory: %w", err)
	}
	if dockerConfigDir != "" {
		_ = os.Setenv("DOCKER_CONFIG", dockerConfigDir)
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
