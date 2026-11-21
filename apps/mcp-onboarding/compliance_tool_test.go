package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestMCPComplianceTool(t *testing.T) {
	store := newTestStore()
	store.SaveToken("test-token", &TokenData{UserLogin: "u", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)})
	mux := http.NewServeMux()
	registerMCPRoutes(mux, newSimpleMCPHandler(), NewAuthMiddleware(store, "https://mcp.example.com").Authenticate, nil)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Token in env, never argv. FAIL exits 1 and fails the test; WARN does not.
	cmd := exec.Command("go", "run", ".", "-url", ts.URL+"/mcp", "-tool", "hello_world") //nolint:gosec // test-only, args are test constants
	cmd.Dir = "../../cli/mcp-compliance"
	cmd.Env = append(os.Environ(), "MCP_COMPLIANCE_TOKEN=test-token")
	out, err := cmd.CombinedOutput()
	t.Logf("mcp-compliance:\n%s", out)
	if err != nil {
		t.Fatalf("mcp-compliance: %v", err)
	}
}
