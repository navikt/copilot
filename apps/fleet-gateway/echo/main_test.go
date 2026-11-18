package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestServer(t *testing.T, texasHandler http.HandlerFunc) (*server, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(texasHandler)
	t.Cleanup(ts.Close)

	cfg := &config{
		Port:                  "8080",
		IntrospectionEndpoint: ts.URL,
		GatewayClientID:       "gw-client-123",
		HTTPClient:            &http.Client{Timeout: 5 * time.Second},
	}
	return &server{cfg: cfg}, ts
}

func TestHealthAndReady(t *testing.T) {
	srv, _ := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("texas must not be called for probe endpoints")
	})
	handler := srv.routes()

	for _, path := range []string{"/health", "/ready"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d, want 200", path, rec.Code)
		}
	}
}

func TestExtractGatewayClientID(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name: "valid fleet-gateway entry",
			raw:  `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-123"}]`,
			want: "gw-123",
		},
		{
			name: "missing fleet-gateway yields empty",
			raw:  `[{"name":"dev-gcp:copilot:other","clientId":"other-123"}]`,
			want: "",
		},
		{
			name: "ambiguous multiple matches fail closed",
			raw:  `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-1"},{"name":"prod-gcp:copilot:fleet-gateway","clientId":"gw-2"}]`,
			want: "",
		},
		{
			name:    "malformed json fails",
			raw:     `{not-json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractGatewayClientID(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEchoAuthValidation(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		claims     map[string]any
		gwID       string
		wantStatus int
	}{
		{
			name:       "missing authorization header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "non-bearer authorization header",
			authHeader: "Basic dXNlcjpwYXNz",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "inactive token",
			authHeader: "Bearer tok-inactive",
			claims:     map[string]any{"active": false},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "app token idtyp=app rejected",
			authHeader: "Bearer tok-app",
			claims:     map[string]any{"active": true, "idtyp": "app", "NAVident": "Z123456", "azp": "gw-client-123"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing NAVident rejected",
			authHeader: "Bearer tok-no-navident",
			claims:     map[string]any{"active": true, "idtyp": "user", "azp": "gw-client-123"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "wrong azp rejected",
			authHeader: "Bearer tok-wrong-azp",
			claims:     map[string]any{"active": true, "idtyp": "user", "NAVident": "Z123456", "azp": "other-client"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "empty gateway client id fails closed",
			authHeader: "Bearer tok-no-gw",
			claims:     map[string]any{"active": true, "idtyp": "user", "NAVident": "Z123456", "azp": "gw-client-123"},
			gwID:       "NONE",
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, tt.claims)
			})
			if tt.gwID == "NONE" {
				srv.cfg.GatewayClientID = ""
			}

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			srv.routes().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d (%s), want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
		})
	}
}

func TestEchoMCPEndpoints_ReturnsClaimsNeverTokens(t *testing.T) {
	t.Setenv(mcpLegacyUntilEnv, "2999-01-01") // initialize-era flow
	var (
		mu              sync.Mutex
		gotTexasPayload map[string]string
	)
	inboundToken := strings.Join([]string{"secret", "entra", "obo", "token"}, ".")

	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		_ = json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		gotTexasPayload = payload
		mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"active":             true,
			"idtyp":              "user",
			"NAVident":           "Z123456",
			"preferred_username": "ola.nordmann@nav.no",
			"azp":                "gw-client-123",
			"aud":                "api://dev-gcp.copilot.fleet-echo",
			"scp":                "access_as_user",
			"sub":                "sub-123",
		})
	})

	handler := srv.routes()

	// 1. initialize
	initReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	initReq.Header.Set("Authorization", "Bearer "+inboundToken)
	initRec := httptest.NewRecorder()
	handler.ServeHTTP(initRec, initReq)
	if initRec.Code != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", initRec.Code, initRec.Body.String())
	}
	if bytes.Contains(initRec.Body.Bytes(), []byte(inboundToken)) {
		t.Fatal("response leaked inbound token")
	}

	mu.Lock()
	if gotTexasPayload["identity_provider"] != "entra_id" || gotTexasPayload["token"] != inboundToken {
		t.Fatalf("unexpected texas payload: %+v", gotTexasPayload)
	}
	mu.Unlock()

	var initResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
			Claims echoClaims `json:"claims"`
		} `json:"result"`
	}
	if err := json.Unmarshal(initRec.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("decode initialize response: %v", err)
	}
	if initResp.JSONRPC != "2.0" || initResp.ID != 1 || initResp.Result.ServerInfo.Name != "fleet-echo" {
		t.Fatalf("unexpected initialize payload: %+v", initResp)
	}
	if initResp.Result.Claims.NAVident != "Z123456" || initResp.Result.Claims.Azp != "gw-client-123" {
		t.Fatalf("unexpected claims: %+v", initResp.Result.Claims)
	}

	// 2. notifications/initialized -> 202
	notifReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	notifReq.Header.Set("Authorization", "Bearer "+inboundToken)
	notifRec := httptest.NewRecorder()
	handler.ServeHTTP(notifRec, notifReq)
	if notifRec.Code != http.StatusAccepted {
		t.Fatalf("notification status=%d, want 202", notifRec.Code)
	}

	// 3. tools/call -> returns claims JSON content
	callReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":"c1","method":"tools/call","params":{"name":"whoami"}}`))
	callReq.Header.Set("Authorization", "Bearer "+inboundToken)
	callRec := httptest.NewRecorder()
	handler.ServeHTTP(callRec, callReq)
	if callRec.Code != http.StatusOK {
		t.Fatalf("tools/call status=%d body=%s", callRec.Code, callRec.Body.String())
	}
	if bytes.Contains(callRec.Body.Bytes(), []byte(inboundToken)) {
		t.Fatal("tools/call response leaked inbound token")
	}
	if !strings.Contains(callRec.Body.String(), "Z123456") {
		t.Fatalf("expected NAVident in tools/call response, got %s", callRec.Body.String())
	}
}

func TestMCP2026Compliance(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"active": true, "idtyp": "user", "NAVident": "Z123456", "azp": "gw-client-123", "aud": "api://dev-gcp.copilot.fleet-echo", "scp": "access_as_user", "sub": "sub-123"})
	})
	runMCP2026Compliance(t, mcpComplianceTarget{checkGET: true, handler: srv.routes(), tool: "whoami", header: map[string]string{"Authorization": "Bearer test-token"}})
}
