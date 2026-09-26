package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A copy nav-pilot saved is the user's: a replacement keeps it, the hash does
// not see it, and a subdirectory the source dropped still goes.
func TestCopyDirKeepsOrigAndDropsStaleDirs(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	write(t, filepath.Join(src, "SKILL.md"), "new\n")
	write(t, filepath.Join(dst, "SKILL.md"), "old\n")
	write(t, filepath.Join(dst, "SKILL.md.orig"), "mine\n")
	write(t, filepath.Join(dst, "notes.orig", "a.txt"), "mine too\n")
	write(t, filepath.Join(dst, "references", "gone.md"), "stale\n")

	if err := CopyDir(src, dst, root); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"SKILL.md.orig", "notes.orig/a.txt"} {
		if _, err := os.Stat(filepath.Join(dst, kept)); err != nil {
			t.Errorf("%s was not kept: %v", kept, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "references")); !os.IsNotExist(err) {
		t.Errorf("the dropped references/ directory is still there: %v", err)
	}
	a, _ := DirHash(src)
	b, _ := DirHash(dst)
	if a != b {
		t.Errorf("saved copies count toward the hash: %s != %s", a, b)
	}
}

// SaveOrig never reads through a symlink: its target could be any file on
// the machine, and the .orig lands in a repository.
func TestSaveOrigRefusesSymlinks(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	write(t, secret, "TOP SECRET\n")

	link := filepath.Join(root, "agent.agent.md")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveOrig(link, filepath.Join(root, "src.md"), root, false); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Errorf("SaveOrig followed a symlinked file: err = %v", err)
	}

	dir := filepath.Join(root, "skill")
	write(t, filepath.Join(dir, "SKILL.md"), "mine\n")
	if err := os.Symlink(secret, filepath.Join(dir, "ref.md")); err != nil {
		t.Fatal(err)
	}
	saved, err := SaveOrig(dir, filepath.Join(root, "nosrc"), root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ref.md.orig")); !os.IsNotExist(err) {
		t.Errorf("SaveOrig copied a symlink's target into the directory (saved %v)", saved)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "SKILL.md.orig")); string(got) != "mine\n" {
		t.Errorf("SKILL.md.orig = %q, want the local copy", got)
	}
}
