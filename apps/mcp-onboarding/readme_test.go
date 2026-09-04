package main

import (
	"os"
	"strings"
	"testing"
)

// TestDocs_NoUnreachableProductionHostname pins the deployed hostname. The
// service has only ever had an intern.nav.no ingress (see .nais/prod-gcp.yaml);
// the public mcp-onboarding.nav.no 404s. Documenting the public host is what
// led GHSA-7hwf-488h-59x8 to overstate how reachable this server is, so the
// wrong name should not come back.
func TestDocs_NoUnreachableProductionHostname(t *testing.T) {
	for _, path := range []string{"README.md", "../../README.md"} {
		b, err := os.ReadFile(path) //nolint:gosec // both paths are literals above
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(b)
		// The bad host is the public one; the good hosts contain it as a
		// substring only after ".intern" / ".intern.dev", so strip those first.
		for _, deployed := range []string{"mcp-onboarding.intern.nav.no", "mcp-onboarding.intern.dev.nav.no"} {
			body = strings.ReplaceAll(body, deployed, "")
		}
		if strings.Contains(body, "mcp-onboarding.nav.no") {
			t.Errorf("%s: refers to mcp-onboarding.nav.no, which is not deployed; use mcp-onboarding.intern.nav.no", path)
		}
		if !strings.Contains(string(b), "mcp-onboarding.intern.nav.no") {
			t.Errorf("%s: expected the deployed hostname mcp-onboarding.intern.nav.no to appear", path)
		}
	}
}
