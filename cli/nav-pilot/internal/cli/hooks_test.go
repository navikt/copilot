package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// hookSource lays out a source checkout that ships one hook: the script plus
// the sidecar that says which tool calls it should see.
func hookSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	hooks := filepath.Join(dir, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "klarsprak-gate.py"), []byte("#!/usr/bin/env python3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "klarsprak-gate"+source.HookMetaSuffix),
		[]byte(`{"matcher":"shell|execute|bash","timeoutSec":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func repoEntries(t *testing.T, target string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(target, ".github", "hooks", source.RepoHooksConfig))
	if err != nil {
		t.Fatalf("reading repo hooks config: %v", err)
	}
	var got struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parsing repo hooks config: %v\n%s", err, data)
	}
	return got.Hooks["preToolUse"]
}

// The repo config is shared and conventional: the user may already have their
// own entries in it, so installing merges and never overwrites.
func TestInstallHookRepoScopeMerges(t *testing.T) {
	src := hookSource(t)
	target := t.TempDir()
	hooksDir := filepath.Join(target, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"version":1,"hooks":{"preToolUse":[{"type":"command","command":"./min-egen.sh","timeoutSec":9}]}}`
	if err := os.WriteFile(filepath.Join(hooksDir, source.RepoHooksConfig), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	scope := ScopeRepo(target)
	result := &installResult{}
	if err := installArtifact(NewSourceResolver(src), scope, nil, KindHook, "klarsprak-gate", false, false, result); err != nil {
		t.Fatalf("installArtifact: %v", err)
	}

	if _, err := os.Stat(filepath.Join(hooksDir, "klarsprak-gate.py")); err != nil {
		t.Errorf("script not installed: %v", err)
	}
	entries := repoEntries(t, target)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (theirs + ours): %#v", len(entries), entries)
	}
	if entries[0]["command"] != "./min-egen.sh" {
		t.Errorf("the pre-existing entry did not survive: %#v", entries[0])
	}
	if entries[1][source.HookMarker] != "klarsprak-gate" {
		t.Errorf("our entry is not marked: %#v", entries[1])
	}
	if entries[1]["matcher"] != "shell|execute|bash" {
		t.Errorf("matcher came from somewhere other than the sidecar: %#v", entries[1])
	}
	cmd, _ := entries[1]["command"].(string)
	if !strings.Contains(cmd, ".github/hooks/klarsprak-gate.py") {
		t.Errorf("command does not point at the installed script: %q", cmd)
	}

	// Installing again must not add a second copy of our entry.
	if err := installArtifact(NewSourceResolver(src), scope, nil, KindHook, "klarsprak-gate", false, true, result); err != nil {
		t.Fatalf("second installArtifact: %v", err)
	}
	if entries := repoEntries(t, target); len(entries) != 2 {
		t.Errorf("reinstall changed the entry count to %d: %#v", len(entries), entries)
	}
}

// Uninstall takes nav-pilot's entries out and leaves the user's alone. The
// config is shared, so removing the file outright would be data loss.
func TestUninstallHookLeavesForeignEntries(t *testing.T) {
	src := hookSource(t)
	target := t.TempDir()
	hooksDir := filepath.Join(target, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"version":1,"hooks":{"preToolUse":[{"type":"command","command":"./min-egen.sh","timeoutSec":9}]}}`
	if err := os.WriteFile(filepath.Join(hooksDir, source.RepoHooksConfig), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	scope := ScopeRepo(target)
	manifest := &Manifest{Name: "test", Hooks: []string{"klarsprak-gate"}}
	result, err := installItems(NewSourceResolver(src), scope, manifest, false, false)
	if err != nil {
		t.Fatalf("installItems: %v", err)
	}
	if err := writeScopedState(scope, &StateFile{Collection: "test", Files: result.Files}); err != nil {
		t.Fatalf("writeScopedState: %v", err)
	}

	if err := cmdUninstall(scope, false); err != nil {
		t.Fatalf("cmdUninstall: %v", err)
	}

	if _, err := os.Stat(filepath.Join(hooksDir, "klarsprak-gate.py")); !os.IsNotExist(err) {
		t.Errorf("script survived uninstall: %v", err)
	}
	entries := repoEntries(t, target)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want the user's one: %#v", len(entries), entries)
	}
	if entries[0]["command"] != "./min-egen.sh" {
		t.Errorf("the wrong entry survived: %#v", entries[0])
	}
}

// A config that held nothing but nav-pilot's entries is removed outright,
// rather than left behind as an empty shell.
func TestUninstallHookRemovesEmptiedConfig(t *testing.T) {
	src := hookSource(t)
	target := t.TempDir()
	scope := ScopeRepo(target)

	manifest := &Manifest{Name: "test", Hooks: []string{"klarsprak-gate"}}
	result, err := installItems(NewSourceResolver(src), scope, manifest, false, false)
	if err != nil {
		t.Fatalf("installItems: %v", err)
	}
	if err := writeScopedState(scope, &StateFile{Collection: "test", Files: result.Files}); err != nil {
		t.Fatal(err)
	}
	if err := cmdUninstall(scope, false); err != nil {
		t.Fatalf("cmdUninstall: %v", err)
	}
	path := filepath.Join(target, ".github", "hooks", source.RepoHooksConfig)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("config survived with nothing in it: %v", err)
	}
}

// User scope is a directory of separate files, one per hook — no merge, and the
// config file is tracked so uninstall takes it away with the script.
func TestInstallHookUserScopeDropsFiles(t *testing.T) {
	src := hookSource(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}

	result := &installResult{}
	if err := installArtifact(NewSourceResolver(src), scope, nil, KindHook, "klarsprak-gate", false, false, result); err != nil {
		t.Fatalf("installArtifact: %v", err)
	}

	hooksDir := filepath.Join(home, ".copilot", "hooks")
	for _, name := range []string{"klarsprak-gate.py", "klarsprak-gate.json"} {
		if _, err := os.Stat(filepath.Join(hooksDir, name)); err != nil {
			t.Errorf("%s missing from %s: %v", name, hooksDir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(hooksDir, source.RepoHooksConfig)); !os.IsNotExist(err) {
		t.Errorf("user scope wrote the shared repo config; it uses one file per hook")
	}

	var tracked []string
	for _, f := range result.Files {
		tracked = append(tracked, f.Path)
		if err := scope.ValidateStatePath(f.Path); err != nil {
			t.Errorf("state path rejected by its own scope: %v", err)
		}
	}
	want := []string{"hooks/klarsprak-gate.py", "hooks/klarsprak-gate.json"}
	for _, w := range want {
		if !contains(tracked, w) {
			t.Errorf("%q not tracked; tracked = %v", w, tracked)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// OpenCode has no tool-deny mechanism, so a hook cannot run there. It must stay
// out of the exported scope, and the state path validator is the backstop.
func TestOpenCodeRefusesHooks(t *testing.T) {
	if err := artifacts.ValidateOpenCodeStatePath("hooks/klarsprak-gate.py"); err == nil {
		t.Error("opencode state accepted a hook path")
	}

	src := hookSource(t)
	out := t.TempDir()
	if _, _, _, _, _, err := artifacts.SyncOpenCodeArtifacts(src, "", out, "v1", "sha", "repo"); err != nil {
		t.Fatalf("SyncOpenCodeArtifacts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "hooks")); !os.IsNotExist(err) {
		t.Errorf("a hooks directory reached the opencode scope: %v", err)
	}
}

// End to end: a real install into a hooks directory the process cannot write,
// with cplt's marker set. The error the user sees must name cplt, name the
// path, and say what to do — not just "permission denied".
//
// The write is denied by mode bits rather than by cplt, because a test cannot
// build a sandbox. What is being asserted is the annotation, and the errno it
// keys on is the same one Seatbelt returns for the real deny.
func TestInstallHookInsideCpltSandboxNamesCplt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the mode bits this test denies with")
	}
	src := hookSource(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(cpltSandboxEnvVar, "1")

	hooksDir := filepath.Join(home, ".copilot", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hooksDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hooksDir, 0o755) })

	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	err = installArtifact(NewSourceResolver(src), scope, nil, KindHook, "klarsprak-gate", false, false, &installResult{})
	if err == nil {
		t.Fatal("install into an unwritable hooks dir succeeded; the test denied nothing")
	}
	msg := err.Error()
	for _, want := range []string{"cplt", cpltSandboxEnvVar, hooksDir, "outside cplt"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q:\n%s", want, msg)
		}
	}
}

// The annotation is keyed on cplt actually being there. Outside a sandbox the
// same permission error must stay the plain one — naming cplt when cplt is not
// involved sends the user somewhere useless.
func TestHookWriteFailureOutsideCpltIsNotBlamedOnIt(t *testing.T) {
	if _, set := os.LookupEnv(cpltSandboxEnvVar); set {
		t.Skip("this test suite is itself running inside cplt")
	}
	err := explainHookWrite(&os.PathError{Op: "mkdir", Path: "/x", Err: syscall.EACCES}, "/x")
	if strings.Contains(err.Error(), "cplt") {
		t.Errorf("blamed cplt outside a cplt sandbox: %v", err)
	}
}

// A failure that is not a denial is not cplt's doing, even inside a sandbox.
func TestHookWriteNonPermissionErrorPassesThrough(t *testing.T) {
	t.Setenv(cpltSandboxEnvVar, "1")
	err := explainHookWrite(&os.PathError{Op: "write", Path: "/x", Err: syscall.ENOSPC}, "/x")
	if strings.Contains(err.Error(), "cplt") {
		t.Errorf("blamed cplt for a full disk: %v", err)
	}
}
