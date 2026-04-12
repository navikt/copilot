package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func fileHash(path string) (string, error) {
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

func dirHash(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		h.Write([]byte(rel))
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// copyFile copies a single file, creating parent directories.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// copyDir copies a directory recursively, creating it fresh (removes stale files).
func copyDir(src, dst string) error {
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
		return copyFile(path, target)
	})
}

func countDirFiles(dir string) int {
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

// ─── Conflict detection ─────────────────────────────────────────────────────

type conflict struct {
	Path    string
	Current string // hash of existing file
	New     string // hash of source file
}

func checkConflict(targetPath, sourcePath string, isDir bool) (*conflict, error) {
	if isDir {
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			return nil, nil
		}
		currentHash, err := dirHash(targetPath)
		if err != nil {
			return nil, err
		}
		newHash, err := dirHash(sourcePath)
		if err != nil {
			return nil, err
		}
		if currentHash == newHash {
			return nil, nil
		}
		return &conflict{Path: targetPath, Current: currentHash, New: newHash}, nil
	}

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil, nil
	}
	currentHash, err := fileHash(targetPath)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", targetPath, err)
	}
	newHash, err := fileHash(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", sourcePath, err)
	}
	if currentHash == newHash {
		return nil, nil
	}
	return &conflict{Path: targetPath, Current: currentHash, New: newHash}, nil
}
