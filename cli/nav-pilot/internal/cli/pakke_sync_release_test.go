package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

// stubRelease makes release discovery answer without GitHub, and counts the
// lookups.
func stubRelease(t *testing.T, outcome releaseOutcome, rel pakkeRelease, err error) *int {
	t.Helper()
	calls := new(int)
	orig := discoverPakkeRelease
	t.Cleanup(func() { discoverPakkeRelease = orig })
	discoverPakkeRelease = func(context.Context, string, string, string) (releaseOutcome, pakkeRelease, error) {
		*calls++
		return outcome, rel, err
	}
	return calls
}

// releaseSyncSource makes sync resolve the Tier 2 fixture at the ref it asks
// for: headSHA for the default branch, exactly the ref otherwise. It records
// the refs.
func releaseSyncSource(t *testing.T, headSHA string) *[]string {
	t.Helper()
	tree := tier2PinSourceTree(t)
	refs := &[]string{}
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		*refs = append(*refs, ref)
		sha := headSHA
		if ref != "" {
			sha = ref
		}
		src := &Source{Dir: tree, SHA: sha, Version: "dev", Repo: sourceRepo}
		return src, attachPakke(src)
	}
	return refs
}

var release041 = pakkeRelease{Version: "0.4.1", SHA: shaA, Tag: "v0.4.1"}

// followingPin installs at shaC and syncs onto release041, so the pin follows
// releases. The default branch is at shaB throughout.
func followingPin(t *testing.T) (*InstallScope, *[]string) {
	t.Helper()
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	refs := releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("setup: sync --apply onto a release = %v. Output:\n%s", err, out)
	}
	return scope, refs
}

func assertPin(t *testing.T, scope *InstallScope, sha, version string, follows bool) {
	t.Helper()
	state, _ := readScopedState(scope)
	if state == nil || state.SourceSHA != sha || state.PakkeVersion != version || state.FollowsReleases != follows {
		t.Fatalf("state = %+v, want pin %s version %q follows=%v", state, sha, version, follows)
	}
}

// TestReleaseSyncPinsTheReleaseNotTheDefaultBranch: a release-backed source
// moves to the release's exact SHA, never to the branch HEAD it also resolves.
func TestReleaseSyncPinsTheReleaseNotTheDefaultBranch(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	refs := releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != errUpdatesAvailable {
		t.Fatalf("sync = %v, want errUpdatesAvailable. Output:\n%s", err, out)
	}
	if !strings.Contains(out, "0.4.1") {
		t.Errorf("sync did not name the release version. Output:\n%s", out)
	}
	assertPin(t, scope, shaC, "", false)

	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
	assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", shaA))
	if !slices.Contains(*refs, shaA) {
		t.Errorf("the release SHA was never fetched; refs = %v", *refs)
	}
	if _, err := os.Stat(pakkeRevisionDir("navikt/grillmester", shaB)); !os.IsNotExist(err) {
		t.Errorf("the default branch revision was materialized (stat err %v)", err)
	}
}

// TestReleaseSyncJSONCarriesVersion: --json names the version when it is known.
func TestReleaseSyncJSONCarriesVersion(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, true) })
	if err != nil {
		t.Fatalf("sync --apply --json = %v. Output:\n%s", err, out)
	}
	var res syncResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", jerr, out)
	}
	if res.Version != "0.4.1" || res.Source != shaA || !res.UpToDate {
		t.Errorf("JSON = %+v, want up to date at %s version 0.4.1", res, shaA)
	}
}

// TestFollowedPinNeverFallsBackToTheDefaultBranch: once a pin follows
// releases, no lookup failure may move it to the branch.
func TestFollowedPinNeverFallsBackToTheDefaultBranch(t *testing.T) {
	for name, stub := range map[string]struct {
		outcome releaseOutcome
		err     error
	}{
		"no metadata any more": {releaseNoMetadata, nil},
		"lookup fails":         {0, errors.New("GitHub API returned 403")},
	} {
		t.Run(name, func(t *testing.T) {
			scope, _ := followingPin(t)
			stubRelease(t, stub.outcome, pakkeRelease{}, stub.err)

			var err error
			out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
			if err == nil || err == errUpdatesAvailable {
				t.Fatalf("sync --apply = %v, want a refusal. Output:\n%s", err, out)
			}
			assertPin(t, scope, shaA, "0.4.1", true)
			if _, statErr := os.Stat(pakkeRevisionDir("navikt/grillmester", shaB)); !os.IsNotExist(statErr) {
				t.Errorf("the default branch revision was materialized (stat err %v)", statErr)
			}
		})
	}
}

// TestUnfollowedPinWithoutReleasesKeepsTodaysBehavior: a source with no
// release metadata syncs from the default branch, exactly as before.
func TestUnfollowedPinWithoutReleasesKeepsTodaysBehavior(t *testing.T) {
	scope := pinEnv(t) // discovery: no metadata
	installPin(t, scope, tier2PinSource(t, shaC))
	releaseSyncSource(t, shaB)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
	}
	assertPin(t, scope, shaB, "", false)
}

// TestUnfollowedPinRefusesOnAFailedLookup: a failed lookup cannot say whether
// the source is release-backed, so it is not read as "no metadata".
func TestUnfollowedPinRefusesOnAFailedLookup(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	releaseSyncSource(t, shaB)
	stubRelease(t, 0, pakkeRelease{}, errors.New("context deadline exceeded"))

	var err error
	captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err == nil || !strings.Contains(err.Error(), "pin is unchanged") {
		t.Fatalf("sync --apply = %v, want a refusal that leaves the pin", err)
	}
	assertPin(t, scope, shaC, "", false)
}

// TestReleaseSyncDoesNotOfferAnOlderRelease: a pin ahead of or diverged from
// the newest stable release stays where it is.
func TestReleaseSyncDoesNotOfferAnOlderRelease(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseNotOffered, release041, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
	}
	if !strings.Contains(out, "not offered") {
		t.Errorf("sync did not say the release is not offered. Output:\n%s", out)
	}
	assertPin(t, scope, shaC, "", false)
	if _, statErr := os.Stat(pakkeRevisionDir("navikt/grillmester", shaA)); !os.IsNotExist(statErr) {
		t.Errorf("the older release was materialized (stat err %v)", statErr)
	}
}

// TestReleaseSyncUpToDate: nothing to fetch when the pin is the release.
func TestReleaseSyncUpToDate(t *testing.T) {
	scope, refs := followingPin(t)
	stubRelease(t, releaseUpToDate, release041, nil)
	before := len(*refs)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
	}
	if !strings.Contains(out, "up to date (0.4.1") {
		t.Errorf("sync did not report up to date with the version. Output:\n%s", out)
	}
	if len(*refs) != before+1 { // the HEAD resolve syncScope always does
		t.Errorf("sync fetched a revision for an up-to-date pin; refs = %v", *refs)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
}

// TestExplicitRefWinsOverReleases: --ref is a pinning choice. It skips the
// lookup, and the pin it writes stops following.
func TestExplicitRefWinsOverReleases(t *testing.T) {
	scope, _ := followingPin(t)
	calls := stubRelease(t, releaseCandidate, pakkeRelease{Version: "9.9.9", SHA: shaC, Tag: "v9.9.9"}, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, shaB, "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply --ref = %v. Output:\n%s", err, out)
	}
	if *calls != 0 {
		t.Errorf("an explicit --ref still looked up releases (%d calls)", *calls)
	}
	assertPin(t, scope, shaB, "", false)
}

// TestReleaseSyncRefusesARevisionThatIsNotTheRelease: what the fetch resolved
// must be the release's SHA.
func TestReleaseSyncRefusesARevisionThatIsNotTheRelease(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	pinnedSyncSource(t, tier2PinSourceTree(t), shaB) // ignores the ref
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err == nil || err == errUpdatesAvailable {
		t.Fatalf("sync --apply = %v, want a refusal", err)
	}
	assertPin(t, scope, shaC, "", false)
}

// TestSyncFlagFollowsReleases: `nav-pilot --sync` runs cmdSyncAuto with
// --apply, and reaches the same release path.
func TestSyncFlagFollowsReleases(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSyncAuto(t.TempDir(), "", "", true, false) })
	if err != nil {
		t.Fatalf("cmdSyncAuto = %v. Output:\n%s", err, out)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
}

// TestPinRevisionRefusesALostUpdate: a pin that moved while this one was being
// materialized is not overwritten.
func TestPinRevisionRefusesALostUpdate(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))

	pinWriteHook = func() {
		if err := writeScopedState(scope, &StateFile{Collection: "grillmester", Scope: scope.Name, SourceRepo: "navikt/grillmester", SourceSHA: shaB}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { pinWriteHook = nil })

	_, err := pinRevision(scope, tier2PinSource(t, shaA), &release041, true)
	if err == nil || !strings.Contains(err.Error(), "pin changed") {
		t.Fatalf("pinRevision over a moved pin = %v, want a refusal", err)
	}
	assertPin(t, scope, shaB, "", false)
}

// TestOldStateDoesNotFollowReleases: a state written before #779 has neither
// key, and reads as not following; a pin that does not follow writes neither.
func TestOldStateDoesNotFollowReleases(t *testing.T) {
	var state StateFile
	if err := json.Unmarshal([]byte(`{"collection":"grillmester","version":"dev","source_repo":"navikt/grillmester","source_sha":"`+shaC+`","files":[]}`), &state); err != nil {
		t.Fatal(err)
	}
	if state.FollowsReleases || state.PakkeVersion != "" {
		t.Errorf("old state = %+v, want not following", state)
	}
	out, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "follows_releases") || strings.Contains(string(out), "pakke_version") {
		t.Errorf("a pin that does not follow wrote the new keys: %s", out)
	}
}
