package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/fleet-control/internal/auth"
)

const (
	gatewayID  = "11111111-1111-1111-1111-111111111111"
	registryID = "22222222-2222-2222-2222-222222222222"
)

// fakeTexas treats the token as the azp of an active app token, with special
// tokens for the other outcomes.
func fakeTexas(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Token string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		switch in.Token {
		case "texas-500":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"` + gatewayID + `"}`))
		case "texas-garbage":
			_, _ = w.Write([]byte("{not json"))
		case "texas-trailing":
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"` + gatewayID + `"}not-json`))
		case "texas-oversize":
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"` + gatewayID + `"}` + strings.Repeat(" ", 1<<20)))
		case "inactive":
			_, _ = w.Write([]byte(`{"active":false,"idtyp":"app","azp":"` + gatewayID + `"}`))
		case "app-with-ident":
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"44444444-4444-4444-4444-444444444444","NAVident":"A111111"}`))
		case "user-token":
			_, _ = w.Write([]byte(`{"active":true,"azp":"` + gatewayID + `","NAVident":"Z999999"}`))
		default:
			// "claims:<json>" returns exactly those claims, marked active.
			if raw, ok := strings.CutPrefix(in.Token, "claims:"); ok {
				claims := map[string]any{}
				_ = json.Unmarshal([]byte(raw), &claims)
				claims["active"] = true
				_ = json.NewEncoder(w).Encode(claims)
				return
			}
			// "user:<azp>:<NAVident>[:<preferred_username>]" is a person's token
			// obtained by <azp>. Without a fourth part preferred_username is the
			// NAVident, so test team lists can key on it; an empty fourth part
			// leaves the claim out.
			// A fifth part sets oid (empty: no oid); otherwise oid follows from the NAVident.
			if parts := strings.Split(in.Token, ":"); len(parts) >= 3 && len(parts) <= 5 && parts[0] == "user" {
				claims := map[string]any{"active": true, "azp": parts[1], "NAVident": parts[2], "preferred_username": parts[2], "tid": testTenant, "oid": testOid(parts[2])}
				if len(parts) == 5 {
					claims["oid"] = parts[4]
					if parts[4] == "" {
						delete(claims, "oid")
					}
				}
				if len(parts) >= 4 {
					claims["preferred_username"] = parts[3]
					if parts[3] == "" {
						delete(claims, "preferred_username")
					}
				}
				_ = json.NewEncoder(w).Encode(claims)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "idtyp": "app", "azp": in.Token})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testOid is a stable fake Entra oid per NAVident.
func testOid(ident string) string {
	if ident == "" {
		return ""
	}
	return fmt.Sprintf("00000000-0000-4000-8000-%06x%06s", ident[0], ident[1:])
}

const testTenant = "62366534-1ec3-4962-8869-9b5535279d0b"

// testAuthz builds the guard the way main does, from the Nais environment.
func testAuthz(t *testing.T) *auth.Authz {
	t.Helper()
	t.Setenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", fakeTexas(t).URL)
	t.Setenv("AZURE_APP_PRE_AUTHORIZED_APPS", "")
	t.Setenv("FLEET_GATEWAY_CLIENT_ID", gatewayID)
	t.Setenv("MCP_REGISTRY_CLIENT_ID", registryID)
	t.Setenv("AZURE_APP_TENANT_ID", testTenant)
	a, err := auth.NewAuthzFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// TestAuthzMatrix runs every endpoint against every caller. The handlers
// behind the guard return 503 (no database), so anything not 401/403 got
// through.
func TestAuthzMatrix(t *testing.T) {
	h := newServer().routes(testAuthz(t), "", nil, nil)
	const through = http.StatusServiceUnavailable
	callers := map[string]string{"gateway": gatewayID, "registry": registryID, "other": "33333333-3333-3333-3333-333333333333", "prefix": gatewayID[:8]}
	want := map[string]map[string]int{
		"/internal/v1/snapshots/routes":   {"gateway": through, "registry": 403, "other": 403, "prefix": 403},
		"/internal/v1/snapshots/registry": {"gateway": 403, "registry": through, "other": 403, "prefix": 403},
	}
	for path, row := range want {
		for name, code := range row {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+callers[name])
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != code {
				t.Errorf("%s as %s: got %d, want %d", path, name, rec.Code, code)
			}
		}
		for _, hdr := range []string{"", "Basic " + gatewayID, "Bearer ", "Bearer a b", "Bearer inactive", "Bearer user-token", "bearer" + strings.Repeat("x", 17000)} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if hdr != "" {
				req.Header.Set("Authorization", hdr)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "invalid_token") {
				t.Errorf("%s with %.20q: got %d, want 401", path, hdr, rec.Code)
			}
		}
	}
}
