package artifacts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The remote is a local git repo with two revisions: reuse must fetch the pin,
// not its newer main. A second, local base exercises nested reuse as well.
func composedOpenCodeFixture(t *testing.T) (team string, checkCleanup func()) {
	t.Helper()
	base, deeper := t.TempDir(), t.TempDir()
	team = t.TempDir()
	writePackage := func(dir, name string) {
		mustWrite(t, filepath.Join(dir, ".nav-pilot/agentpakke.json"), fmt.Sprintf(`{"contractVersion":"1","name":%q,"description":"fixture","layout":{"agents":"content/agents","skills":"content/skills","prompts":"content/prompts","instructions":"content/instructions"},"clients":{"opencode":{"primaryAgents":[%q]}}}`, name, name))
		mustWrite(t, filepath.Join(dir, "content/agents", name+".agent.md"), "---\ndescription: fixture\n---\n"+name)
	}
	writePackage(base, "base-persona")
	writePackage(team, "team-persona")
	writePackage(deeper, "deep-persona")
	mustWrite(t, filepath.Join(base, ".nav-pilot/agentpakke.lock.json"), fmt.Sprintf(`{"contractVersion":"1","source":%q}`, deeper))
	for _, item := range []struct{ dir, name, body string }{
		{base, "observability-debugging", "pinned skill"},
		{base, "collision", "base collision"},
		{team, "collision", "team collision"},
		{team, "team-skill", "team skill"},
		{deeper, "deep-skill", "deep skill"},
	} {
		mustWrite(t, filepath.Join(item.dir, "content/skills", item.name, "SKILL.md"), item.body)
	}
	mustWrite(t, filepath.Join(base, "content/skills/observability-debugging/scripts/query.sh"), "echo pinned\n")
	mustWrite(t, filepath.Join(base, "content/skills/observability-debugging/reference/guide.md"), "pinned reference")
	mustWrite(t, filepath.Join(base, "content/skills/collision/base-only.md"), "must not leak")
	mustWrite(t, filepath.Join(base, "content/prompts/base-command.prompt.md"), "---\ndescription: command\n---\npinned command")
	mustWrite(t, filepath.Join(base, "content/instructions/base-global.instructions.md"), "---\napplyTo: '**'\n---\npinned global")
	mustWrite(t, filepath.Join(base, "content/instructions/base-scoped.instructions.md"), "---\napplyTo: '**/*.go'\n---\npinned scoped")
	mustWrite(t, filepath.Join(base, "copilot-instructions.md"), "pinned root instructions")
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = base
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "pinned")
	sha := git("rev-parse", "HEAD")
	mustWrite(t, filepath.Join(base, "content/skills/observability-debugging/SKILL.md"), "wrong main revision")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "new main")
	mustWrite(t, filepath.Join(team, ".nav-pilot/agentpakke.lock.json"), fmt.Sprintf(`{"contractVersion":"1","source":"fixture/base","sha":%q}`, sha))
	oldURL, oldClone := source.RemoteURLFn, source.CloneRemoteFn
	var checkouts []string
	source.RemoteURLFn = func(repo string) string {
		if repo != "fixture/base" {
			t.Fatalf("unexpected remote: %s", repo)
		}
		return base
	}
	source.CloneRemoteFn = func(ref, repo string) (*source.Source, error) {
		if ref != sha || repo != "fixture/base" {
			t.Fatalf("fetch %s@%s, want pinned base %s", repo, ref, sha)
		}
		src, err := oldClone(ref, repo)
		if err == nil {
			checkouts = append(checkouts, src.TempDir)
		}
		return src, err
	}
	t.Cleanup(func() { source.RemoteURLFn, source.CloneRemoteFn = oldURL, oldClone })
	return team, func() {
		t.Helper()
		if len(checkouts) == 0 {
			t.Error("reuse never resolved the pinned base")
		}
		for _, dir := range checkouts {
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("base checkout not cleaned: %s (%v)", dir, err)
			}
		}
	}
}

func assertComposedOpenCode(t *testing.T, out string, project bool) {
	t.Helper()
	want := map[string]string{
		"skills/observability-debugging/SKILL.md":           "pinned skill",
		"skills/observability-debugging/scripts/query.sh":   "echo pinned",
		"skills/observability-debugging/reference/guide.md": "pinned reference",
		"skills/collision/SKILL.md":                         "team collision",
		"skills/team-skill/SKILL.md":                        "team skill",
		"skills/deep-skill/SKILL.md":                        "deep skill",
		"agents/team-persona.md":                            "mode: primary",
		"agents/base-persona.md":                            "mode: subagent",
		"agents/deep-persona.md":                            "mode: subagent",
		"commands/base-command.md":                          "pinned command",
		"instructions/base-scoped.md":                       "pinned scoped",
		"AGENTS.md":                                         "pinned global",
	}
	for path, text := range want {
		data, err := os.ReadFile(filepath.Join(out, path))
		if err != nil || !strings.Contains(string(data), text) {
			t.Errorf("%s: want %q, got %q (%v)", path, text, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "skills/collision/base-only.md")); !os.IsNotExist(err) {
		t.Error("shadowed base skill's supporting file leaked")
	}
	agents, _ := os.ReadFile(filepath.Join(out, "AGENTS.md"))
	if !strings.Contains(string(agents), "pinned root instructions") {
		t.Error("inherited root instructions absent")
	}
	if strings.Contains(string(agents), "repo-only instruction") != project {
		t.Errorf("repository instruction handling: project=%v, got %s", project, agents)
	}
	refDir := filepath.Join(out, "instructions")
	if project {
		refDir = ".opencode/instructions"
	}
	if !strings.Contains(string(agents), filepath.ToSlash(filepath.Join(refDir, "base-scoped.md"))) {
		t.Error("wrong scoped instruction reference")
	}
}

func TestComposedOpenCodeMaterialization(t *testing.T) {
	for _, client := range []string{"opencode", "pi", "materialize"} {
		t.Run(client, func(t *testing.T) {
			team, cleanup := composedOpenCodeFixture(t)
			out, scope := t.TempDir(), t.TempDir()
			mustWrite(t, filepath.Join(scope, "skills/scope-extra/SKILL.md"), "scope extra")
			mustWrite(t, filepath.Join(scope, "skills/observability-debugging/SKILL.md"), "scope must not shadow base")
			mustWrite(t, filepath.Join(scope, "instructions/repo.instructions.md"), "repo-only instruction")
			if client == "materialize" {
				if _, _, _, _, err := MaterializeOpenCode(team, out, team); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, _, _, _, _, err := SyncOpenCodeArtifacts(client, team, scope, out, "fixture", "team-sha", team); err != nil {
					t.Fatal(err)
				}
			}
			assertComposedOpenCode(t, out, false)
			cleanup()
			if client == "materialize" {
				return
			}
			state, err := ReadOpenCodeState(out)
			if err != nil || state == nil {
				t.Fatalf("state: %v", err)
			}
			if state.SourceRepo != team || state.SourceSHA != "team-sha" {
				t.Error("state lost top-level identity")
			}
			if !slices.ContainsFunc(state.Files, func(f domain.InstalledFile) bool { return f.Path == "skills/observability-debugging/" }) {
				t.Error("inherited skill not tracked")
			}
			// Explicit sync and dry-run must keep inherited files; a local edit
			// must remain conflicted across repeated regeneration.
			r, err := SyncOpenCodeArtifactsReport(client, team, scope, out, "fixture", "team-sha", team, true)
			if err != nil || len(r.Removed) != 0 {
				t.Fatalf("dry run: %+v, %v", r, err)
			}
			assertComposedOpenCode(t, out, false)
			mustWrite(t, filepath.Join(out, "skills/observability-debugging/reference/guide.md"), "user edit")
			for range 2 {
				r, err = SyncOpenCodeArtifactsReport(client, team, scope, out, "fixture", "team-sha", team, false)
				if err != nil || !slices.Contains(r.Conflicts, "skills/observability-debugging/") || len(r.Removed) != 0 {
					t.Fatalf("regeneration: %+v, %v", r, err)
				}
				data, _ := os.ReadFile(filepath.Join(out, "skills/observability-debugging/reference/guide.md"))
				if string(data) != "user edit" {
					t.Error("user edit overwritten")
				}
			}
			cleanup()
		})
	}
}

func TestComposedOpenCodeExport(t *testing.T) {
	team, cleanup := composedOpenCodeFixture(t)
	scope := &domain.InstallScope{Name: "repo", RootDir: t.TempDir()}
	mustWrite(t, filepath.Join(scope.DstPath(), "instructions/repo.instructions.md"), "repo-only instruction")
	if err := ExportOpenCode(scope, "", team, "fixture", false, false, true); err != nil {
		t.Fatal(err)
	}
	assertComposedOpenCode(t, OpenCodeOutputDir(scope), true)
	cleanup()
}

func TestComposedOpenCodeInstructionOverrides(t *testing.T) {
	team, cleanup := composedOpenCodeFixture(t)
	mustWrite(t, filepath.Join(team, "copilot-instructions.md"), "team root instructions")
	mustWrite(t, filepath.Join(team, "content/instructions/base-global.instructions.md"), "team global instructions")
	mustWrite(t, filepath.Join(team, "content/instructions/base-scoped.instructions.md"), "---\napplyTo: '**/*.go'\n---\nteam scoped instructions")
	out := t.TempDir()
	if _, _, _, _, _, err := SyncOpenCodeArtifacts("opencode", team, "", out, "fixture", "", team); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(filepath.Join(out, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"team root instructions", "team global instructions"} {
		if !strings.Contains(string(agents), text) {
			t.Errorf("missing overriding %q", text)
		}
	}
	if strings.Contains(string(agents), "pinned global") || strings.Contains(string(agents), "pinned root instructions") {
		t.Error("shadowed base instructions leaked")
	}
	scoped, err := os.ReadFile(filepath.Join(out, "instructions/base-scoped.md"))
	if err != nil || !strings.Contains(string(scoped), "team scoped instructions") {
		t.Errorf("scoped override: %q (%v)", scoped, err)
	}
	cleanup()
}

func TestOpenCodeRemoteSelfDeclarationDoesNotCompose(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(fmt.Sprintf("pinned=%v", pinned), func(t *testing.T) {
			team := t.TempDir()
			mustWrite(t, filepath.Join(team, ".nav-pilot/agentpakke.json"), `{"contractVersion":"1","name":"team","description":"fixture","layout":{"agents":"agents","skills":"skills"},"clients":{"opencode":{"primaryAgents":["team"]}}}`)
			mustWrite(t, filepath.Join(team, "skills/team/SKILL.md"), "current team skill")
			pin := ""
			if pinned {
				pin = `,"sha":"` + strings.Repeat("a", 40) + `"`
			}
			mustWrite(t, filepath.Join(team, ".nav-pilot/agentpakke.lock.json"), `{"contractVersion":"1","source":"FIXTURE/Team"`+pin+`}`)
			oldClone := source.CloneRemoteFn
			t.Cleanup(func() { source.CloneRemoteFn = oldClone })
			// Only explicit export resolves the top source. Any attempt to fetch
			// its self-declaration would resurrect content from an older revision.
			calls := 0
			source.CloneRemoteFn = func(ref, repo string) (*source.Source, error) {
				calls++
				if ref != "" || repo != "fixture/team" || calls != 1 {
					t.Fatalf("unexpected self-reuse fetch: %s@%s", repo, ref)
				}
				return &source.Source{Dir: team, Repo: repo}, nil
			}
			out := t.TempDir()
			if _, _, _, _, err := MaterializeOpenCode(team, out, "fixture/team"); err != nil {
				t.Fatal(err)
			}
			if _, _, _, _, _, err := SyncOpenCodeArtifacts("opencode", team, "", out, "fixture", "", "fixture/team"); err != nil {
				t.Fatal(err)
			}
			scope := &domain.InstallScope{Name: "repo", RootDir: t.TempDir()}
			if err := ExportOpenCode(scope, "", "fixture/team", "fixture", false, false, true); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Errorf("source fetches = %d, want only the export's top source", calls)
			}
		})
	}
}

func TestComposedOpenCodeRefusalsDoNotWrite(t *testing.T) {
	oldClone := source.CloneRemoteFn
	t.Cleanup(func() { source.CloneRemoteFn = oldClone })
	source.CloneRemoteFn = func(ref, repo string) (*source.Source, error) {
		return nil, fmt.Errorf("unexpected remote fetch: %s@%s", repo, ref)
	}
	for _, failure := range []string{"missing-pin", "cycle", "payload-base", "broken-declaration"} {
		t.Run(failure, func(t *testing.T) {
			team, base := t.TempDir(), t.TempDir()
			manifest := `{"contractVersion":"1","name":"fixture","description":"fixture","layout":{"agents":"agents","skills":"skills"},"clients":{"opencode":{"primaryAgents":["fixture"]}}}`
			mustWrite(t, filepath.Join(team, ".nav-pilot/agentpakke.json"), manifest)
			mustWrite(t, filepath.Join(team, "skills/team/SKILL.md"), "must not be written")
			mustWrite(t, filepath.Join(base, ".nav-pilot/agentpakke.json"), manifest)
			declaration := fmt.Sprintf(`{"contractVersion":"1","source":%q}`, base)
			want := ""
			switch failure {
			case "missing-pin":
				declaration = `{"contractVersion":"1","source":"fixture/unpinned"}`
				want = "without a revision"
			case "cycle":
				mustWrite(t, filepath.Join(base, ".nav-pilot/agentpakke.lock.json"), fmt.Sprintf(`{"contractVersion":"1","source":%q}`, team))
				want = "cycle"
			case "payload-base":
				mustWrite(t, filepath.Join(base, ".nav-pilot/agentpakke.json"), `{"contractVersion":"1","name":"payload","description":"fixture","clients":{"opencode":{"payloads":{"full":{"path":"payload","primaryAgents":["fixture"]}}}}}`)
				want = "Tier 2"
			case "broken-declaration":
				declaration = "not JSON"
				want = "reuse declaration"
			}
			mustWrite(t, filepath.Join(team, ".nav-pilot/agentpakke.lock.json"), declaration)
			for _, path := range []string{"sync", "export", "materialize"} {
				t.Run(path, func(t *testing.T) {
					scope := &domain.InstallScope{Name: "repo", RootDir: t.TempDir()}
					out := OpenCodeOutputDir(scope)
					mustWrite(t, filepath.Join(out, "skills/untouched/SKILL.md"), "user file")
					var err error
					switch path {
					case "sync":
						_, _, _, _, _, err = SyncOpenCodeArtifacts("opencode", team, "", out, "fixture", "", team)
					case "export":
						err = ExportOpenCode(scope, "", team, "fixture", false, true, true)
					case "materialize":
						_, _, _, _, err = MaterializeOpenCode(team, out, team)
					}
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("want %q refusal, got %v", want, err)
					}
					if _, err := os.Stat(filepath.Join(out, "skills/team")); !os.IsNotExist(err) {
						t.Error("composition error partially wrote team content")
					}
					data, err := os.ReadFile(filepath.Join(out, "skills/untouched/SKILL.md"))
					if err != nil || string(data) != "user file" {
						t.Error("composition refusal changed user content")
					}
				})
			}
		})
	}
}

// A dry run's Unchanged says whether --apply would write anything, so an
// edit that keeps the source SHA (a scope extra, here) still counts.
func TestOpenCodeSyncReportUnchanged(t *testing.T) {
	team, cleanup := composedOpenCodeFixture(t)
	defer cleanup()
	out, scope := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(scope, "skills/scope-extra/SKILL.md"), "scope extra")
	mustWrite(t, filepath.Join(scope, "instructions/repo.instructions.md"), "repo-only instruction")
	if r, err := SyncOpenCodeArtifactsReport("opencode", team, scope, out, "fixture", "team-sha", team, true); err != nil || r.Unchanged {
		t.Fatalf("first dry run: %+v, %v", r, err)
	}
	if _, err := SyncOpenCodeArtifactsReport("opencode", team, scope, out, "fixture", "team-sha", team, false); err != nil {
		t.Fatal(err)
	}
	if r, err := SyncOpenCodeArtifactsReport("opencode", team, scope, out, "fixture", "team-sha", team, true); err != nil || !r.Unchanged {
		t.Fatalf("dry run after apply: %+v, %v", r, err)
	}
	// A scope extra (a skill dir) and a source file, both without a new SHA.
	for _, edit := range []string{filepath.Join(scope, "skills/scope-extra/SKILL.md"), filepath.Join(team, "content/agents/team-persona.agent.md")} {
		mustWrite(t, edit, "---\ndescription: edited\n---\nedited")
		if r, err := SyncOpenCodeArtifactsReport("opencode", team, scope, out, "fixture", "team-sha", team, true); err != nil || r.Unchanged {
			t.Fatalf("%s edited, dry run: %+v, %v", edit, r, err)
		}
		if _, err := SyncOpenCodeArtifactsReport("opencode", team, scope, out, "fixture", "team-sha", team, false); err != nil {
			t.Fatal(err)
		}
	}
}
