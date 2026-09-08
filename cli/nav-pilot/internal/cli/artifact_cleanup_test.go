package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// Å slette hook-skriptet uten å fjerne aktiveringsoppføringa lot Copilot CLI
// kjøre en fil som ikke finnes, ved hvert verktøykall som traff matcheren.
func TestRemovingAHookScriptAlsoDeactivatesIt(t *testing.T) {
	root := t.TempDir()
	scope := ScopeRepo(root)
	hooksDir := scope.DstPath(KindHook.Dir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := source.MergeRepoHooks(hooksDir, []source.HookEntry{
		{Name: "vakt", Matcher: "Bash", Command: "python3 vakt.py"},
		{Name: "annen", Matcher: "Bash", Command: "python3 annen.py"},
	}); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(hooksDir, "vakt"+KindHook.Suffix)
	if err := os.WriteFile(script, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Remove(script)
	afterArtifactRemoved(scope, script, false)

	names := source.HookNamesIn(filepath.Join(hooksDir, source.RepoHooksConfig))
	if len(names) != 1 || names[0] != "annen" {
		t.Errorf("oppføringer igjen = %v, ventet bare [annen]", names)
	}
}

// Kontroll: bare den navngitte hooken skal ut. Fjernet vi alle, ville en
// pensjonering av én hook slått av hele teamets øvrige hooks.
func TestRemovingOneHookLeavesTheOthers(t *testing.T) {
	root := t.TempDir()
	scope := ScopeRepo(root)
	hooksDir := scope.DstPath(KindHook.Dir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := source.MergeRepoHooks(hooksDir, []source.HookEntry{
		{Name: "en", Matcher: "Bash", Command: "x"},
		{Name: "to", Matcher: "Bash", Command: "x"},
		{Name: "tre", Matcher: "Bash", Command: "x"},
	}); err != nil {
		t.Fatal(err)
	}
	afterArtifactRemoved(scope, filepath.Join(hooksDir, "to"+KindHook.Suffix), false)

	names := source.HookNamesIn(filepath.Join(hooksDir, source.RepoHooksConfig))
	if strings.Join(names, ",") != "en,tre" {
		t.Errorf("oppføringer igjen = %v, ventet [en tre]", names)
	}
}

// SKILL.md er det som gjør katalogen til en skill. Fjernet vi bare markøren,
// ble en katalog liggende som ikke lenger er en skill og som ingenting rydder.
func TestRemovingASkillMarkerTakesTheDirectory(t *testing.T) {
	root := t.TempDir()
	scope := ScopeRepo(root)
	skillDir := filepath.Join(scope.DstPath(KindSkill.Dir), "gammel")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(skillDir, KindSkill.Marker)
	if err := os.WriteFile(marker, []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "helper.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Remove(marker)
	afterArtifactRemoved(scope, marker, false)

	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		rest, _ := os.ReadDir(skillDir)
		names := make([]string, 0, len(rest))
		for _, e := range rest {
			names = append(names, e.Name())
		}
		t.Errorf("skill-katalogen ble liggende igjen med %v", names)
	}
}

// Kontroll: en vanlig fil skal ikke dra med seg katalogen sin.
func TestRemovingAnAgentLeavesItsDirectory(t *testing.T) {
	root := t.TempDir()
	scope := ScopeRepo(root)
	agentsDir := scope.DstPath(KindAgent.Dir)
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(agentsDir, "annen.agent.md")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	afterArtifactRemoved(scope, filepath.Join(agentsDir, "borte.agent.md"), false)

	if _, err := os.Stat(other); err != nil {
		t.Errorf("naboartefaktet forsvant: %v", err)
	}
}
