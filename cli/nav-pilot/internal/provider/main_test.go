package provider

import (
	"os"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

func TestMain(m *testing.M) {
	// Release checks in the foreground: tests swap FetchLatestVersion.
	artifacts.RefreshInBackground = false
	// Fake clients are shell scripts, and under parallel load exec alone
	// has taken over 2 s (see cli.applyE2ESeams). No test relies on these
	// deadlines firing.
	IsCpltTimeout = 30 * time.Second
	MCPPolicyTimeout = 30 * time.Second
	os.Exit(testhome.Run(m))
}
