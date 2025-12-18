package registry

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/lburgazzoli/olm-extractor/pkg/util/slices"
	tarutil "github.com/lburgazzoli/olm-extractor/pkg/util/tar"
)

// hasAllRequiredContent checks if the extracted directory contains all required paths.
// Returns true if at least one file/directory exists for each required path prefix.
func hasAllRequiredContent(dir string, pathPrefixes []string) bool {
	return slices.All(pathPrefixes, func(prefix string) bool {
		// Remove leading slash for filepath.Join
		cleanPrefix := strings.TrimPrefix(prefix, "/")
		path := filepath.Join(dir, cleanPrefix)

		// Check if path exists (file or directory)
		_, err := os.Stat(path)

		return err == nil
	})
}

// layerContainsRelevantPaths checks if a layer contains any files matching the given path prefixes.
// This function performs a quick scan of tar headers without extracting file contents.
func layerContainsRelevantPaths(layer v1.Layer, pathPrefixes []string) (bool, error) {
	// Get layer content (already uncompressed)
	rc, err := layer.Uncompressed()
	if err != nil {
		return false, fmt.Errorf("failed to get layer content: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	// Scan tar headers
	tr := tar.NewReader(rc)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("failed to read tar header: %w", err)
		}

		// Check if this file matches any of the path prefixes
		// Handle both with and without leading slash
		for _, prefix := range pathPrefixes {
			// Try with leading slash
			if strings.HasPrefix(header.Name, prefix) {
				return true, nil
			}
			// Try without leading slash
			cleanPrefix := strings.TrimPrefix(prefix, "/")
			if strings.HasPrefix(header.Name, cleanPrefix) {
				return true, nil
			}
		}
	}

	return false, nil
}

// unpackImage extracts layers from a container image to a target directory.
// If pathPrefixes is provided, only layers containing files with those prefixes are extracted.
// Layers are scanned in reverse order (most recent first) for efficiency.
func unpackImage(img v1.Image, targetDir string, pathPrefixes []string) error {
	// Get the filesystem layers
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	// Scan layers in reverse order (most recent first)
	extractedCount := 0
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]

		hasContent, err := layerContainsRelevantPaths(layer, pathPrefixes)
		if err != nil {
			return fmt.Errorf("failed to inspect layer: %w", err)
		}

		if !hasContent {
			continue
		}

		if err := extractLayer(layer, targetDir); err != nil {
			return fmt.Errorf("failed to extract layer: %w", err)
		}

		extractedCount++

		if hasAllRequiredContent(targetDir, pathPrefixes) {
			break
		}
	}

	// If we didn't find any relevant content, something is wrong
	if extractedCount == 0 {
		return fmt.Errorf("no layers found containing paths: %v", pathPrefixes)
	}

	return nil
}

// extractLayer extracts a single image layer to the target directory.
func extractLayer(layer v1.Layer, targetDir string) error {
	const dirPerms = 0750

	// Get layer content (already uncompressed)
	rc, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get layer content: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	return tarutil.ExtractAll(rc, targetDir, dirPerms)
}
