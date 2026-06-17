package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeMarkdown_CRLF(t *testing.T) {
	input := []byte("line one\r\nline two\r\nline three\r\n")
	want := "line one\nline two\nline three\n"
	got := string(NormalizeMarkdown(input))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizeMarkdown_TrailingWhitespace(t *testing.T) {
	input := []byte("hello   \nworld\t\t\n")
	want := "hello\nworld\n"
	got := string(NormalizeMarkdown(input))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizeMarkdown_ConsecutiveBlankLines(t *testing.T) {
	input := []byte("paragraph one\n\n\n\nparagraph two\n\n\nend\n")
	want := "paragraph one\n\nparagraph two\n\nend\n"
	got := string(NormalizeMarkdown(input))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizeMarkdown_CombinedNormalization(t *testing.T) {
	input := []byte("# Title  \r\n\r\n\r\nSome text   \r\n\r\nEnd\r\n")
	want := "# Title\n\nSome text\n\nEnd\n"
	got := string(NormalizeMarkdown(input))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizeMarkdown_EmptyInput(t *testing.T) {
	got := string(NormalizeMarkdown([]byte("")))
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestNormalizedFileHash_MDFile(t *testing.T) {
	dir := t.TempDir()

	// Two files with same content but different formatting
	f1 := filepath.Join(dir, "a.md")
	f2 := filepath.Join(dir, "b.md")
	os.WriteFile(f1, []byte("# Title\nContent\n"), 0o644)
	os.WriteFile(f2, []byte("# Title  \r\nContent   \r\n"), 0o644)

	h1, err := NormalizedFileHash(f1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := NormalizedFileHash(f2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("normalized hashes should match: %q != %q", h1, h2)
	}
}

func TestNormalizedFileHash_NonMDFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.json")
	os.WriteFile(f, []byte(`{"key": "value"}`), 0o644)

	// Non-.md file should use raw hash
	normalized, err := NormalizedFileHash(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := FileHash(f)
	if err != nil {
		t.Fatal(err)
	}
	if normalized != raw {
		t.Errorf("non-.md hash should be raw: normalized=%q raw=%q", normalized, raw)
	}
}

func TestDirHash_NormalizesMarkdown(t *testing.T) {
	// Two dirs with same content but different markdown formatting
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	os.WriteFile(filepath.Join(dir1, "SKILL.md"), []byte("# Skill\nContent\n"), 0o644)
	os.WriteFile(filepath.Join(dir1, "metadata.json"), []byte(`{"v":1}`), 0o644)

	os.WriteFile(filepath.Join(dir2, "SKILL.md"), []byte("# Skill  \r\nContent   \r\n"), 0o644)
	os.WriteFile(filepath.Join(dir2, "metadata.json"), []byte(`{"v":1}`), 0o644)

	h1, err := DirHash(dir1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := DirHash(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("dir hashes should match with normalized markdown: %q != %q", h1, h2)
	}
}

func TestDirHash_JsonDiffStillDetected(t *testing.T) {
	// JSON differences should still be detected even if markdown is the same
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	os.WriteFile(filepath.Join(dir1, "SKILL.md"), []byte("# Same\n"), 0o644)
	os.WriteFile(filepath.Join(dir1, "metadata.json"), []byte(`{"v":1}`), 0o644)

	os.WriteFile(filepath.Join(dir2, "SKILL.md"), []byte("# Same\n"), 0o644)
	os.WriteFile(filepath.Join(dir2, "metadata.json"), []byte(`{"v":2}`), 0o644)

	h1, err := DirHash(dir1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := DirHash(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Error("dir hashes should differ when JSON content differs")
	}
}
