package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
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

// TestApplyLocalConfigSaysWhenAnUpgradeDisarmedDispatch: local.Installed() pins
// exact mlx and mlx-lm versions, so a nav-pilot upgrade that bumps a pin flips
// it false on a machine that never changed. Config still says local_enabled
// with a local model selected, the id goes down the hosted path, and it fails
// with an error about something else — unless dispatch says so on the way past.
func TestApplyLocalConfigSaysWhenAnUpgradeDisarmedDispatch(t *testing.T) {
	localTestHome(t)
	if _, err := writeConfigKey("local_enabled", "true"); err != nil {
		t.Fatalf("writing local_enabled: %v", err)
	}

	out := captureStderr(applyLocalConfig)
	if !strings.Contains(out, "nav-pilot alpha local init") {
		t.Errorf("applyLocalConfig said %q on an unprovisioned machine with local_enabled=true, want the fix named", out)
	}

	// And silent for everyone who never asked, which is the promise the whole
	// alpha is behind.
	if _, err := writeConfigKey("local_enabled", "false"); err != nil {
		t.Fatalf("writing local_enabled: %v", err)
	}
	if out := captureStderr(applyLocalConfig); out != "" {
		t.Errorf("applyLocalConfig wrote %q with local dispatch off", out)
	}
}

// captureStderr is captureStdout's other half; applyLocalConfig warns on
// stderr because stdout is a command's answer.
func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	return string(out)
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

// TestLocalOffTakesTheModelOutOfOpenCodeToo is finding 5: nav-pilot's own
// config is not the only place the local model was registered.
//
// `start` writes an mlx provider block into ~/.config/opencode/opencode.json,
// pointing at the loop guard's port. Turning dispatch off only stopped
// nav-pilot from choosing the model — the block stayed, so a developer running
// opencode directly could still select it and reach whatever was listening on
// that port. The rest of the developer's config has to survive.
func TestLocalOffTakesTheModelOutOfOpenCode(t *testing.T) {
	localTestHome(t)
	cfgPath := filepath.Join(t.TempDir(), "opencode.json")
	providerpkg.ConfigPathOverride = cfgPath
	t.Cleanup(func() { providerpkg.ConfigPathOverride = "" })

	models := local.Active().Models
	if len(models) == 0 {
		t.Fatal("the embedded local-model manifest names no models")
	}
	if err := providerpkg.EnsureOpenCodeLocalProvider(models[0], "http://127.0.0.1:54321"); err != nil {
		t.Fatalf("registering the local provider: %v", err)
	}
	// Something of the developer's own, which off must not take with it.
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg["provider"].(map[string]any)["anthropic"] = map[string]any{"name": "theirs"}
	cfg["theme"] = "tokyonight"
	if raw, err = json.Marshal(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	captureStdout(func() {
		if err := cmdLocalOff(); err != nil {
			t.Fatalf("cmdLocalOff() errored: %v", err)
		}
	})

	raw, err = os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading the opencode config back: %v", err)
	}
	cfg = nil
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("`alpha local off` left invalid JSON in the opencode config: %v", err)
	}
	providers, _ := cfg["provider"].(map[string]any)
	if _, found := providers[providerpkg.LocalProviderID]; found {
		t.Errorf("`alpha local off` left the %q provider in %s; the model is still selectable and still points at the guard port",
			providerpkg.LocalProviderID, cfgPath)
	}
	if _, found := providers["anthropic"]; !found {
		t.Error("`alpha local off` removed a provider that was not nav-pilot's")
	}
	if cfg["theme"] != "tokyonight" {
		t.Error("`alpha local off` did not leave the rest of the developer's opencode config alone")
	}
}

// TestLocalOffWithNoOpenCodeConfigIsNotAnError: off has to work on a machine
// where opencode was never configured.
// TestLocalOffTakesTheDispatchPolicyOutOfOpenCode: off removes what the launch
// provisioned, both halves of it. A policy left registered keeps telling every
// session — hosted ones included — to hand work to a worker that is no longer
// reachable.
func TestLocalOffTakesTheDispatchPolicyOutOfOpenCode(t *testing.T) {
	localTestHome(t)
	dir := t.TempDir()
	providerpkg.ConfigPathOverride = filepath.Join(dir, "opencode.json")
	t.Cleanup(func() { providerpkg.ConfigPathOverride = "" })

	models := local.Active().Models
	if len(models) == 0 {
		t.Fatal("the embedded local-model manifest names no models")
	}
	local.SetEnabled(true)
	if err := providerpkg.EnsureOpenCodeLocalPolicy(models[0]); err != nil {
		t.Fatalf("provisioning the dispatch policy: %v", err)
	}
	policy := filepath.Join(dir, "nav-pilot-lokal-dispatch.md")
	if _, err := os.Stat(policy); err != nil {
		t.Fatalf("the dispatch policy was not provisioned: %v", err)
	}

	captureStdout(func() {
		if err := cmdLocalOff(); err != nil {
			t.Fatalf("cmdLocalOff() errored: %v", err)
		}
	})

	if _, err := os.Stat(policy); !os.IsNotExist(err) {
		t.Error("`alpha local off` left the dispatch policy on disk")
	}
	raw, err := os.ReadFile(providerpkg.ConfigPathOverride)
	if err != nil {
		t.Fatalf("reading the opencode config back: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("`alpha local off` left invalid JSON in the opencode config: %v", err)
	}
	if _, found := cfg["instructions"]; found {
		t.Errorf("`alpha local off` left the dispatch policy registered: %v", cfg["instructions"])
	}
}

func TestLocalOffWithNoOpenCodeConfig(t *testing.T) {
	localTestHome(t)
	providerpkg.ConfigPathOverride = filepath.Join(t.TempDir(), "absent.json")
	t.Cleanup(func() { providerpkg.ConfigPathOverride = "" })
	captureStdout(func() {
		if err := cmdLocalOff(); err != nil {
			t.Errorf("cmdLocalOff() with no opencode config = %v, want nil", err)
		}
	})
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

// printedAddress finds every address start names, so the test below can hold
// all of them to the same rule rather than the one that happened to be wrong.
var printedAddress = regexp.MustCompile(`https?://[^\s)]+`)

// TestStartSummaryNamesOnlyAddressesThatAnswer: start printed
// "Client http://127.0.0.1:8081 (nav-pilot's loop guard, which every request
// goes through)" while nothing was listening there — the guard is an
// in-process listener the client launch starts, and it cannot outlive a
// command without a daemon. So a developer who followed the printed
// instructions by hand reached nothing, or reached the unguarded server on
// 8080 instead.
//
// The rule, not the one line: every address this command prints is one a
// client can connect to the moment it returns.
func TestStartSummaryNamesOnlyAddressesThatAnswer(t *testing.T) {
	localTestHome(t)
	server := httptest.NewServer(nil)
	defer server.Close()

	out := startSummary(
		local.Model{Name: "A Model", Model: "mlx-community/x"},
		server.URL, 4242,
		local.WiredLimit{RequiredGB: 36, CurrentGB: 36},
		42*time.Second,
	)

	addrs := printedAddress.FindAllString(out, -1)
	if len(addrs) == 0 {
		t.Fatalf("start printed no address at all, so this test proves nothing:\n%s", out)
	}
	for _, addr := range addrs {
		host := strings.TrimSuffix(strings.TrimPrefix(addr, "http://"), "/")
		conn, err := net.DialTimeout("tcp", host, 5*time.Second)
		if err != nil {
			t.Errorf("start prints %s, but nothing is accepting connections there: %v\n%s", addr, err, out)
			continue
		}
		conn.Close()
	}
	// The guard still has to be named — a turn being ended for you is not
	// something to discover from silence — but as what the launch does.
	if !strings.Contains(out, "Guard") || !strings.Contains(out, "launch") {
		t.Errorf("start no longer says the launch runs the loop guard:\n%s", out)
	}
}

// TestStopDoesNotSignalAPidItDoesNotOwn is the reboot, end to end and with a
// real process.
//
// server.json survives a reboot; the pid in it does not survive as the same
// process. Stop used to check liveness with kill(pid, 0) — which answers "some
// process has this number", never "this is the server" — and then signal the
// *negative* pid, so a developer whose machine had rebooted sent SIGTERM to the
// entire process group of whatever the kernel had handed 8-odd-thousand to
// next. This spawns a stranger, records it as the local server, and runs stop.
//
// The one test in this package that spawns anything, because the bug is about
// what a signal reaches and nothing short of a real process proves that.
func TestStopDoesNotSignalAPidItDoesNotOwn(t *testing.T) {
	home := localTestHome(t)

	stranger := exec.Command("sleep", "30")
	// Its own process group, like the server start puts the server in: this is
	// the group stop used to signal.
	stranger.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := stranger.Start(); err != nil {
		t.Fatalf("spawning a stranger: %v", err)
	}
	t.Cleanup(func() { _ = stranger.Process.Kill() })

	dir := filepath.Join(home, ".nav-pilot", "local")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A record with no recorded start time is exactly what a pre-reboot
	// nav-pilot left behind, and exactly the record that cannot be trusted.
	record := fmt.Sprintf(`{"pid":%d,"model":"mlx-community/x","port":8080,"started":"2026-01-01T00:00:00Z"}`,
		stranger.Process.Pid)
	if err := os.WriteFile(filepath.Join(dir, "server.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	captureStdout(func() {
		if err := cmdLocalStop(); err != nil {
			t.Errorf("cmdLocalStop() errored: %v", err)
		}
	})

	died := make(chan struct{})
	go func() { _ = stranger.Wait(); close(died) }()
	select {
	case <-died:
		t.Fatalf("`alpha local stop` killed pid %d, a process nav-pilot never started", stranger.Process.Pid)
	case <-time.After(time.Second):
	}
}

// TestInitOnlyAsksWhereSomethingCanAnswer is finding 3: `alpha local init`
// exited 0 having done nothing at all.
//
// The confirmation was gated on isInteractive, which reads os.ModeCharDevice —
// and /dev/null is a character device. So on a dispatched run stdin looked like
// a terminal, huh put the question to /dev/null, the read errored, and init
// printed "Cancelled. Nothing was downloaded." and returned nil. A script got a
// successful exit for a machine with no environment on it.
//
// Two halves, and the bug needed both: ask only where an answer can come from,
// and report a refusal as a failure.
func TestInitOnlyAsksWhereSomethingCanAnswer(t *testing.T) {
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()

	fi, err := devnull.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		t.Skip("this platform does not call /dev/null a character device, so the trap does not exist here")
	}
	if providerpkg.IsTerminal(devnull) {
		t.Error("IsTerminal(/dev/null) = true — init would put its question to something that cannot answer")
	}

	// The other half: saying no is a failure, not a quiet success.
	if err := confirmDownload(26, func() (bool, error) { return false, nil }); err == nil {
		t.Error("a cancelled download reported success; a script would carry on as though the environment were provisioned")
	}
	if err := confirmDownload(26, func() (bool, error) { return false, errors.New("no terminal") }); err == nil {
		t.Error("a confirmation that could not be shown reported success")
	}
	if err := confirmDownload(26, func() (bool, error) { return true, nil }); err != nil {
		t.Errorf("confirmDownload after a yes = %v, want nil", err)
	}
}

func TestWrapIndent(t *testing.T) {
	got := wrapIndent("one two three four five", "  ", 12)
	want := "one two\n  three four\n  five"
	if got != want {
		t.Errorf("wrapIndent()\n got: %q\nwant: %q", got, want)
	}
}
