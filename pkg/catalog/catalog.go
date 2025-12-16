package catalog

import (
	"context"
	"fmt"
	"os"

	"github.com/operator-framework/operator-registry/alpha/declcfg"

	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
)

// Config holds catalog resolution configuration.
type Config struct {
	CatalogImage string
	PackageName  string
	Version      string // Optional
	Channel      string // Optional, defaults to package's defaultChannel
}

// ResolveBundleImage resolves a package reference to a bundle image reference.
// It pulls the catalog image, parses the FBC format, finds the requested package/version,
// and returns the bundle image reference.
func ResolveBundleImage(config Config, registryConfig bundle.RegistryConfig, tempDir string) (string, error) {
	// Pull and extract catalog image
	resource, err := bundle.ExtractImage(config.CatalogImage, registryConfig, tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to extract catalog image: %w", err)
	}
	defer resource.Cleanup()

	// Load FBC from extracted directory
	catalog, err := loadCatalog(resource.Dir())
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
func loadCatalog(dir string) (*declcfg.DeclarativeConfig, error) {
	// Try loading from /configs subdirectory first (common for catalog images)
	configsDir := dir + "/configs"
	if info, err := os.Stat(configsDir); err == nil && info.IsDir() {
		cfg, err := declcfg.LoadFS(context.Background(), os.DirFS(configsDir))
		if err != nil {
			return nil, fmt.Errorf("failed to parse catalog from configs directory: %w", err)
		}

		return cfg, nil
	}

	// Fallback to root directory
	cfg, err := declcfg.LoadFS(context.Background(), os.DirFS(dir))
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalog: %w", err)
	}

	return cfg, nil
}

// findPackage finds a package by name in the catalog.
func findPackage(cfg *declcfg.DeclarativeConfig, name string) (*declcfg.Package, error) {
	for i := range cfg.Packages {
		if cfg.Packages[i].Name == name {
			return &cfg.Packages[i], nil
		}
	}

	// List available packages for helpful error message
	available := make([]string, 0, len(cfg.Packages))
	for i := range cfg.Packages {
		available = append(available, cfg.Packages[i].Name)
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("package %q not found in catalog (catalog contains no packages)", name)
	}

	return nil, fmt.Errorf("package %q not found in catalog (available packages: %v)", name, available)
}

// findChannel finds a channel by name for a package in the catalog.
func findChannel(cfg *declcfg.DeclarativeConfig, packageName string, channelName string) (*declcfg.Channel, error) {
	for i := range cfg.Channels {
		if cfg.Channels[i].Package == packageName && cfg.Channels[i].Name == channelName {
			return &cfg.Channels[i], nil
		}
	}

	// List available channels for helpful error message
	var available []string
	for i := range cfg.Channels {
		if cfg.Channels[i].Package == packageName {
			available = append(available, cfg.Channels[i].Name)
		}
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("channel %q not found for package %q (package has no channels)", channelName, packageName)
	}

	return nil, fmt.Errorf("channel %q not found for package %q (available channels: %v)", channelName, packageName, available)
}

// findBundleInChannel finds a bundle in a channel by version or returns the latest.
func findBundleInChannel(channel *declcfg.Channel, version string) (string, error) {
	if version != "" {
		// Find specific version
		for i := range channel.Entries {
			if channel.Entries[i].Name == version {
				return channel.Entries[i].Name, nil
			}
		}

		// List available versions for helpful error message
		var available []string
		for i := range channel.Entries {
			available = append(available, channel.Entries[i].Name)
		}

		return "", fmt.Errorf("version %q not found in channel %q (available versions: %v)", version, channel.Name, available)
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
	for i := range cfg.Bundles {
		if cfg.Bundles[i].Name == bundleName {
			// The bundle image is stored in the bundle's Image field
			if cfg.Bundles[i].Image == "" {
				return "", fmt.Errorf("bundle %q has no image reference", bundleName)
			}

			return cfg.Bundles[i].Image, nil
		}
	}

	return "", fmt.Errorf("bundle %q not found in catalog", bundleName)
}
