package catalog

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"

	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/util/slices"
)

var catalogPathPrefixes = []string{"/configs/"} //nolint:gochecknoglobals

// Config holds catalog resolution configuration.
type Config struct {
	CatalogImage string
	PackageName  string
	Version      string // Optional
	Channel      string // Optional, defaults to package's defaultChannel
}

// ResolveBundleSource determines the bundle source from input and configuration.
// In catalog mode (catalogImage is non-empty), resolves package[:version] to a bundle image.
// In direct mode (catalogImage is empty), returns input as-is (directory path or image reference).
func ResolveBundleSource(
	ctx context.Context,
	input string,
	catalogImage string,
	channel string,
	registryConfig bundle.RegistryConfig,
	tempDir string,
) (string, error) {
	if catalogImage != "" {
		packageName, packageVersion := parsePackageReference(input)

		cfg := Config{
			CatalogImage: catalogImage,
			PackageName:  packageName,
			Version:      packageVersion,
			Channel:      channel,
		}

		bundleImage, err := ResolveBundleImage(ctx, cfg, registryConfig, tempDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve bundle from catalog: %w", err)
		}

		return bundleImage, nil
	}

	return input, nil
}

// parsePackageReference parses a package reference in the format package[:version].
// Returns the package name and optionally the version.
//
//nolint:nonamedreturns // Named returns required to avoid confusing-results linter error
func parsePackageReference(ref string) (pkgName string, pkgVersion string) {
	const packageRefParts = 2 // Number of parts when splitting package:version

	parts := strings.SplitN(ref, ":", packageRefParts)
	if len(parts) == packageRefParts {
		return parts[0], parts[1]
	}

	return parts[0], ""
}

// ResolveBundleImage resolves a package reference to a bundle image reference.
// It pulls the catalog image, parses the FBC format, finds the requested package/version,
// and returns the bundle image reference.
func ResolveBundleImage(ctx context.Context, config Config, registryConfig bundle.RegistryConfig, tempDir string) (string, error) {
	// Pull and extract catalog image with catalog-specific path prefixes
	bundleResource, err := bundle.ExtractImage(ctx, config.CatalogImage, registryConfig, tempDir, catalogPathPrefixes)
	if err != nil {
		return "", fmt.Errorf("failed to extract catalog image: %w", err)
	}
	defer bundleResource.Cleanup()

	// Load FBC from extracted directory
	catalog, err := loadCatalog(ctx, bundleResource.Dir())
	if err != nil {
		return "", fmt.Errorf("failed to load catalog: %w", err)
	}

	// Find package by name
	pkg, err := findPackage(catalog, config.PackageName)
	if err != nil {
		return "", err
	}

	// Determine channel
	channelName := config.Channel
	if channelName == "" {
		if pkg.DefaultChannel == "" {
			return "", fmt.Errorf("package %q has no defaultChannel and --channel was not specified", config.PackageName)
		}
		channelName = pkg.DefaultChannel
	}

	// Find channel
	channel, err := findChannel(catalog, config.PackageName, channelName)
	if err != nil {
		return "", err
	}

	// Find bundle entry
	bundleName, err := findBundleInChannel(channel, config.Version)
	if err != nil {
		return "", err
	}

	// Extract bundle image reference
	bundleImage, err := extractBundleImage(catalog, bundleName)
	if err != nil {
		return "", err
	}

	return bundleImage, nil
}

// loadCatalog loads the FBC declarative config from a directory.
// Catalog images typically have FBC files in a `/configs` subdirectory.
func loadCatalog(ctx context.Context, dir string) (*declcfg.DeclarativeConfig, error) {
	// Try loading from /configs subdirectory first (common for catalog images)
	configsDir := dir + "/configs"
	if info, err := os.Stat(configsDir); err == nil && info.IsDir() {
		cfg, err := declcfg.LoadFS(ctx, os.DirFS(configsDir))
		if err != nil {
			return nil, fmt.Errorf("failed to parse catalog from configs directory: %w", err)
		}

		return cfg, nil
	}

	// Fallback to root directory
	cfg, err := declcfg.LoadFS(ctx, os.DirFS(dir))
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalog: %w", err)
	}

	return cfg, nil
}

// findPackage finds a package by name in the catalog.
func findPackage(cfg *declcfg.DeclarativeConfig, name string) (*declcfg.Package, error) {
	pkg, found := slices.Find(cfg.Packages, func(p declcfg.Package) bool {
		return p.Name == name
	})
	if !found {
		return nil, fmt.Errorf("package %q not found in catalog", name)
	}

	return &pkg, nil
}

// findChannel finds a channel by name for a package in the catalog.
func findChannel(cfg *declcfg.DeclarativeConfig, packageName string, channelName string) (*declcfg.Channel, error) {
	// First, verify the package exists
	packageExists := slices.Any(cfg.Packages, func(p declcfg.Package) bool {
		return p.Name == packageName
	})
	if !packageExists {
		return nil, fmt.Errorf("package %q not found in catalog", packageName)
	}

	// Then, search for the channel
	ch, found := slices.Find(cfg.Channels, func(c declcfg.Channel) bool {
		return c.Package == packageName && c.Name == channelName
	})
	if !found {
		return nil, fmt.Errorf("channel %q not found for package %q", channelName, packageName)
	}

	return &ch, nil
}

// findBundleInChannel finds a bundle in a channel by version or returns the latest.
func findBundleInChannel(channel *declcfg.Channel, version string) (string, error) {
	if version != "" {
		// Find specific version
		entry, found := slices.Find(channel.Entries, func(e declcfg.ChannelEntry) bool {
			return e.Name == version
		})
		if !found {
			return "", fmt.Errorf("version %q not found in channel %q", version, channel.Name)
		}

		return entry.Name, nil
	}

	// Return the head of the channel (latest version)
	if len(channel.Entries) == 0 {
		return "", fmt.Errorf("channel %q has no entries", channel.Name)
	}

	// The channel head is typically the first entry or explicitly marked
	// In FBC, the head is usually the entry without a replaces field pointing to it
	// For simplicity, we'll use the first entry as it's typically the latest
	return channel.Entries[0].Name, nil
}

// extractBundleImage extracts the bundle image reference from a bundle's properties.
func extractBundleImage(cfg *declcfg.DeclarativeConfig, bundleName string) (string, error) {
	bundleEntry, found := slices.Find(cfg.Bundles, func(b declcfg.Bundle) bool {
		return b.Name == bundleName
	})
	if !found {
		return "", fmt.Errorf("bundle %q not found in catalog", bundleName)
	}

	// The bundle image is stored in the bundle's Image field
	if bundleEntry.Image == "" {
		return "", fmt.Errorf("bundle %q has no image reference", bundleName)
	}

	return bundleEntry.Image, nil
}
