package cli

import (
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
		{"key":"qwen3.6-35b","name":"Qwen 3.6 35B A3B","model":"mlx-community/Qwen3.6-35B","backend":"mlx-lm","default":true,"weights_gb":25,"params":{"MLX_OPENCODE_CONTEXT":"65536"},"recommended_for":["default","decide-untrusted-evidence"]},
		{"key":"qwen3.8-27b","name":"Qwen 3.8 27B OptiQ","model":"mlx-community/Qwen3.8-27B","backend":"mlx-lm","weights_gb":19,"params":{"MLX_OPENCODE_CONTEXT":"49152"},"recommended_for":["decide-nuanced","not-a-key"]},
		{"key":"qwen3.8-8bit","name":"Qwen 3.8 27B 8bit","model":"mlx-community/Qwen3.8-27B-8bit","backend":"mlx-lm","weights_gb":30,"params":{},"min_nav_pilot":"2026.09.24-110317-abc1234"},
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
		"qwen3.6-35b":  {"25 GB", "64k", "general, decide: untrusted input", "default, not downloaded"},
		"qwen3.8-27b":  {"*", "19 GB", "48k", "decide: nuanced  ", "downloaded"},
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

// TestPinnedAdvisoryOnStatusNotOnDecide: a developer pinned to a non-default
// model is told once what the default is recommended for, by status, and
// `alpha decide`, which runs in hooks and scripts, never says it.
func TestPinnedAdvisoryOnStatusNotOnDecide(t *testing.T) {
	fakeDecideServer(t, yesMostly)
	home := modelsFixture(t)
	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B"); err != nil {
		t.Fatal(err)
	}
	const advice = "You use qwen3.8-27b. The default qwen3.6-35b is recommended for decide on evidence you don't control."

	_, errOut, _ := runDecide(t, "Is it?", "--options", "yes,no", "--evidence", "-")
	if strings.Contains(errOut, "You use") {
		t.Errorf("decide printed the advisory: %q", errOut)
	}
	if _, err := os.Stat(filepath.Join(home, ".nav-pilot", "local-models.json.advised")); err == nil {
		t.Error("decide marked the advisory as seen")
	}

	status := func() string {
		var out string
		captureStderr(func() {
			out = captureStdout(func() { _ = cmdLocalStatus() })
		})
		return out
	}
	if out := status(); !strings.Contains(out, advice) || !strings.Contains(out, "nav-pilot alpha local use qwen3.6-35b") {
		t.Errorf("first status lacks the advisory:\n%s", out)
	}
	if out := status(); strings.Contains(out, "You use") {
		t.Errorf("second status repeated the advisory:\n%s", out)
	}
}

// TestReplacedLocalModelResolvesToItsReplacement: a local_model the manifest
// removed and lists as replaced loads the replacement, says so once with the
// command that makes it explicit, and leaves the config alone.
func TestReplacedLocalModelResolvesToItsReplacement(t *testing.T) {
	localTestHome(t)
	t.Cleanup(func() { local.SetSelectedModel("") })
	m, err := local.Parse([]byte(`{"schema_version":1,"channel":"alpha","models":[
		{"key":"d","name":"Default","model":"mlx-community/Default","backend":"mlx-lm","default":true,"params":{}},
		{"key":"q38-optiq","name":"OptiQ","model":"mlx-community/Qwen3.8-27B-OptiQ-4bit","backend":"mlx-lm","params":{}}],
		"replaced":{"mlx-community/Qwen3.8-27B-4bit":"q38-optiq"}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := writeConfigKey("local_model", "mlx-community/Qwen3.8-27B-4bit"); err != nil {
		t.Fatal(err)
	}
	var got local.Model
	out := stripANSI(captureStderr(func() { got, err = localModel(m) }))
	if err != nil || got.Model != "mlx-community/Qwen3.8-27B-OptiQ-4bit" {
		t.Errorf("localModel = %q/%v, want the replacement", got.Model, err)
	}
	want := "mlx-community/Qwen3.8-27B-4bit was replaced by q38-optiq; run nav-pilot alpha local use q38-optiq to make it explicit."
	if !strings.Contains(out, want) || strings.Count(out, "was replaced by") != 1 {
		t.Errorf("stderr = %q, want once: %q", out, want)
	}
	if strings.Contains(out, "Using the default") {
		t.Errorf("stderr claims a fallback to the default: %q", out)
	}
	if got := configuredLocalModel(t); got != "mlx-community/Qwen3.8-27B-4bit" {
		t.Errorf("local_model was rewritten to %q", got)
	}
}
