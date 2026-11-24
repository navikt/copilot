package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

const managedRelease = "cplt 2026.11.24-020236-4ea5789\n"

// withManaged isolates HOME (also as the passwd home), fakes the cplt version
// and restores the real launch defaults the provider TestMain stubs out.
func withManaged(t *testing.T, version string) string {
	t.Helper()
	home := isolateHome(t)
	withFleetGatewayDefaults(t, version)
	prevHome, prevManaged := passwdHome, cpltManagedConfig
	t.Cleanup(func() { passwdHome, cpltManagedConfig = prevHome, prevManaged })
	passwdHome = func() (string, error) { return home, nil }
	cpltManagedConfig = CpltManagedConfig
	return home
}

func managedPath(home string) string {
	return filepath.Join(home, ".config", "cplt", "managed.d", "nav-pilot.toml")
}

// fakeCpltEnv records argv and CPLT_MANAGED_CONFIG of the cplt launch.
func fakeCpltEnv(t *testing.T) (args, env string) {
	t.Helper()
	dir := t.TempDir()
	args, env = filepath.Join(dir, "args.txt"), filepath.Join(dir, "env.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + args + "\nprintf '%s' \"$CPLT_MANAGED_CONFIG\" > " + env + "\n"
	if err := testhome.WriteExec(filepath.Join(dir, "cplt"), script); err != nil {
		t.Fatal(err)
	}
	if err := testhome.WriteExec(filepath.Join(dir, "copilot"), "#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return args, env
}

func launchManaged(t *testing.T) (args []string, env string) {
	t.Helper()
	argsFile, envFile := fakeCpltEnv(t)
	if err := launchViaCplt(cpltLaunch{agent: "copilot", displayName: "Copilot"}); err != nil {
		t.Fatalf("launchViaCplt: %v", err)
	}
	a, _ := os.ReadFile(argsFile)
	e, _ := os.ReadFile(envFile)
	return strings.Split(string(a), "\n"), string(e)
}

func TestManagedNewCpltWritesFileAndDropsFlags(t *testing.T) {
	home := withManaged(t, managedRelease)
	args, env := launchManaged(t)
	path := managedPath(home)
	if env != path {
		t.Errorf("CPLT_MANAGED_CONFIG = %q, want %q", env, path)
	}
	if slices.Contains(args, "--allow-private-domain") {
		t.Errorf("per-launch flags not dropped: %q", args)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "managed_by = \"nav-pilot\"\nschema = 1\n\n[proxy]\nallow_private_domains = [\"fleet-gateway.intern.nav.no\", \"fleet-gateway.intern.dev.nav.no\"]\n"
	if string(got) != want {
		t.Errorf("content:\n%s\nwant:\n%s", got, want)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
}

func TestManagedOldOrGarbledCpltKeepsFlags(t *testing.T) {
	for _, v := range []string{"cplt 2026.11.23-195628-9ef3d81\n", "garbled\x00", "cplt 2026.11.24\n", ""} {
		home := withManaged(t, v)
		args, env := launchManaged(t)
		if env != "" {
			t.Errorf("%q: env %q", v, env)
		}
		if _, err := os.Lstat(managedPath(home)); err == nil {
			t.Errorf("%q: file written", v)
		}
		if v == "cplt 2026.11.23-195628-9ef3d81\n" && !slices.Contains(args, "fleet-gateway.intern.nav.no") {
			t.Errorf("old cplt lost flags: %q", args)
		}
	}
}

func TestManagedPreconditionsFailClosed(t *testing.T) {
	cases := map[string]func(t *testing.T, home string){
		"symlinked managed.d": func(t *testing.T, home string) {
			mustMkdir(t, filepath.Join(home, ".config", "cplt"))
			if err := os.Symlink(t.TempDir(), filepath.Join(home, ".config", "cplt", "managed.d")); err != nil {
				t.Fatal(err)
			}
		},
		"group-writable parent": func(t *testing.T, home string) {
			mustMkdir(t, filepath.Join(home, ".config"))
			if err := os.Chmod(filepath.Join(home, ".config"), 0o775); err != nil {
				t.Fatal(err)
			}
		},
		"HOME is not passwd home": func(t *testing.T, home string) {
			other := t.TempDir()
			passwdHome = func() (string, error) { return other, nil }
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			home := withManaged(t, managedRelease)
			setup(t, home)
			path, reason := CpltManagedConfig()
			if path != "" || reason == "" {
				t.Fatalf("path %q reason %q", path, reason)
			}
			args, env := launchManaged(t)
			if env != "" || !slices.Contains(args, "fleet-gateway.intern.dev.nav.no") {
				t.Errorf("env %q args %q", env, args)
			}
			if _, err := os.Stat(managedPath(home)); err == nil {
				t.Error("file written")
			}
		})
	}
}

func TestManagedUnchangedNotRewrittenOthersUntouched(t *testing.T) {
	home := withManaged(t, managedRelease)
	path, reason := CpltManagedConfig()
	if path == "" {
		t.Fatal(reason)
	}
	other := filepath.Join(filepath.Dir(path), "other.toml")
	if err := os.WriteFile(other, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	for _, p := range []string{path, other} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if p, _ := CpltManagedConfig(); p != managedPath(home) {
		t.Fatalf("second run: %q", p)
	}
	for _, p := range []string{path, other} {
		if fi, _ := os.Stat(p); !fi.ModTime().Equal(old) {
			t.Errorf("%s touched", p)
		}
	}
	if b, _ := os.ReadFile(other); string(b) != "x" {
		t.Error("other.toml changed")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 2 {
		t.Errorf("managed.d holds %d entries, want 2 (no temp files left)", len(entries))
	}
}

func TestManagedCheckIsReadOnly(t *testing.T) {
	home := withManaged(t, managedRelease)
	if p, reason := CpltManagedCheck(); p != "" || !strings.Contains(reason, "next launch") {
		t.Fatalf("missing: %q %q", p, reason)
	}
	if _, err := os.Lstat(filepath.Join(home, ".config")); err == nil {
		t.Fatal("check created ~/.config")
	}
	path, _ := CpltManagedConfig()
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if p, reason := CpltManagedCheck(); p != "" || !strings.Contains(reason, "stale") {
		t.Fatalf("stale: %q %q", p, reason)
	}
	if fi, _ := os.Stat(path); !fi.ModTime().Equal(old) {
		t.Error("check rewrote a stale file")
	}
	CpltManagedConfig()
	if p, _ := CpltManagedCheck(); p != path {
		t.Errorf("current: %q", p)
	}
}
