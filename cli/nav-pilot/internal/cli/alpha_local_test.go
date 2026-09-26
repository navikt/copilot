package cli

import (
	"context"
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

	"github.com/BurntSushi/toml"
	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
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
	// #830: init and start resolve the manifest, and that resolution fetches
	// it from raw.githubusercontent.com. local.Cached is the same resolution
	// without the fetch — cache first, embedded copy otherwise — so these
	// tests read a real manifest and still stay off the network.
	orig := resolveLocalManifest
	t.Cleanup(func() { resolveLocalManifest = orig })
	resolveLocalManifest = local.Cached
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
		// Deliberately a word nobody will implement. "restart" used to stand
		// here, and the day it became a real subcommand this test stopped and
		// started the server on whatever machine ran it.
		{name: "an unknown subcommand", args: []string{"local", "frobnicate"}, wantErr: "unknown command"},
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
	policy := filepath.Join(dir, "nav-pilot-local-dispatch.md")
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

// markProvisioned writes the environment stamp `local.Installed` looks for, so
// a test can reach the checks that come after it without provisioning anything.
// The pins are duplicated from internal/local deliberately: they are unexported,
// and a test that silently stopped exercising the code below the Installed check
// would be worse than one that fails loudly when the pin moves.
func markProvisioned(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".nav-pilot", "local")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := `{"mlx_lm":"0.31.3","mlx":"0.32.0"}`
	if err := os.WriteFile(filepath.Join(dir, "env.json"), []byte(stamp), 0o644); err != nil {
		t.Fatal(err)
	}
	if !local.Installed() {
		t.Fatalf("markProvisioned did not satisfy local.Installed(); the version pins in "+
			"internal/local/runtime.go have moved and this helper still writes %s", stamp)
	}
}

// TestLocalStartRefusesMissingWeights: start must not reach mlx-lm without the
// weights on disk. mlx-lm downloads whatever it cannot find, so this guard is
// the only thing between `start` and a silent 23 GB fetch inside readyTimeout,
// which surfaces either as a start that looks pathologically slow or as a
// timeout naming neither cause. Autostart has always refused it; start's own
// comment claimed it did too, and did not.
func TestLocalStartRefusesMissingWeights(t *testing.T) {
	localTestHome(t)
	markProvisioned(t)
	err := cmdLocalStart()
	if err == nil || !strings.Contains(err.Error(), "not on this machine") {
		t.Errorf("cmdLocalStart() without weights = %v, want an error naming the missing weights", err)
	}
}

// TestLocalModelKeySelectsTheServedModel pins the split between the two keys:
// `model` is the session model, `local_model` is what the local server loads.
// While they were one key, a developer running a cloud main agent with a local
// worker could not name a non-default local model at all — `model` held a cloud
// id, Chosen found no match, and start silently loaded the manifest default.
func TestLocalModelKeySelectsTheServedModel(t *testing.T) {
	localTestHome(t)
	t.Cleanup(func() { local.SetSelectedModel("") })

	m := &local.Manifest{Models: []local.Model{
		{Key: "default-one", Model: "org/Default", Default: true},
		{Key: "other", Model: "org/Other"},
	}}

	// Nothing configured: the manifest default.
	if got, err := localModel(m); err != nil || got.Model != "org/Default" {
		t.Errorf("with no local_model, localModel = %q/%v, want org/Default", got.Model, err)
	}

	// A cloud session model must not steer the served model.
	if _, err := writeConfigKey("model", "claude-opus-4.8"); err != nil {
		t.Fatalf("writing model: %v", err)
	}
	if _, err := writeConfigKey("local_model", "org/Other"); err != nil {
		t.Fatalf("writing local_model: %v", err)
	}
	if got, err := localModel(m); err != nil || got.Model != "org/Other" {
		t.Errorf("with local_model = org/Other, localModel = %q/%v, want org/Other", got.Model, err)
	}

	// An id this manifest does not offer falls back to the default, and says so
	// — silence there is how someone spends an afternoon wondering why their
	// choice did nothing.
	if _, err := writeConfigKey("local_model", "org/NotOffered"); err != nil {
		t.Fatalf("writing local_model: %v", err)
	}
	var got local.Model
	out := captureStderr(func() {
		var err error
		if got, err = localModel(m); err != nil {
			t.Errorf("localModel: %v", err)
		}
	})
	if got.Model != "org/Default" {
		t.Errorf("with an unknown local_model, localModel = %q, want the default", got.Model)
	}
	if !strings.Contains(out, "org/NotOffered") || !strings.Contains(out, "org/Default") {
		t.Errorf("no advisory naming the fallback, got: %q", out)
	}
}

// TestLocalModelWithheldFallsBackWithTheReason: a local_model this binary is
// too old for takes the not-offered fallback, and start says why and how to
// update rather than only that the id is not offered.
func TestLocalModelWithheldFallsBackWithTheReason(t *testing.T) {
	localTestHome(t)
	t.Cleanup(func() { local.SetSelectedModel(""); agentpakke.SetVersion("dev") })
	agentpakke.SetVersion("2026.09.20-080000-1111111")
	m, err := local.Parse([]byte(`{"schema_version":1,"channel":"alpha","models":[
		{"key":"d","name":"Default","model":"mlx-community/Default","backend":"mlx-lm","default":true,"params":{}},
		{"key":"big","name":"Qwen 3.8 27B 8bit","model":"mlx-community/Big-8bit","backend":"mlx-lm","params":{},"min_nav_pilot":"2026.09.24-110317-abc1234"}]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := writeConfigKey("local_model", "mlx-community/Big-8bit"); err != nil {
		t.Fatalf("writing local_model: %v", err)
	}
	var got local.Model
	out := strings.Join(strings.Fields(captureStderr(func() {
		got, err = localModel(m)
	})), " ")
	if err != nil || got.Model != "mlx-community/Default" {
		t.Errorf("localModel = %q/%v, want the default", got.Model, err)
	}
	for _, want := range []string{"Qwen 3.8 27B 8bit needs nav-pilot ≥ 2026.09.24-110317-abc1234", "you have 2026.09.20-080000-1111111", "nav-pilot upgrade", "needs a newer nav-pilot", "mlx-community/Default"} {
		if !strings.Contains(out, want) {
			t.Errorf("stderr lacks %q, got: %q", want, out)
		}
	}
}

// TestLocalModelAdvisoryForASessionOnALocalModel: `model` set to a local id
// still means "run this session locally" — it is the only way to say that — but
// it is also what people set when they meant local_model.
func TestLocalModelAdvisoryForASessionOnALocalModel(t *testing.T) {
	localTestHome(t)
	m := &local.Manifest{Models: []local.Model{{Key: "d", Model: "org/Default", Default: true}}}
	local.SetActive(m)
	t.Cleanup(func() { local.SetActive(nil) })

	id := "org/Default"
	warnings := configAdvisories(&Config{Version: 1, Model: &id}, toml.MetaData{})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "local_model") {
		t.Errorf("configAdvisories = %v, want one naming local_model", warnings)
	}
}

// TestLocalOffLeavesLocalModelAlone: off leaves the weights on disk, and the
// key naming which weights to load belongs with them. Resetting it would make
// `off` then `on` quietly forget the developer's choice.
func TestLocalOffLeavesLocalModelAlone(t *testing.T) {
	localTestHome(t)
	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B-4bit"); err != nil {
		t.Fatalf("writing local_model: %v", err)
	}
	captureStderr(func() { captureStdout(func() { _ = cmdLocalOff() }) })
	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if cfg.LocalModel == nil || *cfg.LocalModel != "mlx-community/Qwen3.8-27B-4bit" {
		t.Errorf("local_model = %v after off, want it untouched", cfg.LocalModel)
	}
}

// TestLocalStatusReportsAWithheldCachedEntry: status with dispatch off still
// reads the cached manifest under the running version, so an entry this binary
// is too old for is known rather than hidden behind the embedded copy that
// was parsed while the version was still dev. It is reported only when
// local_model names it: it used to print on every status, start and init,
// unindented in the middle of the status table, whether anyone wanted it or not.
// models always lists it.
func TestLocalStatusReportsAWithheldCachedEntry(t *testing.T) {
	home := localTestHome(t)
	t.Cleanup(func() { agentpakke.SetVersion("dev"); local.SetSelectedModel("") })
	agentpakke.SetVersion("2026.09.20-080000-1111111")
	manifest := `{"schema_version":1,"channel":"alpha","models":[
		{"key":"d","name":"Default","model":"mlx-community/Default","backend":"mlx-lm","default":true,"params":{}},
		{"key":"big","name":"Qwen 3.8 27B 8bit","model":"mlx-community/Big-8bit","backend":"mlx-lm","params":{},"min_nav_pilot":"2026.09.24-110317-abc1234"}]}`
	path := filepath.Join(home, ".nav-pilot", "local-models.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	const reason = "Qwen 3.8 27B 8bit needs nav-pilot ≥ 2026.09.24-110317-abc1234"
	status := func() string {
		out, errOut := captureRun(t, func() {
			if err := cmdLocalStatus(); err != nil {
				t.Errorf("cmdLocalStatus() errored: %v", err)
			}
		})
		return strings.Join(strings.Fields(out+errOut), " ")
	}
	if out := status(); strings.Contains(out, "needs nav-pilot ≥") {
		t.Errorf("status names a withheld entry nobody picked: %q", out)
	}
	if out := strings.Join(strings.Fields(captureStdout(func() { _ = cmdLocalModels() })), " "); !strings.Contains(out, reason) {
		t.Errorf("models does not give the withheld entry's reason: %q", out)
	}
	if _, err := writeConfigKey("local_model", "mlx-community/Big-8bit"); err != nil {
		t.Fatal(err)
	}
	if out := status(); !strings.Contains(out, reason) || !strings.Contains(out, "nav-pilot upgrade") {
		t.Errorf("status with local_model on the withheld entry does not say why: %q", out)
	}
}

// TestStartRaisesTheWiredLimitOnlyWhenAsked: start used to refuse a low limit
// where init raises it. With a terminal it now asks and raises; without one,
// or on a no, it refuses with the command, and sudo never runs.
func TestStartRaisesTheWiredLimitOnlyWhenAsked(t *testing.T) {
	raised := 0
	orig := raiseWiredLimit
	t.Cleanup(func() { raiseWiredLimit = orig })
	raiseWiredLimit = func(context.Context, local.WiredLimit) error { raised++; return nil }

	model := local.Model{Name: "Qwen", Model: "mlx-community/Qwen"}
	wired := local.WiredLimit{RequiredGB: 36, DefaultGB: 36, MachineRAMGB: 48, Command: "sudo sysctl -w iogpu.wired_limit_mb=36864"}
	ctx := context.Background()

	err := raiseWiredForStart(ctx, model, &wired, nil)
	if err == nil || !strings.Contains(err.Error(), wired.Command) || !strings.Contains(err.Error(), "about 36 GB") {
		t.Errorf("without a terminal = %v, want the refusal naming the command and the default", err)
	}
	captureStdout(func() {
		err = raiseWiredForStart(ctx, model, &wired, func() (bool, error) { return false, nil })
	})
	if err == nil || raised != 0 {
		t.Errorf("after a no: err = %v, raised %d times; want a refusal and no sudo", err, raised)
	}
	out := captureStdout(func() {
		err = raiseWiredForStart(ctx, model, &wired, func() (bool, error) { return true, nil })
	})
	if err != nil || raised != 1 || !strings.Contains(out, "Raising the wired-memory limit to 36 GB") {
		t.Errorf("after a yes: err = %v, raised %d times, out %q; want one announced raise", err, raised, out)
	}
	if !wired.Sufficient || wired.Label() != "36 GB" {
		t.Errorf("after a raise the limit reads %q (sufficient %t), want the raised 36 GB", wired.Label(), wired.Sufficient)
	}
}

// The flag loop used to answer -h/--help with the top-level usage before the
// alpha dispatch ever ran, so `alpha --help` hid the alpha commands. Help that
// was asked for goes to stdout, so `| less` and `| grep` work on it; usage
// printed in place of a command stays on stderr.
func TestAlphaHelpFlagsPrintAlphaUsage(t *testing.T) {
	for _, args := range [][]string{
		{"alpha", "help"}, {"alpha", "--help"}, {"alpha", "-h"},
		{"alpha", "local", "--help"}, {"alpha", "local", "-h"}, {"alpha", "local", "help"},
	} {
		var err error
		out, errOut := captureRun(t, func() { err = run(args) })
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out, "nav-pilot alpha: features that are not supported yet") || errOut != "" {
			t.Errorf("%v: want the alpha usage on stdout only\nstdout:\n%s\nstderr:\n%s", args, out, errOut)
		}
	}
	for _, args := range [][]string{{"alpha"}, {"alpha", "local"}} {
		out, errOut := captureRun(t, func() { _ = run(args) })
		if out != "" || !strings.Contains(errOut, "features that are not supported yet") {
			t.Errorf("%v: want the usage on stderr, got stdout %q", args, out)
		}
	}
	for _, args := range [][]string{{"help"}, {"--help"}} {
		out, errOut := captureRun(t, func() { _ = run(args) })
		if !strings.Contains(out, "Nav's Copilot toolkit") || errOut != "" {
			t.Errorf("%v: want the usage on stdout only, got stderr %q", args, errOut)
		}
	}
	out, errOut, _ := runDecide(t, "--help")
	if !strings.Contains(out, "nav-pilot alpha decide") || strings.Contains(out, "features that are not supported yet") || errOut != "" {
		t.Errorf("alpha decide --help: want its own usage on stdout\nstdout:\n%s\nstderr:\n%s", out, errOut)
	}
}

// TestInitWithoutATerminalNeedsYes: a scripted init used to skip the question
// and start a 26 GB download, then a sudo, with nobody there to say no. It now
// refuses with exit 2 and says how to consent, having printed the plan, sudo
// included, and done nothing.
func TestInitWithoutATerminalNeedsYes(t *testing.T) {
	home := localTestHome(t)
	t.Setenv("HF_HOME", filepath.Join(home, "hf"))
	fakeMachine(t, 48, false)
	raised := 0
	orig := raiseWiredLimit
	t.Cleanup(func() { raiseWiredLimit = orig })
	raiseWiredLimit = func(context.Context, local.WiredLimit) error { raised++; return nil }
	devnull, derr := os.Open(os.DevNull)
	if derr != nil {
		t.Fatal(derr)
	}
	origStdin := os.Stdin
	os.Stdin = devnull
	t.Cleanup(func() { os.Stdin = origStdin; devnull.Close() })

	var err error
	out := captureStdout(func() { captureStderr(func() { err = cmdLocalInit(nil) }) })
	var ec *exitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("init without a terminal = %v, want exit 2", err)
	}
	for _, want := range []string{"init would download about", "raise the wired-memory limit with sudo", "Run it in a terminal, or pass --yes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}
	if !strings.Contains(out, "Needs sudo once to raise the wired-memory limit to") {
		t.Errorf("the plan does not mention sudo:\n%s", out)
	}
	if raised != 0 || local.Installed() || strings.Contains(out, "Provisioning") {
		t.Errorf("init did something before it had consent: raised %d, output:\n%s", raised, out)
	}
}

func TestInitConsentNamesWhatNeedsAYes(t *testing.T) {
	for _, tc := range []struct {
		gb     int
		enough bool
		want   string
	}{
		{26, true, "download about 26 GB"},
		{0, false, "raise the wired-memory limit with sudo"},
		{26, false, "download about 26 GB and raise the wired-memory limit with sudo"},
		{0, true, ""},
	} {
		if got := initConsent(tc.gb, local.WiredLimit{Sufficient: tc.enough}); got != tc.want {
			t.Errorf("initConsent(%d, %t) = %q, want %q", tc.gb, tc.enough, got, tc.want)
		}
	}
}

// --yes used to be refused by the top-level flag loop, so `purge --yes` could
// never delete and `init --yes` could not be passed.
func TestAlphaLocalTakesYes(t *testing.T) {
	localTestHome(t)
	var err error
	out := captureStdout(func() { err = run([]string{"alpha", "local", "purge", "--yes"}) })
	if err != nil {
		t.Fatalf("alpha local purge --yes = %v", err)
	}
	if !strings.Contains(out, "Nothing to remove") {
		t.Errorf("purge --yes on an empty machine printed:\n%s", out)
	}
	for _, args := range [][]string{{"list", "--yes"}, {"alpha", "local", "start", "--yes"}} {
		if err := run(args); err == nil || !strings.Contains(err.Error(), "unknown flag: --yes") {
			t.Errorf("%v = %v, want unknown flag", args, err)
		}
	}
}

// F6: status on a fresh machine said "Start it: ... start", which refuses an
// unprovisioned machine.
func TestLocalStatusPointsAFreshMachineAtInit(t *testing.T) {
	localTestHome(t)
	out := captureStdout(func() { _ = cmdLocalStatus() })
	if !strings.Contains(out, "Set it up: nav-pilot alpha local init") || strings.Contains(out, "Start it:") {
		t.Errorf("status on a fresh machine:\n%s", out)
	}
	markProvisioned(t)
	if out := captureStdout(func() { _ = cmdLocalStatus() }); !strings.Contains(out, "Start it: nav-pilot alpha local start") {
		t.Errorf("status on a provisioned machine:\n%s", out)
	}
}

// F11: off pointed at init to come back, and on used a stricter liveness
// check than off, so the two disagreed about a server that was up.
func TestLocalOnAndOffAgree(t *testing.T) {
	localTestHome(t)
	markProvisioned(t)
	if err := local.SaveState(local.State{PID: os.Getpid(), Model: "m", Port: 1}); err != nil {
		t.Fatal(err)
	}
	var off, on string
	captureStderr(func() {
		off = captureStdout(func() { _ = cmdLocalOff() })
		on = captureStdout(func() { _ = cmdLocalOn() })
	})
	if !strings.Contains(off, "nav-pilot alpha local on brings it back") {
		t.Errorf("off does not point at on:\n%s", off)
	}
	if !strings.Contains(off, "still running") || !strings.Contains(on, "still running") {
		t.Errorf("off and on disagree about the server:\noff: %s\non: %s", off, on)
	}
}
