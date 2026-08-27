package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
      "primaryAgents": ["grillmester", "barista"],
      "defaultModel": "inherit",
      "defaultContext": "full",
      "payloads": {
        "full": {"path": "plugin"},
        "focused": {"path": "targets/copilot-cli-focused-v1"}
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

// TestPayloadContextUnknown: an unknown context lists what the manifest does
// declare, sorted.
func TestPayloadContextUnknown(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	stubResolveSource(t, tier2Source(t))

	handled, err := tryPakkeLaunch(ResolvedConfig{Client: "copilot", Source: "navikt/grillmester", PayloadContext: "tiny"})
	if !handled || err == nil {
		t.Fatalf("tryPakkeLaunch(unknown context) = (%v, %v), want (true, error)", handled, err)
	}
	if !strings.Contains(err.Error(), "declared contexts: focused, full") {
		t.Errorf("error should list the declared contexts sorted, got: %v", err)
	}
	assertDefaultPakkeActive(t)
}

// TestPayloadContextSelection pins G3's defaulting: with no flag the manifest's
// defaultContext is staged, and the flag overrides it. Both fixtures point at
// payload paths that carry no payload manifest, so staging fails — after the
// context has been selected, which is what the wrapped error names.
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
			isolatedConfig(t)
			t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
			stubResolveSource(t, tier2Source(t))

			handled, err := tryPakkeLaunch(ResolvedConfig{
				Client: "copilot", Source: "navikt/grillmester", PayloadContext: tt.flag,
			})
			if !handled || err == nil {
				t.Fatalf("tryPakkeLaunch = (%v, %v), want (true, staging error)", handled, err)
			}
			want := `staging the "` + tt.wantContext + `" payload`
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should report %s, got: %v", want, err)
			}
			// Fail-closed: a Tier 2 launch that could not stage never falls
			// back to the legacy launch.
			assertDefaultPakkeActive(t)
		})
	}
}

// TestStagedRootFollowsConfig keeps staged trees inside nav-pilot's own home,
// and out of the developer's when tests redirect the config.
func TestStagedRootFollowsConfig(t *testing.T) {
	cfg := isolatedConfig(t)
	if got, want := stagedRoot(), filepath.Join(filepath.Dir(cfg), "staged"); got != want {
		t.Errorf("stagedRoot() = %q, want %q", got, want)
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
