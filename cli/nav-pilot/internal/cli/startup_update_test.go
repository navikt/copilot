package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// stubStartupUpdate points the startup check at a fixed latest release, a fixed
// owning package manager and a config with or without auto_update, so a test can
// read what the user is told without a network call or a packaged install.
func stubStartupUpdate(t *testing.T, current, latest string, autoUpdate bool, mgr domain.PkgManager) {
	t.Helper()
	path := isolatedConfig(t)
	cfg := "version = 1\n"
	if autoUpdate {
		cfg += "auto_update = true\n"
	}
	mustWrite(t, path, cfg)

	origVersion := Version
	origAssess := assessStaleness
	origManager := packageManager
	origInteractive := isInteractive
	t.Cleanup(func() {
		Version = origVersion
		assessStaleness = origAssess
		packageManager = origManager
		isInteractive = origInteractive
	})
	Version = current
	assessStaleness = func(string) artifacts.StalenessAssessment {
		return artifacts.AssessFromLatest(current, latest, "")
	}
	packageManager = func() domain.PkgManager { return mgr }
	// The upgrade prompt would block on a terminal that is not there.
	isInteractive = func() bool { return false }
}

// TestStartupUpdateNeverAnnouncesWhatBrewMustDo: a Homebrew-managed binary
// cannot be replaced by doUpdate, so the startup check must not announce an
// auto-update it will then decline — it says what to run instead.
func TestStartupUpdateNeverAnnouncesWhatBrewMustDo(t *testing.T) {
	const current = "2026.09.10-065538-661d4c8"
	const latest = "2026.09.12-080624-cfeafb1"

	for _, autoUpdate := range []bool{true, false} {
		name := "auto_update off"
		if autoUpdate {
			name = "auto_update on"
		}
		t.Run(name, func(t *testing.T) {
			stubStartupUpdate(t, current, latest, autoUpdate, domain.PkgBrew)

			var stop bool
			var err error
			out := captureStderrFor(t, func() { stop, err = startupUpdateCheck() })
			if err != nil || stop {
				t.Fatalf("startupUpdateCheck = (%v, %v), want (false, nil)", stop, err)
			}
			if strings.Contains(out, "Auto-updating") {
				t.Errorf("a Homebrew install was told an auto-update was happening; doUpdate would then refuse it. Output:\n%s", out)
			}
			if !strings.Contains(out, "brew upgrade navikt/tap/nav-pilot") {
				t.Errorf("the Homebrew install was not told what to run. Output:\n%s", out)
			}
			if strings.Contains(out, "nav-pilot upgrade") {
				t.Errorf("the Homebrew install was sent to a command that only prints the brew line. Output:\n%s", out)
			}
			if n := strings.Count(strings.TrimSpace(out), "\n"); n != 0 {
				t.Errorf("the Homebrew notice is %d lines, want 1. Output:\n%s", n+1, out)
			}
		})
	}
}

// TestStartupUpdateQuietPeriod: a release younger than
// artifacts.ReleaseQuietPeriod is not announced at all — not as a nudge, not as
// an auto-update. An unparsable version fails open and is announced.
func TestStartupUpdateQuietPeriod(t *testing.T) {
	const current = "2026.09.10-065538-661d4c8"
	fresh := time.Now().UTC().Add(-5*time.Minute).Format("2006.01.02-150405") + "-cfeafb1"
	old := time.Now().UTC().Add(-30*time.Hour).Format("2006.01.02-150405") + "-cfeafb1"

	tests := []struct {
		name       string
		latest     string
		autoUpdate bool
		wantQuiet  bool
	}{
		{"a release minutes old is not nudged about", fresh, false, true},
		{"a release minutes old is not auto-updated to", fresh, true, true},
		{"a release past the quiet period is nudged about", old, false, false},
		// Newer by the lexical comparison versionNewer makes, unreadable by
		// the clock: the nudge has to fail open toward telling the user.
		{"an unreadable timestamp still nudges", "9999.99.99-xxxxxx-cfeafb1", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Homebrew keeps doUpdate (and the network) out of the auto_update
			// case; what is under test is whether anything is printed at all.
			stubStartupUpdate(t, current, tt.latest, tt.autoUpdate, domain.PkgBrew)

			out := captureStderrFor(t, func() {
				if _, err := startupUpdateCheck(); err != nil {
					t.Errorf("startupUpdateCheck = %v", err)
				}
			})
			if tt.wantQuiet && out != "" {
				t.Errorf("a release inside the quiet period interrupted the command. Output:\n%s", out)
			}
			if !tt.wantQuiet && !strings.Contains(out, tt.latest) {
				t.Errorf("the available release was not reported. Output:\n%s", out)
			}
		})
	}
}

// TestExplicitUpdateIgnoresQuietPeriod: `nav-pilot update` is the user asking,
// so the quiet period that silences the startup nudge does not apply to it.
func TestExplicitUpdateIgnoresQuietPeriod(t *testing.T) {
	const current = "2026.09.10-065538-661d4c8"
	fresh := time.Now().UTC().Add(-5*time.Minute).Format("2006.01.02-150405") + "-cfeafb1"
	// Brew-managed so the explicit path answers from packageManager instead of
	// the releases API; the quiet period is the variable under test. The empty
	// PATH keeps the brew branch's cplt lookup off the network too.
	stubStartupUpdate(t, current, fresh, false, domain.PkgBrew)
	t.Setenv("PATH", t.TempDir())

	if quiet := captureStderrFor(t, func() {
		if _, err := startupUpdateCheck(); err != nil {
			t.Fatalf("startupUpdateCheck = %v", err)
		}
	}); quiet != "" {
		t.Fatalf("the startup check spoke about a release inside the quiet period:\n%s", quiet)
	}

	out := captureStdoutFor(t, func() {
		if err := cmdUpdate(); err != nil {
			t.Fatalf("cmdUpdate = %v", err)
		}
	})
	if !strings.Contains(out, "brew upgrade navikt/tap/nav-pilot") {
		t.Errorf("an explicit update said nothing about a release inside the quiet period. Output:\n%s", out)
	}
}

// TestStartupUpdateNeverAnnouncesWhatAptMustDo: the .deb install cannot be
// replaced by doUpdate either, so the startup nudge must point at apt rather
// than offer an auto-update that will be declined — or name brew, which is not
// installed on the machine that got nav-pilot from the apt archive.
func TestStartupUpdateNeverAnnouncesWhatAptMustDo(t *testing.T) {
	const current = "2026.09.10-065538-661d4c8"
	const latest = "2026.09.12-080624-cfeafb1"

	for _, autoUpdate := range []bool{true, false} {
		name := "auto_update off"
		if autoUpdate {
			name = "auto_update on"
		}
		t.Run(name, func(t *testing.T) {
			stubStartupUpdate(t, current, latest, autoUpdate, domain.PkgApt)

			var stop bool
			var err error
			out := captureStderrFor(t, func() { stop, err = startupUpdateCheck() })
			if err != nil || stop {
				t.Fatalf("startupUpdateCheck = (%v, %v)", stop, err)
			}
			if !strings.Contains(out, "sudo apt upgrade nav-pilot") {
				t.Errorf("an apt install was not told to use apt. Output:\n%s", out)
			}
			if strings.Contains(out, "brew") {
				t.Errorf("an apt install was told to use Homebrew. Output:\n%s", out)
			}
			if strings.Contains(out, "Auto-updating") {
				t.Errorf("an update that doUpdate will decline was announced. Output:\n%s", out)
			}
		})
	}
}
