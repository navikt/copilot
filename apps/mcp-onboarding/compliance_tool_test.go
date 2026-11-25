package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/mcp-onboarding/internal/store"
)

func TestMCPComplianceTool(t *testing.T) {
	tokens := store.NewTokenStore()
	tokens.SaveToken("test-token", &store.TokenData{UserLogin: "u", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)})
	mux := http.NewServeMux()
	registerMCPRoutes(mux, newSimpleMCPHandler(), NewAuthMiddleware(tokens, "https://mcp.example.com").Authenticate, nil)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Token in env, never argv. FAIL exits 1 and fails the test; WARN does not.
	cmd := exec.Command("go", "run", ".", "-url", ts.URL+"/mcp", "-tool", "hello_world") //nolint:gosec // test-only, args are test constants
	cmd.Dir = "../../cli/mcp-compliance"
	// Clean env: no inherited MCP_LEGACY_PROTOCOL_UNTIL or MCP_COMPLIANCE_*.
	goEnv := func(k string) string {
		o, _ := exec.Command("go", "env", k).Output() //nolint:gosec // test-only
		return strings.TrimSpace(string(o))
	}
	cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "GOCACHE=" + goEnv("GOCACHE"), "GOMODCACHE=" + goEnv("GOMODCACHE"), "GOTOOLCHAIN=local", "MCP_COMPLIANCE_TOKEN=test-token"}
	out, err := cmd.CombinedOutput()
	t.Logf("mcp-compliance:\n%s", out)
	if err != nil {
		t.Fatalf("mcp-compliance: %v", err)
	}
}
