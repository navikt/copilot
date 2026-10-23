package provider

import (
	"os"
	"path/filepath"
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
	// The developer's own opencode must not pick the major the tests run
	// against: a v1 install turned v2 expectations red. Tests that need
	// another version put their own fake first on PATH or seed versionCache.
	// The fake answers 2.1.3 to any arguments, not only --version.
	bin, err := os.MkdirTemp("", "nav-pilot-test-bin-")
	if err != nil {
		panic(err)
	}
	if err := testhome.WriteExec(filepath.Join(bin, "opencode"), "#!/bin/sh\necho 2.1.3\n"); err != nil {
		panic(err)
	}
	os.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	code := testhome.Run(m)
	os.RemoveAll(bin)
	os.Exit(code)
}
