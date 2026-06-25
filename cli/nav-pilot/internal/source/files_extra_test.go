package source

import (
	"os"
	"path/filepath"
	"testing"
)

// --- FileHash ---

func TestFileHash_Success(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := FileHash(path)
	if err != nil {
		t.Fatalf("FileHash = %v, want nil", err)
	}
	if len(h) != 16 {
		t.Errorf("FileHash len = %d, want 16", len(h))
	}
	// Same content → same hash
	h2, _ := FileHash(path)
	if h != h2 {
		t.Error("FileHash not deterministic")
	}
}

func TestFileHash_NonexistentFile(t *testing.T) {
	_, err := FileHash("/nonexistent/file.txt")
	if err == nil {
		t.Error("FileHash(nonexistent) = nil, want error")
	}
}

// --- CopyFile ---

func TestCopyFile_Success(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.md")
	dst := filepath.Join(tmp, "sub", "dst.md")
	content := []byte("# Hello\n")

	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(src, dst, tmp); err != nil {
		t.Fatalf("CopyFile = %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copied content = %q, want %q", got, content)
	}
}

func TestCopyFile_NonexistentSrc(t *testing.T) {
	tmp := t.TempDir()
	err := CopyFile(filepath.Join(tmp, "nope.md"), filepath.Join(tmp, "dst.md"), tmp)
	if err == nil {
		t.Error("CopyFile(nonexistent src) = nil, want error")
	}
}

// --- CheckConflict ---

func TestCheckConflict_NoExistingTarget(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.md")
	dst := filepath.Join(tmp, "dst.md")
	if err := os.WriteFile(src, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := CheckConflict(dst, src, false)
	if err != nil {
		t.Fatalf("CheckConflict(no target) = %v, want nil", err)
	}
	if c != nil {
		t.Errorf("CheckConflict(no target) = %v, want nil", c)
	}
}

func TestCheckConflict_SameContent(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.md")
	dst := filepath.Join(tmp, "dst.md")
	content := []byte("same content")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := CheckConflict(dst, src, false)
	if err != nil {
		t.Fatalf("CheckConflict(same) = %v, want nil", err)
	}
	if c != nil {
		t.Errorf("CheckConflict(same) = %v, want nil (no conflict)", c)
	}
}

func TestCheckConflict_TrivialDifferences(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.md")
	dst := filepath.Join(tmp, "dst.md")
	// Same content, but different whitespace / line endings
	if err := os.WriteFile(src, []byte("same content \nwith newline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("same content\r\nwith newline\r\n  "), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := CheckConflict(dst, src, false)
	if err != nil {
		t.Fatalf("CheckConflict(trivial diff) = %v, want nil", err)
	}
	if c != nil {
		t.Errorf("CheckConflict(trivial diff) = %v, want nil (should normalize and ignore)", c)
	}
}

func TestCheckConflict_DifferentContent(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.md")
	dst := filepath.Join(tmp, "dst.md")
	if err := os.WriteFile(src, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := CheckConflict(dst, src, false)
	if err != nil {
		t.Fatalf("CheckConflict(diff) = %v, want nil", err)
	}
	if c == nil {
		t.Fatal("CheckConflict(diff) = nil, want conflict")
	}
	if c.Path != dst {
		t.Errorf("Conflict.Path = %q, want %q", c.Path, dst)
	}
}

// --- CopyDir ---

func TestCopyDir_Success(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "file.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyDir(src, dst, tmp); err != nil {
		t.Fatalf("CopyDir = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "file.md")); err != nil {
		t.Errorf("CopyDir didn't copy file: %v", err)
	}
}
