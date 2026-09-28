package cli

import (
	"os"
	"strings"
	"testing"
)

// After an install, the next step names the configured client: Copilot Chat
// only for copilot (#1194).
func TestUseAgentLineNamesTheConfiguredClient(t *testing.T) {
	path := isolatedConfig(t)
	if got := useAgentLine("nav-pilot"); !strings.Contains(got, "Copilot Chat") {
		t.Errorf("no config (copilot): %q", got)
	}
	if err := os.WriteFile(path, []byte("version = 1\nclient = \"opencode\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := useAgentLine("nav-pilot")
	if strings.Contains(got, "Copilot") || !strings.Contains(got, "OpenCode") {
		t.Errorf("client opencode: %q", got)
	}
}
