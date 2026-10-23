package cli

import (
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// TestStartupUpdateOncePerRelease: "available" is said once per release, then
// silent until the next one; --verbose says it every time. Both nudges: the
// package-manager one and the "nav-pilot upgrade" one.
func TestStartupUpdateOncePerRelease(t *testing.T) {
	for name, mgr := range map[string]domain.PkgManager{"brew": domain.PkgBrew, "none": domain.PkgNone} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			const current = "2026.09.10-065538-661d4c8"
			stubStartupUpdate(t, current, current, false, mgr)
			say := func(latest string) string {
				assessStaleness = func(string) artifacts.StalenessAssessment {
					return artifacts.AssessFromLatest(current, latest, "")
				}
				return captureStderrFor(t, func() { _, _ = startupUpdateCheck() })
			}
			if out := say("2026.09.12-080624-cfeafb1"); !strings.Contains(out, "available") {
				t.Fatalf("a new release was silent: %q", out)
			}
			if out := say("2026.09.12-080624-cfeafb1"); out != "" {
				t.Errorf("the same release was said again: %q", out)
			}
			if out := say("2026.09.14-080624-aaaaaaa"); !strings.Contains(out, "2026.09.14") {
				t.Errorf("the next release was silent: %q", out)
			}
			providerpkg.Verbose = true
			t.Cleanup(func() { providerpkg.Verbose = false })
			if out := say("2026.09.14-080624-aaaaaaa"); !strings.Contains(out, "available") {
				t.Errorf("--verbose was silent: %q", out)
			}
		})
	}
}
