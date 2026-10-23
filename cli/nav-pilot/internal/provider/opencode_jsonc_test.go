package provider

import (
	"os"
	"strings"
	"testing"
)

// An opencode.json with comments is read as OpenCode reads it, and never
// rewritten: what nav-pilot would have added goes to this session's
// OPENCODE_CONFIG_CONTENT instead, and what it would remove is an error that
// names it (#1071).
func TestOpenCodeConfigWithComments(t *testing.T) {
	withOpenCodeConfig(t)
	withLocalEnabled(t)
	commented := `{
  // my own settings
  "theme": "tokyonight",
  "instructions": ["/mine.md",],
}
`
	if err := os.WriteFile(ConfigPathOverride, []byte(commented), 0o600); err != nil {
		t.Fatal(err)
	}

	launchConfig, err := EnsureOpenCodeConfig()
	if err != nil {
		t.Fatalf("EnsureOpenCodeConfig on a commented file: %v", err)
	}
	if err := ensureOpenCodeLocalPolicy(launchConfig, "", aLocalModel(t)); err != nil {
		t.Fatalf("ensureOpenCodeLocalPolicy on a commented file: %v", err)
	}
	if got, _ := os.ReadFile(ConfigPathOverride); string(got) != commented {
		t.Fatalf("the commented file was rewritten:\n%s", got)
	}
	if launchConfig["share"] != "disabled" {
		t.Errorf("share default not passed to the session: %v", launchConfig)
	}
	instr, _ := launchConfig["instructions"].([]any)
	if len(instr) != 1 || instr[0] != localPolicyPath() {
		t.Errorf("instructions for the session = %v, want only the policy (OpenCode appends lists)", instr)
	}
	if _, ok := launchConfig["agent"]; !ok {
		t.Errorf("worker binding not passed to the session: %v", launchConfig)
	}
	env := withOpenCodeConfigContent(nil, launchConfig)
	if !strings.Contains(strings.Join(env, "\n"), `"share":"disabled"`) {
		t.Errorf("OPENCODE_CONFIG_CONTENT = %v", env)
	}

	// A write that adds and removes still hands the session the addition.
	launchConfig = map[string]any{}
	stale := `{
  // mine
  "agent": {"local-worker": {"model": "mlx/old"}},
}
`
	if err := os.WriteFile(ConfigPathOverride, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	err = mutateOpenCodeConfig(launchConfig, func(cfg map[string]any) bool {
		delete(cfg, "agent")
		cfg["provider"] = map[string]any{"mlx": map[string]any{"options": map[string]any{"baseURL": "http://127.0.0.1:9/v1"}}}
		return true
	})
	if err == nil || !strings.Contains(err.Error(), "agent") {
		t.Fatalf("add+remove on a commented file = %v, want an error naming agent", err)
	}
	if _, ok := launchConfig["provider"]; !ok {
		t.Fatalf("the addition was lost with the removal: %v", launchConfig)
	}

	// Removing something the commented file holds cannot go through the
	// session, so it says what to remove.
	withPolicy := strings.Replace(commented, `"/mine.md",`, `"/mine.md", "`+localPolicyPath()+`"`, 1)
	if err := os.WriteFile(ConfigPathOverride, []byte(withPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	err = RemoveOpenCodeLocalPolicy()
	if err == nil || !strings.Contains(err.Error(), "has comments") || !strings.Contains(err.Error(), localPolicyPath()) {
		t.Fatalf("RemoveOpenCodeLocalPolicy on a commented file = %v", err)
	}
	if got, _ := os.ReadFile(ConfigPathOverride); string(got) != withPolicy {
		t.Fatal("the commented file was rewritten on removal")
	}
}
