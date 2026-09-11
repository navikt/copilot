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
		"releases list 404":    {0, errReleasesNotFound},
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

// TestUnfollowedPinSkipsOnAFailedLookup: a failed lookup cannot say whether the
// source is release-backed. Sync warns and changes nothing: no release, and
// never the default branch on a guess.
func TestUnfollowedPinSkipsOnAFailedLookup(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	releaseSyncSource(t, shaB)
	stubRelease(t, 0, pakkeRelease{}, errors.New("context deadline exceeded"))

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v, want the update skipped with a warning. Output:\n%s", err, out)
	}
	if !strings.Contains(out, "context deadline exceeded") {
		t.Errorf("sync did not warn. Output:\n%s", out)
	}
	assertPin(t, scope, shaC, "", false)
	if _, statErr := os.Stat(pakkeRevisionDir("navikt/grillmester", shaB)); !os.IsNotExist(statErr) {
		t.Errorf("the default branch revision was materialized (stat err %v)", statErr)
	}

	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, true) })
	var res syncResult
	if err != nil || json.Unmarshal([]byte(out), &res) != nil {
		t.Fatalf("sync --apply --json = %v. Output:\n%s", err, out)
	}
	if res.Warning == "" || !res.Skipped || res.Source != shaC {
		t.Errorf("JSON = %+v, want skipped, a warning and the pin at %s", res, shaC)
	}
}

// TestUnfollowedPrivateRepoSyncsAsBefore: a private repo without GITHUB_TOKEN
// answers 404 for its releases while git still clones it. A pin that does not
// follow releases syncs from the default branch, as it did before releases.
func TestUnfollowedPrivateRepoSyncsAsBefore(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	releaseSyncSource(t, shaB)
	stubRelease(t, 0, pakkeRelease{}, errReleasesNotFound)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
	}
	assertPin(t, scope, shaB, "", false)
	// A private repo that does publish releases must hear why they were not used.
	if !strings.Contains(out, "GITHUB_TOKEN") {
		t.Errorf("sync gave no hint that releases were not visible. Output:\n%s", out)
	}

	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, true) })
	var res syncResult
	if err != nil || json.Unmarshal([]byte(out), &res) != nil {
		t.Fatalf("sync --apply --json = %v. Output:\n%s", err, out)
	}
	if !strings.Contains(res.Warning, "GITHUB_TOKEN") || res.Skipped {
		t.Errorf("JSON = %+v, want a GITHUB_TOKEN warning and not skipped: the sync ran", res)
	}
}

// TestMissingRevisionIsRestoredAtThePin: with nothing to move to, a pin whose
// revision directory is gone is restored at its own SHA by --apply, not
// reported up to date over an empty directory.
func TestMissingRevisionIsRestoredAtThePin(t *testing.T) {
	for name, stub := range map[string]struct {
		outcome releaseOutcome
		err     error
	}{
		"not offered":  {releaseNotOffered, nil},
		"lookup fails": {0, errors.New("GitHub API returned 403")},
	} {
		t.Run(name, func(t *testing.T) {
			scope := pinEnv(t)
			installPin(t, scope, tier2PinSource(t, shaC))
			if err := os.RemoveAll(pakkerRoot()); err != nil {
				t.Fatal(err)
			}
			releaseSyncSource(t, shaB)
			stubRelease(t, stub.outcome, release041, stub.err)

			var err error
			out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
			if err != nil {
				t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
			}
			assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", shaC))
			assertPin(t, scope, shaC, "", false)
			if _, statErr := os.Stat(pakkeRevisionDir("navikt/grillmester", shaB)); !os.IsNotExist(statErr) {
				t.Errorf("the default branch revision was materialized (stat err %v)", statErr)
			}
		})
	}
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

// TestExplicitRefToThePinnedRevisionStopsFollowing: `sync --apply --ref` at
// the SHA a following pin is already at is still a pinning choice. It kept the
// subscription, both where it returned "up to date" and where it restored a
// missing revision.
func TestExplicitRefToThePinnedRevisionStopsFollowing(t *testing.T) {
	for name, wipe := range map[string]bool{"revision on disk": false, "revision missing": true} {
		t.Run(name, func(t *testing.T) {
			scope, _ := followingPin(t)
			if wipe {
				if err := os.RemoveAll(pakkerRoot()); err != nil {
					t.Fatal(err)
				}
			}
			calls := stubRelease(t, releaseCandidate, release041, nil)

			// A plain sync names the command that applies it, --ref included,
			// and says the pin stops following.
			var err error
			out := captureStdoutFor(t, func() { err = cmdSync(scope, shaA, "", false, false) })
			if err != errUpdatesAvailable {
				t.Fatalf("sync --ref <pinned sha> = %v, want errUpdatesAvailable. Output:\n%s", err, out)
			}
			if !strings.Contains(out, "nav-pilot sync --apply --ref "+shaA) || !strings.Contains(out, "stops following") {
				t.Errorf("sync --ref did not advise --apply --ref and say the pin stops following. Output:\n%s", out)
			}
			assertPin(t, scope, shaA, "0.4.1", true)

			out = captureStdoutFor(t, func() { err = cmdSync(scope, shaA, "", true, false) })
			if err != nil {
				t.Fatalf("sync --apply --ref <pinned sha> = %v. Output:\n%s", err, out)
			}
			if *calls != 0 {
				t.Errorf("an explicit --ref looked up releases (%d calls)", *calls)
			}
			assertPin(t, scope, shaA, "", false)
			assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", shaA))
		})
	}
}

// TestUpToDateRestoreDoesNotStartFollowing: a pin that does not follow, at the
// newest release's SHA with its revision gone, is restored as it was. It used
// to adopt the release and start following.
func TestUpToDateRestoreDoesNotStartFollowing(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	if err := os.RemoveAll(pakkerRoot()); err != nil {
		t.Fatal(err)
	}
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseUpToDate, pakkeRelease{Version: "0.4.1", SHA: shaC, Tag: "v0.4.1"}, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
	}
	assertPin(t, scope, shaC, "", false)
	assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", shaC))
}

// TestRestoreJSONCarriesTheClaimedVersion: restoring a following pin's missing
// revision knows its version from the state, and --json says it.
func TestRestoreJSONCarriesTheClaimedVersion(t *testing.T) {
	scope, _ := followingPin(t)
	if err := os.RemoveAll(pakkerRoot()); err != nil {
		t.Fatal(err)
	}
	stubRelease(t, releaseNotOffered, pakkeRelease{Version: "0.4.0", SHA: shaC, Tag: "v0.4.0"}, nil)

	for _, apply := range []bool{false, true} {
		var err error
		out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", apply, true) })
		var res syncResult
		if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
			t.Fatalf("apply=%v: output is not one JSON document (err %v): %v\n%s", apply, err, jerr, out)
		}
		if res.Version != "0.4.1" || res.Source != shaA {
			t.Errorf("apply=%v: JSON = %+v, want source %s version 0.4.1", apply, res, shaA)
		}
	}
	assertPin(t, scope, shaA, "0.4.1", true)
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

	_, err := pinRevision(scope, tier2PinSource(t, shaA), &release041, false, true)
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

// TestStaleReleaseClaimIsIgnored: an older nav-pilot that re-pins keeps
// pakke_version and follows_releases as unknown keys while moving the pin. A
// claim recorded for another SHA says nothing about this one.
func TestStaleReleaseClaimIsIgnored(t *testing.T) {
	scope := pinEnv(t)
	releaseSyncSource(t, shaB)
	// What an older binary leaves after `sync --apply` to HEAD over a pin at 0.4.1.
	if err := writeScopedState(scope, &StateFile{
		Collection: "grillmester", Scope: scope.Name, SourceRepo: "navikt/grillmester", SourceSHA: shaB,
		PakkeVersion: "0.4.1", FollowsReleases: true, PakkeVersionSHA: shaA,
	}); err != nil {
		t.Fatal(err)
	}
	// The revision is on disk, so neither step below is about restoring it.
	if err := os.MkdirAll(pakkeRevisionDir("navikt/grillmester", shaB), 0o755); err != nil {
		t.Fatal(err)
	}

	stubRelease(t, releaseNotOffered, release041, nil)
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, true) })
	var res syncResult
	if err != nil || json.Unmarshal([]byte(out), &res) != nil {
		t.Fatalf("sync --json = %v. Output:\n%s", err, out)
	}
	if res.Version != "" {
		t.Errorf("JSON version = %q for a pin at %s, but 0.4.1 was recorded for %s", res.Version, shaB, shaA)
	}

	stubRelease(t, releaseNoMetadata, pakkeRelease{}, nil)
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil && err != errUpdatesAvailable {
		t.Errorf("sync over a stale claim = %v; it treated a HEAD pin as following releases. Output:\n%s", err, out)
	}
}

// TestLaunchDoesNotUnsubscribeAFollowingPin: a launch resolves the default
// branch. Re-pinning a following pin there, its revision gone, replaced the
// release with HEAD and dropped the subscription without a word.
func TestLaunchDoesNotUnsubscribeAFollowingPin(t *testing.T) {
	scope, _ := followingPin(t)
	if err := os.RemoveAll(pakkerRoot()); err != nil {
		t.Fatal(err)
	}
	head := tier2PinSource(t, shaB)

	var err error
	out := captureStdoutFor(t, func() { _, err = autoPin(head, "copilot") })
	if err == nil || !strings.Contains(err.Error(), "sync --user --apply") {
		t.Fatalf("autoPin over a following pin = %v, want a refusal naming sync --apply. Output:\n%s", err, out)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
}
