package provider

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

// approve writes an approval for the active pakke at the given hash, recorded
// under a cplt that denies ~/.nav-pilot/ — the ordinary case.
func approve(t *testing.T, scope *domain.InstallScope, hash string, hosts ...string) {
	t.Helper()
	approveUnder(t, scope, hash, minCpltStampProtectingNavPilotState, hosts...)
}

// approveUnder writes an approval recorded under a named cplt stamp.
func approveUnder(t *testing.T, scope *domain.InstallScope, hash, stamp string, hosts ...string) {
	t.Helper()
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot",
		Hash: hash, Approved: true, Hosts: hosts, CpltStamp: stamp,
	}); err != nil {
		t.Fatalf("writing the approval: %v", err)
	}
}

// cplt version lines either side of the release that named ~/.nav-pilot/ in
// DENIED_DOTFILES (navikt/cplt#508, commit 446dfbb). The first three are real
// releases; the last is a stamp later than any of them, standing in for a
// future release.
//
// Literals rather than the constant plus or minus something: a stub derived
// from the constant passes whatever the constant says — a year-9999 placeholder
// included — so it would pin the comparison and not the version.
const (
	cpltWithStateDeny   = "cplt 2026.09.14-105131-446dfbb\n"
	cpltBeforeStateDeny = "cplt 2026.09.13-135112-daf7f1f\n"
	cpltAfterStateDeny  = "cplt 2026.10.01-090000-0000000\n"
	cpltLongBeforeDeny  = "cplt 2026.08.17-062831-1008a92\n"
)

// protectingCplt stubs the version probe with the cplt release that denies
// ~/.nav-pilot/, which is the precondition for applying anything at all.
func protectingCplt(t *testing.T) {
	t.Helper()
	stubCpltVersion(t, func() (string, error) { return cpltWithStateDeny, nil })
}

func stubCpltVersion(t *testing.T, probe func() (string, error)) {
	t.Helper()
	previous := probeCpltVersion
	probeCpltVersion = probe
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
// at all until the cplt in front of it names ~/.nav-pilot/ and so cannot have
// it reopened by a user's own allow.write. A version it cannot read counts as
// "does not name it".
//
// Both sides, over real release stamps: a gate that only ever refuses is not a
// gate, and neither is one whose "passing" case is derived from the constant it
// is meant to check.
func TestI3WaiverNeedsACpltThatDeniesNavPilotState(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	refuses := map[string]func() (string, error){
		// The release immediately before #508 landed. If the stamp is ever
		// moved down to something this release meets, this is what says so.
		"the release before the deny landed": func() (string, error) { return cpltBeforeStateDeny, nil },
		"a much older cplt":                  func() (string, error) { return cpltLongBeforeDeny, nil },
		"no cplt at all":                     func() (string, error) { return "", os.ErrNotExist },
		"a version that cannot be read":      func() (string, error) { return "cplt dev\n", nil },
	}
	for name, probe := range refuses {
		t.Run(name, func(t *testing.T) {
			stubCpltVersion(t, probe)
			if got := cpltProposalFlags(); len(got) != 0 {
				t.Errorf("applied a waiver on a cplt that does not deny ~/.nav-pilot/: %q", got)
			}
		})
	}

	// And the other side, or the gate above proves only that nothing ever
	// applies: the release that landed the deny, and one after it, both do.
	applies := map[string]func() (string, error){
		"the release that landed the deny": func() (string, error) { return cpltWithStateDeny, nil },
		"a release after it":               func() (string, error) { return cpltAfterStateDeny, nil },
	}
	for name, probe := range applies {
		t.Run(name, func(t *testing.T) {
			stubCpltVersion(t, probe)
			want := []string{"--allow-private-domain", "cloud.nais.io"}
			if got := cpltProposalFlags(); !slices.Equal(got, want) {
				t.Errorf("flags = %q, want %q", got, want)
			}
		})
	}
}

// A pakke that proposes nothing must cost nothing: no consent file is read and
// no cplt is probed, so a launch on a machine with neither behaves exactly as
// it did before any of this existed.
func TestProposalFreeLaunchProbesNothing(t *testing.T) {
	proposeEnv(t)
	stubCpltVersion(t, func() (string, error) {
		t.Error("a launch with no proposal probed the cplt version")
		return "", nil
	})

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
		"LaunchCopilotResolved":   "copilot_launch.go — argv from copilotLaunchArgs over BuildCopilotArgs, which carries the entries; #859's --pass-env is spliced in ahead of the same separator",
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

	rec, err := artifacts.ReadProposalConsent(scope, "nais-pilot")
	if err != nil || rec == nil || !rec.Approved || rec.Hash != "abc123" {
		t.Fatalf("record did not round-trip: %+v (%v)", rec, err)
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

// Invariant 3, the half a launch-time version check cannot see: an answer given
// while the record was unprotected stays untrustworthy after cplt is upgraded.
// The cplt running now is not the cplt that was running when an agent could
// have written the record (#861 review).
func TestI3ApprovalRecordedUnderAnUnprotectedCpltIsVoid(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t) // the cplt running now is fine

	for name, stamp := range map[string]string{
		"recorded before the deny landed":                   "2026.09.13-135112",
		"recorded under a version nav-pilot could not read": "",
	} {
		t.Run(name, func(t *testing.T) {
			approveUnder(t, scope, proposal.Hash(), stamp, "cloud.nais.io")
			if got := cpltProposalFlags(); len(got) != 0 {
				t.Errorf("applied a waiver recorded under an unprotected cplt: %q", got)
			}
		})
	}
}

// Invariant 3, the third question: cplt's rule names ~/.nav-pilot/, and
// NAV_PILOT_CONFIG can move nav-pilot's state anywhere. A relocated record is
// outside the deny even though the version check says yes (#861 review).
func TestI3NoWaiverWhenTheRecordSitsOutsideTheProtectedTree(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")
	if got := cpltProposalFlags(); len(got) == 0 {
		t.Fatal("the waiver did not apply in the protected tree, so relocating it proves nothing")
	}

	// A supported relocation, outside ~/.nav-pilot/.
	elsewhere := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(elsewhere, "config.toml"))
	approve(t, scope, proposal.Hash(), "cloud.nais.io")
	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("applied a waiver from a record cplt does not protect: %q", got)
	}
}

// A consent file that cannot be read is not an approval. It used to collapse to
// "no records", which is the same verdict by accident and the wrong one by
// design: a caller that cannot see what is there must not decide about it.
func TestUnreadableConsentRecordIsNotAnApproval(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	if err := os.WriteFile(artifacts.ProposalConsentPath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.ApprovedProposal("nais-pilot", proposal.Hash()); err == nil {
		t.Error("a malformed consent file was read as an answer rather than an error")
	}
	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("a malformed consent file produced flags: %q", got)
	}
	_ = scope
}

// Two writers at once must both survive. Atomic replacement keeps the file
// whole and still lets the second rename drop what the first wrote, so the
// read-modify-write runs under a lock (#861 review).
func TestConcurrentAnswersAreNotLost(t *testing.T) {
	proposeEnv(t)
	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- artifacts.WriteProposalConsent(artifacts.ProposalConsent{
				Scope: "repo", Root: fmt.Sprintf("/repo/%d", i), Pakke: "nais-pilot",
				Hash: "h", Approved: true, Hosts: []string{"cloud.nais.io"},
				CpltStamp: minCpltStampProtectingNavPilotState,
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("writing an answer: %v", err)
		}
	}
	for i := 0; i < writers; i++ {
		scope := domain.ScopeRepo(fmt.Sprintf("/repo/%d", i))
		rec, err := artifacts.ReadProposalConsent(scope, "nais-pilot")
		if err != nil {
			t.Fatalf("reading back: %v", err)
		}
		if rec == nil {
			t.Errorf("answer %d was lost", i)
		}
	}
}

// Nothing an agentpakke wrote reaches the terminal as-is. The launch line names
// the pakke, so it goes through the same sanitiser as the prompt does.
func TestLaunchNoticeCarriesNoPakkeControlCharacters(t *testing.T) {
	for name, in := range map[string]string{
		"a forged line":     "nais-pilot\nnothing else changes",
		"an escape":         "nais-pilot\x1b[2Kspoof",
		"a bidi override":   "nais-pilot\u202egnihton",
		"a carriage return": "nais-pilot\rspoof",
	} {
		t.Run(name, func(t *testing.T) {
			got := domain.SafeText(in, 64)
			if strings.ContainsAny(got, "\n\r\x1b") {
				t.Errorf("SafeText(%q) = %q, which can still write a line of its own", in, got)
			}
			if strings.ContainsRune(got, '\u202e') {
				t.Errorf("SafeText(%q) = %q, which can still reorder the line", in, got)
			}
		})
	}
	if got := domain.SafeText(strings.Repeat("x", 100), 10); len([]rune(got)) != 11 {
		t.Errorf("SafeText did not bound the length: %q", got)
	}
	if got, want := domain.SafeText("  ordinary  prose,  æøå  ", 0), "ordinary prose, æøå"; got != want {
		t.Errorf("SafeText mangled ordinary prose: %q, want %q", got, want)
	}
}

// The legacy launch carries two things in the cplt-flag slot now: #859's
// --pass-env NAV_PILOT_SKILLS_DIR and an approved waiver. They are placed by
// different code — insertCpltPassEnv searches for the separator, the waiver is
// built into the vector ahead of it — so this drives the vector the launch
// really assembles rather than either half of it.
func TestI5SkillsDirAndWaiverBothSurviveTheLegacyVector(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	args := copilotLaunchArgs("cplt", domain.ResolvedConfig{Persona: "nais-pilot"}, true, "/skills")
	joined := strings.Join(args, " ")

	sep := slices.Index(args, "--")
	if sep < 0 {
		t.Fatalf("no separator in the vector: %q", args)
	}
	before := strings.Join(args[:sep], " ")
	for _, want := range []string{"--allow-private-domain cloud.nais.io", "--pass-env " + SkillsDirEnv} {
		if !strings.Contains(before, want) {
			t.Errorf("%q is missing from the cplt-flag slot:\n %s", want, joined)
		}
	}
	// And the agent's own arguments are still on the far side of it.
	if !strings.Contains(strings.Join(args[sep:], " "), "--agent nais-pilot") {
		t.Errorf("the agent arguments moved out of the tail: %s", joined)
	}
}

// The same for the staged/opencode/pi seam, which assembles both in cpltArgv.
func TestI5SkillsDirAndWaiverBothSurviveTheStagedVector(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	args := cpltArgv(cpltLaunch{
		agent:     "opencode",
		cpltArgs:  []string{"--allow-read", "/staged/x"},
		skillsDir: "/staged/x/skills",
		agentArgs: []string{"run"},
	})
	sep := slices.Index(args, "--")
	before := strings.Join(args[:sep], " ")
	for _, want := range []string{"--allow-private-domain cloud.nais.io", "--pass-env " + SkillsDirEnv, "--allow-read /staged/x"} {
		if !strings.Contains(before, want) {
			t.Errorf("%q is missing from the cplt-flag slot:\n %q", want, args)
		}
	}
}

// approveReads writes an approval whose read grants are the "~/"-relative
// paths given, under a protecting cplt.
func approveReads(t *testing.T, scope *domain.InstallScope, hash string, reads ...string) {
	t.Helper()
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot",
		Hash: hash, Approved: true, Reads: reads,
		CpltStamp: minCpltStampProtectingNavPilotState,
	}); err != nil {
		t.Fatalf("writing the approval: %v", err)
	}
}

// An approved read grant reaches cplt as --allow-read with the path resolved
// against the home this launch runs under. The record keeps the "~/" form the
// person agreed to; the flag carries an absolute path, because cplt's
// --allow-read is a PathBuf from the shell and expands nothing itself (#885).
func TestApprovedReadGrantReachesTheLaunchAsAnAbsolutePath(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)

	home := os.Getenv("HOME")
	status := filepath.Join(home, "naisdevice", "agent-status.json")
	if err := os.MkdirAll(filepath.Dir(status), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(status, []byte(`{"connectionState":"Connected"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	approveReads(t, scope, proposal.Hash(), "~/naisdevice/agent-status.json")

	argv := cpltArgv(cpltLaunch{agent: "copilot", agentArgs: []string{"--agent", "nais-pilot"}})
	i := slices.Index(argv, "--allow-read")
	if i < 0 || i+1 >= len(argv) {
		t.Fatalf("the approved read grant never reached the launch: %q", argv)
	}
	if got := argv[i+1]; got != status {
		t.Errorf("--allow-read carried %q, want the resolved absolute path %q", got, status)
	}
	if slices.Contains(argv, "--allow-write") || slices.Contains(argv, "--allow-socket") {
		t.Errorf("a read grant widened into something else: %q", argv)
	}
}

// The same manifest ships on macOS and Linux, where naisdevice keeps its state
// in different places, so a pakke names both. A path that is not there, or that
// turns out to be a directory, is dropped rather than handed to cplt: a
// directory grant is a subpath grant, which is the whole trap (#885).
func TestReadGrantsThatAreNotOneNamedFileNeverReachTheLaunch(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)

	home := os.Getenv("HOME")
	if err := os.MkdirAll(filepath.Join(home, "looks-like-a-file.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	approveReads(t, scope, proposal.Hash(),
		"~/.config/naisdevice/agent-status.json", // the other platform's path
		"~/looks-like-a-file.json",               // a directory wearing a file's name
		"/etc/passwd.conf",                       // not home-relative at all
	)

	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("a grant that names no readable file reached the launch: %q", got)
	}
}

// The built-in default is what stands in for a source that ships no manifest,
// and it names itself "nav-pilot" whoever the source is. Since navikt/copilot's
// own pakke proposes a waiver, that identity would otherwise let an approval
// the user gave navikt/copilot apply to a launch of their own manifest-less
// source, which never asked for anything.
//
// The launch of a real manifest is the same test from the other side: the pakke
// was set from the source, so the approval applies.
func TestDefaultPakkeNeverCarriesAnApprovalIntoAForeignLaunch(t *testing.T) {
	scope := proposeEnv(t)
	protectingCplt(t)

	stock := agentpakke.Default()
	proposal := stock.CpltProposal()
	if proposal == nil {
		t.Fatal("the built-in default proposes nothing, so this test pins nothing")
	}
	hosts := proposal.AllowPrivateDomains()
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: stock.Name,
		Hash: proposal.Hash(), Approved: true, Hosts: hosts,
		CpltStamp: minCpltStampProtectingNavPilotState,
	}); err != nil {
		t.Fatalf("writing the approval: %v", err)
	}

	// A manifest-less source: nothing was ever set, so the default stands in.
	previous := source.ActivePakke()
	source.SetActivePakke(nil)
	t.Cleanup(func() { source.SetActivePakke(previous) })

	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("an approval for %s reached a launch running no manifest: %q", stock.Name, got)
	}
	legacy := BuildCopilotArgs("cplt", domain.ResolvedConfig{Persona: "nav-pilot"})
	if slices.Contains(legacy, "--allow-private-domain") {
		t.Errorf("an approval for %s reached the legacy vector of a manifest-less launch: %q", stock.Name, legacy)
	}

	// The same approval, on a launch that did set the pakke from its source.
	source.SetActivePakke(stock)
	got := cpltProposalFlags()
	for _, host := range hosts {
		if !slices.Contains(got, host) {
			t.Errorf("the approval did not reach the launch running its own manifest: %q", got)
		}
	}
}

// doctor reports the effective state, so what it asks has to be the same
// question the launch answers. An approval the launch refuses must not read as
// approved anywhere.
func TestWaiverBlockedReasonAgreesWithTheLaunch(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	record := artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot",
		Hash: proposal.Hash(), Approved: true, Hosts: proposal.AllowPrivateDomains(),
		CpltStamp: minCpltStampProtectingNavPilotState,
	}
	if err := artifacts.WriteProposalConsent(record); err != nil {
		t.Fatalf("writing the approval: %v", err)
	}

	stubCpltVersion(t, func() (string, error) { return cpltBeforeStateDeny, nil })
	reason := WaiverBlockedReason(&record)
	if reason == "" {
		t.Fatal("a cplt that does not deny nav-pilot's state directory gave no reason")
	}
	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("the launch applied a waiver it should have refused: %q", got)
	}

	protectingCplt(t)
	if reason := WaiverBlockedReason(&record); reason != "" {
		t.Errorf("a record the launch accepts was reported as blocked: %s", reason)
	}
	if got := cpltProposalFlags(); len(got) == 0 {
		t.Error("the launch applied nothing for a record WaiverBlockedReason calls fine")
	}
	if WaiverBlockedReason(nil) != "" {
		t.Error("no record is not a blocked record")
	}
}
