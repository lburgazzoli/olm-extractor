package bundle

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/operator-framework/api/pkg/manifests"
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

// buildAuthenticator creates an authentication keychain based on the registry config.
// If explicit credentials are provided, uses them. Otherwise, uses the default keychain
// which automatically reads from ~/.docker/config.json and uses platform keychains.
func buildAuthenticator(config RegistryConfig) authn.Keychain {
	if config.Username != "" && config.Password != "" {
		// Use explicit credentials via a custom keychain
		return &staticKeychain{
			auth: &authn.Basic{
				Username: config.Username,
				Password: config.Password,
			},
		}
	}

	// Use default keychain:
	// - Reads from ~/.docker/config.json
	// - Supports Docker credential helpers (osxkeychain, gcr, ecr-login, etc.)
	// - Uses platform keychain (macOS Keychain, Windows Credential Manager, etc.)
	return authn.DefaultKeychain
}

// staticKeychain implements authn.Keychain for static credentials.
type staticKeychain struct {
	auth authn.Authenticator
}

// Resolve implements authn.Keychain.
func (s *staticKeychain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return s.auth, nil
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

	// Parse image reference
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return resource, fmt.Errorf("failed to parse image reference %q: %w", imageRef, err)
	}

	// Build remote options
	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(buildAuthenticator(config)),
		remote.WithContext(ctx),
	}

	// Configure transport for insecure connections
	if config.Insecure {
		remoteOpts = append(remoteOpts, remote.WithTransport(remote.DefaultTransport))
	}

	// Pull the image
	img, err := remote.Image(ref, remoteOpts...)
	if err != nil {
		if config.Username == "" && config.Password == "" {
			return resource, fmt.Errorf("failed to pull image %s: %w\nEnsure you have authenticated with 'docker login' or credentials are in ~/.docker/config.json", imageRef, err)
		}

		return resource, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	// Extract image to temporary directory
	if err := unpackImage(img, tmpDir); err != nil {
		return resource, fmt.Errorf("failed to extract image: %w", err)
	}

	return resource, nil
}

// unpackImage extracts all layers from a container image to a target directory.
func unpackImage(img v1.Image, targetDir string) error {
	// Get the filesystem layers
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	// Extract each layer
	for _, layer := range layers {
		if err := extractLayer(layer, targetDir); err != nil {
			return fmt.Errorf("failed to extract layer: %w", err)
		}
	}

	return nil
}

// extractLayer extracts a single image layer to the target directory.
func extractLayer(layer v1.Layer, targetDir string) error {
	// Get layer content (already uncompressed)
	rc, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get layer content: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	// Extract tar archive
	tr := tar.NewReader(rc)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if err := extractTarEntry(header, tr, targetDir); err != nil {
			return err
		}
	}

	return nil
}

// extractTarEntry extracts a single tar entry to the target directory.
func extractTarEntry(header *tar.Header, tr *tar.Reader, targetDir string) error {
	// Resolve target path
	//nolint:gosec // Path traversal is checked below
	target := filepath.Join(targetDir, header.Name)

	// Ensure we don't extract outside the target directory (path traversal protection)
	cleanTarget := filepath.Clean(target)
	cleanTargetDir := filepath.Clean(targetDir)
	if !strings.HasPrefix(cleanTarget, cleanTargetDir+string(os.PathSeparator)) &&
		cleanTarget != cleanTargetDir {
		return fmt.Errorf("illegal file path in tar: %s", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return extractDirectory(target)
	case tar.TypeReg:
		return extractFile(target, header, tr)
	case tar.TypeSymlink:
		return extractSymlink(target, header)
	default:
		return nil
	}
}

// extractDirectory creates a directory with secure permissions.
func extractDirectory(target string) error {
	const dirPerms = 0750
	if err := os.MkdirAll(target, dirPerms); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", target, err)
	}

	return nil
}

// extractFile creates a file and writes its contents.
func extractFile(target string, header *tar.Header, tr *tar.Reader) error {
	const dirPerms = 0750
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(target), dirPerms); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create file with mode from tar header
	//nolint:gosec // File path is validated in extractTarEntry
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", target, err)
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err := io.Copy(f, tr); err != nil {
		return fmt.Errorf("failed to write file %s: %w", target, err)
	}

	return nil
}

// extractSymlink creates a symbolic link.
func extractSymlink(target string, header *tar.Header) error {
	const dirPerms = 0750
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(target), dirPerms); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Remove existing file/link if present
	_ = os.Remove(target)

	// Create symlink
	if err := os.Symlink(header.Linkname, target); err != nil {
		return fmt.Errorf("failed to create symlink %s: %w", target, err)
	}

	return nil
}
