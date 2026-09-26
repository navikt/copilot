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

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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

// OrigSuffix marks the copy of a locally edited file that nav-pilot saved
// before replacing it. It is the user's, not part of the artifact: [DirHash]
// leaves it out and [CopyDir] leaves it in place.
const OrigSuffix = ".orig"

// DirHash hashes all files in a directory recursively, except saved
// [OrigSuffix] copies.
// Markdown files (.md) are normalized before hashing for formatting tolerance.
func DirHash(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, OrigSuffix) {
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
// The implementation lives in internal/domain so packages below this one — the
// agentpakke declaration writer among them — can reach the same guard without
// importing this package.
func CheckSymlink(path, boundary string) error {
	return domain.CheckSymlink(path, boundary)
}

// CopyDir copies a directory recursively, creating it fresh (removes stale
// files). Saved [OrigSuffix] copies stay: they are the user's own edits.
// boundary is the trusted root directory; symlink checks stop there.
func CopyDir(src, dst, boundary string) error {
	// B2: Check BEFORE RemoveAll to prevent deleting through symlinks.
	if err := CheckSymlink(dst, boundary); err != nil {
		return err
	}
	if err := removeAllButOrig(dst); err != nil {
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

// removeAllButOrig empties dir of everything except [OrigSuffix] files, and
// removes it outright when there is nothing to keep.
func removeAllButOrig(dir string) error {
	keep := false
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		case strings.HasSuffix(path, OrigSuffix):
			keep = true
			return nil
		}
		return os.Remove(path)
	})
	if os.IsNotExist(err) || (err == nil && !keep) {
		return os.RemoveAll(dir)
	}
	return err
}

// SaveOrig keeps the local copy of an artifact nav-pilot is about to replace,
// as <file>.orig beside it, and returns the paths it wrote. One backup per
// file: the next replacement overwrites it. In a directory only the files that
// differ from src are saved, inside the directory, where [CopyDir] keeps them.
func SaveOrig(local, src, boundary string, isDir bool) ([]string, error) {
	if !isDir {
		return []string{local + OrigSuffix}, CopyFile(local, local+OrigSuffix, boundary)
	}
	var saved []string
	err := filepath.WalkDir(local, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, OrigSuffix) {
			return err
		}
		rel, err := filepath.Rel(local, path)
		if err != nil {
			return err
		}
		mine, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if theirs, err := os.ReadFile(filepath.Join(src, rel)); err == nil && bytes.Equal(mine, theirs) {
			return nil
		}
		saved = append(saved, path+OrigSuffix)
		return CopyFile(path, path+OrigSuffix, boundary)
	})
	return saved, err
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
	currentHash, err := ComparableArtifactHash(targetPath, isDir)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", targetPath, err)
	}
	newHash, err := ComparableArtifactHash(sourcePath, isDir)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", sourcePath, err)
	}
	if currentHash == newHash {
		return nil, nil
	}
	return &Conflict{Path: targetPath, Current: currentHash, New: newHash}, nil
}
