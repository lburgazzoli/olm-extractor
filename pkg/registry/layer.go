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

	// If no path prefixes specified, extract all layers (backward compatibility)
	if len(pathPrefixes) == 0 {
		for _, layer := range layers {
			if err := extractLayer(layer, targetDir); err != nil {
				return fmt.Errorf("failed to extract layer: %w", err)
			}
		}

		return nil
	}

	// Scan layers in reverse order (most recent first)
	extractedCount := 0
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]

		// Check if this layer contains relevant content
		hasContent, err := layerContainsRelevantPaths(layer, pathPrefixes)
		if err != nil {
			return fmt.Errorf("failed to inspect layer: %w", err)
		}

		if hasContent {
			if err := extractLayer(layer, targetDir); err != nil {
				return fmt.Errorf("failed to extract layer: %w", err)
			}
			extractedCount++

			// Check if we have all required content
			if hasAllRequiredContent(targetDir, pathPrefixes) {
				return nil
			}
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
