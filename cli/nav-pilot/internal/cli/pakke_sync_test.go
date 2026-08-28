package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// ─── the pin, not the name, is what identifies a pinned install ──────────────

// renamedPinSourceTree writes the Tier 2 fixture under a different pakke name,
// which is the one thing upstream can change without changing the install.
func renamedPinSourceTree(t *testing.T, name string) string {
	t.Helper()
	manifest := strings.Replace(tier2PinManifestJSON, `"name": "grillmester"`, `"name": "`+name+`"`, 1)
	if manifest == tier2PinManifestJSON {
		t.Fatal("the Tier 2 fixture no longer carries the name this test renames")
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), manifest)
	for _, p := range tier2PinPayloads {
		writeTier2Payload(t, filepath.Join(dir, "dist", p.client, p.context), p.client+"-"+p.context)
	}
	return dir
}

// TestPinnedSyncSurvivesAnUpstreamRename: a pakke that renames itself upstream
// is the same install, pinned to the same source. Keying the sync branch on the
// display name would drop it into a file sync that tracks no files, prints "No
// customization files found to sync." and reports success — the frozen-forever
// dead end the branch exists to close, reached by a rename nobody guards.
func TestPinnedSyncSurvivesAnUpstreamRename(t *testing.T) {
	scope := pinEnv(t)
	first := &Source{Dir: tier2PinSourceTree(t), SHA: "sha-one", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(first); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, first)

	// Same source, new revision, new name.
	pinnedSyncSource(t, renamedPinSourceTree(t, "grillmesteren"), "sha-two")

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != errUpdatesAvailable {
		t.Fatalf("cmdSync after an upstream rename = %v, want errUpdatesAvailable", err)
	}
	if strings.Contains(out, "No customization files found to sync") {
		t.Errorf("the rename dropped the pin into the file-diff dead end. Output:\n%s", out)
	}
	if !strings.Contains(out, "sha-two") {
		t.Errorf("sync did not name the available revision. Output:\n%s", out)
	}

	captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("cmdSync --apply after an upstream rename = %v, want nil", err)
	}
	state, _ := readScopedState(scope)
	if state == nil || state.SourceSHA != "sha-two" {
		t.Fatalf("state after --apply = %+v, want the pin at sha-two", state)
	}
	if state.Collection != "grillmesteren" {
		t.Errorf("state collection = %q, want the pakke's new name", state.Collection)
	}
	assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", "sha-two"))
}

// TestPinnedSyncRefusesADifferentSource: an explicit --source bypasses the B3
// guard because it is the consent gesture for an install. Sync is not an
// install: comparing this scope's pinned SHA against an unrelated repo's HEAD
// treats two sources as revisions of one thing, and --apply would then switch
// the scope's source through a command that only ever claimed to update it.
func TestPinnedSyncRefusesADifferentSource(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))

	pinnedSyncSource(t, tier2PinSourceTree(t), "sha-other")

	var err error
	captureStdoutFor(t, func() { err = cmdSync(scope, "", "navikt/other-pakke", true, false) })
	if err == nil || err == errUpdatesAvailable {
		t.Fatalf("cmdSync --apply --source <other repo> = %v, want a refusal", err)
	}
	if !strings.Contains(err.Error(), "nav-pilot install") {
		t.Errorf("error %q does not point at the command that switches sources", err)
	}

	state, _ := readScopedState(scope)
	if state == nil || state.SourceRepo != "navikt/grillmester" || state.SourceSHA != "sha-one" {
		t.Fatalf("state = %+v, want the pin left on navikt/grillmester@sha-one", state)
	}
	if _, statErr := os.Stat(pakkeSourceDir("navikt/other-pakke")); !os.IsNotExist(statErr) {
		t.Errorf("the refused sync materialized a revision of the other source (stat err %v)", statErr)
	}
}

// ─── uninstall: whose revisions, and saying so ───────────────────────────────

// TestUninstallLeavesAnotherScopesRevisions: pins are user scope only, so a
// zero-file state anywhere else is not one. Recognising a pin by "has a source
// and tracks no files" makes a repo-scope uninstall delete the *user's* pinned
// revisions for that source.
func TestUninstallLeavesAnotherScopesRevisions(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	repo := ScopeRepo(repoTarget(t))
	if err := writeScopedState(repo, &StateFile{
		Collection: "grillmester",
		Scope:      repo.Name,
		SourceRepo: src.Repo,
		SourceSHA:  src.SHA,
	}); err != nil {
		t.Fatal(err)
	}

	captureStdoutFor(t, func() {
		if err := cmdUninstall(repo, false); err != nil {
			t.Fatalf("repo-scope uninstall: %v", err)
		}
	})

	if _, err := os.Stat(pakkeRevisionDir(src.Repo, src.SHA)); err != nil {
		t.Errorf("a repo-scope uninstall removed the user's pinned revision: %v", err)
	}
	if state, _ := readScopedState(scope); state == nil || state.SourceSHA != src.SHA {
		t.Errorf("the user's pin = %+v, want it untouched", state)
	}
}

// TestUninstallDryRunNamesTheRevisions: a pin tracks no files, so the revisions
// are the only thing uninstall removes. A dry run that prints an empty file
// loop and stops describes a command that does nothing at all.
func TestUninstallDryRunNamesTheRevisions(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	revDir := pakkeRevisionDir(src.Repo, src.SHA)
	out := captureStdoutFor(t, func() {
		if err := cmdUninstall(scope, true); err != nil {
			t.Fatalf("dry-run uninstall: %v", err)
		}
	})

	if !strings.Contains(out, revDir) {
		t.Errorf("the dry run did not name the revision it would remove (%s). Output:\n%s", revDir, out)
	}
	if strings.Contains(out, "Would remove 0 items") {
		t.Errorf("the dry run counted nothing, though uninstall removes the revision. Output:\n%s", out)
	}
	if _, err := os.Stat(revDir); err != nil {
		t.Errorf("the dry run removed the revision: %v", err)
	}
}

// ─── list shows what the install materialized ────────────────────────────────

// unknownClientPinManifest declares payloads for two clients with no staged
// launcher — one nav-pilot has never heard of, and one it knows perfectly well
// — which an install materializes like any other.
const unknownClientPinManifest = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Grillmester agentpakke, pre-built",
  "clients": {
    "copilot": {"payloads": {"full": {"path": "dist/copilot/full", "primaryAgents": ["grillmester"]}}},
    "cursor":  {"payloads": {"full": {"path": "dist/cursor/full",  "primaryAgents": ["grillmester"]}}},
    "pi":      {"payloads": {"full": {"path": "dist/pi/full",      "primaryAgents": ["grillmester"]}}}
  }
}`

// TestListShowsPayloadClientsItCannotLaunch: the install materializes every
// payload-bearing client, launcher or not. Listing only the launchable ones
// hides content that is on disk; the useful half is saying which of it this
// binary cannot run.
//
// "Cannot run" has to mean what the launch means by it. A client can be one
// nav-pilot knows (agentpakke.IsKnownClient) and still have no staged launcher
// — `pi` is exactly that — and keying the annotation on the wider set lists
// such a payload with no warning at all, then refuses when someone launches it.
func TestListShowsPayloadClientsItCannotLaunch(t *testing.T) {
	pinEnv(t)

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), unknownClientPinManifest)
	for _, client := range []string{"copilot", "cursor", "pi"} {
		writeTier2Payload(t, filepath.Join(dir, "dist", client, "full"), client+"-full")
	}

	src := &Source{Dir: dir, SHA: "sha-one", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	if _, ok := stagedLaunchers["cursor"]; ok {
		t.Skip("cursor gained a launcher; pick another unlaunchable client for this test")
	}
	// The whole point of the pi case: known to nav-pilot, unlaunchable by it.
	if _, ok := stagedLaunchers["pi"]; ok {
		t.Skip("pi gained a staged launcher; pick another known-but-unlaunchable client")
	}
	if !agentpakke.IsKnownClient("pi") {
		t.Fatal("pi is no longer a known client; this test no longer covers the known-but-unlaunchable gap")
	}
	stubResolveSource(t, src)

	var err error
	out := captureStdoutFor(t, func() { err = cmdList("", "", false, false) })
	if err != nil {
		t.Fatalf("cmdList over a Tier 2 agentpakke = %v", err)
	}
	if !strings.Contains(out, "copilot payloads: full") {
		t.Errorf("the listing dropped a launchable client's payloads. Output:\n%s", out)
	}
	if strings.Contains(out, "cannot launch copilot") {
		t.Errorf("the listing warned about copilot, which this binary launches. Output:\n%s", out)
	}
	for _, client := range []string{"cursor", "pi"} {
		if !strings.Contains(out, client+" payloads: full") {
			t.Errorf("the listing hid payloads the install materializes for %s. Output:\n%s", client, out)
		}
		if !strings.Contains(out, "cannot launch "+client) {
			t.Errorf("the listing did not say %s is unlaunchable by this binary, though a launch refuses it. Output:\n%s", client, out)
		}
	}
}

// TestPinnedSyncRefusesAnUpstreamTierChange: a pinned source that grows a
// layout upstream stops being payload-only, but the revision every launch reads
// is still the payload-only one that was pinned. Keying the branch on the
// source's current shape hands that install to the file sync, which tracks no
// files, prints "No customization files found to sync." and reports success —
// frozen forever, with every command saying it is fine.
func TestPinnedSyncRefusesAnUpstreamTierChange(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	// Upstream grows a layout: the same source, no longer payload-only.
	pinnedSyncSource(t, pakkeSourceTree(t, tier1ManifestJSON), "sha-two")

	for _, apply := range []bool{false, true} {
		var err error
		out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", apply, false) })
		if err == nil {
			t.Fatalf("cmdSync(apply=%v) over a tier-changed pin = nil; it reported success over a frozen install. Output:\n%s", apply, out)
		}
		if err == errUpdatesAvailable {
			t.Errorf("cmdSync(apply=%v) offered an update that --apply cannot materialize", apply)
		}
		if !strings.Contains(err.Error(), "nav-pilot install") {
			t.Errorf("error %q does not point at the command that resolves it", err)
		}
		if strings.Contains(out, "No customization files found to sync") {
			t.Errorf("the tier change dropped the pin into the file-diff dead end. Output:\n%s", out)
		}
	}

	state, _ := readScopedState(scope)
	if state == nil || state.SourceSHA != "sha-one" {
		t.Fatalf("state = %+v, want the pin left at sha-one", state)
	}
	if _, err := os.Stat(pakkeRevisionDir(src.Repo, "sha-two")); !os.IsNotExist(err) {
		t.Errorf("the refused sync materialized sha-two (stat err %v)", err)
	}
	assertRevisionVerifies(t, pakkeRevisionDir(src.Repo, "sha-one"))
}

// TestPinnedSyncRefusesAManifestLessSource: the cross-source refusal is the
// first thing syncPakkePin does, before anything has established that the
// --source it is refusing ships an agentpakke at all. attachPakke leaves
// src.Pakke nil for a manifest-less checkout and returns no error, so naming
// the pakke in that refusal dereferences nil — a Go panic where a refusal was
// intended, and one no manifest-bearing --source can reach.
func TestPinnedSyncRefusesAManifestLessSource(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))

	// A checkout with no .nav-pilot/agentpakke.json: the legacy shape.
	legacy := t.TempDir()
	mustWrite(t, filepath.Join(legacy, "agents", "noop.agent.md"), "---\nname: noop\n---\nBody\n")
	pinnedSyncSource(t, legacy, "sha-other")

	var err error
	captureStdoutFor(t, func() { err = cmdSync(scope, "", "navikt/legacy-repo", true, false) })
	if err == nil || err == errUpdatesAvailable {
		t.Fatalf("cmdSync --apply --source <manifest-less repo> = %v, want a refusal", err)
	}
	if !strings.Contains(err.Error(), "nav-pilot install") {
		t.Errorf("error %q does not point at the command that switches sources", err)
	}

	state, _ := readScopedState(scope)
	if state == nil || state.SourceRepo != "navikt/grillmester" || state.SourceSHA != "sha-one" {
		t.Fatalf("state = %+v, want the pin left on navikt/grillmester@sha-one", state)
	}
}

// ─── a Tier 1 install over a pin ─────────────────────────────────────────────

// TestTier1InstallOverAPinReleasesItsRevisions: a pin tracks no files, so its
// state entry is the only record that the trees under ~/.nav-pilot/pakker
// exist. A Tier 1 install writes a Files-bearing state over it and uninstall is
// gated on the state still being a pin, so without a release at that point the
// revisions are unreachable forever — and the way there is following the advice
// nav-pilot prints itself when a pinned source grows a layout upstream.
func TestTier1InstallOverAPinReleasesItsRevisions(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	sourceDir := pakkeSourceDir(src.Repo)
	if _, err := os.Stat(sourceDir); err != nil {
		t.Fatalf("setup: the pin materialized nothing: %v", err)
	}

	// The refusal in TestPinnedSyncRefusesAnUpstreamTierChange names exactly
	// this command, and it takes the ordinary Tier 1 path.
	tier1 := &Source{Dir: pakkeSourceTree(t, tier1ManifestJSON), SHA: "sha-two", Version: "dev", Repo: src.Repo}
	if err := attachPakke(tier1); err != nil {
		t.Fatal(err)
	}
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("grillmester", tier1, scope, false, false, false); err != nil {
			t.Fatalf("Tier 1 install over a pin: %v", err)
		}
	})

	state, _ := readScopedState(scope)
	if state == nil || len(state.Files) == 0 {
		t.Fatalf("setup: the Tier 1 install recorded no files: %+v", state)
	}
	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		t.Errorf("%s survived the install that replaced the pin (stat err %v); nothing would ever remove it again", sourceDir, err)
	}
}

// ignoredPin installs a pin and then marks an item ignored, which appends a
// zero-hash entry to the pin's state — the one thing that makes a live pin stop
// having an empty file list.
func ignoredPin(t *testing.T) (*InstallScope, *Source) {
	t.Helper()
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	captureStdoutFor(t, func() {
		if err := cmdIgnore("agent", "grillmester", scope, false); err != nil {
			t.Fatalf("cmdIgnore over a pin: %v", err)
		}
	})
	state, _ := readScopedState(scope)
	if state == nil || len(state.Files) == 0 {
		t.Fatalf("setup: the ignore marker was not recorded: %+v", state)
	}
	return scope, src
}

// TestIgnoreOverAPinKeepsItsRevisions: an ignore marker is not installed
// content, and every place that asks "is this a pin" has to agree about that.
// The launch never looked at Files, so it went on reading the revision either
// way; keying the other three on an empty file list instead left the install
// live but unreachable by every command that maintains it.
func TestIgnoreOverAPinKeepsItsRevisions(t *testing.T) {
	t.Run("the revisions stay", func(t *testing.T) {
		_, src := ignoredPin(t)

		// Removing them would refuse the next launch outright, and auto-pin
		// cannot rebuild over a state that now tracks files.
		if _, err := os.Stat(pakkeRevisionDir(src.Repo, src.SHA)); err != nil {
			t.Errorf("an ignore marker took the pinned revisions with it: %v", err)
		}
	})

	t.Run("sync still advances the pin", func(t *testing.T) {
		scope, src := ignoredPin(t)
		pinnedSyncSource(t, tier2PinSourceTree(t), "sha-two")

		var err error
		out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
		if err != errUpdatesAvailable {
			t.Fatalf("cmdSync after an ignore = %v, want errUpdatesAvailable. Output:\n%s", err, out)
		}
		if strings.Contains(out, "No customization files found to sync") {
			t.Errorf("an ignore marker dropped the pin into the file-diff dead end, where it can never advance again. Output:\n%s", out)
		}

		out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
		if err != nil {
			t.Fatalf("cmdSync --apply after an ignore = %v, want nil. Output:\n%s", err, out)
		}
		if state, _ := readScopedState(scope); state == nil || state.SourceSHA != "sha-two" {
			t.Fatalf("state after --apply = %+v, want the pin advanced to sha-two", state)
		}
		assertRevisionVerifies(t, pakkeRevisionDir(src.Repo, "sha-two"))
	})

	t.Run("uninstall still removes the revisions", func(t *testing.T) {
		scope, src := ignoredPin(t)

		captureStdoutFor(t, func() {
			if err := cmdUninstall(scope, false); err != nil {
				t.Fatalf("uninstall after an ignore: %v", err)
			}
		})

		if _, err := os.Stat(pakkeSourceDir(src.Repo)); !os.IsNotExist(err) {
			t.Errorf("uninstall removed the state and left the revisions behind (stat err %v); nothing references them now", err)
		}
	})
}

// TestPinnedSyncOverAWipedRevisionSaysSo: the state can outlive the revision
// directory — ~/.nav-pilot/pakker wiped, a revision hand-deleted — and it is
// still the pin, still what the next launch acts on. Keying the sync branch on
// the directory alone drops that case into the file diff, which tracks no
// files, prints "No customization files found to sync." and returns success:
// the frozen-success this branch exists to close, reached through a missing
// directory rather than a missing branch.
func TestPinnedSyncOverAWipedRevisionSaysSo(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	if err := os.RemoveAll(pakkerRoot()); err != nil {
		t.Fatal(err)
	}

	// The source has not moved: without the missing-revision branch the SHA
	// comparison would report this pin up to date.
	pinnedSyncSource(t, tier2PinSourceTree(t), "sha-one")

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != errUpdatesAvailable {
		t.Fatalf("cmdSync over a pin with no revision behind it = %v, want errUpdatesAvailable. Output:\n%s", err, out)
	}
	if strings.Contains(out, "No customization files found to sync") {
		t.Errorf("the wiped pin fell into the file-diff dead end. Output:\n%s", out)
	}
	if strings.Contains(out, "up to date") {
		t.Errorf("sync called a pin with no revision behind it up to date. Output:\n%s", out)
	}
	if !strings.Contains(out, "sync --apply") {
		t.Errorf("sync did not name the command that restores the revision. Output:\n%s", out)
	}

	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("cmdSync --apply over a wiped pin = %v, want nil. Output:\n%s", err, out)
	}
	assertRevisionVerifies(t, pakkeRevisionDir(src.Repo, "sha-one"))
	if state, _ := readScopedState(scope); state == nil || state.SourceSHA != "sha-one" {
		t.Errorf("state after --apply = %+v, want the pin back at sha-one", state)
	}
}
