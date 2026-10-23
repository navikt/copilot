package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// An item counts only when its file is there under its own kind suffix; a pin
// counts as everything only when its revision is on disk.
func TestInstalledContents(t *testing.T) {
	consentEnv(t)
	root := t.TempDir()
	scope := ScopeRepo(root)
	files := []string{".github/agents/review.extra.agent.md", ".github/hooks/gate.py", ".github/hooks/other.py", ".github/extensions/fmt/extension.mjs"}
	state := &StateFile{SourceRepo: "navikt/plattform", SourceSHA: strings.Repeat("a", 40)}
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		state.Files = append(state.Files, InstalledFile{Path: f})
	}
	shipped := &Manifest{Agents: []string{"review", "review.extra"}, Hooks: []string{"gate", "other"}, Extensions: []string{"fmt"}}

	got := installedContents(scope, shipped, state)
	if strings.Join(got.Agents, ",") != "review.extra" {
		t.Errorf("agents = %v, want only review.extra", got.Agents)
	}
	if len(got.Hooks) != 2 || len(got.Extensions) != 1 {
		t.Errorf("control: hooks %v, extensions %v, want both hooks and fmt", got.Hooks, got.Extensions)
	}
	if err := os.Remove(filepath.Join(root, ".github/hooks/gate.py")); err != nil {
		t.Fatal(err)
	}
	if got := installedContents(scope, shipped, state); strings.Join(got.Hooks, ",") != "other" {
		t.Errorf("a deleted hook still counts: %v", got.Hooks)
	}

	// A pin-shaped state with no files: nothing, unless the revision is on disk.
	pin := &StateFile{SourceRepo: "navikt/plattform", SourceSHA: strings.Repeat("b", 40)}
	if got := installedContents(scope, shipped, pin); len(got.Agents)+len(got.Hooks) != 0 {
		t.Errorf("a pin with no revision on disk counts %+v", got)
	}
	if err := os.MkdirAll(pakkeRevisionDir(pin.SourceRepo, pin.SourceSHA), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := installedContents(scope, shipped, pin); got != shipped {
		t.Errorf("control: a pin on disk should count everything it ships")
	}
}

// Next: sends the user back to the scope this run read.
func TestScopeArgs(t *testing.T) {
	consentEnv(t)
	user, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	if got := scopeArgs(user); got != " --user" {
		t.Errorf("user scope: %q", got)
	}
	here := t.TempDir()
	if out, err := exec.Command("git", "-C", here, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	t.Chdir(here)
	if got := scopeArgs(ScopeRepo(findGitRoot(here))); got != "" {
		t.Errorf("control: the repo you stand in needs no flag, got %q", got)
	}
	other := filepath.Join(t.TempDir(), "a repo")
	if got := scopeArgs(ScopeRepo(other)); got != " --target "+shellQuote(other) || !strings.Contains(got, "'") {
		t.Errorf("another repo: %q", got)
	}
}

// The state names what is installed; the lock is read only when it names none.
func TestInstalledSourcePrefersState(t *testing.T) {
	consentEnv(t)
	installed, declared, root := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	lock := `{"contractVersion":"1","source":"` + declared + `"}`
	if err := os.WriteFile(filepath.Join(root, ".nav-pilot", "agentpakke.lock.json"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	scope := ScopeRepo(root)
	src, err := installedSource(scope, &StateFile{SourceRepo: installed})
	if err != nil || src.Dir != installed {
		t.Fatalf("with a source in state: %v, %+v, want %s", err, src, installed)
	}
	src, err = installedSource(scope, &StateFile{})
	if err != nil || src.Dir != declared {
		t.Fatalf("control, no source in state: %v, %+v, want the lock's %s", err, src, declared)
	}
}

func TestPakkeAccessRefusesDryRun(t *testing.T) {
	isolatedConfig(t)
	err := run([]string{"pakke", "access", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "no --dry-run") {
		t.Errorf("err = %v", err)
	}
	// Control: bump-base keeps its own refusals, --user included.
	if err := run([]string{"pakke", "bump-base", "--user"}); err == nil || !strings.Contains(err.Error(), "no --user") {
		t.Errorf("bump-base --user: %v", err)
	}
}

// pakke access exits non-zero when the answer was not saved.
func TestPutProposalReturnsWriteErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes into read-only directories")
	}
	scope := consentEnv(t)
	src := proposeSource(t, `"cloud.nais.io"`, "")
	answering(t, false)
	if err := putProposal(scope, src, false, false, false); err != nil {
		t.Fatalf("control: a saved answer returns %v", err)
	}
	dir := filepath.Dir(artifacts.ProposalConsentPath())
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	answering(t, true)
	if err := putProposal(scope, src, false, false, true); err == nil {
		t.Error("an answer that could not be saved returned no error")
	}
}

// nav-pilot pakke reads a full-sha pin from the pinned cache: after the first
// fetch it needs no network. The control is the first read, which does fetch.
func TestPakkeSourceIsOfflineAfterFirstFetch(t *testing.T) {
	consentEnv(t)
	origin := t.TempDir()
	manifest := `{"contractVersion":"1","name":"plattform","description":"plattform","layout":{"agents":"agents"},"clients":{"copilot":{"primaryAgents":["grillmester"]}}}`
	if err := os.MkdirAll(filepath.Join(origin, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(origin, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, ".nav-pilot", "agentpakke.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "agents", "grillmester.agent.md"), []byte("---\nname: grillmester\ndescription: x\n---\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", origin, "-c", "user.email=t@example.test", "-c", "user.name=t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", ".")
	git("add", "-A")
	git("commit", "-qm", "innhold")
	sha := git("rev-parse", "HEAD")

	prevCache, prevURL := source.CacheDir, source.RemoteURLFn
	t.Cleanup(func() { source.CacheDir, source.RemoteURLFn = prevCache, prevURL })
	source.CacheDir = t.TempDir()
	source.RemoteURLFn = func(string) string { return origin }

	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	state := &StateFile{Collection: "plattform", SourceRepo: "navikt/plattform", SourceSHA: sha}
	first, err := installedSource(scope, state)
	if err != nil || first.Pakke == nil || first.Pakke.Name != "plattform" {
		t.Fatalf("first read: %v, %+v", err, first)
	}
	first.Cleanup()

	source.RemoteURLFn = func(string) string { return filepath.Join(origin, "gone") }
	again, err := installedSource(scope, state)
	if err != nil || again.Pakke == nil || again.Pakke.Name != "plattform" {
		t.Fatalf("second read went to the network: %v", err)
	}
	again.Cleanup()

	// Control: without the cache the same read needs the gone remote.
	source.CacheDir = ""
	if src, err := installedSource(scope, state); err == nil {
		src.Cleanup()
		t.Fatal("control: a read without the cache succeeded with no remote")
	}
}
