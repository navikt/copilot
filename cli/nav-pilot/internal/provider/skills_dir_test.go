package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// The launch paths NAV_PILOT_SKILLS_DIR has to cover, and the two seams they
// reach cplt through.
//
// Five of the six build a [cpltLaunch] and hand it to launchViaCplt. The sixth,
// LaunchCopilotResolved, builds its own argument vector (BuildCopilotArgs) and
// runs its own exec.Command — which is exactly how a variable wired "at the
// launch" can be wired at only five of six launches without anything failing.
// The two tests below are the guard: one fails if a new cpltLaunch skips the
// field, the other fails if a Launch* function appears that nobody checked.
var knownLaunchPaths = []string{
	"LaunchCopilotResolved", // legacy copilot — its own argv, its own exec
	"LaunchCopilotStaged",   // Tier 2 copilot
	"LaunchOpenCode",        // legacy opencode
	"LaunchOpenCodeStaged",  // Tier 2 opencode
	"LaunchPi",              // legacy pi
	"LaunchPiStaged",        // Tier 2 pi
}

// TestEveryCpltLaunchSetsSkillsDir fails when a cpltLaunch literal in this
// package leaves skillsDir out.
//
// Omission is the failure mode worth a test: a launch path added later that
// simply does not mention the field compiles, runs, and silently starts
// sessions where every skill script is unfindable. Setting it to "" is a fine
// answer — it is the honest answer for a client that materializes nothing —
// but it has to be written down rather than defaulted into.
func TestEveryCpltLaunchSetsSkillsDir(t *testing.T) {
	for file, f := range parsePackage(t) {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if id, ok := lit.Type.(*ast.Ident); !ok || id.Name != "cpltLaunch" {
				return true
			}
			// cpltLaunch{} with no fields is the zero value returned next to an
			// error; nothing is launched from it.
			if len(lit.Elts) == 0 {
				return true
			}
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					t.Errorf("%s: unkeyed cpltLaunch literal — keep it keyed so a missing skillsDir is visible", file)
					return true
				}
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "skillsDir" {
					return true
				}
			}
			t.Errorf("%s: a cpltLaunch literal does not set skillsDir — every launch path must name its materialized skills root, or \"\" when it materializes none (navikt/copilot#858)", file)
			return true
		})
	}
}

// TestKnownLaunchPaths fails when this package grows a Launch* function that
// knownLaunchPaths does not list, so adding a seventh launch path forces a look
// at whether it exports NAV_PILOT_SKILLS_DIR.
func TestKnownLaunchPaths(t *testing.T) {
	var found []string
	for _, f := range parsePackage(t) {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || !strings.HasPrefix(fn.Name.Name, "Launch") {
				continue
			}
			found = append(found, fn.Name.Name)
		}
	}
	sort.Strings(found)
	if !slices.Equal(found, knownLaunchPaths) {
		t.Errorf("the set of launch paths changed\n got: %q\nwant: %q\n\nA new launch path must export %s, and then be added to knownLaunchPaths.", found, knownLaunchPaths, SkillsDirEnv)
	}
}

func parsePackage(t *testing.T) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files[name] = f
	}
	if len(files) == 0 {
		t.Fatal("parsed no files — the package directory is not where the test thinks it is")
	}
	return files
}

// TestMaterializedSkillsDir pins the unset case, which is the whole point of
// the helper: a directory that is absent or empty produces no variable, not a
// path to nothing.
func TestMaterializedSkillsDir(t *testing.T) {
	root := t.TempDir()
	if got := materializedSkillsDir(root); got != "" {
		t.Errorf("no skills/ at all must give \"\", got %q", got)
	}

	skills := filepath.Join(root, "skills")
	if err := os.Mkdir(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := materializedSkillsDir(root); got != "" {
		t.Errorf("an empty skills/ must give \"\", got %q", got)
	}

	if err := os.MkdirAll(filepath.Join(skills, "nais-observability"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := materializedSkillsDir(root); got != skills {
		t.Errorf("materializedSkillsDir = %q, want %q", got, skills)
	}

	if got := materializedSkillsDir(""); got != "" {
		t.Errorf("an unresolvable root must give \"\", got %q", got)
	}
}

// TestCpltArgvSkillsDir pins the shared seam: --pass-env rides between the
// cplt flags and the "--" separator, and an unset skills dir changes nothing.
func TestCpltArgvSkillsDir(t *testing.T) {
	base := cpltLaunch{agent: "opencode", cpltArgs: []string{"--allow-read", "/payload"}, agentArgs: []string{"--agent", "x"}}

	want := []string{"--agent", "opencode", "--allow-read", "/payload", "--", "--agent", "x"}
	if got := cpltArgv(base); !slices.Equal(got, want) {
		t.Errorf("without a skills dir\n got: %q\nwant: %q", got, want)
	}

	withDir := base
	withDir.skillsDir = "/payload/skills"
	want = []string{"--agent", "opencode", "--allow-read", "/payload", "--pass-env", SkillsDirEnv, "--", "--agent", "x"}
	if got := cpltArgv(withDir); !slices.Equal(got, want) {
		t.Errorf("with a skills dir\n got: %q\nwant: %q", got, want)
	}
}

// TestCopilotLaunchArgsSkillsDir pins the second seam. The flag has to land
// before the "--", because after it the copilot binary gets it and does not
// know it; and the plain copilot CLI, which has no cplt to pass anything
// through, must never see it at all.
func TestCopilotLaunchArgsSkillsDir(t *testing.T) {
	resolved := domain.ResolvedConfig{AskUser: true}

	want := []string{"--yes", "--agent", "copilot", "--pass-env", SkillsDirEnv, "--", "--agent", "nav-pilot", "--model", "gpt-6-sol"}
	got := copilotLaunchArgs("cplt", resolved, false, "/home/u/.copilot/skills")
	if !slices.Equal(got, want) {
		t.Errorf("cplt with a skills dir\n got: %q\nwant: %q", got, want)
	}

	plain := []string{"--agent", "nav-pilot", "--model", "gpt-6-sol"}
	if got := copilotLaunchArgs("copilot", resolved, false, "/home/u/.copilot/skills"); !slices.Equal(got, plain) {
		t.Errorf("the plain copilot CLI must not be given cplt's --pass-env\n got: %q\nwant: %q", got, plain)
	}
}

// TestWithSkillsDirEnv pins that an empty dir strips the variable rather than
// passing an inherited one through, and that a nil environment is materialized
// rather than replaced by a single variable.
//
// The strip is the half that is easy to get wrong. Returning the environment
// unchanged reads as "we did not set it", but the client sees whatever the
// developer's shell exported or a previous launch left behind — a path into
// another client's tree, or one that has since been deleted. The documented
// contract is that a skill can test the variable for unset, and that only
// holds if unset means removed.
func TestWithSkillsDirEnv(t *testing.T) {
	stripped := withSkillsDirEnv([]string{"FOO=bar", SkillsDirEnv + "=/stale/skills"}, "")
	if got := telemetry.LookupEnvValue(stripped, SkillsDirEnv); got != "" {
		t.Errorf("an inherited %s must be stripped when nothing was materialized, got %q", SkillsDirEnv, got)
	}
	if !slices.Contains(stripped, "FOO=bar") {
		t.Errorf("the rest of the environment must survive the strip, got %q", stripped)
	}

	t.Setenv(SkillsDirEnv, "/stale/skills")
	if got := telemetry.LookupEnvValue(withSkillsDirEnv(nil, ""), SkillsDirEnv); got != "" {
		t.Errorf("an inherited %s must be stripped from a materialized parent environment too, got %q", SkillsDirEnv, got)
	}

	env := withSkillsDirEnv([]string{"FOO=bar"}, "/x/skills")
	if v := telemetry.LookupEnvValue(env, SkillsDirEnv); v != "/x/skills" {
		t.Errorf("%s = %q, want %q", SkillsDirEnv, v, "/x/skills")
	}
	if v := telemetry.LookupEnvValue(env, "FOO"); v != "bar" {
		t.Errorf("the rest of the environment must survive, FOO = %q", v)
	}

	inherited := withSkillsDirEnv(nil, "/x/skills")
	if len(inherited) <= 1 {
		t.Errorf("a nil environment must be materialized from the parent's, got %q", inherited)
	}
}

// fakeCplt puts a cplt on PATH that records what NAV_PILOT_SKILLS_DIR was in
// its own environment, and returns the file it records to. The shell's
// ${VAR-UNSET} tells apart "not set" from "set to empty", which is exactly the
// distinction under test.
func fakeCplt(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "skillsdir.txt")
	script := "#!/bin/sh\nprintf '%s' \"${" + SkillsDirEnv + "-UNSET}\" > " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return out
}

// isolateHome points every path nav-pilot resolves through the home directory
// at a temp tree, so a launch in a test cannot read — or write — the
// developer's real ~/.copilot, ~/.nav-pilot or ~/.config/opencode.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(home, "config.toml"))
	return home
}

// TestLaunchViaCpltDropsInheritedSkillsDir covers the shared seam — the five
// launch paths that build a cpltLaunch — end to end: an exported
// NAV_PILOT_SKILLS_DIR must not reach the sandbox when this launch
// materialized nothing.
func TestLaunchViaCpltDropsInheritedSkillsDir(t *testing.T) {
	isolateHome(t)
	out := fakeCplt(t)
	t.Setenv(SkillsDirEnv, "/stale/skills")

	if err := launchViaCplt(cpltLaunch{agent: "opencode", displayName: "opencode"}); err != nil {
		t.Fatalf("launchViaCplt: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the fake cplt recorded nothing: %v", err)
	}
	if string(got) != "UNSET" {
		t.Errorf("cplt saw %s=%q, want it unset", SkillsDirEnv, got)
	}
}

// TestLaunchCopilotResolvedDropsInheritedSkillsDir covers the other seam, the
// one that builds its own argument vector and runs its own exec.Command. HOME
// is a temp tree, so there is no ~/.copilot/skills and nothing was
// materialized.
func TestLaunchCopilotResolvedDropsInheritedSkillsDir(t *testing.T) {
	isolateHome(t)
	out := fakeCplt(t)
	t.Setenv(SkillsDirEnv, "/stale/skills")

	if err := LaunchCopilotResolved(domain.ResolvedConfig{Client: "copilot", AskUser: true}); err != nil {
		t.Fatalf("LaunchCopilotResolved: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the fake cplt recorded nothing: %v", err)
	}
	if string(got) != "UNSET" {
		t.Errorf("cplt saw %s=%q, want it unset", SkillsDirEnv, got)
	}
}
