package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// A Tier 1 agentpakke follows stable releases the way a Tier 2 pin does
// (#794): install and sync read the newest stable release's revision, not
// whatever the default branch holds.

// tier1TreeAt writes a Tier 1 fixture whose agent body names the revision, so
// an assertion can tell which one an install or a sync actually read.
func tier1TreeAt(t *testing.T, sha string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), tier1ManifestJSON)
	mustWrite(t, filepath.Join(dir, "plugin", "agents", "grillmester.agent.md"),
		"---\nname: grillmester\ndescription: Chef\n---\nBody at "+sha+"\n")
	mustWrite(t, filepath.Join(dir, "plugin", "skills", "grilling", "SKILL.md"), "# Grilling\n")
	return dir
}

// tier1HeadSource is the source an install starts from: the default branch.
func tier1HeadSource(t *testing.T, sha string) *Source {
	t.Helper()
	src := &Source{Dir: tier1TreeAt(t, sha), SHA: sha, Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke on the Tier 1 fixture: %v", err)
	}
	if payloadOnly(src) {
		t.Fatal("fixture is payload-only; it must be Tier 1")
	}
	return src
}

// tier1ReleaseSource resolves the Tier 1 fixture at any ref, and records the
// refs asked for so a test can prove a release SHA was fetched.
func tier1ReleaseSource(t *testing.T, headSHA string) *[]string {
	t.Helper()
	refs := &[]string{}
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		*refs = append(*refs, ref)
		sha := headSHA
		if ref != "" {
			sha = ref
		}
		src := &Source{Dir: tier1TreeAt(t, sha), SHA: sha, Version: "dev", Repo: sourceRepo}
		return src, attachPakke(src)
	}
	return refs
}

// tier1Install runs a Tier 1 install of the fixture at headSHA into scope.
func tier1Install(t *testing.T, scope *InstallScope, headSHA string) string {
	t.Helper()
	var err error
	out := captureStdoutFor(t, func() {
		err = cmdInstallFromSource("grillmester", tier1HeadSource(t, headSHA), scope, false, false, false)
	})
	if err != nil {
		t.Fatalf("Tier 1 install = %v. Output:\n%s", err, out)
	}
	return out
}

// installedBody is the agent body a Tier 1 install wrote.
func installedBody(t *testing.T, scope *InstallScope) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(scope.RootDir, "agents", "grillmester.agent.md"))
	if err != nil {
		t.Fatalf("reading the installed agent: %v", err)
	}
	return string(b)
}

// assertTier1Release checks the revision a Tier 1 scope reads and the release
// claim recorded for it.
func assertTier1Release(t *testing.T, scope *InstallScope, sha, version string, follows bool) {
	t.Helper()
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = (%+v, %v), want a state", state, err)
	}
	if state.SourceSHA != sha {
		t.Errorf("state.SourceSHA = %s, want %s", state.SourceSHA, sha)
	}
	gotVersion, gotFollows := releaseClaim(state)
	if gotVersion != version || gotFollows != follows {
		t.Errorf("release claim = (%q, %v) [raw %q/%v/%q], want (%q, %v)",
			gotVersion, gotFollows, state.PakkeVersion, state.FollowsReleases, state.PakkeVersionSHA, version, follows)
	}
	if body := installedBody(t, scope); !strings.Contains(body, sha) {
		t.Errorf("the installed agent was written from another revision:\n%s", body)
	}
}

// TestTier1InstallFollowsTheNewestRelease: the install reads the release's
// exact revision, not the default branch it resolved, and records it.
func TestTier1InstallFollowsTheNewestRelease(t *testing.T) {
	scope := pinEnv(t)
	refs := tier1ReleaseSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	tier1Install(t, scope, shaB)

	assertTier1Release(t, scope, shaA, "0.4.1", true)
	if len(*refs) == 0 || (*refs)[0] != shaA {
		t.Errorf("refs fetched = %v, want the release SHA %s", *refs, shaA)
	}
}

// TestTier1SyncMovesOnlyOnANewRelease: the same release is up to date and
// changes nothing; a newer one moves the scope and is recorded.
func TestTier1SyncMovesOnlyOnANewRelease(t *testing.T) {
	scope := pinEnv(t)
	tier1ReleaseSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)
	tier1Install(t, scope, shaB)
	assertTier1Release(t, scope, shaA, "0.4.1", true)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync over the installed release = %v. Output:\n%s", err, out)
	}
	assertTier1Release(t, scope, shaA, "0.4.1", true)

	// A newer release appears.
	stubRelease(t, releaseCandidate, release042, nil)
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync onto a newer release = %v. Output:\n%s", err, out)
	}
	assertTier1Release(t, scope, release042.SHA, "0.4.2", true)
}

// TestTier1InstallWithoutReleasesUsesTheDefaultBranch: a source that publishes
// no release metadata installs exactly as it did before (#794).
func TestTier1InstallWithoutReleasesUsesTheDefaultBranch(t *testing.T) {
	scope := pinEnv(t) // discovery answers: no metadata
	refs := tier1ReleaseSource(t, shaB)

	tier1Install(t, scope, shaB)

	assertTier1Release(t, scope, shaB, "", false)
	if len(*refs) != 0 {
		t.Errorf("a source without release metadata fetched %v", *refs)
	}
}

// TestTier1ReleasePredatingTheManifestFallsBackToHead: the newest release
// names a revision that does not ship this agentpakke — a release cut before
// the manifest existed. Nothing new may fail: the install reads the default
// branch, as it did before releases.
func TestTier1ReleasePredatingTheManifestFallsBackToHead(t *testing.T) {
	scope := pinEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		src := &Source{Dir: legacySourceTree(t), SHA: ref, Version: "dev", Repo: sourceRepo}
		return src, attachPakke(src) // no manifest at the release revision
	}

	tier1Install(t, scope, shaB)

	assertTier1Release(t, scope, shaB, "", false)
}

// TestTier1SyncKeepsTheClaimWhenNothingChanges: a sync that finds nothing to do
// leaves the recorded revision and its release claim intact.
func TestTier1SyncKeepsTheClaimWhenNothingChanges(t *testing.T) {
	scope := pinEnv(t)
	tier1ReleaseSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)
	tier1Install(t, scope, shaB)

	before, _ := readScopedState(scope)
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil {
		t.Fatalf("sync check over an up-to-date release = %v. Output:\n%s", err, out)
	}
	after, _ := readScopedState(scope)
	if after == nil || after.SourceSHA != before.SourceSHA ||
		after.PakkeVersion != before.PakkeVersion ||
		after.PakkeVersionSHA != before.PakkeVersionSHA ||
		after.FollowsReleases != before.FollowsReleases {
		t.Errorf("state = %+v, want it unchanged from %+v", after, before)
	}
	assertTier1Release(t, scope, shaA, "0.4.1", true)
}

// TestTier1InstallKeepsADeclaredPin: a repo that committed a revision in
// .nav-pilot/agentpakke.lock.json has already chosen one, and install must not
// override a choice that was reviewed and merged.
func TestTier1InstallKeepsADeclaredPin(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+shaB+`"}`)
	refs := tier1ReleaseSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	out := captureStdoutFor(t, func() {
		err = cmdInstallFromSource("grillmester", tier1HeadSource(t, shaB), scope, false, false, false)
	})
	if err != nil {
		t.Fatalf("install over a declared pin = %v. Output:\n%s", err, out)
	}
	state, _ := readScopedState(scope)
	if state == nil || state.SourceSHA != shaB || state.FollowsReleases {
		t.Errorf("state = %+v, want the declared revision %s and no subscription", state, shaB)
	}
	if len(*refs) != 0 {
		t.Errorf("install over a declared pin fetched %v", *refs)
	}
}

// TestTier1FollowingNeverFallsBackToTheDefaultBranch: once an install follows
// stable releases, a repo that stops publishing metadata for the package —
// releases deleted, the asset withdrawn, a rename upstream that makes every
// release name another package — must not walk it back onto the default
// branch. That outcome carries a nil error, so it is not caught by the
// failed-lookup branch, and `sync --apply` rewrote every file from the default
// branch and dropped the subscription without a word.
func TestTier1FollowingNeverFallsBackToTheDefaultBranch(t *testing.T) {
	scope := pinEnv(t)
	tier1ReleaseSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)
	tier1Install(t, scope, shaB)
	assertTier1Release(t, scope, shaA, "0.4.1", true)

	stubRelease(t, releaseNoMetadata, pakkeRelease{}, nil) // the metadata is gone

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err == nil || err == errUpdatesAvailable {
		t.Fatalf("sync --apply with the metadata gone = %v, want a refusal. Output:\n%s", err, out)
	}
	if !strings.Contains(err.Error(), "does not fall back to the default branch") {
		t.Errorf("sync refusal = %v, want it to name the default branch", err)
	}
	assertTier1Release(t, scope, shaA, "0.4.1", true)

	out = captureStdoutFor(t, func() {
		err = cmdInstallFromSource("grillmester", tier1HeadSource(t, shaB), scope, false, false, false)
	})
	if err == nil {
		t.Fatalf("install with the metadata gone = nil, want a refusal. Output:\n%s", out)
	}
	assertTier1Release(t, scope, shaA, "0.4.1", true)
}
