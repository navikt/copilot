package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testTeamID    = "11111111-2222-3333-4444-555555555555"
	testAreaID    = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	testClusterID = "99999999-8888-7777-6666-555555555555"
)

func newTestServer(t *testing.T, texasHandler, upstreamHandler http.HandlerFunc) *server {
	t.Helper()
	texasTS := httptest.NewServer(texasHandler)
	t.Cleanup(texasTS.Close)

	upstreamURL := "http://team-catalog.invalid"
	if upstreamHandler != nil {
		ts := httptest.NewServer(upstreamHandler)
		t.Cleanup(ts.Close)
		upstreamURL = ts.URL
	}

	cfg := &config{
		Port:                  "8080",
		IntrospectionEndpoint: texasTS.URL + "/introspect",
		ExchangeEndpoint:      texasTS.URL + "/exchange",
		GatewayClientID:       "gw-client-123",
		TeamCatalogURL:        upstreamURL,
		TeamCatalogScope:      "api://dev-gcp.org.team-catalog-backend/.default",
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	return newServer(cfg)
}

func defaultTexasHandler(oboToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/introspect":
			writeJSON(w, http.StatusOK, map[string]any{
				"active":   true,
				"idtyp":    "user",
				"NAVident": "Z123456",
				"azp":      "gw-client-123",
			})
		case "/exchange":
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token": oboToken,
				"expires_in":   3600,
				"token_type":   "Bearer",
			})
		default:
			http.NotFound(w, r)
		}
	}
}

func callToolModern(t *testing.T, handler http.Handler, bearer, tool string, args any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	params := map[string]any{"name": tool}
	if args != nil {
		params["arguments"] = args
	}
	body := modernBody("tools/call", params)
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	for k, v := range modernHeaders("tools/call", tool) {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func extractToolText(t *testing.T, rpcOut map[string]any) (string, bool) {
	t.Helper()
	res, ok := rpcOut["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result in RPC response: %+v", rpcOut)
	}
	isErr, _ := res["isError"].(bool)
	content, ok := res["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content in tool result: %+v", res)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text, isErr
}

func TestHealthAndReady(t *testing.T) {
	srv := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("texas must not be called for probe endpoints")
	}, nil)
	handler := srv.routes()

	for _, path := range []string{"/health", "/ready"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d, want 200", path, rec.Code)
		}
	}

	incompleteCases := []struct {
		name   string
		mutate func(*config)
	}{
		{"missing IntrospectionEndpoint", func(c *config) { c.IntrospectionEndpoint = "" }},
		{"missing ExchangeEndpoint", func(c *config) { c.ExchangeEndpoint = "" }},
		{"missing GatewayClientID", func(c *config) { c.GatewayClientID = "" }},
		{"missing TeamCatalogScope", func(c *config) { c.TeamCatalogScope = "" }},
	}
	for _, tc := range incompleteCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("texas must not be called for probe endpoints")
			}, nil)
			tc.mutate(s.cfg)
			rec := httptest.NewRecorder()
			s.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("/ready returned %d, want 503", rec.Code)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	t.Setenv("TEAM_CATALOG_URL", "ftp://bad.example")
	if _, err := loadConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid TEAM_CATALOG_URL scheme")
	}

	t.Setenv("TEAM_CATALOG_URL", "http://team-catalog-backend.org")
	t.Setenv("NAIS_TOKEN_EXCHANGE_ENDPOINT", "not-a-url")
	if _, err := loadConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid NAIS_TOKEN_EXCHANGE_ENDPOINT")
	}

	t.Setenv("NAIS_TOKEN_EXCHANGE_ENDPOINT", "http://texas.nais/exchange")
	t.Setenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://texas.nais/introspect")
	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TeamCatalogURL != "http://team-catalog-backend.org" {
		t.Fatalf("TeamCatalogURL = %q", cfg.TeamCatalogURL)
	}
}

func TestExtractGatewayClientID(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		cluster string
		want    string
		wantErr bool
	}{
		{"matching cluster ok", `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-123"}]`, "dev-gcp", "gw-123", false},
		{"other cluster rejected", `[{"name":"prod-gcp:copilot:fleet-gateway","clientId":"gw-prod"}]`, "dev-gcp", "", false},
		{"other namespace rejected", `[{"name":"dev-gcp:other:fleet-gateway","clientId":"gw-other-ns"}]`, "dev-gcp", "", false},
		{"empty cluster rejected", `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-123"}]`, "", "", false},
		{"duplicates yield empty", `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-1"},{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-2"}]`, "dev-gcp", "", false},
		{"malformed json errors", `{not-json`, "dev-gcp", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractGatewayClientID(tt.raw, tt.cluster)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthValidation(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		claims     map[string]any
		gwID       string
		wantStatus int
	}{
		{"missing authorization header", "", nil, "", http.StatusUnauthorized},
		{"non-bearer authorization header", "Basic dXNlcjpwYXNz", nil, "", http.StatusUnauthorized},
		{"inactive token", "Bearer tok-inactive", map[string]any{"active": false}, "", http.StatusUnauthorized},
		{"app token idtyp=app rejected", "Bearer tok-app", map[string]any{"active": true, "idtyp": "app", "NAVident": "Z123456", "azp": "gw-client-123"}, "", http.StatusForbidden},
		{"app token access_as_application role rejected", "Bearer tok-role", map[string]any{"active": true, "roles": []string{"access_as_application"}, "NAVident": "Z123456", "azp": "gw-client-123"}, "", http.StatusForbidden},
		{"missing NAVident rejected", "Bearer tok-no-navident", map[string]any{"active": true, "idtyp": "user", "azp": "gw-client-123"}, "", http.StatusForbidden},
		{"wrong azp rejected", "Bearer tok-wrong-azp", map[string]any{"active": true, "idtyp": "user", "NAVident": "Z123456", "azp": "other-client"}, "", http.StatusForbidden},
		{"empty gateway client id fails closed", "Bearer tok-no-gw", map[string]any{"active": true, "idtyp": "user", "NAVident": "Z123456", "azp": "gw-client-123"}, "NONE", http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, tt.claims)
			}, nil)
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

func TestOBOExchangeAndHeaders(t *testing.T) {
	inboundToken := strings.Join([]string{"inbound", "user", "fixture"}, "-")
	exchangedToken := strings.Join([]string{"obo", "teamcat", "fixture"}, "-")

	var (
		mu             sync.Mutex
		gotExchangeReq map[string]string
		gotUpstreamHdr http.Header
	)

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/introspect":
			writeJSON(w, http.StatusOK, map[string]any{
				"active":   true,
				"idtyp":    "user",
				"NAVident": "Z123456",
				"azp":      "gw-client-123",
			})
		case "/exchange":
			var reqBody map[string]string
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			mu.Lock()
			gotExchangeReq = reqBody
			mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]any{"access_token": exchangedToken})
		}
	}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotUpstreamHdr = r.Header.Clone()
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"content": []map[string]any{}})
	})

	rec, out := callToolModern(t, srv.routes(), inboundToken, "list_teams", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), inboundToken) || strings.Contains(rec.Body.String(), exchangedToken) {
		t.Fatal("response leaked token")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotExchangeReq["identity_provider"] != "entra_id" ||
		gotExchangeReq["target"] != "api://dev-gcp.org.team-catalog-backend/.default" ||
		gotExchangeReq["user_token"] != inboundToken {
		t.Fatalf("unexpected OBO request: %+v", gotExchangeReq)
	}
	if gotUpstreamHdr.Get("Authorization") != "Bearer "+exchangedToken {
		t.Fatalf("upstream Authorization = %q, want Bearer %s", gotUpstreamHdr.Get("Authorization"), exchangedToken)
	}
	if gotUpstreamHdr.Get("Nav-Consumer-Id") != "mcp-team-catalog" {
		t.Fatalf("Nav-Consumer-Id = %q", gotUpstreamHdr.Get("Nav-Consumer-Id"))
	}
	if gotUpstreamHdr.Get("Nav-Call-Id") == "" {
		t.Fatal("expected non-empty Nav-Call-Id header")
	}
	if _, isErr := extractToolText(t, out); isErr {
		t.Fatalf("unexpected tool error: %+v", out)
	}
}

func TestInputValidationTable(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("team-catalog-backend must not be called on invalid inputs")
	})
	h := srv.routes()

	tests := []struct {
		name string
		tool string
		args any
	}{
		{"unknown tool", "delete_team", map[string]any{}},
		{"arguments array instead of object", "list_teams", []string{"a"}},
		{"unknown key", "list_teams", map[string]any{"unknown": "x"}},
		{"bad productAreaId UUID", "list_teams", map[string]any{"productAreaId": "not-a-uuid"}},
		{"bad clusterId UUID", "list_teams", map[string]any{"clusterId": "../secret"}},
		{"bad status value", "list_teams", map[string]any{"status": "DELETED"}},
		{"too many status values", "list_teams", map[string]any{"status": "ACTIVE,PLANNED,INACTIVE,ACTIVE"}},
		{"filter too long", "list_teams", map[string]any{"filter": strings.Repeat("a", maxFilterLen+1)}},
		{"limit zero", "list_teams", map[string]any{"limit": 0}},
		{"limit over 100", "list_teams", map[string]any{"limit": 101}},
		{"negative offset", "list_teams", map[string]any{"offset": -1}},
		{"search_teams missing query", "search_teams", map[string]any{}},
		{"search_teams query too short", "search_teams", map[string]any{"query": "ab"}},
		{"search_teams query whitespace only", "search_teams", map[string]any{"query": "   "}},
		{"search_teams query path traversal", "search_teams", map[string]any{"query": "../admin"}},
		{"search_teams limit over 50", "search_teams", map[string]any{"query": "copilot", "limit": 51}},
		{"get_team missing id", "get_team", map[string]any{}},
		{"get_team bad UUID", "get_team", map[string]any{"id": "1234"}},
		{"get_teams_by_nais_team missing naisTeam", "get_teams_by_nais_team", map[string]any{}},
		{"get_teams_by_nais_team path traversal", "get_teams_by_nais_team", map[string]any{"naisTeam": "../etc"}},
		{"get_product_area bad UUID", "get_product_area", map[string]any{"id": "not-uuid"}},
		{"get_cluster bad UUID", "get_cluster", map[string]any{"id": "not-uuid"}},
		{"get_member_memberships missing navIdent", "get_member_memberships", map[string]any{}},
		{"get_member_memberships bad navIdent", "get_member_memberships", map[string]any{"navIdent": "123456"}},
		{"get_member_memberships email rejected", "get_member_memberships", map[string]any{"navIdent": "a@nav.no"}},
		{"get_nais_team neither teamId nor query", "get_nais_team", map[string]any{}},
		{"get_nais_team both teamId and query", "get_nais_team", map[string]any{"teamId": "copilot", "query": "copilot"}},
		{"get_nais_team query too short", "get_nais_team", map[string]any{"query": "co"}},
		{"get_nais_team bad teamId", "get_nais_team", map[string]any{"teamId": "a/b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, out := callToolModern(t, h, "in", tt.tool, tt.args)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if got := errCode(out); got != mcpCodeInvalidParams {
				t.Fatalf("error code = %d, want %d; body=%s", got, mcpCodeInvalidParams, rec.Body.String())
			}
		})
	}
}

func TestUpstreamErrorDoesNotLeakBodyOrToken(t *testing.T) {
	secretBody := "SENSITIVE_STACKTRACE_AND_DB_DETAILS_XYZ"
	inboundToken := strings.Join([]string{"inbound", "fixture", "jwt"}, "-")
	oboToken := strings.Join([]string{"obo", "fixture", "jwt"}, "-")

	srv := newTestServer(t, defaultTexasHandler(oboToken), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(secretBody))
	})

	rec, out := callToolModern(t, srv.routes(), inboundToken, "list_teams", map[string]any{})
	txt, isErr := extractToolText(t, out)
	if !isErr || !strings.Contains(txt, "500") {
		t.Fatalf("expected isError with 500, got isErr=%v txt=%q", isErr, txt)
	}
	for _, secret := range []string{secretBody, inboundToken, oboToken} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("response leaked %q", secret)
		}
	}
}

func TestUpstreamLimitReader8MiB(t *testing.T) {
	if maxUpstreamBodyBytes != 8<<20 {
		t.Fatalf("maxUpstreamBodyBytes = %d, want %d", maxUpstreamBodyBytes, 8<<20)
	}
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"name":"`))
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxUpstreamBodyBytes+1024))
		_, _ = w.Write([]byte(`"}]}`))
	})

	_, out := callToolModern(t, srv.routes(), "in", "list_teams", map[string]any{})
	txt, isErr := extractToolText(t, out)
	if !isErr || !strings.Contains(txt, "exceeded") {
		t.Fatalf("expected exceeded error, got isErr=%v txt=%q", isErr, txt)
	}
}

func TestUpstreamConcurrencySemaphore(t *testing.T) {
	var (
		inFlight    atomic.Int32
		maxInFlight atomic.Int32
	)
	entered := make(chan struct{}, 10)
	release := make(chan struct{})

	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		cur := inFlight.Add(1)
		for {
			prev := maxInFlight.Load()
			if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		inFlight.Add(-1)
		writeJSON(w, http.StatusOK, map[string]any{"content": []map[string]any{}})
	})
	h := srv.routes()

	var wg sync.WaitGroup
	for i := range 6 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Use distinct filter query? No, upstream URL is identical so use member memberships (uncached)
			ident := fmt.Sprintf("Z%06d", idx)
			rec, out := callToolModern(t, h, "in", "get_member_memberships", map[string]any{"navIdent": ident})
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d", rec.Code)
				return
			}
			if _, isErr := extractToolText(t, out); isErr {
				t.Errorf("unexpected tool error: %+v", out)
			}
		}(i)
	}

	for range maxConcurrentUpstream {
		<-entered
	}
	time.Sleep(30 * time.Millisecond)
	if got := maxInFlight.Load(); got > maxConcurrentUpstream {
		close(release)
		wg.Wait()
		t.Fatalf("max in-flight = %d, want <= %d", got, maxConcurrentUpstream)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	b, _ := json.Marshal(modernBody("tools/call", map[string]any{"name": "get_member_memberships", "arguments": map[string]any{"navIdent": "Z999999"}}))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer in")
	for k, v := range modernHeaders("tools/call", "get_member_memberships") {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var cancelOut map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &cancelOut)
	if _, isErr := extractToolText(t, cancelOut); !isErr {
		close(release)
		wg.Wait()
		t.Fatalf("expected error result on cancelled ctx while waiting for semaphore: %+v", cancelOut)
	}

	close(release)
	wg.Wait()
}

func TestMCP2026Compliance(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/team" {
			writeJSON(w, http.StatusOK, map[string]any{"content": []map[string]any{}})
			return
		}
		http.NotFound(w, r)
	})
	runMCP2026Compliance(t, mcpComplianceTarget{
		checkGET: true,
		handler:  srv.routes(),
		tool:     "list_teams",
		header:   map[string]string{"Authorization": "Bearer test-token"},
	})
}
