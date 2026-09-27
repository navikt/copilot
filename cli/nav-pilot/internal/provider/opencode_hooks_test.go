package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWithOpenCodeConfigContent(t *testing.T) {
	env := withOpenCodeConfigContent([]string{`OPENCODE_CONFIG_CONTENT={"plugin":["mine"],"theme":"x"}`},
		map[string]any{"plugin": []any{"file:///p.js"}, "share": "disabled"})
	var cfg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(env[0], "OPENCODE_CONFIG_CONTENT=")), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg["plugin"].([]any); len(got) != 2 || got[0] != "mine" || got[1] != "file:///p.js" {
		t.Fatalf("plugin = %v", got)
	}
	if cfg["theme"] != "x" || cfg["share"] != "disabled" {
		t.Fatalf("cfg = %v", cfg)
	}
	env = withOpenCodeConfigContent([]string{"OPENCODE_CONFIG_CONTENT=not json"}, map[string]any{"share": "disabled"})
	if env[0] != `OPENCODE_CONFIG_CONTENT={"share":"disabled"}` {
		t.Fatalf("env = %v", env)
	}
}
