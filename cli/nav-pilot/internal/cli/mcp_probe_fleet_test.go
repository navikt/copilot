package cli

import (
	"slices"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// The probe passes the fleet-gateway flag the launch passes, so mcp list and
// enable do not report a block the session never meets (#2829).
func TestMCPProbeTargetCountsFleetGatewayDefault(t *testing.T) {
	isolatedConfig(t)
	t.Cleanup(providerpkg.StubCpltVersion("cplt 2026.11.23-195628-9ef3d81\n"))
	_, extra, ok := mcpProbeTarget("https://fleet-gateway.intern.nav.no/kodeverk/mcp", providerpkg.MCPHostState{})
	if want := []string{"--allow-private-domain", "fleet-gateway.intern.nav.no"}; !ok || !slices.Equal(extra, want) {
		t.Errorf("extra = %v, want %v", extra, want)
	}
	_, extra, _ = mcpProbeTarget("https://mcp.figma.com/mcp", providerpkg.MCPHostState{})
	if extra != nil {
		t.Errorf("extra for a public host = %v, want none", extra)
	}
}
