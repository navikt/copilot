package teamtoken

import "testing"

func TestEntraScopeForTarget(t *testing.T) {
	if got := entraScopeForTarget("dev-gcp:copilot:fleet-echo"); got != "api://dev-gcp.copilot.fleet-echo/.default" {
		t.Fatalf("entraScopeForTarget = %q, want %q", got, "api://dev-gcp.copilot.fleet-echo/.default")
	}
}
