package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// A Tier 2 agentpakke: payload-bearing client entries, no layout. Shaped like
// grillmester's manifest at the pinned reference SHA.
const tier2LaunchManifestJSON = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Grillmester agentpakke",
  "clients": {
    "copilot": {
      "defaultModel": "inherit",
      "defaultContext": "full",
      "payloads": {
        "full": {"path": "plugin", "primaryAgents": ["grillmester", "barista"]},
        "focused": {"path": "targets/copilot-cli-focused-v1", "primaryAgents": ["barista", "grill-inspektor"]}
      }
    }
  }
}`

// failingResolveSource installs a source funnel that fails the test if it is
// called at all — the no-source gate must not resolve anything.
func failingResolveSource(t *testing.T) {
	t.Helper()
	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(ref, sourceRepo string) (*Source, error) {
		t.Fatalf("resolveSource must not be called: launch resolved no source (ref=%q, source=%q)", ref, sourceRepo)
		return nil, nil
	}
}

// assertDefaultPakkeActive pins that a launch left the built-in agentpakke in
// place, so personas and model defaults are exactly today's.
func assertDefaultPakkeActive(t *testing.T) {
	t.Helper()
	if got := providerpkg.PrimaryAgent("copilot"); got != "nav-pilot" {
		t.Errorf("active agentpakke changed: PrimaryAgent(copilot) = %q, want nav-pilot", got)
	}
}

// TestTryPakkeLaunchNoRegression is the no-regression table: every population
// that exists today must take the legacy path, with nothing resolved, nothing
// staged, and the built-in agentpakke still active.
func TestTryPakkeLaunchNoRegression(t *testing.T) {
	t.Run("no source at all", func(t *testing.T) {
		isolatedConfig(t)
		t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
		failingResolveSource(t)

		handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot"})
		if handled || err != nil {
			t.Errorf("tryPakkeLaunch(no source) = (%v, %v), want (false, nil)", handled, err)
		}
		assertDefaultPakkeActive(t)
	})

	t.Run("custom source, manifest-less repo", func(t *testing.T) {
		isolatedConfig(t)
		t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
		src := &Source{Dir: legacySourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/other"}
		if err := attachPakke(src); err != nil {
			t.Fatalf("attachPakke: %v", err)
		}
		stubResolveSource(t, src)

		handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/other"})
		if handled || err != nil {
			t.Errorf("tryPakkeLaunch(manifest-less source) = (%v, %v), want (false, nil)", handled, err)
		}
		assertDefaultPakkeActive(t)
	})

	t.Run("custom source, Tier 1 manifest", func(t *testing.T) {
		isolatedConfig(t)
		t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
		stubResolveSource(t, pakkeSource(t, "navikt/grillmester"))

		handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"})
		if handled || err != nil {
			t.Errorf("tryPakkeLaunch(Tier 1 source) = (%v, %v), want (false, nil)", handled, err)
		}
		assertDefaultPakkeActive(t)
	})

	t.Run("Tier 2 for another client stays on the legacy path", func(t *testing.T) {
		isolatedConfig(t)
		t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
		stubResolveSource(t, tier2Source(t))

		// The fixture declares payloads for copilot only.
		handled, err := tryPakkeLaunch(ResolvedConfig{Client: "opencode", Source: "navikt/grillmester"})
		if handled || err != nil {
			t.Errorf("tryPakkeLaunch(opencode, copilot-only payloads) = (%v, %v), want (false, nil)", handled, err)
		}
		assertDefaultPakkeActive(t)
	})
}

// tier2Source builds a resolved, Tier 2 manifest-bearing source.
func tier2Source(t *testing.T) *Source {
	t.Helper()
	src := &Source{Dir: pakkeSourceTree(t, tier2LaunchManifestJSON), SHA: "def5678", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke on the Tier 2 fixture: %v", err)
	}
	if src.Pakke == nil {
		t.Fatal("Tier 2 fixture attached no manifest")
	}
	return src
}

// TestPayloadContextOnLegacyPath: an explicit --payload-context that cannot
// mean anything is an error, never a silent no-op.
func TestPayloadContextOnLegacyPath(t *testing.T) {
	t.Run("no source", func(t *testing.T) {
		isolatedConfig(t)
		failingResolveSource(t)

		_, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", PayloadContext: "focused"})
		if err == nil {
			t.Fatal("--payload-context without an agentpakke must be an error")
		}
		if !strings.Contains(err.Error(), "--payload-context") || !strings.Contains(err.Error(), defaultSourceRepo) {
			t.Errorf("error should name the flag and the source, got: %v", err)
		}
	})

	t.Run("Tier 1 source", func(t *testing.T) {
		isolatedConfig(t)
		stubResolveSource(t, pakkeSource(t, "navikt/grillmester"))

		_, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester", PayloadContext: "focused"})
		if err == nil {
			t.Fatal("--payload-context against a Tier 1 agentpakke must be an error")
		}
		if !strings.Contains(err.Error(), "navikt/grillmester") || !strings.Contains(err.Error(), "copilot") {
			t.Errorf("error should name the source and the client, got: %v", err)
		}
	})
}

// TestPayloadContextUnknown: an unknown context lists what the *pinned*
// manifest declares, sorted — the manifest the revision was built with, not
// whatever the default branch says today.
func TestPayloadContextUnknown(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))
	failingResolveSource(t)

	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester", PayloadContext: "tiny"})
	if !handled || err == nil {
		t.Fatalf("tryPakkeLaunch(unknown context) = (%v, %v), want (true, error)", handled, err)
	}
	if !strings.Contains(err.Error(), "declared contexts: focused, full") {
		t.Errorf("error should list the declared contexts sorted, got: %v", err)
	}
	assertDefaultPakkeActive(t)
}

// TestPayloadContextSelection pins G3's defaulting against a pinned revision:
// with no flag the manifest's defaultContext is launched, and the flag
// overrides it. The context selected is the subdirectory of the revision the
// client is pointed at, so breaking exactly that tree is how the test observes
// which one was chosen.
func TestPayloadContextSelection(t *testing.T) {
	tests := []struct {
		name        string
		flag        string
		wantContext string
	}{
		{name: "defaults to the manifest's defaultContext", wantContext: "full"},
		{name: "the flag overrides it", flag: "focused", wantContext: "focused"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := pinEnv(t)
			src := tier2PinSource(t, "sha-one")
			installPin(t, scope, src)
			failingResolveSource(t)

			// Plant an unmanifested file in the context that must be chosen,
			// so only a launch that reads that tree fails.
			planted := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), "copilot", tt.wantContext, "planted.md")
			if err := os.WriteFile(planted, []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			handled, err := tryPakkeLaunch(ResolvedConfig{
				Client: "copilot", Source: "navikt/grillmester", PayloadContext: tt.flag,
			})
			if !handled || err == nil {
				t.Fatalf("tryPakkeLaunch = (%v, %v), want (true, a verification error)", handled, err)
			}
			if !strings.Contains(err.Error(), `the pinned "`+tt.wantContext+`" payload`) {
				t.Errorf("error should report the %q context, got: %v", tt.wantContext, err)
			}
			// Fail-closed: a Tier 2 launch that could not verify never falls
			// back to the legacy launch.
			assertDefaultPakkeActive(t)
		})
	}
}

// TestUnresolvableSourceFallsBackToLegacy pins that a source that cannot be
// resolved — offline, or a repo name that no longer exists — does not block the
// launch. The tier is unknown at that point, so the launch may well be one that
// worked before Tier 2 staging existed; it degrades to the legacy path with one
// warning, the way EnsureOpenCodeNavContext always has.
func TestUnresolvableSourceFallsBackToLegacy(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })

	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(string, string) (*Source, error) {
		return nil, errors.New("dial tcp: no route to host")
	}

	origStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stderr = w

	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/copilot"})

	w.Close()
	os.Stderr = origStderr
	var stderr strings.Builder
	io.Copy(&stderr, r)

	if handled || err != nil {
		t.Errorf("tryPakkeLaunch(unresolvable source) = (%v, %v), want (false, nil) — the legacy path", handled, err)
	}
	if !strings.Contains(stderr.String(), "navikt/copilot") {
		t.Errorf("warning should name the source, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "no route to host") {
		t.Errorf("warning should name the reason, got: %q", stderr.String())
	}
	assertDefaultPakkeActive(t)
}

// TestUnresolvableSourceRefusesWhenPayloadIsRemembered is the third case beside
// TestUnresolvableSourceFallsBackToLegacy and TestUnusableManifestAbortsLaunch:
// the source cannot be reached, but an earlier launch already learned that it
// declares payloads for this client. "Nothing yet says this is Tier 2" is not
// true then, so warning and taking the legacy path would inject the user's own
// ~/.copilot behind a warning the client TUI wipes off the screen (#504, U8).
func TestUnresolvableSourceRefusesWhenPayloadIsRemembered(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })

	// What rememberTier wrote on the launch that pinned the revision the user
	// has since deleted.
	writeTierCache(t, map[string]tierCacheEntry{
		tierCacheKey("navikt/grillmester", "copilot"): {Tier: agentpakke.TierPayload, LearnedAt: time.Now()},
	})
	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(string, string) (*Source, error) {
		return nil, errors.New("dial tcp: no route to host")
	}

	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"})
	if !handled || err == nil {
		t.Fatalf("tryPakkeLaunch(offline over a remembered payload source) = (%v, %v), want (true, error) — the launch must refuse, not degrade", handled, err)
	}
	for _, want := range []string{
		"navikt/grillmester",
		"no route to host",
		"nav-pilot sync --apply",
		`nav-pilot config set source ""`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not mention %q, got: %v", want, err)
		}
	}
	assertDefaultPakkeActive(t)
}

// TestTier2LaunchIsNotOfferedUnsandboxed pins the flow fix: a Tier 2 launch is
// refused before the "Launch without the cplt sandbox?" question is asked, so
// the user is never talked into a confirmation that is then overruled. If the
// prompt still ran it would fail without a terminal and return nil.
func TestTier2LaunchIsNotOfferedUnsandboxed(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	stubResolveSource(t, tier2Source(t))

	err := launchClientConfirming(
		ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"}, true)
	if err == nil {
		t.Fatal("a Tier 2 launch must fail rather than reach the unsandboxed confirmation")
	}
}

// contractMajor2ManifestJSON is a manifest on a contract major this nav-pilot
// does not implement — the case that makes the difference between the two
// failure modes matter: every older binary must be told to upgrade, not
// silently dropped to the built-in default.
const contractMajor2ManifestJSON = `{
  "contractVersion": "2",
  "name": "grillmester",
  "description": "Grillmester agentpakke",
  "clients": {"copilot": {"primaryAgents": ["grillmester"]}}
}`

// stubResolveSourceAttaching installs a source funnel that runs the real
// attachPakke over a fixture tree, so its error reaches tryPakkeLaunch exactly
// as the production funnel delivers it (aliases.go: resolveSource returns
// attachPakkeOrCleanup's error verbatim).
func stubResolveSourceAttaching(t *testing.T, dir, repo string) {
	t.Helper()
	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(string, string) (*Source, error) {
		src := &Source{Dir: dir, SHA: "def5678", Version: "dev", Repo: repo}
		return src, attachPakke(src)
	}
}

// TestUnusableManifestAbortsLaunch is the other half of
// TestUnresolvableSourceFallsBackToLegacy: a source that was fetched fine but
// ships a manifest this nav-pilot cannot use must abort the launch with the
// real message, not warn and take the legacy path with the user's own
// ~/.copilot injected.
func TestUnusableManifestAbortsLaunch(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	stubResolveSourceAttaching(t, pakkeSourceTree(t, contractMajor2ManifestJSON), "navikt/grillmester")

	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"})
	if !handled || err == nil {
		t.Fatalf("tryPakkeLaunch(unusable manifest) = (%v, %v), want (true, error) — the launch must abort", handled, err)
	}
	if !errors.Is(err, errUnusableManifest) {
		t.Errorf("error should carry errUnusableManifest, got: %v", err)
	}
	if !strings.Contains(err.Error(), "contractVersion") {
		t.Errorf("error should name the manifest problem, got: %v", err)
	}
	assertDefaultPakkeActive(t)
}

// TestUnusableManifestIsNotCached: a launch that aborted before the tier gate
// learned no tier, so it must not leave an answer behind for the next launch.
func TestUnusableManifestIsNotCached(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	stubResolveSourceAttaching(t, pakkeSourceTree(t, contractMajor2ManifestJSON), "navikt/grillmester")

	if _, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"}); err == nil {
		t.Fatal("setup: the unusable manifest must fail the launch")
	}
	if tier, ok := cachedTier("navikt/grillmester", "copilot"); ok {
		t.Errorf("a failed manifest validation cached tier %d; it must leave the cache untouched", tier)
	}
}

// ─── tier memory ─────────────────────────────────────────────────────────────

// countingResolveSource installs a source funnel that hands out a prepared
// checkout and counts how often it was asked, so a test can pin that a launch
// resolved nothing.
func countingResolveSource(t *testing.T, src *Source) *int {
	t.Helper()
	calls := 0
	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(string, string) (*Source, error) {
		calls++
		return src, nil
	}
	return &calls
}

// tier1Launch runs a Tier 1 launch and fails the test if it did not take the
// legacy path.
func tier1Launch(t *testing.T, source string) {
	t.Helper()
	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: source})
	if handled || err != nil {
		t.Fatalf("tryPakkeLaunch(Tier 1) = (%v, %v), want (false, nil)", handled, err)
	}
}

// TestTierCacheSkipsSecondResolve: resolving a repo-shaped source clones it.
// Once a launch has learned that a source is not Tier 2, the next launch must
// take the legacy path without cloning again to re-learn it.
func TestTierCacheSkipsSecondResolve(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	calls := countingResolveSource(t, pakkeSource(t, "navikt/grillmester"))

	tier1Launch(t, "navikt/grillmester")
	tier1Launch(t, "navikt/grillmester")

	if *calls != 1 {
		t.Errorf("resolveSource called %d times, want 1 — the second launch must reuse the remembered tier", *calls)
	}
	assertDefaultPakkeActive(t)
}

// TestTierCacheExpires: a pakke that changes tier has to be picked up without
// the user knowing a cache exists, so an entry older than tierCacheTTL is not
// trusted.
func TestTierCacheExpires(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	calls := countingResolveSource(t, pakkeSource(t, "navikt/grillmester"))

	tier1Launch(t, "navikt/grillmester")
	ageTierCache(t, tierCacheTTL+time.Minute)
	tier1Launch(t, "navikt/grillmester")

	if *calls != 2 {
		t.Errorf("resolveSource called %d times, want 2 — an expired entry must resolve again", *calls)
	}
}

// ageTierCache backdates every entry in the cache file by d.
func ageTierCache(t *testing.T, d time.Duration) {
	t.Helper()
	data, err := os.ReadFile(tierCachePath())
	if err != nil {
		t.Fatalf("reading the tier cache: %v", err)
	}
	var cache map[string]tierCacheEntry
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("parsing the tier cache: %v", err)
	}
	if len(cache) == 0 {
		t.Fatal("the tier cache is empty; nothing was remembered")
	}
	for k, e := range cache {
		e.LearnedAt = e.LearnedAt.Add(-d)
		cache[k] = e
	}
	data, err = json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tierCachePath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTierCacheKeyedBySource: changing `source` asks a question the cache has
// never been asked, so the new source is resolved rather than inheriting the
// old one's answer.
func TestTierCacheKeyedBySource(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	calls := countingResolveSource(t, pakkeSource(t, "navikt/grillmester"))

	tier1Launch(t, "navikt/grillmester")
	tier1Launch(t, "navikt/other-pakke")

	if *calls != 2 {
		t.Errorf("resolveSource called %d times, want 2 — a different source must resolve", *calls)
	}
}

// TestTierCacheCorruptFileResolves: an unreadable cache degrades to a normal
// resolve. It must never be an error, and never a legacy assumption.
func TestTierCacheCorruptFileResolves(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	src := tier2Source(t)
	calls := countingResolveSource(t, src)

	if err := os.MkdirAll(filepath.Dir(tierCachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tierCachePath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A Tier 2 source: had the corrupt file been read as "legacy", this would
	// have returned (false, nil) without resolving anything.
	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"})
	if !handled || err == nil {
		t.Fatalf("tryPakkeLaunch over a corrupt cache = (%v, %v), want (true, staging error)", handled, err)
	}
	if *calls != 1 {
		t.Errorf("resolveSource called %d times, want 1 — a corrupt cache must resolve normally", *calls)
	}
}

// TestTierCacheRejectsUnknownTier: a well-formed entry can still hold a tier
// no nav-pilot writes. Trusting it would skip the resolve for a source that
// declares a payload and run legacy with the user's own ~/.copilot, which is
// exactly the "assume legacy" this cache may never produce.
func TestTierCacheRejectsUnknownTier(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	calls := countingResolveSource(t, tier2Source(t))

	writeTierCache(t, map[string]tierCacheEntry{
		tierCacheKey("navikt/grillmester", "copilot"): {Tier: 99, LearnedAt: time.Now()},
	})

	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"})
	if !handled || err == nil {
		t.Fatalf("tryPakkeLaunch over a nonsense tier = (%v, %v), want (true, staging error) — the Tier 2 source must be resolved", handled, err)
	}
	if *calls != 1 {
		t.Errorf("resolveSource called %d times, want 1 — an unknown tier must resolve normally", *calls)
	}
}

// TestTierCacheRejectsFutureEntry: a learnedAt in the future (clock stepped
// back, or skewed when the entry was written) makes the age negative, so a
// plain "older than the TTL?" test would trust the entry until wall-clock
// catches up. Out of range is out of range in both directions.
func TestTierCacheRejectsFutureEntry(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	calls := countingResolveSource(t, pakkeSource(t, "navikt/grillmester"))

	tier1Launch(t, "navikt/grillmester")
	ageTierCache(t, -(tierCacheTTL + time.Hour)) // forward, not back
	tier1Launch(t, "navikt/grillmester")

	if *calls != 2 {
		t.Errorf("resolveSource called %d times, want 2 — an entry learned in the future must resolve again", *calls)
	}
}

// writeTierCache replaces the tier cache file with exactly these entries.
func writeTierCache(t *testing.T, cache map[string]tierCacheEntry) {
	t.Helper()
	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(tierCachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tierCachePath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPayloadContextAlwaysResolves: --payload-context asks about payloads only
// the manifest declares, so it must resolve even when the cache holds a
// non-payload answer for the same source.
func TestPayloadContextAlwaysResolves(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	calls := countingResolveSource(t, pakkeSource(t, "navikt/grillmester"))

	tier1Launch(t, "navikt/grillmester")

	_, err := tryPakkeLaunch(ResolvedConfig{
		Client: "copilot", Source: "navikt/grillmester", PayloadContext: "focused",
	})
	if err == nil {
		t.Fatal("--payload-context against a Tier 1 agentpakke must be an error")
	}
	if *calls != 2 {
		t.Errorf("resolveSource called %d times, want 2 — --payload-context must always resolve", *calls)
	}
}

// TestTierCacheLivesWithNavPilotState keeps the remembered decision next to
// config.toml and the staged trees, not in an OS cache directory that may be
// swept between launches.
func TestTierCacheLivesWithNavPilotState(t *testing.T) {
	cfg := isolatedConfig(t)
	if got, want := tierCachePath(), filepath.Join(filepath.Dir(cfg), "tier-cache.json"); got != want {
		t.Errorf("tierCachePath() = %q, want %q", got, want)
	}
}

// mixedPakkeManifestJSON declares a layout *and* a payload: opencode is Tier 1
// and reads the layout, copilot is Tier 2 and ships a pre-built payload.
const mixedPakkeManifestJSON = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Grillmester agentpakke, mixed",
  "clients": {
    "copilot": {"payloads": {"full": {"path": "plugin", "primaryAgents": ["grillmester"]}}},
    "opencode": {"primaryAgents": ["grillmester"]}
  },
  "layout": {"agents": "plugin/agents", "skills": "plugin/skills"}
}`

// TestMixedPakkeRefusesItsPayloadClient: pinning does not cover a pakke that
// ships both a layout and payloads, and the launch says so rather than quietly
// materializing layout content into the config of a client whose manifest
// declares a verified payload. Fail-closed, like every other Tier 2 refusal.
func TestMixedPakkeRefusesItsPayloadClient(t *testing.T) {
	mixedSource := func(t *testing.T) *Source {
		t.Helper()
		src := &Source{Dir: pakkeSourceTree(t, mixedPakkeManifestJSON), SHA: "def5678", Version: "dev", Repo: "navikt/grillmester"}
		if err := attachPakke(src); err != nil {
			t.Fatalf("attachPakke on the mixed fixture: %v", err)
		}
		if src.Pakke.Layout == nil || src.Pakke.Tier("copilot") != agentpakke.TierPayload {
			t.Fatalf("fixture is not a mixed pakke: %+v", src.Pakke)
		}
		return src
	}

	t.Run("the Tier 2 client is refused", func(t *testing.T) {
		isolatedConfig(t)
		t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
		stubResolveSource(t, mixedSource(t))

		handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester"})
		if !handled || err == nil {
			t.Fatalf("tryPakkeLaunch(mixed pakke, Tier 2 client) = (%v, %v), want (true, a refusal)", handled, err)
		}
		for _, want := range []string{"grillmester", "copilot", "ships both"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q does not name %q", err, want)
			}
		}
		// It never reached the pin, so nothing was materialized either.
		if _, statErr := os.Stat(pakkeSourceDir("navikt/grillmester")); !os.IsNotExist(statErr) {
			t.Errorf("the refused launch materialized a revision (stat err %v)", statErr)
		}
		assertDefaultPakkeActive(t)
	})

	t.Run("the Tier 1 client still takes the legacy path", func(t *testing.T) {
		isolatedConfig(t)
		t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
		stubResolveSource(t, mixedSource(t))

		handled, err := tryPakkeLaunch(ResolvedConfig{Client: "opencode", Source: "navikt/grillmester"})
		if handled || err != nil {
			t.Errorf("tryPakkeLaunch(mixed pakke, Tier 1 client) = (%v, %v), want (false, nil)", handled, err)
		}
		assertDefaultPakkeActive(t)
	})
}

// TestRefusedHandoverPrintsNoModelNotice pins the ordering: the model notice
// announces the session a user is about to spend on, so it must not print for a
// launch the handover then refuses. The refusal is provoked by taking the
// opencode launcher away, which is the shape a payload for a client this binary
// cannot stage-launch arrives in.
func TestRefusedHandoverPrintsNoModelNotice(t *testing.T) {
	forceInteractive(t)
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, "sha-one"))
	failingResolveSource(t)

	origLaunchers := stagedLaunchers
	t.Cleanup(func() { stagedLaunchers = origLaunchers })
	stagedLaunchers = map[string]func(ResolvedConfig, providerpkg.StagedLaunch) error{}

	origStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stderr = w

	_, err := tryPakkeLaunch(ResolvedConfig{
		Client: "opencode", Source: "navikt/grillmester", Model: "claude-opus-5",
	})

	w.Close()
	os.Stderr = origStderr
	var stderr strings.Builder
	io.Copy(&stderr, r)

	if err == nil || !strings.Contains(err.Error(), handoverErr) {
		t.Fatalf("launch = %v, want the handover refusal", err)
	}
	if strings.Contains(stderr.String(), "Model:") {
		t.Errorf("a refused launch announced a model: %q", stderr.String())
	}
}
