package cli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
		return e.answer, nil
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
