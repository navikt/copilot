package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// modelsFixture is a cached manifest with a default, a second entry and one
// this version withholds, plus the second entry's weights in a fake Hugging
// Face cache. Nothing here downloads or starts anything.
func modelsFixture(t *testing.T) string {
	t.Helper()
	home := localTestHome(t)
	t.Setenv("HF_HOME", filepath.Join(home, "hf"))
	t.Cleanup(func() { local.SetSelectedModel(""); agentpakke.SetVersion("dev") })
	agentpakke.SetVersion("2026.09.20-080000-1111111")
	manifest := `{"schema_version":1,"channel":"alpha","models":[
		{"key":"qwen3.6-35b","name":"Qwen 3.6 35B A3B","model":"mlx-community/Qwen3.6-35B","backend":"mlx-lm","default":true,"weights_gb":25,"params":{"MLX_OPENCODE_CONTEXT":"65536"},"recommended_for":["decide-untrusted-evidence"]},
		{"key":"qwen3.8-27b","name":"Qwen 3.8 27B OptiQ","model":"mlx-community/Qwen3.8-27B","backend":"mlx-lm","weights_gb":19,"params":{"MLX_OPENCODE_CONTEXT":"49152"},"recommended_for":["decide-nuanced","not-a-key"]},
		{"key":"qwen3.8-8bit","name":"Qwen 3.8 27B 8bit","model":"mlx-community/Qwen3.8-27B-8bit","backend":"mlx-lm","weights_gb":30,"params":{},"min_nav_pilot":"2026.09.24-110317-abc1234"},
		{"key":"qwen-big","name":"Qwen Big","model":"mlx-community/Qwen-Big","backend":"mlx-lm","weights_gb":40,"min_ram_gb":64,"params":{}},
		{"key":"broken","name":"Broken","model":"mlx-community/Broken","backend":"mlx-lm","params":{},"min_nav_pilot":123}]}`
	writeFile(t, filepath.Join(home, ".nav-pilot", "local-models.json"), manifest)
	snap := filepath.Join(home, "hf", "hub", "models--mlx-community--Qwen3.8-27B", "snapshots", "abc")
	writeFile(t, filepath.Join(snap, "config.json"), "{}")
	writeFile(t, filepath.Join(snap, "model.safetensors"), "x")
	return home
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func configuredLocalModel(t *testing.T) string {
	t.Helper()
	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if cfg == nil || cfg.LocalModel == nil {
		return ""
	}
	return *cfg.LocalModel
}

// tableRow returns the line of the models table for a key.
func tableRow(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, " "+key+" ") {
			return line
		}
	}
	return ""
}

func TestLocalModelsTable(t *testing.T) {
	modelsFixture(t)
	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B"); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(func() {
		if err := cmdLocalModels(); err != nil {
			t.Errorf("cmdLocalModels: %v", err)
		}
	})
	for _, want := range []string{"KEY", "NAME", "SIZE", "CONTEXT", "RECOMMENDED", "STATUS", "nav-pilot alpha local use <key>"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	checks := map[string][]string{
		"qwen3.6-35b":  {"25 GB", "64k", "untrusted decide", "default, not downloaded"},
		"qwen3.8-27b":  {"*", "19 GB", "48k", "nuanced decide  ", "downloaded"},
		"qwen3.8-8bit": {"30 GB", "withheld: needs nav-pilot ≥ 2026.09.24-110317-abc1234"},
		"broken":       {"withheld: unreadable min_nav_pilot"},
	}
	for key, wants := range checks {
		row := tableRow(out, key)
		for _, want := range wants {
			if !strings.Contains(row, want) {
				t.Errorf("row %s lacks %q: %q", key, want, row)
			}
		}
	}
	if strings.Contains(out, "not-a-key") {
		t.Errorf("an unknown recommended_for key reached the table:\n%s", out)
	}
	if strings.Contains(tableRow(out, "qwen3.6-35b"), "*") {
		t.Errorf("the default is marked active while local_model names another: %q", tableRow(out, "qwen3.6-35b"))
	}
}

func TestLocalUse(t *testing.T) {
	modelsFixture(t)

	out := captureStdout(func() {
		if err := cmdLocalUse([]string{"qwen3.8-27b"}); err != nil {
			t.Errorf("use by key: %v", err)
		}
	})
	if got := configuredLocalModel(t); got != "mlx-community/Qwen3.8-27B" {
		t.Errorf("use by key wrote local_model = %q", got)
	}
	if !strings.Contains(out, "Downloaded") {
		t.Errorf("use of a downloaded model does not say so:\n%s", out)
	}

	// By id, and the default is written too so the choice is explicit.
	out = captureStdout(func() {
		if err := cmdLocalUse([]string{"mlx-community/Qwen3.6-35B"}); err != nil {
			t.Errorf("use by id: %v", err)
		}
	})
	if got := configuredLocalModel(t); got != "mlx-community/Qwen3.6-35B" {
		t.Errorf("use by id wrote local_model = %q", got)
	}
	if !strings.Contains(out, "about 25 GB") || !strings.Contains(out, "alpha local init") {
		t.Errorf("use of a model not downloaded does not give the size and init:\n%s", out)
	}

	err := cmdLocalUse([]string{"qwen3.8-27"})
	if err == nil || !strings.Contains(err.Error(), "qwen3.8-27b") {
		t.Errorf("use of a near miss = %v, want a suggestion", err)
	}
	err = cmdLocalUse([]string{"qwen3.8-8bit"})
	if err == nil || !strings.Contains(err.Error(), "needs nav-pilot ≥ 2026.09.24-110317-abc1234") {
		t.Errorf("use of a withheld model = %v, want its reason", err)
	}
	if got := configuredLocalModel(t); got != "mlx-community/Qwen3.6-35B" {
		t.Errorf("a refused use changed local_model to %q", got)
	}

	// No argument: the table and the usage, and nothing written.
	out = captureStdout(func() { _ = cmdLocalUse(nil) })
	if !strings.Contains(out, "STATUS") || !strings.Contains(out, "use <key|model-id>") {
		t.Errorf("use with no argument does not print the table and usage:\n%s", out)
	}
}

// TestLocalUseHintsRestartWithoutATerminal: a server on another model is not
// restarted behind the developer's back; without a terminal to ask on, the
// command to do it is printed. The recorded server is this test process, so it
// reads as live and nothing is signalled.
func TestLocalUseHintsRestartWithoutATerminal(t *testing.T) {
	modelsFixture(t)
	if err := local.SaveState(local.State{PID: os.Getpid(), Model: "mlx-community/Qwen3.6-35B", Port: 1}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(func() {
		if err := cmdLocalUse([]string{"qwen3.8-27b"}); err != nil {
			t.Errorf("use: %v", err)
		}
	})
	for _, want := range []string{"still serves", "Load it:", "nav-pilot alpha local restart"} {
		if !strings.Contains(out, want) {
			t.Errorf("use lacks %q:\n%s", want, out)
		}
	}
	if _, running, _ := local.LoadState(); !running {
		t.Error("use touched the recorded server")
	}
}

// TestLocalStatusShowsTheConfiguredModel: status names the model a start would
// load and where the choice came from, stopped or running, and says when the
// running server or a passed-over local_model disagrees with it.
func TestLocalStatusShowsTheConfiguredModel(t *testing.T) {
	modelsFixture(t)
	status := func() string {
		return captureStdout(func() {
			captureStderr(func() {
				if err := cmdLocalStatus(); err != nil {
					t.Errorf("cmdLocalStatus: %v", err)
				}
			})
		})
	}

	if out := status(); !strings.Contains(out, "qwen3.6-35b") || !strings.Contains(out, "Qwen 3.6 35B A3B, manifest default") {
		t.Errorf("status with nothing configured:\n%s", out)
	}

	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B-8bit"); err != nil {
		t.Fatal(err)
	}
	if out := status(); !strings.Contains(out, "needs a newer nav-pilot") || !strings.Contains(out, "manifest default") {
		t.Errorf("status with a withheld local_model does not say it fell back:\n%s", out)
	}

	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B"); err != nil {
		t.Fatal(err)
	}
	if err := local.SaveState(local.State{PID: os.Getpid(), Model: "mlx-community/Qwen3.6-35B", Port: 1}); err != nil {
		t.Fatal(err)
	}
	out := status()
	for _, want := range []string{"qwen3.8-27b", "Qwen 3.8 27B OptiQ, set via local_model", "Serving", "Not the configured model", "nav-pilot alpha local restart"} {
		if !strings.Contains(out, want) {
			t.Errorf("status of a server on another model lacks %q:\n%s", want, out)
		}
	}
}

// atTerminal makes nudges visible, as they are to a developer at a terminal.
func atTerminal(t *testing.T) {
	t.Helper()
	orig := stderrIsTerminal
	stderrIsTerminal = func() bool { return true }
	t.Cleanup(func() { stderrIsTerminal = orig })
}

func statusStderr(t *testing.T) string {
	t.Helper()
	var errOut string
	captureStdout(func() {
		errOut = stripANSI(captureStderr(func() {
			if err := cmdLocalStatus(); err != nil {
				t.Errorf("cmdLocalStatus: %v", err)
			}
		}))
	})
	// Joined back into one line: nudges wrap at the terminal width.
	return strings.Join(strings.Fields(errOut), " ")
}

const pinnedAdvice = "For decide on evidence you don't control, the default qwen3.6-35b is recommended; you use qwen3.8-27b."

// TestPinnedAdvisory: a developer pinned to a non-default model is told once
// what the default is recommended for. Never by `alpha decide`, which runs in
// hooks and scripts, never when nobody is at the terminal, and not right after
// they chose the model themselves with `use`.
func TestPinnedAdvisory(t *testing.T) {
	fakeDecideServer(t, yesMostly)
	modelsFixture(t)
	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B"); err != nil {
		t.Fatal(err)
	}

	// Piped: nothing shown and nothing marked, so the terminal still gets it.
	if out := statusStderr(t); strings.Contains(out, "you use") {
		t.Errorf("status printed the advisory into a pipe:\n%s", out)
	}

	atTerminal(t)
	if _, errOut, _ := runDecide(t, "Is it?", "--options", "yes,no", "--evidence", "-"); strings.Contains(errOut, "you use") {
		t.Errorf("decide printed the advisory: %q", errOut)
	}
	out := statusStderr(t)
	if !strings.Contains(out, pinnedAdvice) || !strings.Contains(out, "nav-pilot alpha local use qwen3.6-35b") {
		t.Errorf("first status at a terminal lacks the advisory:\n%s", out)
	}
	if out := statusStderr(t); strings.Contains(out, "you use") {
		t.Errorf("second status repeated the advisory:\n%s", out)
	}
}

func TestPinnedAdvisoryNotAfterUse(t *testing.T) {
	modelsFixture(t)
	atTerminal(t)
	captureStdout(func() {
		if err := cmdLocalUse([]string{"qwen3.8-27b"}); err != nil {
			t.Errorf("use: %v", err)
		}
	})
	if out := statusStderr(t); strings.Contains(out, "you use") {
		t.Errorf("status right after a deliberate use advised switching back:\n%s", out)
	}
}

// replacedFixture is modelsFixture's cache plus a replaced map pointing the
// plain 4-bit at qwen3.8-27b, and the plain 4-bit's weights on disk. The
// replacement's weights are on disk too unless removed.
func replacedFixture(t *testing.T) (home string) {
	t.Helper()
	home = modelsFixture(t)
	path := filepath.Join(home, ".nav-pilot", "local-models.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withReplaced := strings.TrimSuffix(strings.TrimSpace(string(data)), "}") +
		`,"replaced":{"mlx-community/Qwen3.8-27B-4bit":"qwen3.8-27b"}}`
	writeFile(t, path, withReplaced)
	snap := filepath.Join(home, "hf", "hub", "models--mlx-community--Qwen3.8-27B-4bit", "snapshots", "old")
	writeFile(t, filepath.Join(snap, "config.json"), "{}")
	writeFile(t, filepath.Join(snap, "model.safetensors"), "x")
	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B-4bit"); err != nil {
		t.Fatal(err)
	}
	return home
}

// TestReplacedLocalModel: a local_model the manifest lists as replaced keeps
// running while only its weights are here, moves to the replacement once that
// is downloaded, says which once, and never rewrites the config.
func TestReplacedLocalModel(t *testing.T) {
	home := replacedFixture(t)
	atTerminal(t)
	m, err := cachedManifest()
	if err != nil {
		t.Fatal(err)
	}

	// Replacement downloaded (the fixture has it): use it.
	var got local.Model
	out := stripANSI(captureStderr(func() { got, err = localModel(m) }))
	if err != nil || got.Model != "mlx-community/Qwen3.8-27B" {
		t.Errorf("localModel = %q/%v, want the replacement", got.Model, err)
	}
	want := "mlx-community/Qwen3.8-27B-4bit was replaced by qwen3.8-27b; nav-pilot uses qwen3.8-27b now. To pin it: nav-pilot alpha local use qwen3.8-27b"
	if !strings.Contains(strings.Join(strings.Fields(out), " "), want) || strings.Contains(out, "Using the default") {
		t.Errorf("stderr = %q, want %q", out, want)
	}
	if out := stripANSI(captureStderr(func() { _, _ = localModel(m) })); out != "" {
		t.Errorf("second run repeated the notice: %q", out)
	}

	// Replacement not downloaded: keep serving the old pin, not a download.
	if err := os.RemoveAll(filepath.Join(home, "hf", "hub", "models--mlx-community--Qwen3.8-27B")); err != nil {
		t.Fatal(err)
	}
	out = stripANSI(captureStderr(func() { got, err = localModel(m) }))
	if err != nil || got.Model != "mlx-community/Qwen3.8-27B-4bit" {
		t.Errorf("localModel = %q/%v, want the old pin kept", got.Model, err)
	}
	want = "mlx-community/Qwen3.8-27B-4bit is replaced by qwen3.8-27b (19 GB, not downloaded). Still using mlx-community/Qwen3.8-27B-4bit. Switch when ready: nav-pilot alpha local use qwen3.8-27b && nav-pilot alpha local init"
	if !strings.Contains(strings.Join(strings.Fields(out), " "), want) {
		t.Errorf("stderr = %q, want %q", out, want)
	}
	table := captureStdout(func() { printLocalModels(m) })
	if row := tableRow(table, "mlx-community/Qwen3.8-27B-4bit"); !strings.Contains(row, "*") || !strings.Contains(row, "replaced by qwen3.8-27b") {
		t.Errorf("the kept pin has no marked row:\n%s", table)
	}

	if got := configuredLocalModel(t); got != "mlx-community/Qwen3.8-27B-4bit" {
		t.Errorf("local_model was rewritten to %q", got)
	}

	// use of the old id points at the replacement instead of a near miss.
	err = cmdLocalUse([]string{"mlx-community/Qwen3.8-27B-4bit"})
	if err == nil || !strings.Contains(stripANSI(err.Error()), "was replaced by qwen3.8-27b. Use: nav-pilot alpha local use qwen3.8-27b") {
		t.Errorf("use of a replaced id = %v, want the replacement named", err)
	}

	// purge offers the old weights that are actually on disk.
	purge := captureStdout(func() { _ = cmdLocalPurge(nil) })
	if !strings.Contains(purge, "models--mlx-community--Qwen3.8-27B-4bit") {
		t.Errorf("purge does not list the old pinned weights:\n%s", purge)
	}
}

// TestDroppedPinNamesTheFix: a local_model the manifest no longer offers
// warns on every run, so the warning says how to make it stop.
func TestDroppedPinNamesTheFix(t *testing.T) {
	modelsFixture(t)
	if _, err := writeConfigKey("local_model", "mlx-community/Gone"); err != nil {
		t.Fatal(err)
	}
	m, _ := cachedManifest()
	out := stripANSI(captureStderr(func() { _, _ = localModel(m) }))
	if !strings.Contains(out, "Pick one to silence this: nav-pilot alpha local use <key>") {
		t.Errorf("stderr = %q, want the fix named", out)
	}
}

// fakeMachine stands in for the sysctl reads behind checkWiredLimit: a machine
// with ramGB of memory whose wired-memory limit is, or is not, high enough.
// A model with a min_ram_gb above ramGB is refused, as CheckWiredLimit does.
func fakeMachine(t *testing.T, ramGB int, sufficient bool) {
	t.Helper()
	orig := checkWiredLimit
	t.Cleanup(func() { checkWiredLimit = orig })
	checkWiredLimit = func(m local.Model) (local.WiredLimit, error) {
		w := local.WiredLimit{RequiredGB: m.WiredLimitGB, MachineRAMGB: ramGB, Sufficient: sufficient,
			DefaultGB: ramGB * 3 / 4, Command: "sudo sysctl -w iogpu.wired_limit_mb=36864"}
		if m.MinRAMGB > ramGB {
			return w, errors.New("needs a bigger machine")
		}
		return w, nil
	}
}

// TestLocalUseRefusesAModelThisMachineCannotHold: the table said "needs 64 GB
// RAM" and use answered with a ✓, leaving init and start to refuse later.
func TestLocalUseRefusesAModelThisMachineCannotHold(t *testing.T) {
	modelsFixture(t)
	fakeMachine(t, 48, true)

	err := cmdLocalUse([]string{"qwen-big"})
	if err == nil {
		t.Fatal("use accepted a model that needs 64 GB on a 48 GB machine")
	}
	for _, want := range []string{"qwen-big needs 64 GB RAM; this machine has 48 GB", "nav-pilot alpha local models"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}
	if got := configuredLocalModel(t); got != "" {
		t.Errorf("a refused use wrote local_model = %q", got)
	}
	if code := exitCodeFor(err); code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}
