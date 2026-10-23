package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

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
