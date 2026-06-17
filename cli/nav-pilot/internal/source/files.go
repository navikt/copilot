package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileHash hashes a file by its raw bytes (truncated to 16 hex chars).
func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// NormalizeMarkdown normalizes markdown content for comparison:
//   - CRLF → LF
//   - Trim trailing whitespace per line
//   - Collapse consecutive blank lines to a single blank line
func NormalizeMarkdown(data []byte) []byte {
	// CRLF → LF
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))

	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	prevBlank := false
	for _, line := range lines {
		trimmed := bytes.TrimRight(line, " \t")
		blank := len(trimmed) == 0
		if blank && prevBlank {
			continue
		}
		out = append(out, trimmed)
		prevBlank = blank
	}
	return bytes.Join(out, []byte("\n"))
}

// NormalizedFileHash hashes a file after normalizing markdown content.
// For non-.md files, falls back to raw FileHash.
func NormalizedFileHash(path string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		return FileHash(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	normalized := NormalizeMarkdown(data)
	h := sha256.New()
	h.Write(normalized)
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// DirHash hashes all files in a directory recursively.
// Markdown files (.md) are normalized before hashing for formatting tolerance.
func DirHash(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		h.Write([]byte(rel))

		if strings.HasSuffix(strings.ToLower(rel), ".md") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			h.Write(NormalizeMarkdown(data))
		} else {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// CopyFile copies a single file atomically, creating parent directories.
// Refuses to overwrite symlinks to prevent writing outside the repo.
// boundary is the trusted root directory; symlink checks stop there.
func CopyFile(src, dst, boundary string) error {
	// B2: Check BEFORE MkdirAll to prevent creating directories through symlinks.
	if err := CheckSymlink(dst, boundary); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// I4: Atomic write via temp file + rename
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".nav-pilot-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	return os.Rename(tmpPath, dst)
}

// CheckSymlink detects symlinks in the path chain between path and boundary.
// Walks up from the file's parent directory, checking each component with Lstat.
// Stops at boundary (the trusted root) to avoid false positives from system
// symlinks like /var → /private/var on macOS.
//
// Preconditions: boundary must be a non-empty absolute path. path must be
// lexically under boundary (verified internally).
func CheckSymlink(path, boundary string) error {
	if boundary == "" || !filepath.IsAbs(boundary) {
		return fmt.Errorf("checkSymlink: boundary must be a non-empty absolute path, got %q", boundary)
	}

	cleanPath := filepath.Clean(path)
	cleanBoundary := filepath.Clean(boundary)

	// Verify path is lexically under (or equal to) boundary.
	rel, err := filepath.Rel(cleanBoundary, cleanPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is not under boundary %q", path, boundary)
	}

	// Check the file itself if it exists
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink: %s", path)
	}

	// If path IS boundary, no intermediate directories to check.
	if cleanPath == cleanBoundary {
		return nil
	}

	// Walk from parent directory up to (but not including) boundary.
	dir := filepath.Clean(filepath.Dir(path))
	for dir != cleanBoundary {
		info, err := os.Lstat(dir)
		if err != nil {
			// Directory doesn't exist yet; MkdirAll will create it.
			dir = filepath.Dir(dir)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(dir)
			return fmt.Errorf("refusing to write through symlinked directory: %s -> %s", dir, target)
		}
		dir = filepath.Dir(dir)
	}
	return nil
}

// CopyDir copies a directory recursively, creating it fresh (removes stale files).
// boundary is the trusted root directory; symlink checks stop there.
func CopyDir(src, dst, boundary string) error {
	// B2: Check BEFORE RemoveAll to prevent deleting through symlinks.
	if err := CheckSymlink(dst, boundary); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return CopyFile(path, target, boundary)
	})
}

// CountDirFiles counts all files in dir recursively.
func CountDirFiles(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// ─── Artifact-level helpers ──────────────────────────────────────────────────
// These abstract over the file/directory distinction so callers don't branch.

// CopyArtifact copies a file or directory from src to dst.
func CopyArtifact(src, dst, rootDir string, isDir bool) error {
	if isDir {
		return CopyDir(src, dst, rootDir)
	}
	return CopyFile(src, dst, rootDir)
}

// RawArtifactHash returns the hash used for state/integrity tracking.
// For directories, markdown files are normalized (DirHash). For single files, raw bytes.
func RawArtifactHash(path string, isDir bool) (string, error) {
	if isDir {
		return DirHash(path)
	}
	return FileHash(path)
}

// ComparableArtifactHash returns the hash used for sync comparison.
// Normalizes markdown content (whitespace, line endings) so trivial
// formatting changes don't trigger false update notifications.
func ComparableArtifactHash(path string, isDir bool) (string, error) {
	if isDir {
		return DirHash(path)
	}
	return NormalizedFileHash(path)
}

// ─── Conflict detection ─────────────────────────────────────────────────────

// Conflict represents a conflicting artifact (existing file differs from source).
type Conflict struct {
	Path    string
	Current string // hash of existing file
	New     string // hash of source file
}

// CheckConflict detects if the target differs from the source artifact.
// Returns nil if no conflict (file absent or hashes match).
func CheckConflict(targetPath, sourcePath string, isDir bool) (*Conflict, error) {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil, nil
	}
	currentHash, err := RawArtifactHash(targetPath, isDir)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", targetPath, err)
	}
	newHash, err := RawArtifactHash(sourcePath, isDir)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", sourcePath, err)
	}
	if currentHash == newHash {
		return nil, nil
	}
	return &Conflict{Path: targetPath, Current: currentHash, New: newHash}, nil
}
