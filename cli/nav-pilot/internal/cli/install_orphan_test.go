package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// An artifact renamed upstream used to stay installed forever: state is rebuilt
// from what the install wrote, so the old path fell out of it, and sync only
// walks the state. Copilot CLI listed both lokal-arbeider and local-worker for
// three days because of this.
func TestRemoveOrphansDeletesWhatTheSourceStoppedShipping(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) string {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		h, err := source.RawArtifactHash(full, false)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}

	renamedAway := write("agents/lokal-arbeider.agent.md", "the old name")
	edited := write("agents/kafka.agent.md", "the developer changed this")
	kept := write("agents/local-worker.agent.md", "the new name")

	prior := &StateFile{Files: []domain.InstalledFile{
		{Path: "agents/lokal-arbeider.agent.md", Hash: renamedAway},
		// Recorded with the hash it had when nav-pilot wrote it, which is not
		// what is on disk now.
		{Path: "agents/kafka.agent.md", Hash: "sha256:something-else"},
		{Path: "agents/local-worker.agent.md", Hash: kept},
	}}
	_ = edited

	scope := &InstallScope{RootDir: root}
	removeOrphans(scope, prior, []domain.InstalledFile{
		{Path: "agents/local-worker.agent.md", Hash: kept},
	})

	if _, err := os.Stat(filepath.Join(root, "agents/lokal-arbeider.agent.md")); !os.IsNotExist(err) {
		t.Error("the renamed-away artifact is still installed")
	}
	for _, keep := range []string{"agents/kafka.agent.md", "agents/local-worker.agent.md"} {
		if _, err := os.Stat(filepath.Join(root, keep)); err != nil {
			t.Errorf("%s should have been left alone: %v", keep, err)
		}
	}
}
