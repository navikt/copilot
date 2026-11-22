package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// runComplianceTool runs cli/mcp-compliance against url. Token goes in the
// environment, never argv. FAIL exits 1 and fails the test; WARN does not.
func runComplianceTool(t *testing.T, url, token string, tool string) {
	t.Helper()
	cmd := exec.Command("go", "run", ".", "-url", url, "-tool", tool) //nolint:gosec // test-only, args are test constants
	cmd.Dir = "../../cli/mcp-compliance"
	cmd.Env = complianceEnv("MCP_COMPLIANCE_TOKEN="+token, "MCP_COMPLIANCE_APP_TOKEN=app-token")
	out, err := cmd.CombinedOutput()
	t.Logf("mcp-compliance:\n%s", out)
	if err != nil {
		t.Fatalf("mcp-compliance: %v", err)
	}
}

// strictTexas is the test-only fake Texas: it accepts "test-token" as a user and "app-token" as an app.
func strictTexas(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/introspect" {
			var in struct {
				Token string `json:"token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in.Token == "app-token" {
				writeJSON(w, http.StatusOK, map[string]any{"active": true, "idtyp": "app", "azp": "gw-client-123"})
				return
			}
			if in.Token != "test-token" {
				writeJSON(w, http.StatusOK, map[string]any{"active": false})
				return
			}
		}
		next(w, r)
	}
}

func TestMCPComplianceTool(t *testing.T) {
	srv := newTestServer(t, strictTexas(defaultTexasHandler("obo-tok")), nil)
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	// shortcut: no -legacy; the default test config has no legacy window.
	runComplianceTool(t, ts.URL+"/mcp", "test-token", "list_teams")
}

func TestUnauthorizedChallenge(t *testing.T) {
	texas := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/introspect" {
			writeJSON(w, http.StatusOK, map[string]any{"active": false})
			return
		}
		http.NotFound(w, r)
	}
	h := newTestServer(t, texas, nil).routes()
	for name, c := range map[string]struct{ auth, want string }{
		"missing token": {"", `Bearer realm="mcp-team-catalog"`},
		"invalid token": {"Bearer bad", `Bearer error="invalid_token"`},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != c.want {
				t.Fatalf("WWW-Authenticate = %q, want %q", got, c.want)
			}
		})
	}
}

// complianceEnv is a clean environment: only what go needs, so a caller's
// MCP_LEGACY_PROTOCOL_UNTIL or MCP_COMPLIANCE_* never leaks into the run.
func complianceEnv(extra ...string) []string {
	env := []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "GOCACHE=" + goEnv("GOCACHE"), "GOMODCACHE=" + goEnv("GOMODCACHE"), "GOTOOLCHAIN=local"}
	return append(env, extra...)
}

func goEnv(k string) string {
	out, _ := exec.Command("go", "env", k).Output() //nolint:gosec // test-only
	return strings.TrimSpace(string(out))
}
