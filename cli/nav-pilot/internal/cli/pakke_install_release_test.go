package cli

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// A new pin starts on the newest stable release (#779): install and the first
// launch run discovery, so a new install is not stranded on default-branch
// HEAD, ahead of every release.

func assertNoPin(t *testing.T, scope *InstallScope) {
	t.Helper()
	if state, _ := readScopedState(scope); state != nil {
		t.Errorf("state = %+v, want nothing pinned", state)
	}
}

func assertNotMaterialized(t *testing.T, sha string) {
	t.Helper()
	if _, err := os.Stat(pakkeRevisionDir("navikt/grillmester", sha)); !os.IsNotExist(err) {
		t.Errorf("revision %s was materialized (stat err %v)", shortSHA(sha), err)
	}
}

// TestInstallStartsOnTheRelease: an install of a release-backed source pins
// the release's exact SHA and follows releases; a dry run says so and writes
// nothing.
func TestInstallStartsOnTheRelease(t *testing.T) {
	scope := pinEnv(t)
	refs := releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	out := captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, shaB), true, false) })
	if err != nil {
		t.Fatalf("install --dry-run = %v. Output:\n%s", err, out)
	}
	if !strings.Contains(out, "0.4.1 ("+shortSHA(shaA)+")") {
		t.Errorf("dry run did not name the release. Output:\n%s", out)
	}
	assertNoPin(t, scope)

	out = captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, shaB), false, true) })
	if err != nil {
		t.Fatalf("install --json = %v. Output:\n%s", err, out)
	}
	var doc map[string]any
	if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", jerr, out)
	}
	if doc["source_sha"] != shaA || doc["pakke_version"] != "0.4.1" || doc["follows_releases"] != true {
		t.Errorf("JSON = %v, want the release %s at 0.4.1, following", doc, shaA)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
	assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", shaA))
	if !slices.Contains(*refs, shaA) {
		t.Errorf("the release SHA was never fetched; refs = %v", *refs)
	}
	assertNotMaterialized(t, shaB)
}

// TestInstallWithoutReleasesPinsTheSource: no metadata is today's install.
func TestInstallWithoutReleasesPinsTheSource(t *testing.T) {
	scope := pinEnv(t) // discovery: no metadata
	refs := releaseSyncSource(t, shaA)

	installPin(t, scope, tier2PinSource(t, shaB))
	assertPin(t, scope, shaB, "", false)
	if len(*refs) != 0 {
		t.Errorf("an install without release metadata fetched %v", *refs)
	}
}

// TestInstallRefusesOnAFailedLookup: nothing to keep and nothing confirmed, so
// nothing is pinned, and never HEAD.
func TestInstallRefusesOnAFailedLookup(t *testing.T) {
	scope := pinEnv(t)
	releaseSyncSource(t, shaA)
	stubRelease(t, 0, pakkeRelease{}, errors.New("GitHub API returned 403"))

	var err error
	out := captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, shaB), false, false) })
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "Nothing was pinned") {
		t.Fatalf("install over a failed lookup = %v, want a refusal naming the cause. Output:\n%s", err, out)
	}
	assertNoPin(t, scope)
	assertNotMaterialized(t, shaB)
}

// TestInstallRefusesAReleaseThatIsNotPayloadOnly: the release is another
// revision than the one the install routed on, and may have grown a layout.
func TestInstallRefusesAReleaseThatIsNotPayloadOnly(t *testing.T) {
	scope := pinEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	tree := tier2PinSourceTree(t)
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		src := &Source{Dir: tree, SHA: ref, Version: "dev", Repo: sourceRepo}
		if err := attachPakke(src); err != nil {
			return nil, err
		}
		src.Pakke.Layout = &agentpakke.Layout{Agents: "agents"}
		return src, nil
	}

	var err error
	captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, shaB), false, false) })
	if err == nil || !strings.Contains(err.Error(), "pre-built payloads only") {
		t.Fatalf("install of a mixed release = %v, want a refusal", err)
	}
	assertNoPin(t, scope)
}

// TestInstallSkipsTheLookup: --ref, --frozen, a path source and repo scope
// never look releases up.
func TestInstallSkipsTheLookup(t *testing.T) {
	for name, tc := range map[string]struct {
		setup   func(t *testing.T, src *Source) *InstallScope // nil scope: user
		refused bool
	}{
		"explicit ref": {setup: func(t *testing.T, _ *Source) *InstallScope {
			installRef = shaB
			t.Cleanup(func() { installRef = "" })
			return nil
		}},
		"frozen": {setup: func(t *testing.T, _ *Source) *InstallScope {
			installFrozen = true
			t.Cleanup(func() { installFrozen = false })
			return nil
		}},
		"path source": {refused: true, setup: func(_ *testing.T, src *Source) *InstallScope {
			src.Repo = src.Dir
			return nil
		}},
		"repo scope": {refused: true, setup: func(t *testing.T, _ *Source) *InstallScope {
			return ScopeRepo(t.TempDir())
		}},
	} {
		t.Run(name, func(t *testing.T) {
			scope := pinEnv(t)
			calls := stubRelease(t, releaseCandidate, release041, nil)
			src := tier2PinSource(t, shaB)
			if s := tc.setup(t, src); s != nil {
				scope = s
			}

			var err error
			out := captureStdoutFor(t, func() { err = installPakkePin(scope, src, false, false) })
			if *calls != 0 {
				t.Errorf("looked up releases %d time(s)", *calls)
			}
			if tc.refused {
				if err == nil {
					t.Errorf("install = nil, want today's refusal. Output:\n%s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("install = %v. Output:\n%s", err, out)
			}
			assertPin(t, scope, shaB, "", false)
		})
	}
}

// TestFirstLaunchStartsOnTheRelease: the launch that pins an un-installed
// source pins the release, and hands that revision to the client.
func TestFirstLaunchStartsOnTheRelease(t *testing.T) {
	scope := pinEnv(t)
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var rev *Source
	var err error
	out := captureStdoutFor(t, func() { rev, err = autoPin(tier2PinSource(t, shaB), "copilot") })
	if err != nil {
		t.Fatalf("autoPin = %v. Output:\n%s", err, out)
	}
	if rev.SHA != shaA || rev.Dir != pakkeRevisionDir("navikt/grillmester", shaA) {
		t.Errorf("launch revision = %s at %s, want the release %s", rev.SHA, rev.Dir, shaA)
	}
	if !strings.Contains(out, "0.4.1") {
		t.Errorf("the launch did not name the release. Output:\n%s", out)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
	assertNotMaterialized(t, shaB)
}

// TestFirstLaunchRefusesOnAFailedLookup: as install, nothing is pinned.
func TestFirstLaunchRefusesOnAFailedLookup(t *testing.T) {
	scope := pinEnv(t)
	releaseSyncSource(t, shaA)
	stubRelease(t, 0, pakkeRelease{}, errors.New("context deadline exceeded"))

	var err error
	captureStdoutFor(t, func() { _, err = autoPin(tier2PinSource(t, shaB), "copilot") })
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("autoPin over a failed lookup = %v, want a refusal", err)
	}
	assertNoPin(t, scope)
	assertNotMaterialized(t, shaB)
}

// TestLaunchOfAnExistingPinDoesNotLookUp: a recorded pin whose revision is gone
// is not a first launch. Moving it is the startup-prompt slice's decision.
func TestLaunchOfAnExistingPinDoesNotLookUp(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	if err := os.RemoveAll(pakkerRoot()); err != nil {
		t.Fatal(err)
	}
	releaseSyncSource(t, shaA)
	calls := stubRelease(t, releaseCandidate, release041, nil)

	var err error
	out := captureStdoutFor(t, func() { _, err = autoPin(tier2PinSource(t, shaB), "copilot") })
	if err != nil {
		t.Fatalf("autoPin = %v. Output:\n%s", err, out)
	}
	if *calls != 0 {
		t.Errorf("a launch over an existing pin looked up releases %d time(s)", *calls)
	}
	assertPin(t, scope, shaB, "", false)
}

// TestInstallRefFlagSkipsTheLookup: run() hands --ref to the Tier 2 install,
// and the ref does not outlive the invocation.
func TestInstallRefFlagSkipsTheLookup(t *testing.T) {
	isolatedRun(t)
	scope := pinEnv(t)
	calls := stubRelease(t, releaseCandidate, release041, nil)
	stubResolveSource(t, tier2PinSource(t, shaB))

	var err error
	out := captureStdoutFor(t, func() { err = run([]string{"install", "--user", "--ref", shaB, "grillmester"}) })
	if err != nil {
		t.Fatalf("install --ref = %v. Output:\n%s", err, out)
	}
	if *calls != 0 {
		t.Errorf("install --ref looked up releases %d time(s)", *calls)
	}
	assertPin(t, scope, shaB, "", false)
	if installRef != "" {
		t.Errorf("installRef = %q after the invocation", installRef)
	}
}

// ─── status ──────────────────────────────────────────────────────────────────

func listStatus(t *testing.T, scope *InstallScope, jsonOutput bool) string {
	t.Helper()
	var err error
	out := captureStdoutFor(t, func() { err = cmdListInstalledScoped(scope, false, jsonOutput) })
	if err != nil {
		t.Fatalf("list --installed = %v, want nil. Output:\n%s", err, out)
	}
	return out
}

func statusJSON(t *testing.T, out string) *pakkeReleaseStatus {
	t.Helper()
	var doc struct {
		Pakke *pakkeReleaseStatus `json:"pakke"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, out)
	}
	return doc.Pakke
}

// TestStatusShowsVersionAndPendingRelease: version, pinned SHA, subscription
// and a newer release, in text and in both JSON shapes.
func TestStatusShowsVersionAndPendingRelease(t *testing.T) {
	scope, _ := followingPin(t)
	pending := pakkeRelease{Version: "0.4.2", SHA: shaC, Tag: "v0.4.2"}
	stubRelease(t, releaseCandidate, pending, nil)

	out := listStatus(t, scope, false)
	for _, want := range []string{
		"Package:     0.4.1 (pinned at " + shortSHA(shaA) + ")",
		"follows stable releases: yes",
		"Release 0.4.2 (" + shortSHA(shaC) + ") is available",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status does not say %q. Output:\n%s", want, out)
		}
	}
	st := statusJSON(t, listStatus(t, scope, true))
	if st == nil || st.Version != "0.4.1" || st.PinnedSHA != shaA || !st.FollowsReleases || st.PendingRelease == nil || *st.PendingRelease != pending {
		t.Errorf("JSON pakke = %+v, want 0.4.1 at %s, following, 0.4.2 pending", st, shaA)
	}

	var err error
	out = captureStdoutFor(t, func() { err = cmdListInstalledAuto(t.TempDir(), true) })
	var auto struct {
		Scopes []struct {
			Scope string              `json:"scope"`
			Pakke *pakkeReleaseStatus `json:"pakke"`
		} `json:"scopes"`
	}
	if err != nil || json.Unmarshal([]byte(out), &auto) != nil {
		t.Fatalf("list --installed --json (auto) = %v. Output:\n%s", err, out)
	}
	if !slices.ContainsFunc(auto.Scopes, func(s struct {
		Scope string              `json:"scope"`
		Pakke *pakkeReleaseStatus `json:"pakke"`
	}) bool {
		return s.Scope == "user" && s.Pakke != nil && s.Pakke.PendingRelease != nil
	}) {
		t.Errorf("auto JSON has no user pakke with a pending release:\n%s", out)
	}

	stubRelease(t, releaseUpToDate, release041, nil)
	if out := listStatus(t, scope, false); strings.Contains(out, "is available") {
		t.Errorf("an up-to-date pin reported a pending release. Output:\n%s", out)
	}
}

// TestStatusReportsAFailedReleaseCheck: the failure is printed, and the command
// still succeeds.
func TestStatusReportsAFailedReleaseCheck(t *testing.T) {
	scope, _ := followingPin(t)
	stubRelease(t, 0, pakkeRelease{}, errors.New("GitHub API returned 403"))

	if out := listStatus(t, scope, false); !strings.Contains(out, "release check failed: GitHub API returned 403") {
		t.Errorf("status did not report the failed check. Output:\n%s", out)
	}
	st := statusJSON(t, listStatus(t, scope, true))
	if st == nil || st.ReleaseCheckError != "GitHub API returned 403" || st.Version != "0.4.1" {
		t.Errorf("JSON pakke = %+v, want the error and the installed version", st)
	}
}

// TestStatusIgnoresAStaleVersionClaim: a version recorded for another SHA is
// not this pin's version, and the pin does not follow releases.
func TestStatusIgnoresAStaleVersionClaim(t *testing.T) {
	scope := pinEnv(t)
	if err := writeScopedState(scope, &StateFile{
		Collection: "grillmester", Scope: scope.Name, SourceRepo: "navikt/grillmester", SourceSHA: shaB,
		PakkeVersion: "0.4.1", FollowsReleases: true, PakkeVersionSHA: shaA,
	}); err != nil {
		t.Fatal(err)
	}
	stubRelease(t, releaseNotOffered, release041, nil)

	out := listStatus(t, scope, false)
	if strings.Contains(out, "0.4.1") || !strings.Contains(out, "follows stable releases: no") {
		t.Errorf("status trusted a claim recorded for %s. Output:\n%s", shortSHA(shaA), out)
	}
	st := statusJSON(t, listStatus(t, scope, true))
	if st == nil || st.Version != "" || st.FollowsReleases || st.PinnedSHA != shaB || st.PendingRelease != nil {
		t.Errorf("JSON pakke = %+v, want pinned at %s, no version, not following, nothing pending", st, shaB)
	}
}

// TestStatusDoesNotLookUpForAFileInstall: only a pin has releases to report.
func TestStatusDoesNotLookUpForAFileInstall(t *testing.T) {
	scope := pinEnv(t)
	calls := stubRelease(t, releaseCandidate, release041, nil)
	if err := writeScopedState(scope, &StateFile{
		Collection: "fullstack", Scope: scope.Name, SourceRepo: "navikt/copilot", SourceSHA: shaB,
		Files: []InstalledFile{{Path: "agents/a.agent.md", Hash: "abc"}},
	}); err != nil {
		t.Fatal(err)
	}

	out := listStatus(t, scope, false)
	jsonOut := listStatus(t, scope, true)
	if *calls != 0 || strings.Contains(out, "Package:") || strings.Contains(jsonOut, `"pakke"`) {
		t.Errorf("a file install got release status (%d lookups). Output:\n%s\n%s", *calls, out, jsonOut)
	}
}

// TestInstallFallsBackWhenReleasesAreNotVisible: a private repo without
// GITHUB_TOKEN answers 404 for its releases, and still installs from its
// default branch as it did before releases existed, with a warning.
func TestInstallFallsBackWhenReleasesAreNotVisible(t *testing.T) {
	scope := pinEnv(t)
	refs := releaseSyncSource(t, shaA)
	stubRelease(t, 0, pakkeRelease{}, errReleasesNotFound)

	var err error
	stderr := captureStderrFor(t, func() {
		captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, shaB), false, false) })
	})
	if err != nil {
		t.Fatalf("install over a 404 for releases = %v, want the default branch", err)
	}
	if !strings.Contains(stderr, "set GITHUB_TOKEN") {
		t.Errorf("no warning on stderr: %q", stderr)
	}
	assertPin(t, scope, shaB, "", false)
	if len(*refs) != 0 {
		t.Errorf("fetched %v for a repo whose releases are not visible", *refs)
	}
}

// TestFirstLaunchRefusesAReleaseWithoutThisClientsPayload: the tier gate ran on
// the resolved revision. A release that ships no payload for the launched client
// is not pinned, rather than pinned and then refused by the launch.
func TestFirstLaunchRefusesAReleaseWithoutThisClientsPayload(t *testing.T) {
	scope := pinEnv(t)
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	// The fixture declares copilot, opencode and pi.
	captureStdoutFor(t, func() { _, err = autoPin(tier2PinSource(t, shaB), "claude") })
	if err == nil || !strings.Contains(err.Error(), "no pre-built payload for claude") {
		t.Fatalf("autoPin of a release without the client's payload = %v, want a refusal", err)
	}
	assertNoPin(t, scope)
	assertNotMaterialized(t, shaA)
}

// TestInstallRefToTheFollowedRevisionStopsFollowing: install --ref at the SHA
// a following pin is already at is a pinning choice, as sync --ref is, and the
// pin stops following. Without explicit, pinRevision kept the claim, since a nil
// release over the same revision is otherwise a re-materialization.
func TestInstallRefToTheFollowedRevisionStopsFollowing(t *testing.T) {
	scope, _ := followingPin(t)
	calls := stubRelease(t, releaseCandidate, release041, nil)
	installRef = shaA
	t.Cleanup(func() { installRef = "" })

	var err error
	out := captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, shaA), false, false) })
	if err != nil {
		t.Fatalf("install --ref <pinned sha> = %v. Output:\n%s", err, out)
	}
	if *calls != 0 {
		t.Errorf("an explicit --ref looked up releases (%d calls)", *calls)
	}
	assertPin(t, scope, shaA, "", false)
}

// TestReinstallOverAFollowingPinRefusesA404: a re-install whose releases list
// answers 404 does not move a following pin to the default branch; sync fails
// closed in the same situation.
func TestReinstallOverAFollowingPinRefusesA404(t *testing.T) {
	scope, _ := followingPin(t)
	stubRelease(t, 0, pakkeRelease{}, errReleasesNotFound)

	var err error
	captureStderrFor(t, func() {
		captureStdoutFor(t, func() { err = installPakkePin(scope, tier2PinSource(t, shaB), false, false) })
	})
	if err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") || !strings.Contains(err.Error(), "--ref") {
		t.Fatalf("re-install over a following pin with a 404 = %v, want a refusal naming GITHUB_TOKEN and --ref", err)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
	assertNotMaterialized(t, shaB)
}
