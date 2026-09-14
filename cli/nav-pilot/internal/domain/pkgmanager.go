package domain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PkgManager is the package manager that owns a binary. Two things depend on
// knowing it. A binary a package manager tracks must not be replaced in place:
// the manager's database then reports a version that is not on disk, and its
// next upgrade silently reverts the change. And advice has to name a command
// the machine actually has — `brew upgrade` helps nobody on a Debian box that
// installed from the apt archive.
type PkgManager struct {
	Name  string // as diagnostics report it; empty when no package manager owns the binary
	Label string // as prose names it
}

var (
	PkgNone = PkgManager{}
	PkgBrew = PkgManager{Name: "homebrew", Label: "Homebrew"}
	PkgApt  = PkgManager{Name: "apt", Label: "apt"}
)

// Pick returns the apt string when dpkg is the manager and the Homebrew string
// otherwise — including for a binary nothing owns, which keeps the advice that
// stood before the apt archive existed.
func (m PkgManager) Pick(brew, apt string) string {
	if m == PkgApt {
		return apt
	}
	return brew
}

// PkgOwner reports which package manager owns the binary at path. It is a
// variable so a test can assert what a packaged install is told without being
// installed from a package.
var PkgOwner = func(path string) PkgManager {
	if path == "" {
		return PkgNone
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return PkgNone
	}
	if strings.Contains(resolved, "/Cellar/") || strings.Contains(resolved, "/homebrew/") {
		return PkgBrew
	}
	if runtime.GOOS == "linux" && dpkgOwns(resolved) {
		return PkgApt
	}
	return PkgNone
}

// PkgSelf reports which package manager owns the running nav-pilot binary.
func PkgSelf() PkgManager {
	self, err := os.Executable()
	if err != nil {
		return PkgNone
	}
	return PkgOwner(self)
}

// PkgForInstall names the manager that should install a tool the machine does
// not have yet. Nothing owns it, so there is no owner to ask and the platform
// decides: apt where dpkg is in charge, Homebrew everywhere else — including a
// Linux without dpkg, which is the advice it already got.
//
// The one owner worth asking first is nav-pilot's own: a machine that got
// nav-pilot from the apt archive has the archive configured, and one that got
// it from Homebrew has brew — on Linux too, where the platform rule alone would
// send a Linuxbrew user to an archive they never added.
func PkgForInstall() PkgManager {
	if self := PkgSelf(); self != PkgNone {
		return self
	}
	if runtime.GOOS == "linux" && dpkgQuery() != "" {
		return PkgApt
	}
	return PkgBrew
}

// dpkgQuery locates dpkg-query: PATH first, so a test can plant one, then the
// path dpkg itself installs to. Ownership must not hinge on PATH — a restricted
// PATH without /usr/bin would otherwise read as "no dpkg", and the guard that
// keeps self-update off a .deb binary would be the thing that switched off.
func dpkgQuery() string {
	if p, err := exec.LookPath("dpkg-query"); err == nil {
		return p
	}
	if _, err := os.Stat("/usr/bin/dpkg-query"); err == nil {
		return "/usr/bin/dpkg-query"
	}
	return ""
}

// pkgLookupTimeout bounds the dpkg lookup. A package check must never be what
// makes a command feel slow.
const pkgLookupTimeout = 2 * time.Second

// dpkgOwns reports whether dpkg tracks path. The path test comes first, so the
// installs dpkg never owns — every macOS one, /usr/local/bin, ~/.local/bin —
// cost nothing; only a path a .deb could have written is confirmed with
// dpkg-query. The path alone is not enough: a hand-built binary dropped into
// /usr/bin would otherwise be told to run `sudo apt upgrade`, which cannot
// upgrade it, while self-update refused — no way forward at all. Every failure
// (no dpkg-query, a timeout, a path dpkg disowns) answers false, so the worst
// case is the behaviour we had before.
func dpkgOwns(path string) bool {
	if !strings.HasPrefix(path, "/usr/bin/") {
		return false
	}
	query := dpkgQuery()
	if query == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), pkgLookupTimeout)
	defer cancel()
	return exec.CommandContext(ctx, query, "-S", path).Run() == nil
}
