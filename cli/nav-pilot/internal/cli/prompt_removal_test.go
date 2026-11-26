package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// TestSyncRemovesInstalledPromptAndNamesTheSkill: the source dropped its prompt
// files (#604). Sync removes the one nav-pilot installed and tracks, says once
// which skill replaces it, and leaves a prompt the user wrote alone.
func TestSyncRemovesInstalledPromptAndNamesTheSkill(t *testing.T) {
	isolatedConfig(t)
	dir := repoTarget(t)
	sourceDir := t.TempDir()

	installed := filepath.Join(dir, ".github", "prompts", "kafka-topic.prompt.md")
	mine := filepath.Join(dir, ".github", "prompts", "mine.prompt.md")
	mustWrite(t, installed, "# Kafka topic\n")
	mustWrite(t, mine, "# My own prompt\n")
	hash, err := rawArtifactHash(installed, false)
	if err != nil {
		t.Fatal(err)
	}
	writeState(dir, &StateFile{
		Collection: "kotlin-backend",
		Version:    "2026.06",
		SourceRepo: "my-custom/repo",
		Files:      []InstalledFile{{Path: ".github/prompts/kafka-topic.prompt.md", Hash: hash}},
	})

	mustWrite(t, filepath.Join(sourceDir, "skills", "kafka", "SKILL.md"), "---\nname: kafka\n---\n")
	writeRetired(t, sourceDir, `{"paths":{"prompts/kafka-topic.prompt.md":["`+blobHash([]byte("# Kafka topic\n"))+`"]},"replacements":{"prompts/kafka-topic.prompt.md":"kafka"}}`)

	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "new-sha", Version: "2026.07", Repo: "my-custom/repo"}, nil
	}

	scope := ScopeRepo(dir)
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(installed); !os.IsNotExist(statErr) {
		t.Errorf("sync kept the installed prompt: %v", statErr)
	}
	if _, statErr := os.Stat(mine); statErr != nil {
		t.Errorf("sync touched a prompt the user wrote: %v", statErr)
	}
	note := "use the kafka skill instead"
	if n := strings.Count(out, note); n != 1 {
		t.Errorf("want the replacement named exactly once, got %d:\n%s", n, out)
	}

	// The next sync has nothing to say about it.
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if strings.Contains(out, note) {
		t.Errorf("second sync repeated the notice:\n%s", out)
	}
}
