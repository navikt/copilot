package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// fakeBaseAPI serves the GitHub API the freshness check reads: an empty
// releases list for every repo (no release metadata, so the default branch is
// the target), the files in files keyed "repo/path", and one compare answer.
// It returns a counter of compare requests.
func fakeBaseAPI(t *testing.T, files map[string]string, compare string) *int {
	t.Helper()
	realBaseFreshness(t)
	compares := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/repos/")
		switch {
		case strings.HasSuffix(p, "/releases"):
			fmt.Fprint(w, "[]")
		case strings.Contains(p, "/compare/"):
			*compares++
			fmt.Fprint(w, compare)
		case strings.Contains(p, "/contents/"):
			body, ok := files[strings.Replace(p, "/contents/", "/", 1)]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			fmt.Fprint(w, body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	orig := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = orig })
	return compares
}

// realBaseFreshness undoes TestMain's offline stubs for one test.
func realBaseFreshness(t *testing.T) {
	t.Helper()
	origLag, origFile := lookupBaseLag, githubFileJSON
	t.Cleanup(func() { lookupBaseLag, githubFileJSON = origLag, origFile })
	lookupBaseLag, githubFileJSON = lookupBaseLagHTTP, githubFileJSONHTTP
}

// staleCompare is GitHub's answer for a pin 11 commits behind, the oldest of
// them 16 days old.
func staleCompare() string {
	return fmt.Sprintf(`{"status":"ahead","ahead_by":11,"commits":[{"commit":{"committer":{"date":%q}}}]}`,
		time.Now().Add(-16*24*time.Hour-time.Hour).UTC().Format(time.RFC3339))
}

const basePin = "6dc457badd90b781fa707f5b2a3700144859b839"

func TestBaseLagStaleCurrentAndOffline(t *testing.T) {
	t.Run("stale", func(t *testing.T) {
		fakeBaseAPI(t, nil, staleCompare())
		out := captureStdoutFor(t, func() {
			warnBaseLag(context.Background(), os.Stdout, "", "repo", "nais/pilot", "navikt/copilot", "nav-pilot", basePin)
		})
		for _, want := range []string{"nais/pilot pins navikt/copilot at 6dc457b", agentpakke.DeclarationPath,
			"11 commit(s) and 16 day(s) behind its default branch", "owners of nais/pilot should bump it"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Count(out, "\n") != 1 {
			t.Errorf("want one line, got:\n%s", out)
		}
	})
	t.Run("current", func(t *testing.T) {
		fakeBaseAPI(t, nil, `{"status":"identical","ahead_by":0}`)
		if out := captureStdoutFor(t, func() {
			warnBaseLag(context.Background(), os.Stdout, "", "repo", "nais/pilot", "navikt/copilot", "nav-pilot", basePin)
		}); out != "" {
			t.Errorf("a current pin printed:\n%s", out)
		}
	})
	t.Run("offline", func(t *testing.T) {
		realBaseFreshness(t)
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close() // nothing listens: connection refused, as offline
		orig := githubAPIBase
		githubAPIBase = srv.URL
		t.Cleanup(func() { githubAPIBase = orig })
		lag, err := lookupBaseLag(context.Background(), "navikt/copilot", "nav-pilot", basePin)
		if err == nil || lag != nil {
			t.Fatalf("lookup against a dead host = %v, %v; want an error", lag, err)
		}
		out := captureStdoutFor(t, func() {
			warnBaseLag(context.Background(), os.Stdout, "", "repo", "nais/pilot", "navikt/copilot", "nav-pilot", basePin)
		})
		if out != "" {
			t.Errorf("offline must be silent, printed:\n%s", out)
		}
	})
}

// A path-shaped base is a working tree with no revision: no request at all.
func TestBaseLagSkipsPathBase(t *testing.T) {
	compares := fakeBaseAPI(t, nil, staleCompare())
	warnBaseLag(context.Background(), os.Stdout, "", "repo", "nais/pilot", t.TempDir(), "x", basePin)
	if *compares != 0 {
		t.Error("a path base was looked up")
	}
}

// doctor reads the lock and the base's name from GitHub at the revisions the
// scope's state records, and warns without failing.
func TestDoctorWarnsAboutAStaleBase(t *testing.T) {
	pakkeSHA := strings.Repeat("a", 40)
	fakeBaseAPI(t, map[string]string{
		"nais/pilot/" + agentpakke.DeclarationPath:  `{"contractVersion":"1","source":"navikt/copilot","sha":"` + basePin + `"}`,
		"navikt/copilot/" + agentpakke.ManifestPath: `{"contractVersion":"1","name":"nav-pilot"}`,
	}, staleCompare())
	state := &StateFile{SourceRepo: "nais/pilot", SourceSHA: pakkeSHA, Collection: "pilot",
		Files: []InstalledFile{{Path: ".github/agents/x.agent.md", Hash: "h"}}}
	out := captureStdoutFor(t, func() { reportScopeBaseLag(ScopeRepo(repoTarget(t)), state) })
	if !strings.Contains(out, "nais/pilot pins navikt/copilot at 6dc457b") {
		t.Errorf("doctor did not warn:\n%s", out)
	}
	// A Tier 1 install with no files left has the shape of a pin, but no
	// revision on disk, and is still checked.
	state.Files = nil
	out = captureStdoutFor(t, func() { reportScopeBaseLag(ScopeRepo(repoTarget(t)), state) })
	if !strings.Contains(out, "nais/pilot pins navikt/copilot at 6dc457b") {
		t.Errorf("doctor skipped a Tier 1 scope with no files:\n%s", out)
	}

	// The default source reuses nothing and is never asked about.
	compares := fakeBaseAPI(t, nil, staleCompare())
	reportScopeBaseLag(ScopeRepo(repoTarget(t)), &StateFile{SourceRepo: "navikt/copilot", SourceSHA: pakkeSHA})
	if *compares != 0 {
		t.Error("doctor looked up a base for the default source")
	}
}

// gitRepoWith commits each tree in turn to a new repository and returns the
// directory and the commit SHAs, oldest first.
func gitRepoWith(t *testing.T, trees ...map[string]string) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "main", ".")
	var shas []string
	for i, tree := range trees {
		for p, body := range tree {
			path := filepath.Join(dir, p)
			if body == "" {
				_ = os.Remove(path)
				continue
			}
			mustWrite(t, path, body)
		}
		git(t, dir, "add", "-A")
		git(t, dir, "commit", "--quiet", "-m", fmt.Sprint("commit ", i))
		shas = append(shas, git(t, dir, "rev-parse", "HEAD"))
	}
	return dir, shas
}

const baseManifest = `{"contractVersion":"1","name":"basepakke","description":"Base","clients":{"copilot":{}},"layout":{"agents":"agents"}}`

// remotes points each repo name at a repository on disk, so the real clone
// plumbing runs.
func remotes(t *testing.T, dirs map[string]string) {
	t.Helper()
	orig := source.RemoteURLFn
	t.Cleanup(func() { source.RemoteURLFn = orig })
	source.RemoteURLFn = func(repo string) string { return "file://" + dirs[repo] }
}

// TestSyncWarnsAboutAStaleBaseThroughRealGit crosses both boundaries the
// check stands on: the pakke and its base are real git repositories cloned by
// the real plumbing, so the pin the warning names is the one compose read from
// the pakke's committed lock, and the GitHub API is answered over real HTTP.
func TestSyncWarnsAboutAStaleBaseThroughRealGit(t *testing.T) {
	isolatedConfig(t)
	stubRelease(t, releaseNoMetadata, pakkeRelease{}, nil)
	fakeBaseAPI(t, nil, staleCompare())

	baseDir, base := gitRepoWith(t,
		map[string]string{agentpakke.ManifestPath: baseManifest, "agents/reviewer.agent.md": "---\nname: reviewer\nmodel: gpt-5\n---\nold\n"},
		map[string]string{"agents/reviewer.agent.md": "---\nname: reviewer\nmodel: gpt-6\n---\nnew\n"})
	pakkeDir, _ := gitRepoWith(t, map[string]string{
		agentpakke.ManifestPath:              tier1ManifestJSON,
		"plugin/agents/grillmester.agent.md": "---\nname: grillmester\ndescription: Chef\n---\nhi\n",
		"plugin/skills/grilling/SKILL.md":    "# Grilling\n",
		agentpakke.DeclarationPath:           `{"contractVersion":"1","source":"navikt/basepakke","sha":"` + base[0] + `"}`,
	})
	remotes(t, map[string]string{"navikt/grillmester": pakkeDir, "navikt/basepakke": baseDir})

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester"}`)
	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "", false, false, false); err != nil {
			t.Fatalf("install: %v", err)
		}
	})
	out := captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", false, false); err != nil && !errors.Is(err, errUpdatesAvailable) {
			t.Fatalf("sync: %v", err)
		}
	})
	want := "navikt/grillmester pins navikt/basepakke at " + shortSHA(base[0])
	if !strings.Contains(out, want) || !strings.Contains(out, "11 commit(s)") {
		t.Errorf("sync did not warn %q:\n%s", want, out)
	}
	if got := readDeclarationAt(t, pakkeDir).SHA; got != base[0] {
		t.Errorf("sync moved the pakke's pin to %s", got)
	}
}

func readDeclarationAt(t *testing.T, root string) *agentpakke.Declaration {
	t.Helper()
	d, err := agentpakke.LoadDeclaration(root)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// bump-base clones both revisions for real and reports the model pin moves.
func TestPakkeBumpBase(t *testing.T) {
	isolatedConfig(t)
	baseDir, base := gitRepoWith(t,
		map[string]string{
			agentpakke.ManifestPath:    baseManifest,
			"agents/reviewer.agent.md": "---\nname: reviewer\nmodel: gpt-5.3-codex\n---\nold\n",
			"agents/writer.agent.md":   "---\nname: writer\n---\nv1\n",
			"agents/retired.agent.md":  "---\nname: retired\n---\nbye\n",
		},
		map[string]string{
			"agents/reviewer.agent.md": "---\nname: reviewer\nmodel: gpt-6-sol\n---\nnew\n",
			"agents/writer.agent.md":   "---\nname: writer\n---\nv2\n",
			"agents/retired.agent.md":  "",
			"agents/fresh.agent.md":    "---\nname: fresh\nmodel: claude-opus-5.5\n---\nhi\n",
			agentpakke.ManifestPath:    strings.Replace(baseManifest, `"name"`, `"minNavPilotVersion":"2026.01.01-000000","name"`, 1),
		})
	remotes(t, map[string]string{"navikt/basepakke": baseDir})
	pakke := t.TempDir()
	mustWrite(t, agentpakke.DeclarationFilePath(pakke),
		`{"contractVersion":"1","source":"navikt/basepakke","sha":"`+base[0]+`","items":{"reviewer":"agent"}}`)

	t.Run("lookup fails", func(t *testing.T) {
		stubRelease(t, 0, pakkeRelease{}, errors.New("offline"))
		if err := cmdPakkeBumpBase(pakke); err == nil || !strings.Contains(err.Error(), "The pin is unchanged") {
			t.Errorf("err = %v", err)
		}
		if got := readDeclarationAt(t, pakke).SHA; got != base[0] {
			t.Errorf("a failed lookup moved the pin to %s", got)
		}
	})

	stubRelease(t, releaseNoMetadata, pakkeRelease{}, nil)
	out := captureStdoutFor(t, func() {
		if err := cmdPakkeBumpBase(pakke); err != nil {
			t.Fatal(err)
		}
	})
	d := readDeclarationAt(t, pakke)
	if d.SHA != base[1] {
		t.Errorf("pin = %s, want %s", d.SHA, base[1])
	}
	if d.Items["reviewer"] != "agent" {
		t.Errorf("the bump dropped items: %v", d.Items)
	}
	if d.MinNavPilotVersion != "2026.01.01-000000" {
		t.Errorf("minNavPilotVersion = %q, want the new base's", d.MinNavPilotVersion)
	}
	for _, want := range []string{
		"from `" + shortSHA(base[0]) + "` to `" + shortSHA(base[1]) + "`, its default branch",
		"- `reviewer`: model `gpt-5.3-codex` → `gpt-6-sol`",
		"- `fresh`: added, model `claude-opus-5.5`",
		"- `retired`: removed",
		"- `writer`: changed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}

	// A second run finds the pin current and writes nothing.
	if out := captureStdoutFor(t, func() {
		if err := cmdPakkeBumpBase(pakke); err != nil {
			t.Fatal(err)
		}
	}); out != "" {
		t.Errorf("a current pin printed a summary:\n%s", out)
	}
}

// A base that itself reuses a pakke passes that pakke's agents on, so moving
// only its own pin is an agent change the summary must show.
func TestPakkeBumpBaseSeesAgentsInheritedByTheBase(t *testing.T) {
	isolatedConfig(t)
	stubRelease(t, releaseNoMetadata, pakkeRelease{}, nil)
	deepDir, deep := gitRepoWith(t,
		map[string]string{agentpakke.ManifestPath: strings.Replace(baseManifest, "basepakke", "deep", 1),
			"agents/deep.agent.md": "---\nname: deep\nmodel: gpt-5\n---\nx\n"},
		map[string]string{"agents/deep.agent.md": "---\nname: deep\nmodel: gpt-6\n---\nx\n"})
	lock := func(sha string) string {
		return `{"contractVersion":"1","source":"navikt/deep","sha":"` + sha + `"}`
	}
	baseDir, base := gitRepoWith(t,
		map[string]string{agentpakke.ManifestPath: baseManifest, "agents/own.agent.md": "---\nname: own\n---\nx\n",
			agentpakke.DeclarationPath: lock(deep[0])},
		map[string]string{agentpakke.DeclarationPath: lock(deep[1])})
	remotes(t, map[string]string{"navikt/basepakke": baseDir, "navikt/deep": deepDir})
	pakke := t.TempDir()
	mustWrite(t, agentpakke.DeclarationFilePath(pakke), `{"contractVersion":"1","source":"navikt/basepakke","sha":"`+base[0]+`"}`)

	out := captureStdoutFor(t, func() {
		if err := cmdPakkeBumpBase(pakke); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "- `deep`: model `gpt-5` → `gpt-6`") || strings.Contains(out, "`own`") {
		t.Errorf("summary:\n%s", out)
	}
}

// The base's own reuse pin decides whether a bump goes ahead: a new revision
// whose reuse does not resolve is refused, and an old one that does not is the
// broken pin a bump repairs.
func TestPakkeBumpBaseWhenTheBaseReuseDoesNotResolve(t *testing.T) {
	isolatedConfig(t)
	stubRelease(t, releaseNoMetadata, pakkeRelease{}, nil)
	deepDir, deep := gitRepoWith(t, map[string]string{
		agentpakke.ManifestPath: strings.Replace(baseManifest, "basepakke", "deep", 1),
		"agents/deep.agent.md":  "---\nname: deep\n---\nx\n"})
	lock := func(sha string) string {
		return `{"contractVersion":"1","source":"navikt/deep","sha":"` + sha + `"}`
	}
	missing := strings.Repeat("d", 40) // a commit navikt/deep does not have
	bump := func(t *testing.T, oldLock, newLock string) (string, string, error) {
		baseDir, base := gitRepoWith(t,
			map[string]string{agentpakke.ManifestPath: baseManifest, "agents/own.agent.md": "---\nname: own\n---\nx\n",
				agentpakke.DeclarationPath: oldLock},
			map[string]string{agentpakke.DeclarationPath: newLock})
		remotes(t, map[string]string{"navikt/basepakke": baseDir, "navikt/deep": deepDir})
		pakke := t.TempDir()
		mustWrite(t, agentpakke.DeclarationFilePath(pakke), `{"contractVersion":"1","source":"navikt/basepakke","sha":"`+base[0]+`"}`)
		var err error
		out := captureStdoutFor(t, func() { err = cmdPakkeBumpBase(pakke) })
		if got := readDeclarationAt(t, pakke).SHA; err == nil && got != base[1] || err != nil && got != base[0] {
			t.Errorf("pin = %s after err = %v (old %s, new %s)", got, err, base[0], base[1])
		}
		return out, base[1], err
	}

	t.Run("new revision does not compose", func(t *testing.T) {
		_, next, err := bump(t, lock(deep[0]), lock(missing))
		if err == nil || !strings.Contains(err.Error(), "composing navikt/basepakke at "+shortSHA(next)) ||
			!strings.Contains(err.Error(), "The pin is unchanged") || strings.Contains(err.Error(), "Nothing was installed") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("old revision does not compose", func(t *testing.T) {
		out, _, err := bump(t, lock(missing), lock(deep[0]))
		if err != nil {
			t.Fatalf("a bump away from a broken pin was refused: %v", err)
		}
		if !strings.Contains(out, "The previous revision could not be composed") || !strings.Contains(out, "- `deep`: added") {
			t.Errorf("summary:\n%s", out)
		}
	})
}

func TestPakkeBumpBaseRefusesDryRun(t *testing.T) {
	isolatedConfig(t)
	err := run([]string{"pakke", "bump-base", "--dry-run", "--target", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "no --dry-run") {
		t.Errorf("err = %v", err)
	}
}

// A Tier 2 pin with its revision on disk ships payloads, and a lock in it is
// inert: doctor asks nothing about it.
func TestDoctorSkipsATier2PinWithoutARequest(t *testing.T) {
	isolatedConfig(t)
	origLag, origFile := lookupBaseLag, githubFileJSON
	t.Cleanup(func() { lookupBaseLag, githubFileJSON = origLag, origFile })
	githubFileJSON = func(context.Context, string, string, string, any) error {
		t.Error("doctor read a file for a Tier 2 pin")
		return errOfflineForTests
	}
	lookupBaseLag = func(context.Context, string, string, string) (*baseLag, error) {
		t.Error("doctor looked up a base for a Tier 2 pin")
		return nil, nil
	}
	state := &StateFile{SourceRepo: "nais/pilot", SourceSHA: strings.Repeat("a", 40)}
	if err := os.MkdirAll(pakkeRevisionDir(state.SourceRepo, state.SourceSHA), 0o755); err != nil {
		t.Fatal(err)
	}
	reportScopeBaseLag(ScopeRepo(repoTarget(t)), state)
}

// doctor spends one deadline on the whole check: the file reads and the lag
// lookup share it, rather than each starting a fresh one.
func TestDoctorBaseLagSharesOneDeadline(t *testing.T) {
	var deadlines []time.Time
	origLag, origFile := lookupBaseLag, githubFileJSON
	t.Cleanup(func() { lookupBaseLag, githubFileJSON = origLag, origFile })
	githubFileJSON = func(ctx context.Context, repo, path, ref string, v any) error {
		d, _ := ctx.Deadline()
		deadlines = append(deadlines, d)
		body := `{"name":"nav-pilot"}`
		if path == agentpakke.DeclarationPath {
			body = `{"contractVersion":"1","source":"navikt/copilot","sha":"` + basePin + `"}`
		}
		return json.Unmarshal([]byte(body), v)
	}
	lookupBaseLag = func(ctx context.Context, _, _, _ string) (*baseLag, error) {
		d, _ := ctx.Deadline()
		deadlines = append(deadlines, d)
		return nil, nil
	}
	reportScopeBaseLag(ScopeRepo(repoTarget(t)), &StateFile{SourceRepo: "nais/pilot", SourceSHA: strings.Repeat("a", 40),
		Files: []InstalledFile{{Path: ".github/agents/x.agent.md", Hash: "h"}}})
	if len(deadlines) != 3 {
		t.Fatalf("calls = %d, want 3", len(deadlines))
	}
	for _, d := range deadlines {
		if d.IsZero() || !d.Equal(deadlines[0]) {
			t.Errorf("deadlines differ: %v", deadlines)
			break
		}
	}
}

// sync --apply moves the compatibility statement with the pin, as install
// writes it.
func TestBumpDeclarationSHACopiesMinNavPilotVersion(t *testing.T) {
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+strings.Repeat("a", 40)+`","minNavPilotVersion":"2025.01.01-000000"}`)
	src := &Source{Repo: "navikt/grillmester", SHA: strings.Repeat("b", 40),
		Pakke: &agentpakke.Manifest{Name: "grillmester", MinNavPilotVersion: "2026.01.01-000000"}}
	captureStdoutFor(t, func() { bumpDeclarationSHA(scope, src, false) })
	if d := readDeclaration(t, scope); d.SHA != src.SHA || d.MinNavPilotVersion != "2026.01.01-000000" {
		t.Errorf("declaration = %s / %q", d.SHA, d.MinNavPilotVersion)
	}
}

// The launch path must never pay for this check: only sync, doctor and the
// file that defines it may name it.
func TestBaseFreshnessIsOffTheLaunchPath(t *testing.T) {
	names := map[string]bool{"warnBaseLag": true, "lookupBaseLag": true, "reportScopeBaseLag": true, "lookupBaseLagHTTP": true, "githubFileJSON": true, "githubFileJSONHTTP": true}
	allowed := map[string]bool{"base_freshness.go": true, "sync.go": true, "doctor.go": true}
	files, _ := filepath.Glob("*.go")
	provider, _ := filepath.Glob("../provider/*.go")
	fset := token.NewFileSet()
	for _, path := range append(files, provider...) {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || allowed[base] {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && names[id.Name] {
				t.Errorf("%s names %s; the base freshness check belongs to sync and doctor only", fset.Position(id.Pos()), id.Name)
			}
			return true
		})
	}
}
