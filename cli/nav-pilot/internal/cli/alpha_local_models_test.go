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
		{"key":"qwen3.6-35b","name":"Qwen 3.6 35B A3B","model":"mlx-community/Qwen3.6-35B","backend":"mlx-lm","default":true,"weights_gb":25,"params":{"MLX_OPENCODE_CONTEXT":"65536"}},
		{"key":"qwen3.8-27b","name":"Qwen 3.8 27B OptiQ","model":"mlx-community/Qwen3.8-27B","backend":"mlx-lm","weights_gb":19,"params":{"MLX_OPENCODE_CONTEXT":"49152"}},
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
	for _, want := range []string{"KEY", "NAME", "SIZE", "CONTEXT", "STATUS", "nav-pilot alpha local use <key>"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	checks := map[string][]string{
		"qwen3.6-35b":  {"25 GB", "64k", "default, not downloaded"},
		"qwen3.8-27b":  {"*", "19 GB", "48k", "downloaded"},
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
