package provider

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// One cplt line per recommended version, for every client cplt launches: shown
// below cpltRecommended once, again when the recommendation rises, never at or
// above it.
func TestCpltRecommendedNotice(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	t.Cleanup(func() { versionCache.Delete("opencode") })
	prev := probeCpltVersion
	t.Cleanup(func() { probeCpltVersion = prev })
	versionCache.Store("opencode", versionAnswer{"opencode v2.1.2\n", nil, time.Hour})
	launch := func(agent, cplt string) string {
		t.Helper()
		probeCpltVersion = func() (string, error) { return cplt, nil }
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stderr
		os.Stderr = w
		warnCpltRecommended(agent)
		os.Stderr = orig
		w.Close()
		out, _ := io.ReadAll(r)
		return string(out)
	}
	old := "cplt " + cpltDenyEnv + "-7f0fd16\n"
	at := "cplt " + cpltRecommended + "-7f0fd16\n"

	first := launch("opencode", old)
	want := "cplt " + cpltRecommended + " or newer is recommended: it runs Nais MCP through cplt's broker, lets GitHub MCP work in opencode, and blocks the API keys opencode's voice feature reads. Update cplt: "
	if !strings.Contains(first, want) || !strings.Contains(first, cpltUpgradeHint()) || strings.Count(first, "\n") != 1 {
		t.Errorf("notice %q, want one line with %q", first, want)
	}
	// Once per recommended version and reason set: the same opencode launch
	// again says nothing.
	if again := launch("opencode", "cplt 2026.10.22-172931-b37a8ab\n"); again != "" {
		t.Errorf("same recommendation again: %q, want nothing", again)
	}
	// The opencode line already gave the neutral reason, so Copilot stays
	// silent; switching back and forth repeats neither.
	if next := launch("copilot", old); next != "" {
		t.Errorf("copilot after opencode: %q, want nothing", next)
	}
	for _, agent := range []string{"opencode", "copilot"} {
		if again := launch(agent, old); again != "" {
			t.Errorf("%s again: %q, want nothing", agent, again)
		}
	}
	// A higher recommendation than the one last seen says it again.
	SeenChanged("cplt-recommended", cpltDenyEnv)
	if next := launch("copilot", old); !strings.Contains(next, "recommended: it runs Nais MCP through cplt's broker. Update cplt: ") {
		t.Errorf("raised recommendation: %q, want the client-neutral notice", next)
	}
	// At or above: silent, and the seen state is cleared, so a later old cplt
	// says it again.
	for _, c := range []string{at, "cplt 2026.11.01-000000-abcdef0\n"} {
		if out := launch("opencode", c); out != "" {
			t.Errorf("cplt %q: %q, want nothing", c, out)
		}
	}
	if out := launch("pi", old); out == "" {
		t.Error("after an up-to-date cplt, an old one again: want the notice")
	}
	// #1962: seen under Copilot first, the opencode reasons still reach an
	// OpenCode launch (the control above: the same client again is silent).
	if out := launch("opencode", old); !strings.Contains(out, "lets GitHub MCP work in opencode") {
		t.Errorf("opencode after copilot saw the neutral line: %q, want the opencode reasons", out)
	}
}

// --deny-env for the voice keys only on opencode 2 with cplt at or above
// cpltDenyEnv, even below cpltRecommended: an older cplt rejects the flag. "tier1" is the
// helper opencode_launch.go appends to cpltFlags; "staged" is the real spec.
func TestDenyVoiceEnv(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { SetActivePakke(nil); versionCache.Delete("opencode") })
	prev := probeCpltVersion
	t.Cleanup(func() { probeCpltVersion = prev })
	staged := StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"}
	at := "cplt " + cpltDenyEnv + "-7f0fd16\n"
	for _, c := range []struct {
		opencode, cplt string
		err            error
		want           bool
	}{
		{"opencode v2.1.3\n", at, nil, true},
		{"opencode v2.1.3\n", "cplt 2026.10.23-130112-abcdef0\n", nil, true}, // below cpltRecommended
		{"opencode v2.1.3\n", "cplt " + cpltRecommended + "-abcdef0\n", nil, true},
		{"opencode v2.1.3\n", "cplt 2026.11.01-000000-abcdef0\n", nil, true},
		{"opencode v2.1.3\n", "cplt 2026.10.23-053641-abcdef0\n", nil, false},
		{"opencode v2.1.3\n", "cplt dev\n", nil, false},                 // unparseable
		{"opencode v2.1.3\n", "", errors.New("probe timed out"), false}, // the launch still proceeds
		{"1.18.35\n", at, nil, false},                                   // control: opencode 1 never gets it
	} {
		versionCache.Store("opencode", versionAnswer{c.opencode, nil, time.Hour})
		probeCpltVersion = func() (string, error) { return c.cplt, c.err }
		spec, err := buildStagedOpenCodeSpec(domain.ResolvedConfig{Client: "opencode"}, staged)
		if err != nil {
			t.Fatalf("%q %q: %v, want the launch to proceed", c.opencode, c.cplt, err)
		}
		for name, args := range map[string][]string{"tier1": withDenyVoiceEnv(nil), "staged": spec.cpltArgs} {
			for _, v := range openCodeVoiceEnv {
				i := slices.Index(args, v)
				if got := i > 0 && args[i-1] == "--deny-env"; got != c.want {
					t.Errorf("%s %q %q: args %v, want --deny-env %s: %v", name, c.opencode, c.cplt, args, v, c.want)
				}
			}
		}
	}
}
