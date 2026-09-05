package source

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIsNavCopilotCheckout pins dev-mode auto-detection across the manifest's
// three states. The broken-manifest rows are the point: a manifest mid-edit
// must not silently hand the developer upstream HEAD (#670 review) — the
// module marker keeps navikt/copilot detected so the fail-closed attach can
// name the parse error, while other agentpakke repos stay un-auto-detected
// exactly as when their manifest is valid.
func TestIsNavCopilotCheckout(t *testing.T) {
	valid := `{"contractVersion":"1","name":"nav-pilot","description":"d","clients":{"copilot":{"primaryAgents":["nav-pilot"]}},"layout":{"agents":"agents","skills":"skills"}}`
	other := `{"contractVersion":"1","name":"grillmester","description":"d","clients":{"copilot":{"primaryAgents":["grillmester"]}},"layout":{"agents":"agents","skills":"skills"}}`

	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  bool
	}{
		{"valid manifest named nav-pilot", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, ".nav-pilot", "agentpakke.json"), valid)
		}, true},
		{"valid manifest, another pakke", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, ".nav-pilot", "agentpakke.json"), other)
		}, false},
		{"broken manifest with the nav-pilot module marker", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, ".nav-pilot", "agentpakke.json"), `{"name": "nav-pi`)
			writeFile(t, filepath.Join(root, "cli", "nav-pilot", "main.go"), "package main\n")
		}, true},
		{"broken manifest without the marker", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, ".nav-pilot", "agentpakke.json"), `{"name": "grill`)
		}, false},
		{"no manifest, collections dir (pre-collapse)", func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, "collections"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"nothing", func(t *testing.T, root string) {}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)
			if got := isNavCopilotCheckout(root); got != tt.want {
				t.Errorf("isNavCopilotCheckout = %v, want %v", got, tt.want)
			}
		})
	}
}
