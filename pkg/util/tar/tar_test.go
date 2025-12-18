package tar_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	tarutil "github.com/lburgazzoli/olm-extractor/pkg/util/tar"

	. "github.com/onsi/gomega"
)

func TestExtractDirectory(t *testing.T) {
	t.Run("creates directory with specified permissions", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "testdir")

		err := tarutil.ExtractDirectory(target, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Stat(target)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.IsDir()).To(BeTrue())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0750)))
	})

	t.Run("creates nested directories", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "a", "b", "c")

		err := tarutil.ExtractDirectory(target, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Stat(target)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.IsDir()).To(BeTrue())
	})

	t.Run("succeeds if directory already exists", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "existing")

		err := os.Mkdir(target, 0755)
		g.Expect(err).ToNot(HaveOccurred())

		err = tarutil.ExtractDirectory(target, 0750)
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestExtractFile(t *testing.T) {
	t.Run("creates file with content and permissions", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "testfile.txt")
		content := []byte("test content")

		header := &tar.Header{
			Name: "testfile.txt",
			Mode: 0644,
			Size: int64(len(content)),
		}
		tr := tar.NewReader(bytes.NewReader(content))

		err := tarutil.ExtractFile(target, header, tr, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		data, err := os.ReadFile(target)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(content))

		info, err := os.Stat(target)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0644)))
	})

	t.Run("creates parent directories if needed", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "a", "b", "testfile.txt")
		content := []byte("nested file")

		header := &tar.Header{
			Name: "a/b/testfile.txt",
			Mode: 0644,
			Size: int64(len(content)),
		}
		tr := tar.NewReader(bytes.NewReader(content))

		err := tarutil.ExtractFile(target, header, tr, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		data, err := os.ReadFile(target)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(content))
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "testfile.txt")

		err := os.WriteFile(target, []byte("old content"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		newContent := []byte("new content")
		header := &tar.Header{
			Name: "testfile.txt",
			Mode: 0644,
			Size: int64(len(newContent)),
		}
		tr := tar.NewReader(bytes.NewReader(newContent))

		err = tarutil.ExtractFile(target, header, tr, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		data, err := os.ReadFile(target)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(newContent))
	})

	t.Run("creates executable file", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "script.sh")
		content := []byte("#!/bin/bash\necho hello")

		header := &tar.Header{
			Name: "script.sh",
			Mode: 0755,
			Size: int64(len(content)),
		}
		tr := tar.NewReader(bytes.NewReader(content))

		err := tarutil.ExtractFile(target, header, tr, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Stat(target)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0755)))
	})
}

func TestExtractSymlink(t *testing.T) {
	t.Run("creates symbolic link", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		targetFile := filepath.Join(tmpDir, "target.txt")
		linkPath := filepath.Join(tmpDir, "link.txt")

		err := os.WriteFile(targetFile, []byte("target content"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		header := &tar.Header{
			Name:     "link.txt",
			Linkname: "target.txt",
		}

		err = tarutil.ExtractSymlink(linkPath, header, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Lstat(linkPath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode()&os.ModeSymlink).To(Equal(os.ModeSymlink))

		linkTarget, err := os.Readlink(linkPath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(linkTarget).To(Equal("target.txt"))
	})

	t.Run("creates parent directories if needed", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		linkPath := filepath.Join(tmpDir, "a", "b", "link.txt")

		header := &tar.Header{
			Name:     "a/b/link.txt",
			Linkname: "../../target.txt",
		}

		err := tarutil.ExtractSymlink(linkPath, header, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Lstat(linkPath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode()&os.ModeSymlink).To(Equal(os.ModeSymlink))
	})

	t.Run("replaces existing file with symlink", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		linkPath := filepath.Join(tmpDir, "link.txt")

		err := os.WriteFile(linkPath, []byte("existing file"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		header := &tar.Header{
			Name:     "link.txt",
			Linkname: "target.txt",
		}

		err = tarutil.ExtractSymlink(linkPath, header, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Lstat(linkPath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode()&os.ModeSymlink).To(Equal(os.ModeSymlink))
	})
}

func TestExtractEntry(t *testing.T) {
	t.Run("extracts directory entry", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		header := &tar.Header{
			Name:     "testdir/",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		}

		err := tarutil.ExtractEntry(header, nil, tmpDir, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Stat(filepath.Join(tmpDir, "testdir"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.IsDir()).To(BeTrue())
	})

	t.Run("extracts file entry", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		content := []byte("file content")

		header := &tar.Header{
			Name:     "testfile.txt",
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     int64(len(content)),
		}
		tr := tar.NewReader(bytes.NewReader(content))

		err := tarutil.ExtractEntry(header, tr, tmpDir, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		data, err := os.ReadFile(filepath.Join(tmpDir, "testfile.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(content))
	})

	t.Run("extracts symlink entry", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		header := &tar.Header{
			Name:     "link.txt",
			Typeflag: tar.TypeSymlink,
			Linkname: "target.txt",
		}

		err := tarutil.ExtractEntry(header, nil, tmpDir, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		info, err := os.Lstat(filepath.Join(tmpDir, "link.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode()&os.ModeSymlink).To(Equal(os.ModeSymlink))
	})

	t.Run("ignores unsupported entry types", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		header := &tar.Header{
			Name:     "device",
			Typeflag: tar.TypeBlock,
		}

		err := tarutil.ExtractEntry(header, nil, tmpDir, 0750)

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("prevents path traversal with ..", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		header := &tar.Header{
			Name:     "../../../etc/passwd",
			Typeflag: tar.TypeReg,
			Mode:     0644,
		}

		err := tarutil.ExtractEntry(header, nil, tmpDir, 0750)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("illegal file path"))
	})

	t.Run("prevents path traversal with absolute path", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		header := &tar.Header{
			Name:     "/etc/passwd",
			Typeflag: tar.TypeReg,
			Mode:     0644,
		}

		err := tarutil.ExtractEntry(header, nil, tmpDir, 0750)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("illegal file path"))
	})

	t.Run("allows valid nested paths", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		content := []byte("nested content")

		header := &tar.Header{
			Name:     "a/b/c/file.txt",
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     int64(len(content)),
		}
		tr := tar.NewReader(bytes.NewReader(content))

		err := tarutil.ExtractEntry(header, tr, tmpDir, 0750)

		g.Expect(err).ToNot(HaveOccurred())
		data, err := os.ReadFile(filepath.Join(tmpDir, "a", "b", "c", "file.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(content))
	})
}

func TestExtractAll(t *testing.T) {
	t.Run("extracts complete tar archive", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		// Create a tar archive in memory
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)

		// Add directory
		err := tw.WriteHeader(&tar.Header{
			Name:     "testdir/",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Add file
		fileContent := []byte("test content")
		err = tw.WriteHeader(&tar.Header{
			Name:     "testdir/file.txt",
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     int64(len(fileContent)),
		})
		g.Expect(err).ToNot(HaveOccurred())
		_, err = tw.Write(fileContent)
		g.Expect(err).ToNot(HaveOccurred())

		// Add symlink
		err = tw.WriteHeader(&tar.Header{
			Name:     "testdir/link.txt",
			Typeflag: tar.TypeSymlink,
			Linkname: "file.txt",
		})
		g.Expect(err).ToNot(HaveOccurred())

		err = tw.Close()
		g.Expect(err).ToNot(HaveOccurred())

		// Extract using ExtractAll
		err = tarutil.ExtractAll(&buf, tmpDir, 0750)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify extraction
		dirInfo, err := os.Stat(filepath.Join(tmpDir, "testdir"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(dirInfo.IsDir()).To(BeTrue())

		fileData, err := os.ReadFile(filepath.Join(tmpDir, "testdir", "file.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(fileData).To(Equal(fileContent))

		linkInfo, err := os.Lstat(filepath.Join(tmpDir, "testdir", "link.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(linkInfo.Mode()&os.ModeSymlink).To(Equal(os.ModeSymlink))
	})

	t.Run("handles empty tar archive", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		err := tw.Close()
		g.Expect(err).ToNot(HaveOccurred())

		err = tarutil.ExtractAll(&buf, tmpDir, 0750)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("returns error for invalid tar data", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		invalidData := bytes.NewReader([]byte("not a tar archive"))

		err := tarutil.ExtractAll(invalidData, tmpDir, 0750)
		g.Expect(err).To(HaveOccurred())
	})
}

func TestExtractEntryIntegration(t *testing.T) {
	t.Run("extracts complete tar archive", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		// Create a tar archive in memory
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)

		// Add directory
		err := tw.WriteHeader(&tar.Header{
			Name:     "dir/",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Add file
		fileContent := []byte("hello world")
		err = tw.WriteHeader(&tar.Header{
			Name:     "dir/file.txt",
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     int64(len(fileContent)),
		})
		g.Expect(err).ToNot(HaveOccurred())
		_, err = tw.Write(fileContent)
		g.Expect(err).ToNot(HaveOccurred())

		// Add symlink
		err = tw.WriteHeader(&tar.Header{
			Name:     "dir/link.txt",
			Typeflag: tar.TypeSymlink,
			Linkname: "file.txt",
		})
		g.Expect(err).ToNot(HaveOccurred())

		err = tw.Close()
		g.Expect(err).ToNot(HaveOccurred())

		// Extract the archive
		tr := tar.NewReader(&buf)
		for {
			header, err := tr.Next()
			if err != nil {
				break
			}

			err = tarutil.ExtractEntry(header, tr, tmpDir, 0750)
			g.Expect(err).ToNot(HaveOccurred())
		}

		// Verify extraction
		dirInfo, err := os.Stat(filepath.Join(tmpDir, "dir"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(dirInfo.IsDir()).To(BeTrue())

		fileData, err := os.ReadFile(filepath.Join(tmpDir, "dir", "file.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(fileData).To(Equal(fileContent))

		linkInfo, err := os.Lstat(filepath.Join(tmpDir, "dir", "link.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(linkInfo.Mode()&os.ModeSymlink).To(Equal(os.ModeSymlink))
	})
}

