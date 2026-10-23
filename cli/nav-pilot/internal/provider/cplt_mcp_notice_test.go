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

func TestCpltMCPNotice(t *testing.T) {
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
		warnOpenCode1()
		os.Stderr = orig
		w.Close()
		out, _ := io.ReadAll(r)
		return string(out)
	}
	const v2 = "opencode v2.1.2\n"
	old := "cplt " + openCode2CpltFloor + "-b37a8ab\n"

	// Control: the same old cplt under opencode 1 says nothing about cplt.
	if out := launch("1.18.32\n", old); strings.Contains(out, "github-mcp") {
		t.Errorf("opencode 1: %q, want no cplt notice", out)
	}
	first := launch(v2, old)
	for _, want := range []string{"io.github.navikt/github-mcp", openCode2CpltRecommended, "navikt/cplt#807", "navikt/cplt#817", cpltUpgradeHint()} {
		if !strings.Contains(first, want) {
			t.Errorf("notice %q lacks %q", first, want)
		}
	}
	if again := launch(v2, old); again != "" {
		t.Errorf("same cplt again: %q, want nothing", again)
	}
	if next := launch(v2, "cplt 2026.10.23-053641-abcdef0\n"); !strings.Contains(next, "github-mcp") {
		t.Errorf("new cplt below recommended: %q, want notice", next)
	}
	for _, c := range []string{"cplt " + openCode2CpltRecommended + "-7f0fd16\n", "cplt 2026.11.01-000000-abcdef0\n"} {
		if out := launch(v2, c); out != "" {
			t.Errorf("cplt %q: %q, want nothing", c, out)
		}
	}
}

// --deny-env for the voice keys only on opencode 2 with cplt at or above
// openCode2CpltRecommended: an older cplt rejects the flag. "tier1" is the
// helper opencode_launch.go appends to cpltFlags; "staged" is the real spec.
func TestDenyVoiceEnv(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { SetActivePakke(nil); versionCache.Delete("opencode") })
	prev := probeCpltVersion
	t.Cleanup(func() { probeCpltVersion = prev })
	staged := StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"}
	at := "cplt " + openCode2CpltRecommended + "-7f0fd16\n"
	for _, c := range []struct {
		opencode, cplt string
		err            error
		want           bool
	}{
		{"opencode v2.1.3\n", at, nil, true},
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
