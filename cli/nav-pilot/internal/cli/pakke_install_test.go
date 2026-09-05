package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// ─── Tier 2 pin fixtures ─────────────────────────────────────────────────────

// tier2PinManifestJSON is a payload-only agentpakke with the shape the pin has
// to get right: two launchable clients at two contexts each, plus a
// payload-bearing client this binary has no staged launcher for.
const tier2PinManifestJSON = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Grillmester agentpakke, pre-built",
  "clients": {
    "copilot": {
      "defaultContext": "full",
      "payloads": {
        "full":    {"path": "dist/copilot/full",    "primaryAgents": ["grillmester"]},
        "focused": {"path": "dist/copilot/focused", "primaryAgents": ["barista"]}
      }
    },
    "opencode": {
      "payloads": {
        "full":    {"path": "dist/opencode/full",    "primaryAgents": ["grillmester"]},
        "focused": {"path": "dist/opencode/focused", "primaryAgents": ["barista"]}
      }
    },
    "pi": {
      "payloads": {
        "full": {"path": "dist/pi/full", "primaryAgents": ["grillmester"]}
      }
    }
  }
}`

// tier2PinPayloads names every client × context the fixture declares, which is
// also what a materialized revision must hold.
var tier2PinPayloads = []struct{ client, context string }{
	{"copilot", "full"},
	{"copilot", "focused"},
	{"opencode", "full"},
	{"opencode", "focused"},
	{"pi", "full"},
}

// unlaunchableClient is declared payload-bearing by the fixture but has no
// staged launcher in this binary, so a launch for it runs the whole pinned path
// — pin lookup, verification, SetActivePakke — and then stops at the handover
// instead of executing a client. That is what lets a test tell "refused" from
// "handed over" without a cplt on the machine.
const unlaunchableClient = "pi"

// handoverErr is the error a launch that reached the handover returns.
const handoverErr = "cannot launch staged payloads for that client"

// tier2PinSourceTree writes a conforming payload-only agentpakke: a real file
// per payload, at a real digest and a declared mode, so the verification the
// pin depends on has something it can actually reject.
func tier2PinSourceTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), tier2PinManifestJSON)
	for _, p := range tier2PinPayloads {
		writeTier2Payload(t, filepath.Join(dir, "dist", p.client, p.context), p.client+"-"+p.context)
	}
	return dir
}

// writeTier2Payload writes a two-file payload tree and the payload manifest
// that describes it: one 0644 file and one 0755 file, so both modes the
// contract allows are exercised.
func writeTier2Payload(t *testing.T, dir, label string) {
	t.Helper()
	body := "---\nname: grillmester\ndescription: " + label + "\n---\nBody " + label + "\n"
	script := "#!/bin/sh\necho " + label + "\n"
	mustWrite(t, filepath.Join(dir, "agents", "grillmester.agent.md"), body)
	mustWrite(t, filepath.Join(dir, "scripts", "hook.sh"), script)
	if err := os.Chmod(filepath.Join(dir, "scripts", "hook.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, agentpakke.PayloadManifestFile), fmt.Sprintf(
		`{"schemaVersion":1,"files":{`+
			`"agents/grillmester.agent.md":{"sha256":"%s","mode":"0644"},`+
			`"scripts/hook.sh":{"sha256":"%s","mode":"0755"}}}`,
		digestOf(body), digestOf(script)))
}

func digestOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// tier2PinSource builds a resolved, conforming payload-only source at a SHA.
func tier2PinSource(t *testing.T, sha string) *Source {
	t.Helper()
	src := &Source{Dir: tier2PinSourceTree(t), SHA: sha, Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke on the Tier 2 fixture: %v", err)
	}
	if !payloadOnly(src) {
		t.Fatalf("fixture is not payload-only: %+v", src.Pakke)
	}
	return src
}

// pinEnv isolates both roots the pin touches: NAV_PILOT_CONFIG for
// pakkerRoot(), and HOME for the user scope the pin is recorded in.
func pinEnv(t *testing.T) *InstallScope {
	t.Helper()
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

// installPin runs a real Tier 2 install into user scope.
func installPin(t *testing.T, scope *InstallScope, src *Source) {
	t.Helper()
	if err := installPakkePin(scope, src, false, false); err != nil {
		t.Fatalf("installPakkePin: %v", err)
	}
}

// assertRevisionVerifies fails unless every declared context of the revision is
// present and passes the exact hash walk.
func assertRevisionVerifies(t *testing.T, revDir string) {
	t.Helper()
	for _, p := range tier2PinPayloads {
		dir := filepath.Join(revDir, p.client, p.context)
		if err := agentpakke.VerifyPayloadExact(dir, filepath.Join(dir, agentpakke.PayloadManifestFile)); err != nil {
			t.Errorf("VerifyPayloadExact(%s/%s) = %v, want nil", p.client, p.context, err)
		}
	}
}

// revisionNames lists what a source's revision directory holds.
func revisionNames(t *testing.T, repo string) []string {
	t.Helper()
	entries, err := os.ReadDir(pakkeSourceDir(repo))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// ─── install: what a pin materializes ────────────────────────────────────────

// TestInstallMaterializesEveryDeclaredContext: the pin is what every later
// launch reads, so it must hold every context the manifest declares — not only
// the default one, which is all a launch happens to ask for first.
func TestInstallMaterializesEveryDeclaredContext(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	revDir := pakkeRevisionDir(src.Repo, src.SHA)
	assertRevisionVerifies(t, revDir)

	for _, client := range []string{"copilot", "opencode"} {
		for _, context := range []string{"full", "focused"} {
			if _, err := os.Stat(filepath.Join(revDir, client, context)); err != nil {
				t.Errorf("revision is missing %s/%s: %v", client, context, err)
			}
		}
	}
}

// TestInstallPinsTheAgentpakkeManifest: the revision carries the manifest the
// payloads were built with, at the conventional path, so the launch reads
// persona, model and the compatibility gate from the pinned revision rather
// than from whatever the default branch says today.
func TestInstallPinsTheAgentpakkeManifest(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	revDir := pakkeRevisionDir(src.Repo, src.SHA)
	if _, err := os.Stat(filepath.Join(revDir, agentpakke.ManifestDir, agentpakke.ManifestFile)); err != nil {
		t.Fatalf("the revision does not carry %s: %v", agentpakke.ManifestPath, err)
	}

	pinned := &Source{Dir: revDir, SHA: src.SHA, Repo: src.Repo}
	if err := attachPakke(pinned); err != nil {
		t.Fatalf("attachPakke on the revision directory: %v", err)
	}
	if pinned.Pakke == nil {
		t.Fatal("the revision directory loaded no manifest")
	}
	got := pinned.Pakke.PayloadPrimaryAgents("copilot", "focused")
	want := src.Pakke.PayloadPrimaryAgents("copilot", "focused")
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("pinned primaryAgents = %v, want %v", got, want)
	}
}

// TestInstallMaterializesUnlaunchableClients: a payload for a client this
// binary cannot stage-launch is still materialized. Filtering to the launchable
// ones would be a second client list to keep in step with the launch switch.
func TestInstallMaterializesUnlaunchableClients(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	dir := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), unlaunchableClient, "full")
	if err := agentpakke.VerifyPayloadExact(dir, filepath.Join(dir, agentpakke.PayloadManifestFile)); err != nil {
		t.Errorf("the %s payload was not materialized: %v", unlaunchableClient, err)
	}
}

// TestRevisionIsPublishedByRename: a revision directory is either complete or
// absent. Staging happens in a sibling temp directory and is published with one
// rename, so a failure on the last context leaves nothing a launch could find.
func TestRevisionIsPublishedByRename(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")

	// pi/full is staged last (clients sorted, contexts sorted), so breaking it
	// guarantees the failure lands with the revision otherwise complete.
	mustWrite(t, filepath.Join(src.Dir, "dist", unlaunchableClient, "full", "agents", "grillmester.agent.md"), "tampered\n")

	if _, err := materializeRevision(src); err == nil {
		t.Fatal("materializeRevision = nil with a payload that does not verify, want an error")
	}
	if _, err := os.Stat(pakkeRevisionDir(src.Repo, src.SHA)); !os.IsNotExist(err) {
		t.Errorf("a failed materialization published %s (stat err %v); the revision must not exist until it is complete",
			pakkeRevisionDir(src.Repo, src.SHA), err)
	}

	if err := installPakkePin(scope, src, false, false); err == nil {
		t.Fatal("installPakkePin = nil for a source that does not verify, want an error")
	}
	if state, _ := readScopedState(scope); state != nil {
		t.Error("a failed install wrote a state file; the pin must be recorded only after the revision is published")
	}
}

// plantStagingTree writes a complete, verifiable tree under a .tmp-* name —
// what a materialization holds just before its rename, and what a crash leaves
// behind.
func plantStagingTree(t *testing.T, src *Source, name string) string {
	t.Helper()
	dir := filepath.Join(pakkeSourceDir(src.Repo), name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := stageRevision(src, dir); err != nil {
		t.Fatalf("setting up the staging tree: %v", err)
	}
	return dir
}

// TestCrashLeftoverTmpIsNotLaunched: a materialization that died before its
// rename leaves a .tmp-* tree holding a complete, verifiable revision. No
// launch may reach it, because a lookup names the recorded SHA rather than
// whatever the source's directory happens to hold — the state below records a
// pin, so the lookup runs all the way to the stat and answers "no pin" for the
// reason this test is named after, not because there was nothing to read.
func TestCrashLeftoverTmpIsNotLaunched(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	leftover := plantStagingTree(t, src, ".tmp-crashed")

	if err := writeScopedState(scope, &StateFile{
		Collection: src.Pakke.Name,
		Scope:      scope.Name,
		SourceRepo: src.Repo,
		SourceSHA:  src.SHA,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pakkeRevisionDir(src.Repo, src.SHA)); !os.IsNotExist(err) {
		t.Fatalf("setup: %s exists (stat err %v); only the .tmp-* tree may be on disk", pakkeRevisionDir(src.Repo, src.SHA), err)
	}

	failingResolveSource(t)
	rev, err := pinnedRevision(src.Repo)
	if err != nil {
		t.Fatalf("pinnedRevision over a leftover tmp tree = %v, want no pin and no error", err)
	}
	if rev != nil {
		t.Errorf("pinnedRevision returned %s; the lookup must name the recorded SHA, not %s", rev.Dir, leftover)
	}
}

// TestPruneLeavesLiveStagingTreesAlone: a .tmp-* directory is not garbage, it
// is another process's revision being built right now — a launch racing
// `sync --apply`, or two first launches at different SHAs. Deleting one
// mid-write fails that process, or, if the timing lands between staging and the
// rename, publishes a half-deleted tree as a revision. materializeRevision
// removes its own staging tree on every error path, so a leak needs a hard kill
// and stays bounded.
func TestPruneLeavesLiveStagingTreesAlone(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	live := plantStagingTree(t, src, ".tmp-inflight")

	installPin(t, scope, src)

	if _, err := os.Stat(live); err != nil {
		t.Errorf("the prune removed %s (%v); a .tmp-* tree may belong to a materialization in flight", live, err)
	}
	assertRevisionVerifies(t, pakkeRevisionDir(src.Repo, src.SHA))
}

// TestBrokenRevisionIsRebuiltNotAdopted: a revision directory that exists but
// does not verify is rebuilt, not handed back. A crash between the rename and
// the file data reaching disk leaves exactly that, and adopting it unverified
// wedges the SHA permanently — every launch fails closed, sync says "up to
// date", and an install of the same SHA prints "pinned at <sha>" over a tree it
// never looked at.
func TestBrokenRevisionIsRebuiltNotAdopted(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	revDir := pakkeRevisionDir(src.Repo, src.SHA)
	broken := filepath.Join(revDir, "copilot", "full", "agents", "grillmester.agent.md")
	if err := os.WriteFile(broken, []byte("truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyRevision(src, revDir); err == nil {
		t.Fatal("setup: the revision still verifies after being broken")
	}

	installPin(t, scope, src)

	if err := verifyRevision(src, revDir); err != nil {
		t.Errorf("the install adopted the broken revision instead of rebuilding it: %v", err)
	}
	assertRevisionVerifies(t, revDir)
}

// TestConcurrentMaterializeShareOneRevision: two first launches at once both
// build a tree, one wins the rename, and the loser adopts the winner's revision
// rather than failing on a directory that is not empty.
func TestConcurrentMaterializeShareOneRevision(t *testing.T) {
	pinEnv(t)
	src := tier2PinSource(t, "sha-one")

	const n = 4
	dirs := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dirs[i], errs[i] = materializeRevision(src)
		}()
	}
	wg.Wait()

	want := pakkeRevisionDir(src.Repo, src.SHA)
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("concurrent materializeRevision #%d = %v, want nil", i, errs[i])
		}
		if dirs[i] != want {
			t.Errorf("concurrent materializeRevision #%d = %q, want %q", i, dirs[i], want)
		}
	}
	assertRevisionVerifies(t, want)
	if got := revisionNames(t, src.Repo); len(got) != 1 || got[0] != src.SHA {
		t.Errorf("revision directory holds %v, want exactly [%s]", got, src.SHA)
	}
}

// TestLocalRematerializeNeverPublishesAHalfDeletedRevision: a local source is
// rebuilt in place on every launch, and the revision directory it rebuilds is
// what a running session was handed — provider/staged_launch.go passes the
// payload directory as OPENCODE_CONFIG_DIR and to cplt as --allow-read. The old
// tree was removed before the new one was renamed in, so a session started in
// another window walked the first one's config directory file by file while it
// was being emptied: the directory still there, its files going one by one
// (#504, U9).
//
// Moving the outgoing tree aside instead makes every state the path is ever
// observed in a complete one — the old revision, then the new one. It is not
// gone at once: there is still one rename's worth of "not there yet" between
// the two, since exchanging two directories atomically is not portable. That is
// a moment, not the duration of a recursive delete, and "absent" is a state a
// reader can retry, while "there but half empty" is one it cannot tell from a
// broken pakke.
//
// The assertion is one-sided on purpose. The fixed code cannot publish a
// partial tree — the only thing that ever appears at the path is a completed
// rename — so the test cannot flake; the removal it replaced is many unlinks
// long, so the watcher below sees it.
func TestLocalRematerializeNeverPublishesAHalfDeletedRevision(t *testing.T) {
	pinEnv(t)
	tree := tier2PinSourceTree(t)
	src := localPinSource(t, tree)

	revDir, err := materializeRevision(src)
	if err != nil {
		t.Fatal(err)
	}
	agentFile := filepath.Join(revDir, unlaunchableClient, "full", "agents", "grillmester.agent.md")

	// What a completed revision holds at its root: one directory per client,
	// plus the pinned manifest's.
	entries, err := os.ReadDir(revDir)
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := len(entries)

	// The descriptor a live session holds on its config directory. It reads on
	// through the republish, because the tree it was opened on is moved rather
	// than deleted under it.
	live, err := os.Open(agentFile)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()

	// Each round is the author editing the working tree and starting a second
	// window while the first session is still running.
	for i := range 20 {
		writeTier2Payload(t, filepath.Join(tree, "dist", unlaunchableClient, "full"), fmt.Sprintf("edit-%d", i))

		var partial atomic.Bool
		done := make(chan error, 1)
		go func() {
			_, err := materializeRevision(src)
			done <- err
		}()
		for {
			// One os.Open resolves the path once, so the listing that follows
			// describes whatever directory was at revDir at that instant, even
			// if it is renamed a moment later. Two stats would race the rename
			// and report a partial tree that was never published.
			//
			// Resolving once is necessary but not sufficient, and that gap is
			// what flaked this test on macOS (#668). The open can land on the
			// outgoing tree microseconds before the republish renames it to
			// aside; os.RemoveAll(aside) then empties it under this descriptor,
			// and on APFS readdir reports the shrinking directory rather than
			// the tree as it stood when it was opened. (Measured: a dirfd held
			// across a concurrent RemoveAll returns a partial listing in
			// roughly one run in two hundred on APFS. An open *file* descriptor
			// is unaffected, which is why the live handle below reads through
			// cleanly on both platforms.)
			//
			// That short listing is the retired revision going away on
			// schedule, not a half-deleted one being published, and the two are
			// told apart by identity rather than by timing: revDir is only ever
			// created by renaming a fully staged tree onto it and only ever
			// leaves by being renamed aside, so a listing counts against the
			// invariant only when the directory it was taken on is still the
			// one at revDir. Under the removal this fix replaced the old tree
			// was emptied in place, at the path, so the descriptor and the path
			// stayed the same directory and the check below still catches it.
			if d, err := os.Open(revDir); err == nil {
				names, readErr := d.Readdirnames(-1)
				opened, openedErr := d.Stat()
				d.Close()
				current, currentErr := os.Stat(revDir)
				stillPublished := openedErr == nil && currentErr == nil && os.SameFile(opened, current)
				// A readdir error on the tree that is still at revDir counts as
				// a partial listing rather than as no observation. Some
				// platforms return a short listing together with a non-nil
				// error under a concurrent rename or remove, and swallowing
				// that would let the test miss exactly what it is looking for.
				if stillPublished && (readErr != nil || len(names) != wantEntries) {
					partial.Store(true)
				}
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("re-materializing a local source: %v", err)
				}
				if partial.Load() {
					t.Fatalf("round %d: %s was published half deleted; a session reading it as OPENCODE_CONFIG_DIR sees its own config files disappear one by one", i, revDir)
				}
			default:
				continue
			}
			break
		}
	}

	body, err := io.ReadAll(live)
	if err != nil {
		t.Errorf("the live session's descriptor stopped reading after the republish: %v", err)
	}
	if !strings.Contains(string(body), unlaunchableClient+"-full") {
		t.Errorf("the live descriptor read %q, want the revision it was opened on", body)
	}
	fresh, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fresh), "edit-19") {
		t.Errorf("the published revision is %q, want the last edit", fresh)
	}
	if names := revisionNames(t, src.Repo); len(names) != 1 || names[0] != src.SHA {
		t.Errorf("the revision root holds %v, want exactly [%s]: no staging or set-aside tree may be left behind", names, src.SHA)
	}
}

// ─── install: scope, state and what a pin replaces ───────────────────────────

// TestTier2InstallRefusesRepoScope: the pin is read from user scope by every
// launch, so a repo-scope pin would be a state file nothing ever reads — an
// install that reports success and changes nothing.
func TestTier2InstallRefusesRepoScope(t *testing.T) {
	pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	target := repoTarget(t)
	scope := ScopeRepo(target)

	err := cmdInstallFromSource("grillmester", src, scope, false, false, false)
	if err == nil {
		t.Fatal("a repo-scope Tier 2 install succeeded, want a refusal")
	}
	if !strings.Contains(err.Error(), "nav-pilot install --user") {
		t.Errorf("error %q does not point at the scope that works", err)
	}
	if state, _ := readScopedState(scope); state != nil {
		t.Error("the refused install wrote a repo-scope state file")
	}
	if entries, _ := os.ReadDir(filepath.Join(target, ".github")); len(entries) > 0 {
		t.Errorf("the refused install left %d entries under .github/", len(entries))
	}
}

// TestUpdateKeepsExactlyTwoRevisions: the current pin and the one it replaced.
func TestUpdateKeepsExactlyTwoRevisions(t *testing.T) {
	scope := pinEnv(t)
	tree := tier2PinSourceTree(t)
	for _, sha := range []string{"sha-one", "sha-two", "sha-three"} {
		src := &Source{Dir: tree, SHA: sha, Version: "dev", Repo: "navikt/grillmester"}
		if err := attachPakke(src); err != nil {
			t.Fatal(err)
		}
		installPin(t, scope, src)
	}

	got := revisionNames(t, "navikt/grillmester")
	want := []string{"sha-three", "sha-two"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("kept revisions %v, want %v", got, want)
	}
}

// TestOldRevisionSurvivesOneUpdate: a directory handed to a running session is
// still there, and still verifiable, after the pin moves on once. Content
// addressing is what makes that free — the new revision is a new directory.
func TestOldRevisionSurvivesOneUpdate(t *testing.T) {
	scope := pinEnv(t)
	tree := tier2PinSourceTree(t)

	first := &Source{Dir: tree, SHA: "sha-one", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(first); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, first)
	handedOut := pakkeRevisionDir(first.Repo, first.SHA)

	second := &Source{Dir: tree, SHA: "sha-two", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(second); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, second)

	assertRevisionVerifies(t, handedOut)
}

// TestSourceSwitchRemovesOrphanedTier1Files: installing a Tier 2 agentpakke
// over a Tier 1 install from another source removes the files the outgoing
// install left, which nothing would track any more. The explicit install is the
// consent gesture for that.
func TestSourceSwitchRemovesOrphanedTier1Files(t *testing.T) {
	scope := pinEnv(t)

	legacy := &Source{Dir: legacySourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/other"}
	if err := attachPakke(legacy); err != nil {
		t.Fatal(err)
	}
	if err := cmdInstallFromSource("fullstack", legacy, scope, false, false, false); err != nil {
		t.Fatalf("Tier 1 install: %v", err)
	}
	installed := filepath.Join(scope.RootDir, "agents", "test-a.agent.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("setup: the Tier 1 install wrote nothing: %v", err)
	}

	installPin(t, scope, tier2PinSource(t, "sha-one"))

	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Errorf("%s survived the source switch (stat err %v); it is orphaned and must be removed", installed, err)
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = (%v, %v), want the new pin", state, err)
	}
	if state.SourceRepo != "navikt/grillmester" || state.SourceSHA != "sha-one" || len(state.Files) != 0 {
		t.Errorf("state = %+v, want a zero-item pin on navikt/grillmester@sha-one", state)
	}
}

// TestPakkerRootFollowsConfig keeps pinned revisions inside nav-pilot's own
// home, and out of the developer's when tests redirect the config.
func TestPakkerRootFollowsConfig(t *testing.T) {
	cfg := isolatedConfig(t)
	if got, want := pakkerRoot(), filepath.Join(filepath.Dir(cfg), "pakker"); got != want {
		t.Errorf("pakkerRoot() = %q, want %q", got, want)
	}
}

// ─── the launch reads the pin ────────────────────────────────────────────────

// launchPinned runs the launch path for the client the fixture declares but
// this binary cannot stage-launch, so the launch runs to the very end —
// verification done, active agentpakke set — without executing anything.
func launchPinned(t *testing.T, context string) error {
	t.Helper()
	_, err := tryPakkeLaunch(ResolvedConfig{
		Client: unlaunchableClient, Source: "navikt/grillmester", PayloadContext: context,
	})
	return err
}

// assertHandedOver fails unless the launch got all the way to the handover.
func assertHandedOver(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), handoverErr) {
		t.Fatalf("launch = %v, want it to reach the handover for %s", err, unlaunchableClient)
	}
	// The pinned manifest is what SetActivePakke installed, so the payload
	// roster it declares is the one the launch would have handed over.
	if got := providerpkg.PrimaryAgentFor("copilot", "full"); got != "grillmester" {
		t.Errorf("the pinned agentpakke was not activated: PrimaryAgentFor(copilot, full) = %q", got)
	}
}

// assertRefusedBeforeHandover fails unless the launch stopped before it could
// point a client at the tree.
func assertRefusedBeforeHandover(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatal("launch = nil, want a refusal")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("launch error %q does not name %q", err, want)
	}
	if strings.Contains(err.Error(), handoverErr) {
		t.Fatal("the launch reached the handover; it must refuse before that")
	}
	assertDefaultPakkeActive(t)
}

// TestPinnedLaunchDoesNotResolve is the acceptance point: once a source is
// installed, launching it reads the local revision and clones nothing.
func TestPinnedLaunchDoesNotResolve(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))

	failingResolveSource(t)
	assertHandedOver(t, launchPinned(t, ""))
}

// TestPinnedLaunchUsesRecordedSHANotHEAD: the pin does not advance because the
// default branch moved. A new revision arrives through sync, not by launching
// again.
func TestPinnedLaunchUsesRecordedSHANotHEAD(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))

	moved := tier2PinSource(t, "sha-two")
	stubResolveSource(t, moved)

	assertHandedOver(t, launchPinned(t, ""))
	if _, err := os.Stat(pakkeRevisionDir(moved.Repo, "sha-two")); !os.IsNotExist(err) {
		t.Errorf("the launch materialized sha-two (stat err %v); only sync advances the pin", err)
	}
}

// TestPinnedLaunchVerifiesBeforeHandingOver: the tree has been on disk since
// the pin was made, so it is verified where it lies, not trusted.
func TestPinnedLaunchVerifiesBeforeHandingOver(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	target := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), unlaunchableClient, "full", "agents", "grillmester.agent.md")
	if err := os.WriteFile(target, []byte("---\nname: grillmester\n---\nflipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	failingResolveSource(t)
	assertRefusedBeforeHandover(t, launchPinned(t, ""), "does not match its manifest")
}

// TestPinnedLaunchRejectsPlantedExtraFile: the verifier walks the tree in both
// directions, so a file the manifest does not list is a refusal. This is the
// check an aged tree needs most — planting a file is the cheapest tamper.
func TestPinnedLaunchRejectsPlantedExtraFile(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	planted := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), unlaunchableClient, "full", "agents", "planted.agent.md")
	if err := os.WriteFile(planted, []byte("---\nname: planted\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	failingResolveSource(t)
	assertRefusedBeforeHandover(t, launchPinned(t, ""), "unmanifested")
}

// TestPinnedLaunchRejectsSymlinkSwap: swapping a manifested file for a symlink
// to identical content is refused on the link itself, before the content is
// ever read.
func TestPinnedLaunchRejectsSymlinkSwap(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	dir := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), unlaunchableClient, "full")
	target := filepath.Join(dir, "agents", "grillmester.agent.md")
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	twin := filepath.Join(t.TempDir(), "twin.md")
	if err := os.WriteFile(twin, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(twin, target); err != nil {
		t.Fatal(err)
	}

	failingResolveSource(t)
	assertRefusedBeforeHandover(t, launchPinned(t, ""), "symlink")
}

// TestPinnedRevisionWithUnusableManifestFailsClosed: a pinned manifest this
// binary cannot use must say so. Falling through to a resolve would put the
// user back on a moving default branch the day a pakke bumps its contract
// major — which is the failure the pin exists to remove.
func TestPinnedRevisionWithUnusableManifestFailsClosed(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	pinnedManifest := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), agentpakke.ManifestDir, agentpakke.ManifestFile)
	data, err := os.ReadFile(pinnedManifest)
	if err != nil {
		t.Fatal(err)
	}
	bumped := strings.Replace(string(data), `"contractVersion": "1"`, `"contractVersion": "2"`, 1)
	if bumped == string(data) {
		t.Fatal("setup: contractVersion was not rewritten")
	}
	if err := os.WriteFile(pinnedManifest, []byte(bumped), 0o644); err != nil {
		t.Fatal(err)
	}

	failingResolveSource(t)
	handled, err := tryPakkeLaunch(ResolvedConfig{Client: unlaunchableClient, Source: "navikt/grillmester"})
	if !handled || err == nil {
		t.Fatalf("tryPakkeLaunch(unusable pinned manifest) = (%v, %v), want (true, error)", handled, err)
	}
	if !strings.Contains(err.Error(), "contractVersion") {
		t.Errorf("error should name the manifest problem, got: %v", err)
	}
	assertDefaultPakkeActive(t)
}

// TestPinIgnoresMismatchedSourceRepo: a scope installed from one agentpakke
// must not hand its revision to a launch configured for another.
func TestPinIgnoresMismatchedSourceRepo(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))

	rev, err := pinnedRevision("navikt/somewhere-else")
	if err != nil {
		t.Fatalf("pinnedRevision = %v, want no error", err)
	}
	if rev != nil {
		t.Errorf("pinnedRevision for a different source returned %s; the recorded repo must match", rev.Dir)
	}
}

// TestUninstalledSourceStillResolvesLive: a state entry naming a SHA whose
// revision is gone — an older install, or a hand-deleted directory — falls
// through to a normal resolve rather than failing.
func TestUninstalledSourceStillResolvesLive(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	if err := os.RemoveAll(pakkeSourceDir(src.Repo)); err != nil {
		t.Fatal(err)
	}

	rev, err := pinnedRevision(src.Repo)
	if err != nil {
		t.Fatalf("pinnedRevision with no revision on disk = %v, want no error", err)
	}
	if rev != nil {
		t.Fatalf("pinnedRevision returned %s for a revision that is not there", rev.Dir)
	}

	// And the launch that follows resolves, re-pins and runs.
	stubResolveSource(t, tier2PinSource(t, "sha-one"))
	assertHandedOver(t, launchPinned(t, ""))
}

// ─── auto-pin on first launch ────────────────────────────────────────────────

// TestFirstLaunchAutoPins: an un-installed payload-only source is pinned by the
// launch that first needs it, rather than refused or staged to a temp tree.
func TestFirstLaunchAutoPins(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	calls := countingResolveSource(t, src)

	assertHandedOver(t, launchPinned(t, ""))

	if *calls != 1 {
		t.Errorf("resolveSource called %d times, want 1", *calls)
	}
	assertRevisionVerifies(t, pakkeRevisionDir(src.Repo, src.SHA))
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = (%v, %v), want the pin the launch wrote", state, err)
	}
	if state.SourceRepo != src.Repo || state.SourceSHA != src.SHA {
		t.Errorf("state = %+v, want a pin on %s@%s", state, src.Repo, src.SHA)
	}
}

// TestSecondLaunchDoesNotResolve: auto-pin is once per revision, not once per
// launch. Writing the revision without recording the pin would leave every
// launch re-cloning.
func TestSecondLaunchDoesNotResolve(t *testing.T) {
	pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	calls := countingResolveSource(t, src)

	assertHandedOver(t, launchPinned(t, ""))
	if *calls != 1 {
		t.Fatalf("setup: resolveSource called %d times on the first launch, want 1", *calls)
	}

	providerpkg.SetActivePakke(nil)
	failingResolveSource(t)
	assertHandedOver(t, launchPinned(t, ""))
}

// TestAutoPinRefusesToClobberForeignInstall: pinning replaces the scope's
// state, and the install path removes the outgoing install's files with the
// user's explicit consent. A launch has no such consent, so it refuses instead.
func TestAutoPinRefusesToClobberForeignInstall(t *testing.T) {
	scope := pinEnv(t)

	legacy := &Source{Dir: legacySourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/other"}
	if err := attachPakke(legacy); err != nil {
		t.Fatal(err)
	}
	if err := cmdInstallFromSource("fullstack", legacy, scope, false, false, false); err != nil {
		t.Fatalf("Tier 1 install: %v", err)
	}
	installed := filepath.Join(scope.RootDir, "agents", "test-a.agent.md")

	stubResolveSource(t, tier2PinSource(t, "sha-one"))
	assertRefusedBeforeHandover(t, launchPinned(t, ""), "nav-pilot install --user")

	if _, err := os.Stat(installed); err != nil {
		t.Errorf("the refused launch removed %s: %v", installed, err)
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil || state.SourceRepo != "navikt/other" {
		t.Errorf("state = (%+v, %v), want the untouched Tier 1 install", state, err)
	}
}

// ─── the Tier 1 guard that step 4 must not have widened ──────────────────────

// TestEmptyTier1LayoutStillErrors: making the empty-contents error conditional
// on a declared layout must not disarm it for a Tier 1 pakke whose layout paths
// ship nothing. That message is the reason the condition exists.
func TestEmptyTier1LayoutStillErrors(t *testing.T) {
	isolatedConfig(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), `{
	  "contractVersion": "1",
	  "name": "tom",
	  "description": "d",
	  "clients": {"copilot": {"primaryAgents": ["a"]}},
	  "layout": {"agents": "plugin/agents", "skills": "plugin/skills"}
	}`)
	mustWrite(t, filepath.Join(dir, "plugin", "agents", ".keep"), "")
	mustWrite(t, filepath.Join(dir, "plugin", "skills", ".keep"), "")

	src := &Source{Dir: dir, SHA: "abc1234", Version: "dev", Repo: "navikt/tom"}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	_, err := pakkeContents(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatal("pakkeContents = nil for a layout that ships nothing, want an error")
	}
	if !strings.Contains(err.Error(), "declares a layout but ships no agents") {
		t.Errorf("error %q is not the empty-layout message", err)
	}
}

// ─── sync is the update path ─────────────────────────────────────────────────

// pinnedSyncSource makes sync resolve to the given tree at the given SHA,
// manifest attached, exactly as the real resolver would.
func pinnedSyncSource(t *testing.T, tree, sha string) {
	t.Helper()
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		src := &Source{Dir: tree, SHA: sha, Version: "dev", Repo: sourceRepo}
		return src, attachPakke(src)
	}
}

// TestPinnedSyncReportsAndAdvances: a pinned install's update unit is the
// revision, so sync must compare SHAs and re-pin rather than fall through the
// file diff to "No customization files found to sync." and report success over
// an install that can never move.
func TestPinnedSyncReportsAndAdvances(t *testing.T) {
	scope := pinEnv(t)
	tree := tier2PinSourceTree(t)
	first := &Source{Dir: tree, SHA: "sha-one", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(first); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, first)

	// A newer revision, without --apply: reported, not applied.
	pinnedSyncSource(t, tree, "sha-two")
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != errUpdatesAvailable {
		t.Fatalf("cmdSync = %v, want errUpdatesAvailable", err)
	}
	if strings.Contains(out, "No customization files found to sync") {
		t.Errorf("sync took the file-diff dead end instead of the pin path. Output:\n%s", out)
	}
	if !strings.Contains(out, "sha-two") {
		t.Errorf("sync did not name the available revision. Output:\n%s", out)
	}
	if state, _ := readScopedState(scope); state == nil || state.SourceSHA != "sha-one" {
		t.Errorf("a check-only sync moved the pin: %+v", state)
	}
	if _, statErr := os.Stat(pakkeRevisionDir("navikt/grillmester", "sha-two")); !os.IsNotExist(statErr) {
		t.Errorf("a check-only sync materialized sha-two (stat err %v)", statErr)
	}

	// With --apply: the new revision is materialized and the pin swaps.
	captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("cmdSync --apply = %v, want nil", err)
	}
	state, _ := readScopedState(scope)
	if state == nil || state.SourceSHA != "sha-two" {
		t.Fatalf("state after --apply = %+v, want the pin at sha-two", state)
	}
	assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", "sha-two"))

	// Same SHA again: up to date, no error, no new revision.
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil {
		t.Fatalf("cmdSync at the pinned SHA = %v, want nil", err)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("sync at the pinned SHA did not report it. Output:\n%s", out)
	}
}

// ─── uninstall drops the revisions ───────────────────────────────────────────

// TestUninstallRemovesRevisions: a pinned install materializes nothing inside
// the scope, so uninstall's file loop removes nothing. If it does not also drop
// the revisions, uninstall leaves fully materialized payload trees that nothing
// will ever reference or remove.
func TestUninstallRemovesRevisions(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))
	sourceDir := pakkeSourceDir("navikt/grillmester")
	if _, err := os.Stat(sourceDir); err != nil {
		t.Fatalf("setup: the install materialized nothing: %v", err)
	}

	captureStdoutFor(t, func() {
		if err := cmdUninstall(scope, true); err != nil {
			t.Fatalf("dry-run uninstall: %v", err)
		}
	})
	if _, err := os.Stat(sourceDir); err != nil {
		t.Errorf("the dry run removed the revisions: %v", err)
	}

	captureStdoutFor(t, func() {
		if err := cmdUninstall(scope, false); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
	})
	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		t.Errorf("%s survived uninstall (stat err %v)", sourceDir, err)
	}
	if state, _ := readScopedState(scope); state != nil {
		t.Errorf("uninstall left the state file: %+v", state)
	}
}

// TestSourceSwitchRemovesOldRevisions: a Tier 2 install tracks no files, so
// what a switch between two Tier 2 sources leaves behind is whole materialized
// payload trees. Nothing would reach them again — the prune reads only the
// incoming source's directory, and uninstall's state names the incoming source
// — so the switch removes them.
func TestSourceSwitchRemovesOldRevisions(t *testing.T) {
	scope := pinEnv(t)

	outgoing := tier2PinSource(t, "sha-a")
	installPin(t, scope, outgoing)
	if _, err := os.Stat(pakkeRevisionDir(outgoing.Repo, outgoing.SHA)); err != nil {
		t.Fatalf("setup: the first install materialized nothing: %v", err)
	}

	incoming := &Source{Dir: tier2PinSourceTree(t), SHA: "sha-b", Version: "dev", Repo: "navikt/other-pakke"}
	if err := attachPakke(incoming); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, incoming)

	if _, err := os.Stat(pakkeSourceDir(outgoing.Repo)); !os.IsNotExist(err) {
		t.Errorf("%s survived the switch to %s (stat err %v); its revisions are unreachable and must be removed",
			pakkeSourceDir(outgoing.Repo), incoming.Repo, err)
	}
	assertRevisionVerifies(t, pakkeRevisionDir(incoming.Repo, incoming.SHA))
}

// TestDryRunValidatesTheSource: a dry run answering "would install" for a
// source the real install refuses is worse than no dry run at all. Validation
// writes nothing, so there is nothing to skip it for.
func TestDryRunValidatesTheSource(t *testing.T) {
	t.Run("a source that does not verify is refused", func(t *testing.T) {
		scope := pinEnv(t)
		src := tier2PinSource(t, "sha-one")
		mustWrite(t, filepath.Join(src.Dir, "dist", "copilot", "full", "agents", "grillmester.agent.md"), "tampered\n")

		err := installPakkePin(scope, src, true, false)
		if err == nil {
			t.Fatal("dry run over a source that does not verify = nil, want the refusal the real install gives")
		}
		if !strings.Contains(err.Error(), "does not conform") {
			t.Errorf("error %q does not name the conformance failure", err)
		}
	})

	t.Run("a conforming source stays a no-op", func(t *testing.T) {
		scope := pinEnv(t)
		src := tier2PinSource(t, "sha-one")

		if err := installPakkePin(scope, src, true, false); err != nil {
			t.Fatalf("dry run over a conforming source = %v, want nil", err)
		}
		if _, err := os.Stat(pakkerRoot()); !os.IsNotExist(err) {
			t.Errorf("the dry run materialized something (stat err %v)", err)
		}
		if state, _ := readScopedState(scope); state != nil {
			t.Error("the dry run wrote a state file")
		}
	})
}

// TestPreTrackingInstallDoesNotWipeEveryRevision: an install predating source
// tracking records no source repo, and that empty string is not a source to
// remove the revisions of — pakkeSourceDir("") is the root holding every
// source's revisions, the one just materialized included.
func TestPreTrackingInstallDoesNotWipeEveryRevision(t *testing.T) {
	scope := pinEnv(t)
	if err := writeScopedState(scope, &StateFile{
		Collection: "fullstack",
		Scope:      "user",
		Files:      []InstalledFile{{Path: "agents/gone.agent.md", Hash: "x"}},
	}); err != nil {
		t.Fatal(err)
	}

	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	assertRevisionVerifies(t, pakkeRevisionDir(src.Repo, src.SHA))
}

// ─── what a pin replaces, and who is allowed to replace it ───────────────────

// tier1InstallFrom performs a Tier 1 install from a repo id and returns a path
// the install wrote, so a later assertion can tell whether it survived.
func tier1InstallFrom(t *testing.T, scope *InstallScope, repo string) string {
	t.Helper()
	legacy := &Source{Dir: legacySourceTree(t), SHA: "abc1234", Version: "dev", Repo: repo}
	if err := attachPakke(legacy); err != nil {
		t.Fatal(err)
	}
	if err := cmdInstallFromSource("fullstack", legacy, scope, false, false, false); err != nil {
		t.Fatalf("Tier 1 install from %s: %v", repo, err)
	}
	installed := filepath.Join(scope.RootDir, "agents", "test-a.agent.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("setup: the Tier 1 install wrote nothing: %v", err)
	}
	return installed
}

// TestSameRepoTierChangeRemovesOrphanedFiles: a pakke that shipped a layout and
// then goes payload-only is a shape change nav-pilot must pick up without a
// migration. The pin that replaces its state is a zero-item entry, so the files
// the old install tracked lose their only record — they would sit in ~/.copilot
// forever, invisible to uninstall and to sync. The repo not changing makes no
// difference to that; the tier changing is the whole of it.
func TestSameRepoTierChangeRemovesOrphanedFiles(t *testing.T) {
	scope := pinEnv(t)
	installed := tier1InstallFrom(t, scope, "navikt/grillmester")

	src := tier2PinSource(t, "sha-one")
	if !sameSourceRepo(src.Repo, "navikt/grillmester") {
		t.Fatalf("setup: the fixture must come from the same repo as the Tier 1 install, got %s", src.Repo)
	}
	installPin(t, scope, src)

	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Errorf("%s survived the tier change (stat err %v); nothing tracks it any more and it must be removed", installed, err)
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = (%+v, %v), want the new pin", state, err)
	}
	if len(state.Files) != 0 || state.SourceSHA != "sha-one" {
		t.Errorf("state = %+v, want a zero-item pin at sha-one", state)
	}
}

// TestAutoPinRefusesToClobberSameRepoFiles: the removal above is what an
// explicit install is consent for. A launch has no consent gesture at all, so
// the same shape change met on the launch path must refuse and name the command
// that does — not silently delete the user's Tier 1 content on the way to a
// client.
func TestAutoPinRefusesToClobberSameRepoFiles(t *testing.T) {
	scope := pinEnv(t)
	installed := tier1InstallFrom(t, scope, "navikt/grillmester")

	stubResolveSource(t, tier2PinSource(t, "sha-one"))
	launchErr := launchPinned(t, "")

	// The data loss first: it is what the refusal exists to prevent.
	if _, err := os.Stat(installed); err != nil {
		t.Errorf("the launch removed %s: %v", installed, err)
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil || len(state.Files) == 0 {
		t.Errorf("state = (%+v, %v), want the untouched Tier 1 install", state, err)
	}
	assertRefusedBeforeHandover(t, launchErr, "nav-pilot install --user")
}

// TestAutoPinRefusesToClobberAnotherSourcesPin: a Tier 2 pin tracks zero files,
// so a guard that only looks at tracked files lets a one-off `--source B`
// launch through — and pinning B removes every revision of A, as a side effect
// of a launch, while promising in the very message beside it that it will not.
// A user alternating sources would re-clone and re-materialize on every launch.
func TestAutoPinRefusesToClobberAnotherSourcesPin(t *testing.T) {
	scope := pinEnv(t)
	pinned := tier2PinSource(t, "sha-one")
	installPin(t, scope, pinned)

	other := &Source{Dir: tier2PinSourceTree(t), SHA: "sha-b", Version: "dev", Repo: "navikt/other-pakke"}
	if err := attachPakke(other); err != nil {
		t.Fatal(err)
	}
	stubResolveSource(t, other)

	_, err := tryPakkeLaunch(ResolvedConfig{Client: unlaunchableClient, Source: other.Repo})

	// The data loss first: it is what the refusal exists to prevent.
	assertRevisionVerifies(t, pakkeRevisionDir(pinned.Repo, pinned.SHA))
	assertRefusedBeforeHandover(t, err, "nav-pilot install --user")

	state, stateErr := readScopedState(scope)
	if stateErr != nil || state == nil || state.SourceSHA != "sha-one" || !sameSourceRepo(state.SourceRepo, pinned.Repo) {
		t.Errorf("state = (%+v, %v), want the untouched pin on %s@sha-one", state, stateErr, pinned.Repo)
	}
	if _, err := os.Stat(pakkeSourceDir(other.Repo)); !os.IsNotExist(err) {
		t.Errorf("the refused launch materialized %s (stat err %v)", pakkeSourceDir(other.Repo), err)
	}
}

// TestFlattenCollisionKeepsTheIncomingRevision: "/" → "-" flattening puts
// navikt/a-b and navikt-a/b in one directory. Switching between them removes
// the outgoing source's revisions, and that removal must not take the revision
// the same call just materialized with it — the state would then name a
// directory that does not exist, after printing "Installed".
func TestFlattenCollisionKeepsTheIncomingRevision(t *testing.T) {
	scope := pinEnv(t)

	outgoing := &Source{Dir: tier2PinSourceTree(t), SHA: "sha-a", Version: "dev", Repo: "navikt/a-b"}
	incoming := &Source{Dir: tier2PinSourceTree(t), SHA: "sha-b", Version: "dev", Repo: "navikt-a/b"}
	for _, src := range []*Source{outgoing, incoming} {
		if err := attachPakke(src); err != nil {
			t.Fatal(err)
		}
	}
	if pakkeSourceDir(outgoing.Repo) != pakkeSourceDir(incoming.Repo) {
		t.Fatalf("setup: %s and %s do not collide", outgoing.Repo, incoming.Repo)
	}

	installPin(t, scope, outgoing)
	installPin(t, scope, incoming)

	revDir := pakkeRevisionDir(incoming.Repo, incoming.SHA)
	if _, err := os.Stat(revDir); err != nil {
		t.Fatalf("the install removed the revision it just published (%v); %s names a directory that is not there", err, revDir)
	}
	assertRevisionVerifies(t, revDir)
	if state, _ := readScopedState(scope); state == nil || state.SourceSHA != "sha-b" {
		t.Errorf("state = %+v, want the pin at sha-b", state)
	}
}

// ─── a local path source is never pinned ─────────────────────────────────────

// localPinSource resolves an absolute-path source over a working tree, the way
// an agentpakke author develops one: no clone, and no SHA worth the name.
func localPinSource(t *testing.T, tree string) *Source {
	t.Helper()
	src := &Source{Dir: tree, SHA: "unknown", Version: "dev", Repo: tree}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	return src
}

// TestLocalSourceIsNeverPinned: `source = /path/to/pakke` is how the pakke is
// developed, and a non-git directory's SHA is the literal "unknown". Pinning
// one would freeze the working tree at whatever it held on the first launch —
// the revision would be adopted forever, and sync would compare "unknown"
// against "unknown" and report it up to date. So it re-materializes every
// launch, and no pin is written for it anywhere.
func TestLocalSourceIsNeverPinned(t *testing.T) {
	t.Run("every launch resolves and re-materializes", func(t *testing.T) {
		scope := pinEnv(t)
		tree := tier2PinSourceTree(t)
		calls := countingResolveSource(t, localPinSource(t, tree))

		launch := func() error {
			providerpkg.SetActivePakke(nil)
			_, err := tryPakkeLaunch(ResolvedConfig{Client: unlaunchableClient, Source: tree})
			return err
		}

		assertHandedOver(t, launch())
		if state, _ := readScopedState(scope); state != nil {
			t.Errorf("the launch pinned a local source: %+v", state)
		}

		// An edit to the working tree is what the developer loop is for.
		writeTier2Payload(t, filepath.Join(tree, "dist", unlaunchableClient, "full"), "edited")
		assertHandedOver(t, launch())

		if *calls != 2 {
			t.Errorf("resolveSource called %d times over two launches, want 2: a local source is re-read every launch", *calls)
		}
		body, err := os.ReadFile(filepath.Join(pakkeRevisionDir(tree, "unknown"),
			unlaunchableClient, "full", "agents", "grillmester.agent.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "edited") {
			t.Errorf("the launch handed over a stale revision: %q does not carry the edit", body)
		}
	})

	t.Run("a new commit does not leave the old revision behind", func(t *testing.T) {
		pinEnv(t)
		tree := tier2PinSourceTree(t)

		// A git-backed working tree resolves to its short HEAD, so the SHA
		// moves with every commit its author launches from — and a local
		// source records no state, so uninstall can never reach what those
		// launches materialize. Only the launch itself can.
		sha := "commit-one"
		orig := resolveSource
		t.Cleanup(func() { resolveSource = orig })
		resolveSource = func(string, string) (*Source, error) {
			src := &Source{Dir: tree, SHA: sha, Version: "dev", Repo: tree}
			return src, attachPakke(src)
		}

		launch := func() {
			providerpkg.SetActivePakke(nil)
			_, err := tryPakkeLaunch(ResolvedConfig{Client: unlaunchableClient, Source: tree})
			assertHandedOver(t, err)
		}

		launch()
		sha = "commit-two"
		launch()

		names := revisionNames(t, tree)
		if len(names) != 1 || names[0] != "commit-two" {
			t.Errorf("revisions after launching two commits = %v, want only [commit-two]: nothing else ever removes a local source's revisions", names)
		}
	})

	t.Run("installing one is refused", func(t *testing.T) {
		scope := pinEnv(t)
		src := localPinSource(t, tier2PinSourceTree(t))

		err := installPakkePin(scope, src, false, false)
		if err == nil {
			t.Fatal("installPakkePin over a local source = nil, want a refusal: there is no immutable revision to pin")
		}
		if !strings.Contains(err.Error(), "no immutable revision") {
			t.Errorf("error %q does not say why a local path cannot be pinned", err)
		}
		if state, _ := readScopedState(scope); state != nil {
			t.Errorf("the refused install wrote a pin: %+v", state)
		}
	})

	t.Run("a stale local revision is not adopted as a pin", func(t *testing.T) {
		pinEnv(t)
		tree := tier2PinSourceTree(t)
		src := localPinSource(t, tree)
		if _, err := materializeRevision(src); err != nil {
			t.Fatal(err)
		}

		rev, err := pinnedRevision(tree)
		if err != nil {
			t.Fatalf("pinnedRevision(local path) = %v, want no pin and no error", err)
		}
		if rev != nil {
			t.Errorf("pinnedRevision returned %s for a local path; its revision says nothing about the working tree", rev.Dir)
		}
	})
}

// TestPinnedInstallJSONEmitsOnlyJSON: an install that replaces a Tier 1 install
// removes the files it leaves behind and says so, and that report is written to
// the same stdout --json puts its document on. Anything parsing `nav-pilot
// install --user --json` gets a stream that is not JSON.
func TestPinnedInstallJSONEmitsOnlyJSON(t *testing.T) {
	scope := pinEnv(t)

	legacy := &Source{Dir: legacySourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/other"}
	if err := attachPakke(legacy); err != nil {
		t.Fatal(err)
	}
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("fullstack", legacy, scope, false, false, false); err != nil {
			t.Fatalf("Tier 1 install: %v", err)
		}
	})
	if state, _ := readScopedState(scope); state == nil || len(state.Files) == 0 {
		t.Fatalf("setup: the Tier 1 install recorded no files to remove: %+v", state)
	}

	var err error
	out := captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, "sha-one"), false, true) })
	if err != nil {
		t.Fatalf("installPakkePin --json: %v", err)
	}

	var doc map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(out), &doc); jsonErr != nil {
		t.Fatalf("install --json did not emit a JSON document (%v). Output:\n%s", jsonErr, out)
	}
	if doc["command"] != "install" || doc["source_sha"] != "sha-one" {
		t.Errorf("install --json document = %v, want the pin at sha-one", doc)
	}
}

// TestRecasedRepoIdKeepsOneRevisionDirectory: repo ids are case-insensitive on
// GitHub and sameSourceRepo folds case, so re-casing the configured source is
// the same install to every guard in the pin path — the B3 guard passes, the
// previous pin is carried over, uninstall keys on the state's repo. If the
// revision directory is named byte-exact instead, that install materializes
// under a second directory on a case-sensitive filesystem and the first one is
// left named by nothing: not the launch, not the prune, not uninstall.
func TestRecasedRepoIdKeepsOneRevisionDirectory(t *testing.T) {
	scope := pinEnv(t)

	if !sameSourceRepo("navikt/grillmester", "Navikt/Grillmester") {
		t.Fatal("sameSourceRepo no longer folds case; this test no longer describes the hazard")
	}
	// The direct check, because a case-insensitive filesystem hides the
	// consequence below while the disagreement is still there.
	if got, want := pakkeSourceDir("Navikt/Grillmester"), pakkeSourceDir("navikt/grillmester"); got != want {
		t.Errorf("pakkeSourceDir(%q) = %q, want %q: sameSourceRepo folds case and this does not, so a re-cased source id strands the revisions under the old name",
			"Navikt/Grillmester", got, want)
	}

	lower := tier2PinSource(t, "sha-one")
	installPin(t, scope, lower)

	upper := &Source{Dir: tier2PinSourceTree(t), SHA: "sha-two", Version: "dev", Repo: "Navikt/Grillmester"}
	if err := attachPakke(upper); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, upper)

	entries, err := os.ReadDir(pakkerRoot())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		t.Errorf("%s holds %v, want one directory: re-casing the source id is the same install, not a second one", pakkerRoot(), names)
	}
	assertRevisionVerifies(t, pakkeRevisionDir(upper.Repo, "sha-two"))
	if _, err := os.Stat(pakkeRevisionDir(lower.Repo, "sha-one")); err != nil {
		t.Errorf("the re-cased install dropped the revision it replaced (%v); a session running it survives one update", err)
	}
}

// The most likely Tier 2 launch failure — a pinned tree that drifted on disk —
// must name the command that rebuilds it (#504 U6).
func TestPayloadDriftRefusalNamesSyncApply(t *testing.T) {
	scope := pinEnv(t)
	src := tier2PinSource(t, "sha-one")
	installPin(t, scope, src)

	target := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), unlaunchableClient, "full", "agents", "grillmester.agent.md")
	if err := os.WriteFile(target, []byte("---\nname: grillmester\n---\nflipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	failingResolveSource(t)
	assertRefusedBeforeHandover(t, launchPinned(t, ""), "nav-pilot sync --apply")
}

// A client this binary cannot stage-launch must point at the binary upgrade,
// not dead-end (#504 U6).
func TestUnlaunchableClientRefusalNamesUpdate(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))

	err := launchPinned(t, "")
	if err == nil || !strings.Contains(err.Error(), handoverErr) {
		t.Fatalf("launch = %v, want the handover refusal", err)
	}
	if !strings.Contains(err.Error(), "nav-pilot update") {
		t.Errorf("handover refusal %q names no command to run", err)
	}
}

// TestFailedStateWriteLosesNothing pins the ordering of #504 U7. The old order
// deleted the outgoing Tier 1 install's files before writing the replacement
// state, so a state write that failed had already destroyed the install while
// the state on disk still named every deleted file. Every removal now runs
// after the new state is written: a failed write must abort the install with
// the outgoing files still on disk and still recorded.
func TestFailedStateWriteLosesNothing(t *testing.T) {
	scope := pinEnv(t)
	installed := tier1InstallFrom(t, scope, "navikt/grillmester")

	before, err := readScopedState(scope)
	if err != nil || before == nil || len(before.Files) == 0 {
		t.Fatalf("setup: readScopedState = (%+v, %v), want a Tier 1 state tracking files", before, err)
	}

	origWrite := writeScopedState
	writeScopedState = func(scope *InstallScope, state *StateFile) error {
		return errors.New("injected: disk full")
	}
	t.Cleanup(func() { writeScopedState = origWrite })

	err = installPakkePin(scope, tier2PinSource(t, "sha-one"), false, false)
	if err == nil || !strings.Contains(err.Error(), "writing state") {
		t.Fatalf("installPakkePin = %v, want the state-write failure", err)
	}

	if _, statErr := os.Stat(installed); statErr != nil {
		t.Errorf("%s is gone after a failed state write (stat: %v); the install was deleted before its replacement was recorded", installed, statErr)
	}
	writeScopedState = origWrite
	after, err := readScopedState(scope)
	if err != nil || after == nil {
		t.Fatalf("readScopedState after the failed install = (%+v, %v)", after, err)
	}
	if len(after.Files) != len(before.Files) {
		t.Errorf("state tracks %d files after the failed install, want the outgoing install's %d intact", len(after.Files), len(before.Files))
	}
}
