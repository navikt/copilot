package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// The consent half of #858 step 2, as a person meets it: asked at install and
// at any sync where the block changed, once per scope, pakke and content hash.

func proposeManifestJSON(hosts, extra string) string {
	return `{
  "contractVersion": "1",
  "name": "nais-pilot",
  "description": "test",
  "clients": { "copilot": { "primaryAgents": ["nais-pilot"] } },
  "layout": { "agents": "agents" },
  "policies": { "propose": { "cplt": {
    "reason": "the observability skill queries Mimir on cloud.nais.io",
    "proxy": { "allow_private_domains": [` + hosts + `] }` + extra + `
  } } }
}`
}

// proposeSource returns a source whose agentpakke proposes the given hosts.
func proposeSource(t *testing.T, hosts, extra string) *Source {
	t.Helper()
	m, err := agentpakke.Parse([]byte(proposeManifestJSON(hosts, extra)))
	if err != nil {
		t.Fatalf("parsing the test manifest: %v", err)
	}
	return &Source{Dir: t.TempDir(), Repo: "navikt/nais-pilot", SHA: "deadbee", Pakke: m}
}

// consentEnv isolates nav-pilot's state directory and the home the user scope
// is derived from, and makes the prompt answerable by the test.
func consentEnv(t *testing.T) *InstallScope {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, ".nav-pilot", "config.toml"))
	// A terminal is the precondition for the question being asked at all.
	previous := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = previous })
	scope, err := ScopeUser()
	if err != nil {
		t.Fatalf("user scope: %v", err)
	}
	return scope
}

// answering stubs the prompt and counts how often it was put.
func answering(t *testing.T, approve bool) *int {
	t.Helper()
	var asked int
	previous := askProposalConsent
	askProposalConsent = func(title, description string, value *bool) error {
		asked++
		*value = approve
		return nil
	}
	t.Cleanup(func() { askProposalConsent = previous })
	return &asked
}

// Invariant 1: without an approval nothing is recorded and nothing applies.
// A question that was never answered is not a "no" either — Ctrl-C leaves no
// record, so the next install asks again.
func TestI1AnAbandonedQuestionRecordsNothing(t *testing.T) {
	scope := consentEnv(t)
	previous := askProposalConsent
	askProposalConsent = func(string, string, *bool) error { return os.ErrClosed }
	defer func() { askProposalConsent = previous }()

	noteProposalConsent(scope, proposeSource(t, `"cloud.nais.io"`, ""), false, false)
	if rec := artifacts.ReadProposalConsent(scope, "nais-pilot"); rec != nil {
		t.Errorf("an abandoned question was recorded as an answer: %+v", rec)
	}
}

// Approving records the hash and the hosts, and asks once.
func TestI1ApprovalIsRecordedOncePerHash(t *testing.T) {
	scope := consentEnv(t)
	asked := answering(t, true)
	src := proposeSource(t, `"cloud.nais.io"`, "")

	noteProposalConsent(scope, src, false, false)
	noteProposalConsent(scope, src, false, false)

	if *asked != 1 {
		t.Errorf("asked %d times for one unchanged proposal, want 1", *asked)
	}
	rec := artifacts.ReadProposalConsent(scope, "nais-pilot")
	if rec == nil || !rec.Approved {
		t.Fatalf("no approval recorded: %+v", rec)
	}
	if want := src.Pakke.CpltProposal().Hash(); rec.Hash != want {
		t.Errorf("recorded hash %q, want the proposal's %q", rec.Hash, want)
	}
	if !slices.Equal(rec.Hosts, []string{"cloud.nais.io"}) {
		t.Errorf("recorded hosts %q", rec.Hosts)
	}
}

// Invariant 2: a changed block voids the previous approval and asks again — and
// the question shows what moved, because approving a change sight unseen is the
// thing a content hash exists to prevent.
func TestI2ChangedBlockVoidsTheApprovalAndAsksAgain(t *testing.T) {
	scope := consentEnv(t)
	answering(t, true)
	noteProposalConsent(scope, proposeSource(t, `"cloud.nais.io"`, ""), false, false)
	first := artifacts.ReadProposalConsent(scope, "nais-pilot")

	var asked int
	var description string
	previous := askProposalConsent
	askProposalConsent = func(title, desc string, value *bool) error {
		asked++
		description = desc
		*value = true
		return nil
	}
	defer func() { askProposalConsent = previous }()

	widened := proposeSource(t, `"cloud.nais.io", "intern.nav.no"`, "")
	noteProposalConsent(scope, widened, false, false)

	if asked != 1 {
		t.Fatalf("a widened proposal was asked about %d times, want 1", asked)
	}
	if !strings.Contains(description, "+ intern.nav.no") {
		t.Errorf("the question does not show the difference:\n%s", description)
	}
	after := artifacts.ReadProposalConsent(scope, "nais-pilot")
	if after == nil || after.Hash == first.Hash {
		t.Errorf("the record still carries the previous revision's hash: %+v", after)
	}
	if !slices.Equal(after.Hosts, []string{"cloud.nais.io", "intern.nav.no"}) {
		t.Errorf("recorded hosts %q", after.Hosts)
	}
}

// A decline installs anyway — someone may want the pakke for its other skills —
// and applies nothing. The record exists so the same answer is not asked for
// twice.
func TestI1DeclineInstallsAppliesNothingAndIsNotAskedAgain(t *testing.T) {
	scope := consentEnv(t)
	asked := answering(t, false)
	src := proposeSource(t, `"cloud.nais.io"`, "")

	noteProposalConsent(scope, src, false, false)
	noteProposalConsent(scope, src, false, false)

	if *asked != 1 {
		t.Errorf("asked %d times after a decline, want 1", *asked)
	}
	rec := artifacts.ReadProposalConsent(scope, "nais-pilot")
	if rec == nil {
		t.Fatal("the decline was not recorded, so it will be asked again")
	}
	if rec.Approved || len(rec.Hosts) != 0 {
		t.Errorf("a decline recorded something to apply: %+v", rec)
	}
	if got := artifacts.ApprovedPrivateDomains("nais-pilot", rec.Hash); len(got) != 0 {
		t.Errorf("a declined proposal yielded launch hosts: %q", got)
	}
}

// A declined pakke still tells the user what breaks and how to allow it by
// hand. That line is the whole reason a decline is a usable answer.
func TestDeclineNamesTheWorkflowAndTheOneLiner(t *testing.T) {
	scope := consentEnv(t)
	answering(t, false)
	out := captureStdoutFor(t, func() {
		noteProposalConsent(scope, proposeSource(t, `"cloud.nais.io"`, ""), false, false)
	})
	if !strings.Contains(out, "queries Mimir") {
		t.Errorf("the decline does not say which workflow fails:\n%s", out)
	}
	if !strings.Contains(out, "cplt config set proxy.allow_private_domains cloud.nais.io") {
		t.Errorf("the decline does not carry the manual one-liner:\n%s", out)
	}
}

// Invariant 8: a run with nothing that can answer never approves — and records
// nothing either, so the next terminal install still asks.
func TestI8NonInteractiveNeverApproves(t *testing.T) {
	scope := consentEnv(t)
	previous := isInteractive
	isInteractive = func() bool { return false }
	defer func() { isInteractive = previous }()
	previousAsk := askProposalConsent
	askProposalConsent = func(string, string, *bool) error {
		t.Error("a non-interactive run put the question")
		return nil
	}
	defer func() { askProposalConsent = previousAsk }()

	noteProposalConsent(scope, proposeSource(t, `"cloud.nais.io"`, ""), false, false)
	if rec := artifacts.ReadProposalConsent(scope, "nais-pilot"); rec != nil {
		t.Errorf("a non-interactive run recorded an answer: %+v", rec)
	}
}

// Invariant 7: a key nav-pilot does not implement is reported at install and
// honoured nowhere — not even by being part of what an approval covers.
func TestI7UnknownCpltKeyIsInertAndReported(t *testing.T) {
	scope := consentEnv(t)
	answering(t, true)
	src := proposeSource(t, `"cloud.nais.io"`, `, "some_future_cplt_key": {"on": true}`)

	out := captureStderrFor(t, func() { noteProposalConsent(scope, src, false, false) })
	if !strings.Contains(out, "some_future_cplt_key") {
		t.Errorf("the inert key was not reported:\n%s", out)
	}
	rec := artifacts.ReadProposalConsent(scope, "nais-pilot")
	if rec == nil || !slices.Equal(rec.Hosts, []string{"cloud.nais.io"}) {
		t.Errorf("the approval covers more than the implemented key: %+v", rec)
	}
}

// Invariant 6: uninstall takes the record with it, so no waiver from that pakke
// survives. The launch derives its flags from the record alone, so removing it
// is the whole of the removal.
func TestI6UninstallRemovesTheRecord(t *testing.T) {
	consentEnv(t)
	answering(t, true)
	src := proposeSource(t, `"cloud.nais.io"`, "")

	// A repository holding the pakke, so the record is keyed by a root the
	// launch lookup will actually consult from inside it.
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("git dir: %v", err)
	}
	scope := ScopeRepo(target)
	if err := writeState(target, &StateFile{Collection: "nais-pilot"}); err != nil {
		t.Fatalf("writing state: %v", err)
	}
	noteProposalConsent(scope, src, false, false)

	if rec := artifacts.ReadProposalConsent(scope, "nais-pilot"); rec == nil || !rec.Approved {
		t.Fatal("the approval was never recorded, so removing it proves nothing")
	}

	// The command, not the helper: a removal the uninstall path forgets to call
	// is exactly the failure this invariant is about.
	if err := cmdUninstall(scope, false, false); err != nil {
		t.Fatalf("cmdUninstall: %v", err)
	}

	if rec := artifacts.ReadProposalConsent(scope, "nais-pilot"); rec != nil {
		t.Errorf("the record outlived the uninstall, so the waiver would too: %+v", rec)
	}
}

// Invariant 4: consent is nav-pilot's own state and nothing else's. The whole
// flow must not create a cplt configuration file, its local or trust
// directories, or a .cplt.toml anywhere.
func TestI4NothingIsWrittenIntoCpltConfiguration(t *testing.T) {
	scope := consentEnv(t)
	answering(t, true)
	noteProposalConsent(scope, proposeSource(t, `"cloud.nais.io"`, ""), false, false)

	home := os.Getenv("HOME")
	for _, forbidden := range []string{
		filepath.Join(home, ".config", "cplt"),
		filepath.Join(home, ".config", "cplt", "local"),
		filepath.Join(home, ".config", "cplt", "trust"),
		filepath.Join(home, ".cplt.toml"),
	} {
		if _, err := os.Stat(forbidden); err == nil {
			t.Errorf("nav-pilot created %s on a pakke's behalf", forbidden)
		}
	}
	if err := filepath.WalkDir(home, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Name() == ".cplt.toml" {
			t.Errorf("nav-pilot wrote %s", path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walking %s: %v", home, err)
	}
}

// A scope is a scope: an approval in one repository does not answer for
// another, which is what keeps a repo-scope waiver from following the user
// around.
func TestApprovalIsPerScope(t *testing.T) {
	consentEnv(t)
	answering(t, true)
	src := proposeSource(t, `"cloud.nais.io"`, "")

	repoA := domain.ScopeRepo(t.TempDir())
	repoB := domain.ScopeRepo(t.TempDir())
	noteProposalConsent(repoA, src, false, false)

	if rec := artifacts.ReadProposalConsent(repoB, "nais-pilot"); rec != nil {
		t.Errorf("one repository's answer applied to another: %+v", rec)
	}
}

// A dry run answers nothing and records nothing: `sync` without --apply is a
// preview, and a preview that prompts is not one.
func TestDryRunNeverAsks(t *testing.T) {
	scope := consentEnv(t)
	previous := askProposalConsent
	askProposalConsent = func(string, string, *bool) error {
		t.Error("a dry run put the question")
		return nil
	}
	defer func() { askProposalConsent = previous }()
	noteProposalConsent(scope, proposeSource(t, `"cloud.nais.io"`, ""), true, false)
	if rec := artifacts.ReadProposalConsent(scope, "nais-pilot"); rec != nil {
		t.Errorf("a dry run recorded an answer: %+v", rec)
	}
}

// The install path itself asks, not only the helper: a refactor that stops
// routing through the question is exactly what this pins. It drives the real
// `install` over a pakke on disk that proposes a waiver.
func TestInstallAsksAboutTheProposal(t *testing.T) {
	consentEnv(t)
	asked := answering(t, true)

	dir := t.TempDir()
	writePakke(t, dir, "nais-pilot", "nais-pilot")
	manifest := strings.TrimSuffix(proposeManifestJSON(`"cloud.nais.io"`, ""), "\n")
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	// A path source: not pinnable, so the install resolves no release and this
	// test reaches no network.
	src := &Source{Dir: dir, SHA: "abc1234", Version: "dev", Repo: dir}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}

	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	scope := ScopeRepo(target)
	if err := cmdInstallFromSource("nais-pilot", src, scope, false, false, false); err != nil {
		t.Fatalf("install: %v", err)
	}

	if *asked != 1 {
		t.Errorf("the install asked %d times, want 1", *asked)
	}
	if rec := artifacts.ReadProposalConsent(scope, "nais-pilot"); rec == nil || !rec.Approved {
		t.Errorf("the install recorded no approval: %+v", rec)
	}
}
