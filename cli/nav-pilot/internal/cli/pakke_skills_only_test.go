package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// skillsOnlyManifestJSON is a pakke that ships no agent at all: no
// primaryAgents, no layout.agents. The contract used to make that shape
// unrepresentable, so a team publishing a skills pack had to invent a persona
// to hold it (#799).
const skillsOnlyManifestJSON = `{
  "contractVersion": "1",
  "name": "grillpakka",
  "description": "Bare skills",
  "clients": {"copilot": {}},
  "layout": {"skills": "plugin/skills"}
}`

// skillsOnlySourceTree builds a checkout of that pakke with one skill in it.
func skillsOnlySourceTree(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), skillsOnlyManifestJSON)
	mustWrite(t, filepath.Join(dir, "plugin", "skills", "grilling", "SKILL.md"), body)
	return dir
}

func skillsOnlySource(t *testing.T, dir, sha string) *Source {
	t.Helper()
	src := &Source{Dir: dir, SHA: sha, Version: "dev", Repo: "navikt/grillpakka"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke on a skills-only pakke: %v", err)
	}
	if src.Pakke == nil {
		t.Fatal("attachPakke left no manifest on a skills-only pakke")
	}
	return src
}

// A pakke with no agent installs its content in both scopes and syncs like any
// other Tier 1 pakke.
func TestSkillsOnlyPakkeInstallsInBothScopes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope func(t *testing.T) (*InstallScope, string)
	}{
		{"repo", func(t *testing.T) (*InstallScope, string) {
			target := repoTarget(t)
			return ScopeRepo(target), filepath.Join(target, ".github")
		}},
		{"user", func(t *testing.T) (*InstallScope, string) {
			scope, err := ScopeUser()
			if err != nil {
				t.Fatal(err)
			}
			return scope, scope.RootDir
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolatedConfig(t)
			t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
			scope, dir := tc.scope(t)

			src := skillsOnlySource(t, skillsOnlySourceTree(t, "# Grilling\n"), "sha-one")
			captureStdoutFor(t, func() {
				if err := cmdInstallFromSource("grillpakka", src, scope, false, false, false); err != nil {
					t.Fatalf("cmdInstallFromSource: %v", err)
				}
			})

			skill := filepath.Join(dir, "skills", "grilling", "SKILL.md")
			if _, err := os.Stat(skill); err != nil {
				t.Fatalf("the skill was not installed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "agents")); err == nil {
				t.Error("a pakke with no agents materialized an agents directory")
			}

			// And it syncs: the upstream skill changes, --apply lands it.
			pinnedSyncSource(t, skillsOnlySourceTree(t, "# Grilling, revidert\n"), "sha-two")
			captureStdoutFor(t, func() {
				if err := cmdSync(scope, "", "", true, false); err != nil {
					t.Fatalf("cmdSync --apply: %v", err)
				}
			})
			got, err := os.ReadFile(skill)
			if err != nil {
				t.Fatalf("reading the synced skill: %v", err)
			}
			if !strings.Contains(string(got), "revidert") {
				t.Errorf("sync did not update the skill, got %q", got)
			}
		})
	}
}

// There is no persona to hand the client, so the launch refuses — naming the
// pakke, and in words nobody can mistake for a manifest that failed to load.
//
// This drives launchClientConfirming rather than the check directly: the check
// belongs at the common launch boundary, and a unit test on it stays green if
// the call is deleted from the launch path. The fake client records every
// non-probe invocation, so the assertion is both halves — refused, and refused
// before the client starts (#799).
func TestSkillsOnlyPakkeRefusesToLaunch(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	launched := fakeCopilotOnlyOnPath(t)
	stubResolveSource(t, skillsOnlySource(t, skillsOnlySourceTree(t, "# Grilling\n"), "sha-one"))

	err := launchClientConfirming(ResolvedConfig{Client: "copilot", Source: "navikt/grillpakka"}, false)
	if err == nil {
		t.Fatal("a pakke that ships no agent must refuse to launch")
	}
	if !strings.Contains(err.Error(), "grillpakka") {
		t.Errorf("the refusal must name the pakke, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no agent") {
		t.Errorf("the refusal must say the pakke ships no agent, got: %v", err)
	}
	// A broken or unloadable manifest refuses in the contract's words; this is
	// a conforming manifest, and telling the author to go fix it sends them
	// looking for a fault that is not there.
	for _, wrong := range []string{"does not conform", "agentpakke contract"} {
		if strings.Contains(err.Error(), wrong) {
			t.Errorf("the refusal reads like a manifest failure (%q), got: %v", wrong, err)
		}
	}
	if _, err := os.Stat(launched); err == nil {
		t.Error("the client was launched even though the pakke ships no agent")
	}
}
