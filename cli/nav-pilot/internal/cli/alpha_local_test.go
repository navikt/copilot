package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// localTestHome points HOME and the config file at temp dirs, so nothing here
// reads or writes the developer's own ~/.nav-pilot — including the record of a
// local server they may actually have running.
func localTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(home, ".nav-pilot", "config.toml"))
	t.Cleanup(func() { local.SetEnabled(false) })
	return home
}

func TestAlphaDispatch(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no arguments prints usage", args: nil},
		{name: "help prints usage", args: []string{"help"}},
		{name: "local with no subcommand prints usage", args: []string{"local"}},
		{name: "an unknown group", args: []string{"quantum"}, wantErr: "unknown alpha group"},
		{name: "an unknown subcommand", args: []string{"local", "restart"}, wantErr: "unknown command"},
		{name: "a near miss is suggested", args: []string{"local", "statuss"}, wantErr: "Did you mean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmdAlpha(tt.args)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("cmdAlpha(%q) = %v, want nil", tt.args, err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("cmdAlpha(%q) = %v, want an error containing %q", tt.args, err, tt.wantErr)
			}
		})
	}
}

// TestAlphaIsAKnownCommand keeps the argument pre-scan out of `alpha`'s
// arguments: an unknown command has its launch-override flags stripped before
// dispatch, which would eat anything this group ever takes.
func TestAlphaIsAKnownCommand(t *testing.T) {
	if !isKnownCommand("alpha") {
		t.Error("alpha is not a known command, so its arguments go through the launch-override pre-scan")
	}
}

// TestApplyLocalConfigNeedsBothHalves is the opt-in enforcement, from the CLI
// side: the config alone does not turn local dispatch on. A developer whose
// config says yes but whose machine has no provisioned environment is a
// developer for whom nothing local exists — the same state as never having
// asked.
//
// Only the negative half is checked here: writing a stamp that satisfies
// local.Installed means naming the pinned mlx versions, which belong to that
// package and are pinned by its own TestInstalledFollowsThePins.
func TestApplyLocalConfigNeedsBothHalves(t *testing.T) {
	localTestHome(t)

	if _, err := writeConfigKey("local_enabled", "true"); err != nil {
		t.Fatalf("writing local_enabled: %v", err)
	}
	applyLocalConfig()

	if local.Enabled() {
		t.Error("local dispatch was enabled with no provisioned environment on disk")
	}
	for _, m := range local.Active().Models {
		if local.IsLocal(m.Model) {
			t.Errorf("IsLocal(%q) = true with no provisioned environment", m.Model)
		}
	}
}

// TestApplyLocalConfigCarriesTheLoopGuardThreshold: the threshold is read from
// config whether or not local is on, so `config get` and the launch agree.
func TestApplyLocalConfigCarriesTheLoopGuardThreshold(t *testing.T) {
	localTestHome(t)
	t.Cleanup(func() { local.SetLoopGuardRepeat(local.DefaultLoopGuardRepeat) })

	if _, err := writeConfigKey("local_loop_guard", "3"); err != nil {
		t.Fatalf("writing local_loop_guard: %v", err)
	}
	applyLocalConfig()
	if got := local.LoopGuardRepeat(); got != 3 {
		t.Errorf("LoopGuardRepeat() = %d, want the configured 3", got)
	}
}

func TestLocalLoopGuardDefaults(t *testing.T) {
	if got := localLoopGuard(ResolvedConfig{}); got != local.DefaultLoopGuardRepeat {
		t.Errorf("localLoopGuard(unset) = %d, want %d", got, local.DefaultLoopGuardRepeat)
	}
	if got := localLoopGuard(ResolvedConfig{LocalLoopGuard: 25}); got != 25 {
		t.Errorf("localLoopGuard(25) = %d, want 25", got)
	}
	// Below 2 is refused by validateConfig, so this only answers for a config
	// that predates the key or was hand-edited.
	if got := localLoopGuard(ResolvedConfig{LocalLoopGuard: 1}); got != local.DefaultLoopGuardRepeat {
		t.Errorf("localLoopGuard(1) = %d, want the default", got)
	}
}

func TestLoopGuardConfigKeyRefusesAThresholdThatIsNotAGuard(t *testing.T) {
	localTestHome(t)
	if _, err := writeConfigKey("local_loop_guard", "1"); err == nil {
		t.Error("local_loop_guard = 1 was accepted; one tool call is not a loop")
	}
	if _, err := writeConfigKey("local_loop_guard", "8"); err != nil {
		t.Errorf("local_loop_guard = 8 was refused: %v", err)
	}
}

// TestLocalOffDisablesDispatchAndResetsALocalModel: with dispatch off, a local
// model id left in the config would be sent to a hosted provider that has never
// heard of it, failing several layers down with an error about something else.
func TestLocalOffDisablesDispatchAndResetsALocalModel(t *testing.T) {
	localTestHome(t)

	models := local.Active().Models
	if len(models) == 0 {
		t.Fatal("the embedded local-model manifest names no models")
	}
	if _, err := writeConfigKey("local_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := writeConfigKey("model", models[0].Model); err != nil {
		t.Fatal(err)
	}

	if err := cmdLocalOff(); err != nil {
		t.Fatalf("cmdLocalOff() errored: %v", err)
	}

	cfg, err := readConfig()
	if err != nil || cfg == nil {
		t.Fatalf("reading the config back: %v", err)
	}
	if cfg.LocalEnabled == nil || *cfg.LocalEnabled {
		t.Error("local_enabled is still on after `alpha local off`")
	}
	if cfg.Model == nil || *cfg.Model != "auto" {
		t.Errorf("model = %v after `alpha local off`, want it reset to auto", cfg.Model)
	}
	// The config it wrote must still be one nav-pilot will launch with.
	if err := validateConfig(cfg); err != nil {
		t.Errorf("`alpha local off` left an invalid config: %v", err)
	}
}

// TestLocalOffLeavesAHostedModelAlone: off only resets a model that is local.
func TestLocalOffLeavesAHostedModelAlone(t *testing.T) {
	localTestHome(t)
	if _, err := writeConfigKey("model", "claude-opus-5"); err != nil {
		t.Fatal(err)
	}
	if err := cmdLocalOff(); err != nil {
		t.Fatalf("cmdLocalOff() errored: %v", err)
	}
	cfg, _ := readConfig()
	if cfg == nil || cfg.Model == nil || *cfg.Model != "claude-opus-5" {
		t.Errorf("`alpha local off` changed a hosted model to %v", cfg.Model)
	}
}

// TestLocalStatusWithNothingRunning: the "not started" state, which is what
// every developer who has not opted in would see.
func TestLocalStatusWithNothingRunning(t *testing.T) {
	localTestHome(t)
	out := captureStdout(func() {
		if err := cmdLocalStatus(); err != nil {
			t.Errorf("cmdLocalStatus() errored: %v", err)
		}
	})
	for _, want := range []string{string(local.HealthNotStarted), "not provisioned", "off"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output does not mention %q:\n%s", want, out)
		}
	}
}

func TestLocalStopWithNothingRunning(t *testing.T) {
	localTestHome(t)
	out := captureStdout(func() {
		if err := cmdLocalStop(); err != nil {
			t.Errorf("cmdLocalStop() errored: %v", err)
		}
	})
	if !strings.Contains(out, "No local server") {
		t.Errorf("stop output does not say nothing was running:\n%s", out)
	}
}

// TestLocalStartRefusesAnUnprovisionedMachine: start must not reach the
// manifest, the network or a spawn before it has checked that init was run.
func TestLocalStartRefusesAnUnprovisionedMachine(t *testing.T) {
	localTestHome(t)
	err := cmdLocalStart()
	if err == nil || !strings.Contains(err.Error(), "alpha local init") {
		t.Errorf("cmdLocalStart() on an unprovisioned machine = %v, want an error naming init", err)
	}
}

// TestLocalStatusReportsACrashedServer: a recorded pid that is gone reports
// crashed, not "starting". The two need different responses and only one of
// them is worth waiting through.
func TestLocalStatusReportsACrashedServer(t *testing.T) {
	home := localTestHome(t)
	dir := filepath.Join(home, ".nav-pilot", "local")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A pid that cannot be running: 0 is refused by LoadState, and this one is
	// past any plausible pid_max on the platforms nav-pilot runs on.
	record := `{"pid":4194303,"model":"mlx-community/x","port":8080,"started":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "server.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(func() {
		if err := cmdLocalStatus(); err != nil {
			t.Errorf("cmdLocalStatus() errored: %v", err)
		}
	})
	if !strings.Contains(out, string(local.HealthCrashed)) {
		t.Errorf("status of a pid that is gone does not report %q:\n%s", local.HealthCrashed, out)
	}
}

func TestWrapIndent(t *testing.T) {
	got := wrapIndent("one two three four five", "  ", 12)
	want := "one two\n  three four\n  five"
	if got != want {
		t.Errorf("wrapIndent()\n got: %q\nwant: %q", got, want)
	}
}
