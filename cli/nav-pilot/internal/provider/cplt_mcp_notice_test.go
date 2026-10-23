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
	launch := func(opencode, cplt string) string {
		t.Helper()
		versionCache.Store("opencode", versionAnswer{opencode, nil, time.Hour})
		probeCpltVersion = func() (string, error) { return cplt, nil }
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stderr
		os.Stderr = w
		warnCpltRecommended()
		os.Stderr = orig
		w.Close()
		out, _ := io.ReadAll(r)
		return string(out)
	}
	const v2 = "opencode v2.1.2\n"
	old := "cplt " + cpltDenyEnv + "-7f0fd16\n"

	first := launch(v2, old)
	want := "cplt " + cpltRecommended + " or newer is recommended: GitHub MCP and Nais MCP through cplt's broker, and blocking API keys opencode's voice feature reads. Update: "
	if !strings.Contains(first, want) || !strings.Contains(first, cpltUpgradeHint()) || strings.Count(first, "\n") != 1 {
		t.Errorf("notice %q, want one line with %q", first, want)
	}
	// Once per recommended version: another old cplt or client says nothing.
	if again := launch("", "cplt 2026.10.22-172931-b37a8ab\n"); again != "" {
		t.Errorf("same recommendation again: %q, want nothing", again)
	}
	// A higher recommendation than the one last seen says it again; without
	// opencode 2 only the reason every client gains.
	SeenChanged("cplt-recommended", cpltDenyEnv)
	if next := launch("", old); !strings.Contains(next, "recommended: Nais MCP through cplt's broker. Update: ") {
		t.Errorf("raised recommendation: %q, want notice", next)
	}
	// Control: at and above the recommendation, silent even when unseen.
	for _, c := range []string{"cplt " + cpltRecommended + "-7f0fd16\n", "cplt 2026.11.01-000000-abcdef0\n"} {
		SeenChanged("cplt-recommended", "")
		if out := launch(v2, c); out != "" {
			t.Errorf("cplt %q: %q, want nothing", c, out)
		}
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
