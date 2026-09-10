package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/huh"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// The startup prompt (#779): a launch in a terminal offers a newer stable
// release of the pinned agentpakke, from a cache, and moves the pin only on
// "Yes".

var release042 = pakkeRelease{Version: "0.4.2", SHA: strings.Repeat("d", 40), Tag: "v0.4.2"}

// promptEnv pins grillmester at shaC in a terminal, with the default branch at
// shaB. It records what the client was launched from and every question asked,
// answering each with answer.
type promptEnv struct {
	scope    *InstallScope
	launched string
	asked    []string
	answer   bool
	refs     *[]string
	// confirmErr is what the prompt returns with the answer (Ctrl-C).
	confirmErr error
	// onAsk runs while the question is open.
	onAsk func()
}

func newPromptEnv(t *testing.T) *promptEnv {
	t.Helper()
	e := &promptEnv{scope: pinEnv(t)}
	installPin(t, e.scope, tier2PinSource(t, shaC))
	e.refs = releaseSyncSource(t, shaB)
	forceInteractive(t)

	origLaunchers := stagedLaunchers
	t.Cleanup(func() { stagedLaunchers = origLaunchers })
	stagedLaunchers = map[string]func(ResolvedConfig, providerpkg.StagedLaunch) error{
		"copilot": func(_ ResolvedConfig, l providerpkg.StagedLaunch) error { e.launched = l.Dir; return nil },
	}
	origConfirm := confirmPakkeRelease
	t.Cleanup(func() { confirmPakkeRelease = origConfirm })
	confirmPakkeRelease = func(title string) (bool, error) {
		e.asked = append(e.asked, title)
		if e.onAsk != nil {
			e.onAsk()
		}
		return e.answer, e.confirmErr
	}
	return e
}

// launch runs the launch and returns its stderr.
func (e *promptEnv) launch(t *testing.T) string {
	t.Helper()
	e.launched = ""
	var err error
	stderr := captureStderrFor(t, func() {
		captureStdoutFor(t, func() {
			_, err = tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"})
		})
	})
	if err != nil {
		t.Fatalf("launch = %v. Stderr:\n%s", err, stderr)
	}
	return stderr
}

func (e *promptEnv) assertLaunchedFrom(t *testing.T, sha string) {
	t.Helper()
	if want := filepath.Join(pakkeRevisionDir("navikt/grillmester", sha), "copilot", "full"); e.launched != want {
		t.Errorf("launched from %q, want %q", e.launched, want)
	}
}

// ageReleaseCache moves every entry's check back by d.
func ageReleaseCache(t *testing.T, d time.Duration) {
	t.Helper()
	cache := readPakkeReleaseCache()
	if len(cache) == 0 {
		t.Fatal("no release cache to age")
	}
	for k, e := range cache {
		e.CheckedAt = e.CheckedAt.Add(-d)
		cache[k] = e
	}
	writePakkeReleaseCache(cache)
}

// TestReleasePromptOnlyInATerminal: no terminal, no lookup, no prompt, no write.
func TestReleasePromptOnlyInATerminal(t *testing.T) {
	e := newPromptEnv(t)
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })
	calls := stubRelease(t, releaseCandidate, release041, nil)

	e.launch(t)
	if *calls != 0 || len(e.asked) != 0 {
		t.Errorf("a launch without a terminal looked up %d time(s) and asked %v", *calls, e.asked)
	}
	if _, err := os.Stat(pakkeReleaseCachePath()); !os.IsNotExist(err) {
		t.Errorf("a launch without a terminal wrote the release cache (stat err %v)", err)
	}
	assertPin(t, e.scope, shaC, "", false)
	e.assertLaunchedFrom(t, shaC)
}

// TestReleasePromptCacheFreshness: an answer is trusted for 24 hours, a failure
// for one, and a failure prints one line.
func TestReleasePromptCacheFreshness(t *testing.T) {
	e := newPromptEnv(t)
	calls := stubRelease(t, releaseUpToDate, pakkeRelease{}, nil)

	e.launch(t)
	e.launch(t)
	if *calls != 1 {
		t.Errorf("two launches looked up %d time(s), want 1", *calls)
	}
	ageReleaseCache(t, 2*time.Hour)
	e.launch(t)
	if *calls != 1 {
		t.Errorf("a 2-hour-old answer was looked up again (%d lookups)", *calls)
	}
	ageReleaseCache(t, 23*time.Hour)
	e.launch(t)
	if *calls != 2 {
		t.Errorf("a 25-hour-old answer was trusted (%d lookups)", *calls)
	}

	calls = stubRelease(t, 0, pakkeRelease{}, errors.New("GitHub API returned 403\nsecond line"))
	ageReleaseCache(t, 25*time.Hour)
	stderr := e.launch(t)
	if strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, "release check for grillmester failed: GitHub API returned 403") {
		t.Errorf("a failed lookup printed %q, want one short line", stderr)
	}
	e.assertLaunchedFrom(t, shaC)
	ageReleaseCache(t, 30*time.Minute)
	if e.launch(t); *calls != 1 {
		t.Errorf("a failure 30 minutes old was retried (%d lookups)", *calls)
	}
	ageReleaseCache(t, time.Hour)
	if e.launch(t); *calls != 2 {
		t.Errorf("a failure 90 minutes old was not retried (%d lookups)", *calls)
	}
	if len(e.asked) != 0 {
		t.Errorf("asked %v without a candidate", e.asked)
	}
}

// TestReleasePromptRemembersNo: "No" is remembered for that version, across
// lookups, and a newer version asks again.
func TestReleasePromptRemembersNo(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)

	e.launch(t)
	if want := "grillmester 0.4.1 is available (you have " + shortSHA(shaC) + "). Update now?"; !slices.Equal(e.asked, []string{want}) {
		t.Fatalf("asked %q, want %q", e.asked, want)
	}
	assertPin(t, e.scope, shaC, "", false)
	e.assertLaunchedFrom(t, shaC)

	e.launch(t)
	ageReleaseCache(t, 25*time.Hour)
	e.launch(t)
	if len(e.asked) != 1 {
		t.Errorf("asked again about a dismissed version: %q", e.asked)
	}

	stubRelease(t, releaseCandidate, release042, nil)
	ageReleaseCache(t, 25*time.Hour)
	e.launch(t)
	if len(e.asked) != 2 || !strings.Contains(e.asked[1], "0.4.2") {
		t.Errorf("a newer version was not offered: %q", e.asked)
	}
}

// TestReleasePromptDropsACandidateForAMovedPin: a cached candidate is about the
// pin it was looked up for. Once sync has moved the pin, it is not offered.
func TestReleasePromptDropsACandidateForAMovedPin(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	e.answer = false
	e.launch(t) // caches 0.4.1 as a candidate over shaC
	cache := readPakkeReleaseCache()
	for k, entry := range cache {
		entry.Dismissed = ""
		cache[k] = entry
	}
	writePakkeReleaseCache(cache)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(e.scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Output:\n%s", err, out)
	}
	assertPin(t, e.scope, shaA, "0.4.1", true)

	calls := stubRelease(t, releaseUpToDate, release041, nil)
	e.asked = nil
	e.launch(t)
	if len(e.asked) != 0 {
		t.Errorf("offered a cached candidate the pin is already on: %q", e.asked)
	}
	if *calls != 1 {
		t.Errorf("the moved pin was not looked up again (%d lookups)", *calls)
	}
	e.assertLaunchedFrom(t, shaA)
}

// TestReleasePromptYesPinsTheRelease: "Yes" pins exactly the offered SHA, as a
// release, and launches it. The default branch is never resolved.
func TestReleasePromptYesPinsTheRelease(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	e.answer = true

	e.launch(t)
	assertPin(t, e.scope, shaA, "0.4.1", true)
	assertRevisionVerifies(t, pakkeRevisionDir("navikt/grillmester", shaA))
	e.assertLaunchedFrom(t, shaA)
	if !slices.Equal(*e.refs, []string{shaA}) {
		t.Errorf("resolved refs %q, want only the release SHA", *e.refs)
	}
	assertNotMaterialized(t, shaB)
}

// TestReleasePromptFailedUpdateLaunchesThePin: an update that cannot be pinned
// says why and launches the pin it had.
func TestReleasePromptFailedUpdateLaunchesThePin(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	e.answer = true
	resolveSourceForSync = func(string, string) (*Source, error) { return nil, errors.New("dial tcp: no route to host") }

	stderr := e.launch(t)
	if !strings.Contains(stderr, "Could not update grillmester") || !strings.Contains(stderr, "no route to host") {
		t.Errorf("the failed update was not reported. Stderr:\n%s", stderr)
	}
	assertPin(t, e.scope, shaC, "", false)
	e.assertLaunchedFrom(t, shaC)
}

// TestDeletingTheReleaseCacheKeepsThePin: the cache holds lookups and answers,
// never the pin. Deleting it forgets "No" and nothing else.
func TestDeletingTheReleaseCacheKeepsThePin(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	before, err := os.ReadFile(e.scope.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	e.launch(t)

	if err := os.Remove(pakkeReleaseCachePath()); err != nil {
		t.Fatal(err)
	}
	e.launch(t)
	after, err := os.ReadFile(e.scope.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("deleting the release cache changed the state:\nbefore %s\nafter  %s", before, after)
	}
	if len(e.asked) != 2 {
		t.Errorf("asked %d time(s), want the forgotten answer asked again", len(e.asked))
	}
	e.assertLaunchedFrom(t, shaC)
}

// TestReleasePromptRechecksBeforePinning: the offer can be a day old. A release
// gone by the time of "Yes" is not pinned, and the next launch looks again.
func TestReleasePromptRechecksBeforePinning(t *testing.T) {
	for name, now := range map[string]struct {
		outcome releaseOutcome
		rel     pakkeRelease
	}{
		"deleted":    {releaseNoMetadata, pakkeRelease{}},
		"superseded": {releaseCandidate, release042},
	} {
		t.Run(name, func(t *testing.T) {
			e := newPromptEnv(t)
			stubRelease(t, releaseCandidate, release041, nil)
			e.answer = true
			var calls *int
			e.onAsk = func() { calls = stubRelease(t, now.outcome, now.rel, nil) }

			stderr := e.launch(t)
			if !strings.Contains(stderr, "no longer the stable release on offer") {
				t.Errorf("the changed release was not reported. Stderr:\n%s", stderr)
			}
			assertPin(t, e.scope, shaC, "", false)
			e.assertLaunchedFrom(t, shaC)
			if len(*e.refs) != 0 {
				t.Errorf("fetched %q for a release no longer on offer", *e.refs)
			}
			e.onAsk, e.answer = nil, false // an offer now is answered No, so only lookups count
			if e.launch(t); *calls != 2 {
				t.Errorf("the next launch trusted the cached offer (%d lookups, want the recheck and one more)", *calls)
			}
		})
	}
}

// TestReleasePromptPinsTheRecheckedRelease: the pin records what the recheck
// found, not the day-old offer, when both name the same SHA.
func TestReleasePromptPinsTheRecheckedRelease(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	e.answer = true
	e.onAsk = func() {
		stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.4.9", SHA: shaA, Tag: "v0.4.9"}, nil)
	}

	e.launch(t)
	assertPin(t, e.scope, shaA, "0.4.9", true)
	e.assertLaunchedFrom(t, shaA)
}

// TestReleasePrompt404: a releases list GitHub answers 404 for (a private repo
// without GITHUB_TOKEN) is no metadata for a pin that does not follow
// releases, silently and for a day, as sync and install read it. A following
// pin still gets the line. Through the real discovery, not the stub.
func TestReleasePrompt404(t *testing.T) {
	e := newPromptEnv(t)
	t.Setenv("GITHUB_TOKEN", "")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	origBase, origDiscover := githubAPIBase, discoverPakkeRelease
	t.Cleanup(func() { githubAPIBase, discoverPakkeRelease = origBase, origDiscover })
	githubAPIBase, discoverPakkeRelease = srv.URL, discoverPakkeReleaseHTTP

	if stderr := e.launch(t); stderr != "" || hits != 1 {
		t.Errorf("an unfollowed pin's 404 printed %q after %d request(s), want silence after 1", stderr, hits)
	}
	ageReleaseCache(t, 2*time.Hour)
	if e.launch(t); hits != 1 {
		t.Errorf("a 2-hour-old 404 was retried like a failure (%d requests)", hits)
	}
	e.assertLaunchedFrom(t, shaC)

	state, _ := readScopedState(e.scope)
	state.PakkeVersion, state.FollowsReleases, state.PakkeVersionSHA = "0.4.1", true, state.SourceSHA
	if err := writeScopedState(e.scope, state); err != nil {
		t.Fatal(err)
	}
	ageReleaseCache(t, 25*time.Hour)
	if stderr := e.launch(t); !strings.Contains(stderr, "release check for grillmester failed") {
		t.Errorf("a following pin's 404 was silent. Stderr:\n%s", stderr)
	}
}

// TestReleasePromptCtrlCIsNotNo: an aborted question launches the pin and
// comes back next launch.
func TestReleasePromptCtrlCIsNotNo(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	e.answer, e.confirmErr = true, huh.ErrUserAborted

	e.launch(t)
	assertPin(t, e.scope, shaC, "", false)
	e.assertLaunchedFrom(t, shaC)
	e.launch(t)
	if len(e.asked) != 2 {
		t.Errorf("asked %d time(s), want Ctrl-C not to count as No", len(e.asked))
	}
}

// TestReleasePromptPinChangedWhileAsked: a pin moved while the question was
// open is not overwritten by the answer.
func TestReleasePromptPinChangedWhileAsked(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	e.answer = true
	e.onAsk = func() {
		state, _ := readScopedState(e.scope)
		state.SourceSHA = shaB
		if err := writeScopedState(e.scope, state); err != nil {
			t.Fatal(err)
		}
	}

	stderr := e.launch(t)
	if !strings.Contains(stderr, "the pin changed to") {
		t.Errorf("the moved pin was not reported. Stderr:\n%s", stderr)
	}
	assertPin(t, e.scope, shaB, "", false)
	e.assertLaunchedFrom(t, shaC)
}

// TestReleasePromptReleaseWithoutThisClient: a release that no longer ships a
// payload for the client being launched is not pinned.
func TestReleasePromptReleaseWithoutThisClient(t *testing.T) {
	e := newPromptEnv(t)
	stubRelease(t, releaseCandidate, release041, nil)
	e.answer = true
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, repo string) (*Source, error) {
		src, err := orig(ref, repo)
		if err == nil {
			delete(src.Pakke.Clients, "copilot")
		}
		return src, err
	}

	stderr := e.launch(t)
	if !strings.Contains(stderr, `declares no "full" payload for copilot`) {
		t.Errorf("the missing payload was not reported. Stderr:\n%s", stderr)
	}
	assertPin(t, e.scope, shaC, "", false)
	e.assertLaunchedFrom(t, shaC)
}

// TestReleasePromptCorruptCache: a cache that does not parse is no cache.
func TestReleasePromptCorruptCache(t *testing.T) {
	e := newPromptEnv(t)
	calls := stubRelease(t, releaseCandidate, release041, nil)
	mustWrite(t, pakkeReleaseCachePath(), "{not json")

	e.launch(t)
	if *calls != 1 || len(e.asked) != 1 {
		t.Errorf("over a corrupt cache: %d lookup(s), asked %q; want one of each", *calls, e.asked)
	}
	if len(readPakkeReleaseCache()) != 1 {
		t.Error("the corrupt cache was not replaced")
	}
}

// TestReleaseCacheWriteReplacesTheFile: the cache is written to a temporary
// file and renamed over the old one, so a reader never sees half a file.
func TestReleaseCacheWriteReplacesTheFile(t *testing.T) {
	isolatedConfig(t)
	writePakkeReleaseCache(map[string]pakkeReleaseCacheEntry{"a": {PinnedSHA: shaA}})
	before, err := os.Stat(pakkeReleaseCachePath())
	if err != nil {
		t.Fatal(err)
	}
	writePakkeReleaseCache(map[string]pakkeReleaseCacheEntry{"b": {PinnedSHA: shaB}})
	after, err := os.Stat(pakkeReleaseCachePath())
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("the cache was rewritten in place, not replaced")
	}
	entries, _ := os.ReadDir(filepath.Dir(pakkeReleaseCachePath()))
	for _, de := range entries {
		if strings.HasPrefix(de.Name(), ".pakke-releases-") {
			t.Errorf("temporary file %s left behind", de.Name())
		}
	}
	if c := readPakkeReleaseCache(); len(c) != 1 || c["b"].PinnedSHA != shaB {
		t.Errorf("cache = %+v, want the second write", c)
	}
}
