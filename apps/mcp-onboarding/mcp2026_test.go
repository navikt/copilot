package main

import "testing"

func TestMCP2026Compliance(t *testing.T) {
	runMCP2026Compliance(t, mcpComplianceTarget{handler: newSimpleMCPHandler(), tool: "hello_world", checkGET: true})
}
