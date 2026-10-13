package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// TestClientPageListsWarnedSettings keeps the settings table on the client
// page (/nav-pilot/klienter) honest: every setting opencode or pi warns about
// must have a row there.
func TestClientPageListsWarnedSettings(t *testing.T) {
	page, err := os.ReadFile("../../../../apps/my-copilot/src/app/(nb)/(nav-pilot)/nav-pilot/klienter/page.tsx")
	if err != nil {
		t.Fatalf("read client page: %v", err)
	}
	r := domain.ResolvedConfig{Mode: "autopilot", ReasoningEffort: "high", ContextTier: "large", AllowAllTools: true, AskUser: false, LogLevel: "debug"}
	warnings := append(OpenCodeUnsupportedConfigWarnings(r), PiUnsupportedConfigWarnings(r)...)
	if len(warnings) < 10 {
		t.Fatalf("expected a warning per unsupported setting, got %q", warnings)
	}
	for _, w := range warnings {
		key := strings.Fields(w)[0]
		if !strings.Contains(string(page), `key: "`+key+`"`) {
			t.Errorf("setting %q has a client warning but no row in SETTINGS on the client page", key)
		}
	}
}
