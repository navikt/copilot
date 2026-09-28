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
	t.Cleanup(func() { openCodeLaunchConfig = map[string]any{} })
	commented := `{
  // my own settings
  "theme": "tokyonight",
  "instructions": ["/mine.md",],
}
`
	if err := os.WriteFile(ConfigPathOverride, []byte(commented), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureOpenCodeConfig(); err != nil {
		t.Fatalf("EnsureOpenCodeConfig on a commented file: %v", err)
	}
	if err := EnsureOpenCodeLocalPolicy(aLocalModel(t)); err != nil {
		t.Fatalf("EnsureOpenCodeLocalPolicy on a commented file: %v", err)
	}
	if got, _ := os.ReadFile(ConfigPathOverride); string(got) != commented {
		t.Fatalf("the commented file was rewritten:\n%s", got)
	}
	if openCodeLaunchConfig["share"] != "disabled" {
		t.Errorf("share default not passed to the session: %v", openCodeLaunchConfig)
	}
	instr, _ := openCodeLaunchConfig["instructions"].([]any)
	if len(instr) != 1 || instr[0] != localPolicyPath() {
		t.Errorf("instructions for the session = %v, want only the policy (OpenCode appends lists)", instr)
	}
	if _, ok := openCodeLaunchConfig["agent"]; !ok {
		t.Errorf("worker binding not passed to the session: %v", openCodeLaunchConfig)
	}
	env := withOpenCodeConfigContent(nil, openCodeLaunchConfig)
	if !strings.Contains(strings.Join(env, "\n"), `"share":"disabled"`) {
		t.Errorf("OPENCODE_CONFIG_CONTENT = %v", env)
	}

	// Removing something the commented file holds cannot go through the
	// session, so it says what to remove.
	withPolicy := strings.Replace(commented, `"/mine.md",`, `"/mine.md", "`+localPolicyPath()+`"`, 1)
	if err := os.WriteFile(ConfigPathOverride, []byte(withPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	err := RemoveOpenCodeLocalPolicy()
	if err == nil || !strings.Contains(err.Error(), "has comments") || !strings.Contains(err.Error(), localPolicyPath()) {
		t.Fatalf("RemoveOpenCodeLocalPolicy on a commented file = %v", err)
	}
	if got, _ := os.ReadFile(ConfigPathOverride); string(got) != withPolicy {
		t.Fatal("the commented file was rewritten on removal")
	}
}
