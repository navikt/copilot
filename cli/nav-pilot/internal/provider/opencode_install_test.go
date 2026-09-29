package provider

import (
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// The installer's pinned release has to stay inside the tested range when
// the range moves (#1345).
func TestOpenCodeInstallVersionInsideTestedRange(t *testing.T) {
	v, err := parseClientVersion("opencode", OpenCodeInstallVersion)
	if err != nil {
		t.Fatal(err)
	}
	rng, err := agentpakke.ParseVersionRange(OpenCodeTestedRange)
	if err != nil {
		t.Fatal(err)
	}
	if !rng.Contains(v) {
		t.Fatalf("OpenCodeInstallVersion %s is outside OpenCodeTestedRange %s", OpenCodeInstallVersion, OpenCodeTestedRange)
	}
}
