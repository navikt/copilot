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
	rng, err := agentpakke.ParseVersionRange(OpenCode1TestedRange)
	if err != nil {
		t.Fatal(err)
	}
	if !rng.Contains(v) {
		t.Fatalf("OpenCodeInstallVersion %s is outside OpenCode1TestedRange %s", OpenCodeInstallVersion, OpenCode1TestedRange)
	}
}

// The opencode 2 pin in every install hint stays inside the tested range too.
func TestOpenCode2InstallVersionInsideTestedRange(t *testing.T) {
	v, err := parseClientVersion("opencode", OpenCode2InstallVersion)
	if err != nil {
		t.Fatal(err)
	}
	rng, err := agentpakke.ParseVersionRange(OpenCode2TestedRange)
	if err != nil {
		t.Fatal(err)
	}
	if !rng.Contains(v) {
		t.Fatalf("OpenCode2InstallVersion %s is outside OpenCode2TestedRange %s", OpenCode2InstallVersion, OpenCode2TestedRange)
	}
	// Control: the range does refuse a release past it.
	if past, _ := parseClientVersion("opencode", "2.2.0"); rng.Contains(past) {
		t.Fatalf("OpenCode2TestedRange %s contains 2.2.0", OpenCode2TestedRange)
	}
}
