package provider

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The launch half of #858 step 2. Every test here drives the real
// [defaultCpltProposalFlags] — the consent file, the hash comparison and the
// cplt version gate — rather than a stub, because the thing worth pinning is
// that an unapproved proposal reaches no launch vector.

const proposeTestManifest = `{
  "contractVersion": "1",
  "name": "nais-pilot",
  "description": "test",
  "clients": { "copilot": { "primaryAgents": ["nais-pilot"] } },
  "layout": { "agents": "agents" },
  "policies": { "propose": { "cplt": {
    "reason": "the observability skill queries Mimir on cloud.nais.io",
    "proxy": { "allow_private_domains": ["cloud.nais.io"] }
  } } }
}`

// proposeEnv isolates nav-pilot's state directory and the home the user scope
// is derived from, so nothing here reads or writes the real ~/.nav-pilot or
// ~/.copilot. It returns the user scope the records are keyed by.
func proposeEnv(t *testing.T) *domain.InstallScope {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, ".nav-pilot", "config.toml"))
	scope, err := domain.ScopeUser()
	if err != nil {
		t.Fatalf("user scope: %v", err)
	}
	return scope
}

// activeProposal installs the test manifest as the pakke a launch reads, and
// returns its proposal.
func activeProposal(t *testing.T) *agentpakke.CpltProposal {
	t.Helper()
	m, err := agentpakke.Parse([]byte(proposeTestManifest))
	if err != nil {
		t.Fatalf("parsing the test manifest: %v", err)
	}
	previous := source.ActivePakke()
	source.SetActivePakke(m)
	t.Cleanup(func() { source.SetActivePakke(previous) })
	return m.CpltProposal()
}

// approve writes an approval for the active pakke at the given hash.
func approve(t *testing.T, scope *domain.InstallScope, hash string, hosts ...string) {
	t.Helper()
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot",
		Hash: hash, Approved: true, Hosts: hosts,
	}); err != nil {
		t.Fatalf("writing the approval: %v", err)
	}
}

// protectingCplt stubs the version probe with a cplt at the release that
// write-protects nav-pilot's state directory, which is the precondition for
// applying anything at all.
func protectingCplt(t *testing.T) {
	t.Helper()
	previous := probeCpltVersion
	probeCpltVersion = func() (string, error) {
		return "cplt " + minCpltStampProtectingNavPilotState + "-abcdef0\n", nil
	}
	t.Cleanup(func() { probeCpltVersion = previous })
}

// Invariant 1: a proposal changes no launch until an approval exists for
// exactly its content hash, in a scope this launch reads from.
func TestI1ProposalHasNoEffectWithoutApproval(t *testing.T) {
	proposeEnv(t)
	activeProposal(t)
	protectingCplt(t)

	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("an unapproved proposal produced flags: %q", got)
	}
	argv := cpltArgv(cpltLaunch{agent: "copilot", agentArgs: []string{"--agent", "nais-pilot"}})
	if slices.Contains(argv, "--allow-private-domain") {
		t.Errorf("an unapproved proposal reached the staged launch vector: %q", argv)
	}
	legacy := BuildCopilotArgs("cplt", domain.ResolvedConfig{Persona: "nais-pilot"})
	if slices.Contains(legacy, "--allow-private-domain") {
		t.Errorf("an unapproved proposal reached the legacy launch vector: %q", legacy)
	}
}

// Invariant 1, the other half, and invariant 5: an approval reaches every
// launch path, in the cplt-flag slot before the "--" separator.
func TestI5ApprovedEntriesReachBothLaunchSeams(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	staged := strings.Join(cpltArgv(cpltLaunch{
		agent:     "copilot",
		cpltArgs:  []string{"--allow-read", "/staged/x"},
		agentArgs: []string{"--agent", "nais-pilot"},
	}), " ")
	if !strings.Contains(staged, "--allow-read /staged/x --allow-private-domain cloud.nais.io --") {
		t.Errorf("staged seam: entry missing or outside the cplt-flag slot: %s", staged)
	}

	legacy := strings.Join(BuildCopilotArgs("cplt", domain.ResolvedConfig{Persona: "nais-pilot"}), " ")
	if !strings.Contains(legacy, "--agent copilot --allow-private-domain cloud.nais.io --") {
		t.Errorf("legacy seam: entry missing or outside the cplt-flag slot: %s", legacy)
	}
}

// The legacy launch without cplt runs the plain copilot binary, which has no
// such flag. An entry must never reach it.
func TestI5UnsandboxedLaunchNeverCarriesTheEntries(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	args := BuildCopilotArgs("copilot", domain.ResolvedConfig{Persona: "nais-pilot"})
	if slices.Contains(args, "--allow-private-domain") {
		t.Errorf("a cplt flag reached the plain copilot binary: %q", args)
	}
}

// Invariant 2 at the launch: an approval of an earlier revision of the block
// does not carry over to a changed one. The record still exists and still names
// the pakke; only the hash moved.
func TestI2ChangedBlockVoidsTheApprovalAtLaunch(t *testing.T) {
	scope := proposeEnv(t)
	activeProposal(t)
	protectingCplt(t)
	approve(t, scope, "the-hash-of-an-earlier-revision", "cloud.nais.io")

	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("a superseded approval still applied: %q", got)
	}
}

// Invariant 3: nav-pilot cannot enforce the record being unwritable from inside
// a session — that is a cplt kernel deny — so it refuses to act on the record
// at all until the cplt in front of it has one. A version it cannot read counts
// as "does not have it".
func TestI3NoWaiverAppliedBelowStateProtectingCplt(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	for name, probe := range map[string]func() (string, error){
		"an older cplt":   func() (string, error) { return "cplt 2026.08.17-062831-1008a92\n", nil },
		"no cplt at all":  func() (string, error) { return "", os.ErrNotExist },
		"an unreadable v": func() (string, error) { return "cplt dev\n", nil },
	} {
		t.Run(name, func(t *testing.T) {
			previous := probeCpltVersion
			probeCpltVersion = probe
			defer func() { probeCpltVersion = previous }()
			if got := cpltProposalFlags(); len(got) != 0 {
				t.Errorf("applied a waiver on a cplt that does not protect nav-pilot's state: %q", got)
			}
		})
	}
}

// A pakke that proposes nothing must cost nothing: no consent file is read and
// no cplt is probed, so a launch on a machine with neither behaves exactly as
// it did before any of this existed.
func TestProposalFreeLaunchProbesNothing(t *testing.T) {
	proposeEnv(t)
	previous := probeCpltVersion
	probeCpltVersion = func() (string, error) {
		t.Error("a launch with no proposal probed the cplt version")
		return "", nil
	}
	defer func() { probeCpltVersion = previous }()

	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("the default agentpakke produced flags: %q", got)
	}
}

// Invariant 4: nothing is written on cplt's behalf. The record is nav-pilot's
// own file under its own state directory, and reading it must not create a
// cplt configuration, a local or trust directory, or a .cplt.toml.
func TestI4NothingIsWrittenIntoCpltConfiguration(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")
	cpltProposalFlags()

	home := os.Getenv("HOME")
	for _, forbidden := range []string{
		filepath.Join(home, ".config", "cplt"),
		filepath.Join(home, ".cplt.toml"),
	} {
		if _, err := os.Stat(forbidden); err == nil {
			t.Errorf("nav-pilot created %s on a pakke's behalf", forbidden)
		}
	}
	// And the record it did write is nav-pilot's own.
	if _, err := os.Stat(artifacts.ProposalConsentPath()); err != nil {
		t.Errorf("the consent record is not where it is supposed to be: %v", err)
	}
	if !strings.Contains(artifacts.ProposalConsentPath(), ".nav-pilot") {
		t.Errorf("the consent record lives outside nav-pilot's state directory: %s", artifacts.ProposalConsentPath())
	}
}

// Invariant 5, the structural half: the entries reach cplt through exactly two
// argv builders, and this test fails when a third process-spawning path appears
// in the package — the one way an approved waiver could silently stop applying
// to a launch that needs it.
//
// Keyed on the function that spawns the process, not on the file, so moving one
// between files is not a failure and adding one anywhere is.
func TestI5NoUnaccountedCpltLaunchPath(t *testing.T) {
	// Each entry says how that spawner gets its argv. The two launches are the
	// two seams; the rest are `--version` probes and diagnostics, which carry no
	// agent session and so carry no waiver.
	accounted := map[string]string{
		"launchViaCplt":           "cplt.go — argv from cpltArgv, which carries the entries",
		"LaunchCopilotResolved":   "copilot_launch.go — argv from BuildCopilotArgs, which carries the entries",
		"runStagedProbe":          "runtime_gate.go — a bounded --version probe, no session",
		"isCplt":                  "copilot_launch.go — a --version probe, no session",
		"printCopilotDiagnostics": "copilot_launch.go — diagnostics, no session",
	}

	for _, name := range processSpawners(t, ".") {
		if _, ok := accounted[name]; !ok {
			t.Errorf("%s spawns a process and is not accounted for.\n"+
				"If it launches cplt with an agent, build its argv through cpltArgv or BuildCopilotArgs so an "+
				"approved sandbox proposal reaches it (#858, invariant 5). If it does not, add it to this test's list.", name)
		}
	}
}

// processSpawners names every function and package-level function variable in
// the package's non-test sources that calls exec.Command or exec.CommandContext.
func processSpawners(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		// Every non-test source, build tags and all: a launch behind a build
		// tag is still a launch, and this scan exists to notice one.
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files = append(files, file)
	}
	var found []string
	spawns := func(n ast.Node) bool {
		var yes bool
		ast.Inspect(n, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "exec" && strings.HasPrefix(sel.Sel.Name, "Command") {
				yes = true
			}
			return true
		})
		return yes
	}
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if spawns(d) {
					found = append(found, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, value := range vs.Values {
						if i < len(vs.Names) && spawns(value) {
							found = append(found, vs.Names[i].Name)
						}
					}
				}
			}
		}
	}
	if len(found) == 0 {
		t.Fatal("the scan found no process spawners at all, so it is not scanning anything")
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// The record on disk is what a launch acts on, so its shape is part of the
// contract between the two halves: the CLI writes it, the launch reads it.
func TestConsentRecordRoundTrips(t *testing.T) {
	scope := proposeEnv(t)
	approve(t, scope, "abc123", "cloud.nais.io", "intern.nav.no")

	rec := artifacts.ReadProposalConsent(scope, "nais-pilot")
	if rec == nil || !rec.Approved || rec.Hash != "abc123" {
		t.Fatalf("record did not round-trip: %+v", rec)
	}
	data, err := os.ReadFile(artifacts.ProposalConsentPath())
	if err != nil {
		t.Fatalf("reading the record: %v", err)
	}
	var file struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("the record is not readable JSON: %v", err)
	}
	if len(file.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(file.Records))
	}
}
