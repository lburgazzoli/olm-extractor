package registry

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Resource encapsulates all resources associated with an extracted container image.
// It manages temporary directories, providing a single cleanup method that is
// safe to call even on partially initialized resources.
type Resource struct {
	dir    string
	tmpDir string
}

// Dir returns the directory path containing the unpacked image.
func (r *Resource) Dir() string {
	return r.dir
}

// Cleanup releases all resources held by the Resource.
// It is idempotent and safe to call on zero-value or partially initialized resources.
func (r *Resource) Cleanup() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// Option configures image extraction behavior.
type Option func(*options)

// options holds all configuration for image extraction.
type options struct {
	insecure     bool
	username     string
	password     string
	tempDir      string
	pathPrefixes []string
}

// WithInsecure allows insecure connections to registries (HTTP or self-signed certificates).
func WithInsecure(insecure bool) Option {
	return func(o *options) {
		o.insecure = insecure
	}
}

// WithAuth configures explicit registry authentication credentials.
func WithAuth(username string, password string) Option {
	return func(o *options) {
		o.username = username
		o.password = password
	}
}

// WithTempDir specifies where temporary files should be created.
func WithTempDir(dir string) Option {
	return func(o *options) {
		o.tempDir = dir
	}
}

// WithPathPrefixes specifies which paths to extract from the image layers.
// Only layers containing files with these prefixes will be extracted.
// This significantly improves performance by skipping base OS layers.
func WithPathPrefixes(prefixes []string) Option {
	return func(o *options) {
		o.pathPrefixes = prefixes
	}
}

// ExtractImage pulls a container image and extracts it to a temporary directory.
// Returns a Resource containing all created resources.
// On error, returns a partial Resource that is safe to clean up.
func ExtractImage(ctx context.Context, imageRef string, opts ...Option) (Resource, error) {
	// Apply options
	cfg := options{}
	for _, opt := range opts {
		opt(&cfg)
	}

	resource := Resource{}

	// Create temporary directory for unpacked image
	tmpDir, err := os.MkdirTemp(cfg.tempDir, "image-extract-*")
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

	remoteOpts := []remote.Option{remote.WithContext(ctx)}

	if cfg.username != "" && cfg.password != "" {
		remoteOpts = append(remoteOpts, remote.WithAuth(&authn.Basic{
			Username: cfg.username,
			Password: cfg.password,
		}))
	} else {
		// DefaultKeychain reads from Docker config, credential helpers, and platform keychains
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	if cfg.insecure {
		remoteOpts = append(remoteOpts, remote.WithTransport(remote.DefaultTransport))
	}

	// Pull the image
	img, err := remote.Image(ref, remoteOpts...)
	if err != nil {
		if cfg.username == "" && cfg.password == "" {
			return resource, fmt.Errorf("failed to pull image %s: %w\nEnsure you have authenticated with 'docker login' or credentials are in ~/.docker/config.json", imageRef, err)
		}

		return resource, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	// Extract image to temporary directory
	if err := unpackImage(img, tmpDir, cfg.pathPrefixes); err != nil {
		return resource, fmt.Errorf("failed to extract image: %w", err)
	}

	return resource, nil
}
