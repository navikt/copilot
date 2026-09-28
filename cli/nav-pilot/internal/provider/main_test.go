package provider

import (
	"os"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

func TestMain(m *testing.M) {
	// Release checks in the foreground: tests swap FetchLatestVersion.
	artifacts.RefreshInBackground = false
	os.Exit(testhome.Run(m))
}
