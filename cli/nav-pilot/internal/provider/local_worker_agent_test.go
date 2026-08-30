package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// TestLocalWorkerAgentPath: dispatch needs the agent to exist, and its absence
// is silent — the launch just runs in the cloud. This is the only thing that
// can tell a developer why.
func TestLocalWorkerAgentPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, found := LocalWorkerAgentPath()
	if found {
		t.Errorf("reported the agent present in an empty config dir (%s)", path)
	}
	want := filepath.Join(dir, "opencode", "agents", local.WorkerAgent+".md")
	if path != want {
		t.Errorf("looked in %s, want %s", path, want)
	}

	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("---\nname: local-worker\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found := LocalWorkerAgentPath(); !found {
		t.Error("did not find the agent that is there")
	}

	// A directory of that name is not an agent opencode can load.
	os.Remove(want)
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, found := LocalWorkerAgentPath(); found {
		t.Error("counted a directory as the agent file")
	}
}
