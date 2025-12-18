package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractAll extracts all entries from a tar archive to the target directory.
// It reads from the provided io.Reader and extracts each entry using ExtractEntry.
func ExtractAll(reader io.Reader, targetDir string, dirPerms os.FileMode) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if err := ExtractEntry(header, tr, targetDir, dirPerms); err != nil {
			return err
		}
	}

	return nil
}

// ExtractEntry extracts a single tar entry to the target directory.
// It validates that the extraction path does not escape the target directory (path traversal protection).
func ExtractEntry(header *tar.Header, tr *tar.Reader, targetDir string, dirPerms os.FileMode) error {
	// Check for absolute paths in tar entry name (path traversal attempt)
	if filepath.IsAbs(header.Name) {
		return fmt.Errorf("illegal file path in tar: %s", header.Name)
	}

	// Resolve target path
	//nolint:gosec // Path traversal is checked above and below
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
		return ExtractDirectory(target, dirPerms)
	case tar.TypeReg:
		return ExtractFile(target, header, tr, dirPerms)
	case tar.TypeSymlink:
		return ExtractSymlink(target, header, dirPerms)
	default:
		return nil
	}
}

// ExtractDirectory creates a directory with the specified permissions.
func ExtractDirectory(target string, perms os.FileMode) error {
	if err := os.MkdirAll(target, perms); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", target, err)
	}

	return nil
}

// ExtractFile creates a file and writes its contents from the tar reader.
// The file is created with the mode from the tar header.
// Parent directories are created with the specified dirPerms if needed.
func ExtractFile(target string, header *tar.Header, tr *tar.Reader, dirPerms os.FileMode) error {
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(target), dirPerms); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create file with mode from tar header
	//nolint:gosec // File path is validated in ExtractEntry
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

// ExtractSymlink creates a symbolic link.
// Parent directories are created with the specified dirPerms if needed.
func ExtractSymlink(target string, header *tar.Header, dirPerms os.FileMode) error {
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
