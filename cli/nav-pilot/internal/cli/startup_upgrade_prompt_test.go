package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// stubUpgradePrompt sets up a terminal user whose binary is 23 days behind a
// release well past the quiet period, and records whether "Upgrade now?" was
// asked. existing writes a config without auto_update, as an existing user
// has; otherwise the home holds no nav-pilot state at all.
func stubUpgradePrompt(t *testing.T, existing bool) (asked *bool) {
	t.Helper()
	const current = "2026.09.01-120000-aaaaaaa"
	const latest = "2026.09.24-120000-bbbbbbb"
	path := isolatedConfig(t)
	if existing {
		mustWrite(t, path, "version = 1\n")
	}

	origVersion, origAssess, origManager := Version, assessStaleness, packageManager
	origInteractive, origConfirm, origAPI := isInteractive, confirmUpgrade, releasesAPI
	t.Cleanup(func() {
		Version, assessStaleness, packageManager = origVersion, origAssess, origManager
		isInteractive, confirmUpgrade, releasesAPI = origInteractive, origConfirm, origAPI
		startedFresh = false
	})
	Version = current
	assessStaleness = func(string) artifacts.StalenessAssessment {
		return artifacts.AssessFromLatest(current, latest, "")
	}
	packageManager = func() domain.PkgManager { return domain.PkgManager{} }
	isInteractive = func() bool { return true }
	// The auto-update a new install runs fails here, offline, and the
	// command goes on: what matters is that nothing asked first.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	releasesAPI = srv.URL
	startedFresh = noNavPilotState()
	if startedFresh == existing {
		t.Fatalf("startedFresh = %v with existing = %v; the setup is not the case under test", startedFresh, existing)
	}

	asked = new(bool)
	confirmUpgrade = func(title string) (bool, error) {
		if strings.Contains(title, "Upgrade now?") {
			*asked = true
		}
		return false, nil
	}
	if a := assessStaleness(Version); a.SkewDays <= 7 {
		t.Fatalf("SkewDays = %d, want more than 7", a.SkewDays)
	}
	return asked
}

// TestNewInstallNeverAsksUpgradeNow: a fresh install takes auto_update on, so a
// binary more than 7 days behind updates itself instead of asking (#2791).
func TestNewInstallNeverAsksUpgradeNow(t *testing.T) {
	asked := stubUpgradePrompt(t, false)
	out := captureStderrFor(t, func() {
		if _, err := startupUpdateCheck(); err != nil {
			t.Fatalf("startupUpdateCheck = %v", err)
		}
	})
	if *asked {
		t.Fatal(`a new install was asked "Upgrade now?"`)
	}
	if !strings.Contains(out, "Auto-updating nav-pilot") {
		t.Errorf("a new install did not try the auto-update. Stderr:\n%s", out)
	}
}

// TestExistingUserStillAsksUpgradeNow is the control: an existing user without
// auto_update still gets the question.
func TestExistingUserStillAsksUpgradeNow(t *testing.T) {
	asked := stubUpgradePrompt(t, true)
	captureStderrFor(t, func() {
		if _, err := startupUpdateCheck(); err != nil {
			t.Fatalf("startupUpdateCheck = %v", err)
		}
	})
	if !*asked {
		t.Fatal(`an existing user without auto_update was not asked "Upgrade now?"`)
	}
}
