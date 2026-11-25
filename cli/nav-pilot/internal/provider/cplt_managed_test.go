package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
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

func TestManagedNewCpltWritesFileAndKeepsFlags(t *testing.T) {
	home := withManaged(t, managedRelease)
	args, env := launchManaged(t)
	path := managedPath(home)
	if env != path {
		t.Errorf("CPLT_MANAGED_CONFIG = %q, want %q", env, path)
	}
	if !slices.Contains(args, "fleet-gateway.intern.nav.no") || !slices.Contains(args, "fleet-gateway.intern.dev.nav.no") {
		t.Errorf("per-launch flags must stay with the managed file: %q", args)
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
			path, reason := CpltManagedConfig(nil)
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
	path, reason := CpltManagedConfig(nil)
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
	if p, _ := CpltManagedConfig(nil); p != managedPath(home) {
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
	path, _ := CpltManagedConfig(nil)
	if err := os.WriteFile(path, []byte(cpltManagedHeader+"\n[proxy]\nallow_private_domains = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if p, _ := CpltManagedCheck(); p != path {
		t.Fatalf("an older nav-pilot file counts as present: %q", p)
	}
	if fi, _ := os.Stat(path); !fi.ModTime().Equal(old) {
		t.Error("check rewrote a stale file")
	}
	CpltManagedConfig(nil)
	if p, _ := CpltManagedCheck(); p != path {
		t.Errorf("current: %q", p)
	}
}

func TestManagedRefusesFileNotRegular(t *testing.T) {
	home := withManaged(t, managedRelease)
	dir := filepath.Dir(managedPath(home))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "elsewhere.toml")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, managedPath(home)); err != nil {
		t.Fatal(err)
	}
	if p, reason := CpltManagedConfig(nil); p != "" || !strings.Contains(reason, "regular file") {
		t.Errorf("symlinked nav-pilot.toml: path %q reason %q", p, reason)
	}
	if fi, _ := os.Lstat(managedPath(home)); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink replaced")
	}
}

func TestManagedRewritesLooseModeOrHardLink(t *testing.T) {
	home := withManaged(t, managedRelease)
	path, _ := CpltManagedConfig(nil)
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	CpltManagedConfig(nil)
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v not fixed", fi.Mode().Perm())
	}
	if err := os.Link(path, filepath.Join(home, "second-link")); err != nil {
		t.Fatal(err)
	}
	CpltManagedConfig(nil)
	fi, _ := os.Stat(path)
	if st := fi.Sys().(*syscall.Stat_t); st.Nlink != 1 {
		t.Errorf("nlink %d, cplt refuses a hard-linked file", st.Nlink)
	}
}

func TestManagedOverridesInheritedEnv(t *testing.T) {
	home := withManaged(t, managedRelease)
	t.Setenv(CpltManagedEnv, filepath.Join(home, "planted.toml"))
	if _, env := launchManaged(t); env != managedPath(home) {
		t.Errorf("CPLT_MANAGED_CONFIG = %q, want %q", env, managedPath(home))
	}
}

func TestManagedRewritesOversizedFile(t *testing.T) {
	home := withManaged(t, managedRelease)
	path, _ := CpltManagedConfig(nil)
	big := append(cpltManagedContent(nil), make([]byte, 1<<20)...)
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatal(err)
	}
	if p, _ := CpltManagedConfig(nil); p != managedPath(home) {
		t.Fatalf("path %q", p)
	}
	if got, _ := os.ReadFile(path); string(got) != string(cpltManagedContent(nil)) {
		t.Errorf("oversized file not rewritten (%d bytes)", len(got))
	}
}

// fakeProposal stands in for an approved pakke waiver and MCP registry grant.
func fakeProposal(t *testing.T, flags ...string) {
	t.Helper()
	prev := cpltProposalFlags
	t.Cleanup(func() { cpltProposalFlags = prev })
	calls := 0
	cpltProposalFlags = func() []string { calls++; return flags }
	t.Cleanup(func() {
		if calls != 1 {
			t.Errorf("proposal flags computed %d times, want 1", calls)
		}
	})
}

func managedHosts(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, list, _ := strings.Cut(string(b), "allow_private_domains = [")
	list, _, _ = strings.Cut(list, "]")
	var hosts []string
	for _, q := range strings.Split(list, ", ") {
		hosts = append(hosts, strings.Trim(q, `"`))
	}
	return hosts
}

func TestManagedHostsEqualFlagHosts(t *testing.T) {
	home := withManaged(t, managedRelease)
	fakeProposal(t, "--allow-private-domain", "mimir.nav.cloud.nais.io", "--allow-read", "/x",
		"--allow-private-domain", "mcp.intern.nav.no")
	args, env := launchManaged(t)
	if env != managedPath(home) {
		t.Fatalf("env %q", env)
	}
	flags := proposalPrivateHosts(args)
	file := managedHosts(t, env)
	slices.Sort(flags)
	slices.Sort(file)
	if !slices.Equal(flags, file) {
		t.Errorf("file hosts %q, flag hosts %q", file, flags)
	}
}

func TestManagedBadHostWritesNothing(t *testing.T) {
	for _, h := range []string{"*.nav.no", "10.0.0.1", "localhost", "Mimir.nav.no", "a.nav.no:443", "nav", "a..no", "a.no\"]"} {
		home := withManaged(t, managedRelease)
		fakeProposal(t, "--allow-private-domain", h)
		args, env := launchManaged(t)
		if env != "" || !slices.Contains(args, h) {
			t.Errorf("%q: env %q, flag kept %v", h, env, slices.Contains(args, h))
		}
		if _, err := os.Lstat(managedPath(home)); err == nil {
			t.Errorf("%q: file written", h)
		}
	}
}

func TestManagedLeavesForeignFile(t *testing.T) {
	home := withManaged(t, managedRelease)
	path := managedPath(home)
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte("[proxy]\nallow_private_domains = [\"x.no\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, reason := CpltManagedConfig(nil); p != "" || !strings.Contains(reason, "not written by nav-pilot") {
		t.Errorf("path %q reason %q", p, reason)
	}
	if b, _ := os.ReadFile(path); !strings.HasPrefix(string(b), "[proxy]") {
		t.Error("foreign file overwritten")
	}
}

func TestManagedUpgradesOlderFile(t *testing.T) {
	home := withManaged(t, managedRelease)
	path, _ := CpltManagedConfig(nil)
	if p, _ := CpltManagedConfig([]string{"mimir.nav.cloud.nais.io"}); p != managedPath(home) {
		t.Fatalf("path %q", p)
	}
	if h := managedHosts(t, path); !slices.Contains(h, "mimir.nav.cloud.nais.io") || len(h) != 3 {
		t.Errorf("hosts %q", h)
	}
}
