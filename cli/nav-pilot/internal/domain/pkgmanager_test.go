package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeDpkgQuery puts a dpkg-query with a fixed exit code on PATH.
func fakeDpkgQuery(t *testing.T, exit int) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "dpkg-query")
	if err := os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\nexit %d\n", exit)), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// TestDpkgOwns: the path alone cannot decide. A hand-built binary in /usr/bin
// must still be allowed to update itself — telling it `sudo apt upgrade` while
// refusing to update leaves the user with no way forward — so dpkg has to
// confirm.
func TestDpkgOwns(t *testing.T) {
	// PkgOwner is what limits dpkg to Linux; the check itself is the same shell
	// call everywhere, so it is tested everywhere.
	t.Run("dpkg owns it", func(t *testing.T) {
		fakeDpkgQuery(t, 0)
		if !dpkgOwns("/usr/bin/nav-pilot") {
			t.Error("a binary dpkg reports as its own was treated as unmanaged")
		}
	})
	t.Run("dpkg disowns it", func(t *testing.T) {
		fakeDpkgQuery(t, 1)
		if dpkgOwns("/usr/bin/nav-pilot") {
			t.Error("a hand-built binary in /usr/bin was refused an update it can do")
		}
	})
	t.Run("outside dpkg territory", func(t *testing.T) {
		// dpkg-query answers yes to everything here: only the path test can keep
		// the usual installs from paying for a process spawn.
		fakeDpkgQuery(t, 0)
		if dpkgOwns("/usr/local/bin/nav-pilot") {
			t.Error("a /usr/local/bin install asked dpkg about itself")
		}
	})
	t.Run("no dpkg-query", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if dpkgOwns("/usr/bin/nav-pilot") {
			t.Error("a missing dpkg-query was read as proof of ownership")
		}
	})
}

// TestPkgOwner covers the two answers that do not need a Linux: a binary under
// a Homebrew prefix, and one that no package manager put there.
func TestPkgOwner(t *testing.T) {
	bin := func(t *testing.T, rel string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}

	if got := PkgOwner(bin(t, "homebrew/Cellar/nav-pilot/1.0/bin/nav-pilot")); got != PkgBrew {
		t.Errorf("a binary in a Homebrew Cellar reported %+v, want %+v", got, PkgBrew)
	}
	if got := PkgOwner(bin(t, "build/nav-pilot")); got != PkgNone {
		t.Errorf("a self-built binary reported %+v, want no owner", got)
	}
	if got := PkgOwner(""); got != PkgNone {
		t.Errorf("an unknown path reported %+v, want no owner", got)
	}
	if got := PkgOwner(filepath.Join(t.TempDir(), "gone")); got != PkgNone {
		t.Errorf("a path that does not exist reported %+v, want no owner", got)
	}
}

// TestPick: apt advice goes only to an apt install. Everything else keeps the
// Homebrew string it had before the apt archive existed.
func TestPick(t *testing.T) {
	const brew, apt = "brew upgrade cplt", "sudo apt upgrade cplt"
	for _, tt := range []struct {
		mgr  PkgManager
		want string
	}{
		{PkgApt, apt},
		{PkgBrew, brew},
		{PkgNone, brew},
	} {
		if got := tt.mgr.Pick(brew, apt); got != tt.want {
			t.Errorf("%q picked %q, want %q", tt.mgr.Name, got, tt.want)
		}
	}
}

// TestPkgForInstallFollowsNavPilotsOwner: a missing tool is installed the way
// nav-pilot itself was, when that is known. The platform rule is the fallback,
// not the first answer — a Linuxbrew nav-pilot has brew, not the apt archive.
func TestPkgForInstallFollowsNavPilotsOwner(t *testing.T) {
	for _, mgr := range []PkgManager{PkgBrew, PkgApt} {
		t.Run(mgr.Name, func(t *testing.T) {
			orig := PkgOwner
			t.Cleanup(func() { PkgOwner = orig })
			PkgOwner = func(string) PkgManager { return mgr }
			if got := PkgForInstall(); got != mgr {
				t.Errorf("PkgForInstall = %+v, want nav-pilot's own manager %+v", got, mgr)
			}
		})
	}
}
