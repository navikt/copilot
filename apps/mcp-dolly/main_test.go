package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestServer(t *testing.T, texasHandler, dollyHandler http.HandlerFunc) *server {
	t.Helper()
	texasTS := httptest.NewServer(texasHandler)
	t.Cleanup(texasTS.Close)

	var dollyURL string
	if dollyHandler != nil {
		dollyTS := httptest.NewServer(dollyHandler)
		t.Cleanup(dollyTS.Close)
		dollyURL = dollyTS.URL
	} else {
		dollyURL = "http://dolly.invalid"
	}

	cfg := &config{
		Port:                  "8080",
		IntrospectionEndpoint: texasTS.URL + "/introspect",
		ExchangeEndpoint:      texasTS.URL + "/exchange",
		GatewayClientID:       "gw-client-123",
		DollyURL:              dollyURL,
		DollyScope:            "api://dev-gcp.dolly.dolly-backend/.default",
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

	var readyBody map[string]string
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &readyBody); err != nil || readyBody["status"] != "ready" {
		t.Fatalf("/ready body = %s, want status=ready", rec.Body.String())
	}

	incompleteCases := []struct {
		name   string
		mutate func(*config)
	}{
		{"missing IntrospectionEndpoint", func(c *config) { c.IntrospectionEndpoint = "" }},
		{"missing ExchangeEndpoint", func(c *config) { c.ExchangeEndpoint = "" }},
		{"missing GatewayClientID", func(c *config) { c.GatewayClientID = "" }},
		{"missing DollyScope", func(c *config) { c.DollyScope = "" }},
	}
	for _, tc := range incompleteCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("texas must not be called for probe endpoints")
			}, nil)
			tc.mutate(s.cfg)
			h := s.routes()

			healthRec := httptest.NewRecorder()
			h.ServeHTTP(healthRec, httptest.NewRequest(http.MethodGet, "/health", nil))
			if healthRec.Code != http.StatusOK {
				t.Fatalf("/health returned %d, want 200", healthRec.Code)
			}

			readyRec := httptest.NewRecorder()
			h.ServeHTTP(readyRec, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if readyRec.Code != http.StatusServiceUnavailable {
				t.Fatalf("/ready returned %d, want 503", readyRec.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(readyRec.Body.Bytes(), &body); err != nil || body["status"] != "not_ready" {
				t.Fatalf("/ready body = %s, want status=not_ready", readyRec.Body.String())
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	t.Setenv("DOLLY_URL", "ftp://bad.example")
	if _, err := loadConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid DOLLY_URL scheme")
	}

	t.Setenv("DOLLY_URL", "https://dolly-backend.example")
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
	if cfg.DollyURL != "https://dolly-backend.example" {
		t.Fatalf("DollyURL = %q", cfg.DollyURL)
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
		{
			name:    "matching cluster ok",
			raw:     `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-123"}]`,
			cluster: "dev-gcp",
			want:    "gw-123",
		},
		{
			name:    "other cluster rejected",
			raw:     `[{"name":"prod-gcp:copilot:fleet-gateway","clientId":"gw-prod"}]`,
			cluster: "dev-gcp",
			want:    "",
		},
		{
			name:    "other namespace rejected",
			raw:     `[{"name":"dev-gcp:other:fleet-gateway","clientId":"gw-other-ns"}]`,
			cluster: "dev-gcp",
			want:    "",
		},
		{
			name:    "other app rejected",
			raw:     `[{"name":"dev-gcp:copilot:other","clientId":"other-123"}]`,
			cluster: "dev-gcp",
			want:    "",
		},
		{
			name:    "bare fleet-gateway rejected",
			raw:     `[{"name":"fleet-gateway","clientId":"gw-bare"}]`,
			cluster: "dev-gcp",
			want:    "",
		},
		{
			name:    "empty cluster rejected",
			raw:     `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-123"}]`,
			cluster: "",
			want:    "",
		},
		{
			name:    "duplicates yield empty",
			raw:     `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-1"},{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-2"}]`,
			cluster: "dev-gcp",
			want:    "",
		},
		{
			name:    "malformed json errors",
			raw:     `{not-json`,
			cluster: "dev-gcp",
			wantErr: true,
		},
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
			name:       "bearer exceeding max length",
			authHeader: "Bearer " + strings.Repeat("a", maxBearerLen+1),
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
			name:       "app token access_as_application role rejected",
			authHeader: "Bearer tok-role",
			claims:     map[string]any{"active": true, "roles": []string{"access_as_application"}, "NAVident": "Z123456", "azp": "gw-client-123"},
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
	exchangedToken := strings.Join([]string{"obo", "dolly", "fixture"}, "-")

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
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token": exchangedToken,
			})
		}
	}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotUpstreamHdr = r.Header.Clone()
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"brukerId": "b1",
		})
	})

	rec, out := callToolModern(t, srv.routes(), inboundToken, "dolly_current_user", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), inboundToken) || strings.Contains(rec.Body.String(), exchangedToken) {
		t.Fatal("response leaked token")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotExchangeReq["identity_provider"] != "entra_id" ||
		gotExchangeReq["target"] != "api://dev-gcp.dolly.dolly-backend/.default" ||
		gotExchangeReq["user_token"] != inboundToken {
		t.Fatalf("unexpected OBO request: %+v", gotExchangeReq)
	}
	if gotUpstreamHdr.Get("Authorization") != "Bearer "+exchangedToken {
		t.Fatalf("upstream Authorization = %q, want Bearer %s", gotUpstreamHdr.Get("Authorization"), exchangedToken)
	}
	if gotUpstreamHdr.Get("Nav-Consumer-Id") != "mcp-dolly" {
		t.Fatalf("Nav-Consumer-Id = %q", gotUpstreamHdr.Get("Nav-Consumer-Id"))
	}
	if gotUpstreamHdr.Get("Nav-Call-Id") == "" {
		t.Fatal("expected non-empty Nav-Call-Id header")
	}

	text, isErr := extractToolText(t, out)
	if isErr {
		t.Fatalf("unexpected tool error: %s", text)
	}
	if !strings.Contains(text, `"brukerId":"b1"`) {
		t.Fatalf("unexpected tool text: %s", text)
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

	rec, out := callToolModern(t, srv.routes(), inboundToken, "dolly_current_user", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	txt, isErr := extractToolText(t, out)
	if !isErr {
		t.Fatalf("expected isError=true, got %s", txt)
	}
	if !strings.Contains(txt, "500") {
		t.Fatalf("expected status code 500 in error text, got %q", txt)
	}
	for _, secret := range []string{secretBody, inboundToken, oboToken} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("response leaked %q", secret)
		}
	}
}

func TestTexasExchangeErrorDoesNotLeakBodyOrToken(t *testing.T) {
	secretTexas := "TEXAS_INTERNAL_ERROR_SECRET"
	inboundToken := "inbound-secret-jwt"

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
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(secretTexas))
		}
	}, nil)

	rec, out := callToolModern(t, srv.routes(), inboundToken, "dolly_current_user", map[string]any{})
	txt, isErr := extractToolText(t, out)
	if !isErr {
		t.Fatalf("expected isError=true, got %s", txt)
	}
	if !strings.Contains(txt, "502") {
		t.Fatalf("expected status code 502, got %q", txt)
	}
	if strings.Contains(rec.Body.String(), secretTexas) || strings.Contains(rec.Body.String(), inboundToken) {
		t.Fatal("response leaked texas body or inbound token")
	}
}

func TestUpstreamLimitReader1MiB(t *testing.T) {
	if maxUpstreamBodyBytes != 1<<20 {
		t.Fatalf("maxUpstreamBodyBytes = %d, want %d", maxUpstreamBodyBytes, 1<<20)
	}
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"brukerId":"`))
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxUpstreamBodyBytes+1024))
		_, _ = w.Write([]byte(`"]}`))
	})

	_, out := callToolModern(t, srv.routes(), "in", "dolly_current_user", map[string]any{})
	txt, isErr := extractToolText(t, out)
	if !isErr {
		t.Fatalf("expected error for >1 MiB upstream body, got %s", txt)
	}
	if !strings.Contains(txt, "exceeded") {
		t.Fatalf("unexpected error text: %q", txt)
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
		writeJSON(w, http.StatusOK, map[string]any{"brukerId": "b1"})
	})
	h := srv.routes()

	const totalRequests = 6
	var wg sync.WaitGroup
	for range totalRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec, out := callToolModern(t, h, "in", "dolly_current_user", map[string]any{})
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
				return
			}
			_, isErr := extractToolText(t, out)
			if isErr {
				t.Errorf("unexpected tool error: %+v", out)
			}
		}()
	}

	// Wait for the first two requests to block inside upstream.
	for range maxConcurrentUpstream {
		<-entered
	}
	// Give queued goroutines a moment to attempt entry, then verify cap holds.
	time.Sleep(30 * time.Millisecond)
	if got := maxInFlight.Load(); got > maxConcurrentUpstream {
		close(release)
		wg.Wait()
		t.Fatalf("max in-flight = %d, want <= %d", got, maxConcurrentUpstream)
	}

	// While slots are full, a cancelled context must return an error result immediately.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	b, _ := json.Marshal(modernBody("tools/call", map[string]any{"name": "dolly_current_user", "arguments": map[string]any{}}))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer in")
	for k, v := range modernHeaders("tools/call", "dolly_current_user") {
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

	if got := maxInFlight.Load(); got != maxConcurrentUpstream {
		t.Fatalf("max in-flight = %d, want %d", got, maxConcurrentUpstream)
	}
}

func TestJSONRPCIDValidation(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"brukerId": "b1"})
	})
	h := srv.routes()

	sendRaw := func(t *testing.T, legacy string, body string, hdr map[string]string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		t.Setenv(mcpLegacyUntilEnv, legacy)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		if rec.Body.Len() > 0 {
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
		}
		return rec, out
	}

	assertInvalidRequest := func(t *testing.T, rec *httptest.ResponseRecorder, out map[string]any) {
		t.Helper()
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
		if got := errCode(out); got != -32600 {
			t.Fatalf("error code = %d, want -32600; body=%s", got, rec.Body.String())
		}
		if idVal, hasID := out["id"]; !hasID || idVal != nil {
			t.Fatalf("id = %v (present=%v), want null", idVal, hasID)
		}
	}

	t.Run("tools/list without id modern", func(t *testing.T) {
		b := modernBody("tools/list", nil)
		delete(b, "id")
		raw, _ := json.Marshal(b)
		rec, out := sendRaw(t, "", string(raw), modernHeaders("tools/list", ""))
		assertInvalidRequest(t, rec, out)
	})

	t.Run("tools/list without id legacy", func(t *testing.T) {
		rec, out := sendRaw(t, "2999-01-01", `{"jsonrpc":"2.0","method":"tools/list"}`, nil)
		assertInvalidRequest(t, rec, out)
	})

	t.Run("tools/call with id null modern", func(t *testing.T) {
		b := modernBody("tools/call", map[string]any{"name": "dolly_current_user", "arguments": map[string]any{}})
		b["id"] = nil
		raw, _ := json.Marshal(b)
		rec, out := sendRaw(t, "", string(raw), modernHeaders("tools/call", "dolly_current_user"))
		assertInvalidRequest(t, rec, out)
	})

	t.Run("tools/call with id null legacy", func(t *testing.T) {
		rec, out := sendRaw(t, "2999-01-01", `{"jsonrpc":"2.0","id":null,"method":"tools/call","params":{"name":"dolly_current_user","arguments":{}}}`, nil)
		assertInvalidRequest(t, rec, out)
	})

	t.Run("notifications/initialized with id null modern", func(t *testing.T) {
		b := modernBody("notifications/initialized", nil)
		b["id"] = nil
		raw, _ := json.Marshal(b)
		rec, out := sendRaw(t, "", string(raw), modernHeaders("notifications/initialized", ""))
		assertInvalidRequest(t, rec, out)
	})

	t.Run("notifications/initialized with id null legacy", func(t *testing.T) {
		rec, out := sendRaw(t, "2999-01-01", `{"jsonrpc":"2.0","id":null,"method":"notifications/initialized"}`, nil)
		assertInvalidRequest(t, rec, out)
	})

	t.Run("notifications/initialized with no id modern returns 202", func(t *testing.T) {
		b := modernBody("notifications/initialized", nil)
		delete(b, "id")
		raw, _ := json.Marshal(b)
		rec, _ := sendRaw(t, "", string(raw), modernHeaders("notifications/initialized", ""))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("notifications/initialized with no id legacy returns 202", func(t *testing.T) {
		rec, _ := sendRaw(t, "2999-01-01", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202; body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestMcpMethodBase64Header(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), nil)
	tgt := mcpComplianceTarget{
		handler: srv.routes(),
		tool:    "dolly_current_user",
		header:  map[string]string{"Authorization": "Bearer test-token"},
	}

	encoded := "=?base64?" + base64.StdEncoding.EncodeToString([]byte("tools/list")) + "?="
	rec, _ := mcpCall(t, tgt, http.MethodPost, modernBody("tools/list", nil), with(modernHeaders("tools/list", ""), "Mcp-Method", encoded))
	if rec.Code != http.StatusOK {
		t.Fatalf("base64 Mcp-Method status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	recBad, outBad := mcpCall(t, tgt, http.MethodPost, modernBody("tools/list", nil), with(modernHeaders("tools/list", ""), "Mcp-Method", "=?base64?!!!?="))
	if recBad.Code != http.StatusBadRequest || errCode(outBad) != mcpCodeHeaderMismatch {
		t.Fatalf("malformed base64 Mcp-Method: status=%d code=%d body=%s", recBad.Code, errCode(outBad), recBad.Body.String())
	}
}

func TestMCP2026Compliance(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/bruker/current" {
			writeJSON(w, http.StatusOK, map[string]any{"brukerId": "b1"})
			return
		}
		http.NotFound(w, r)
	})
	runMCP2026Compliance(t, mcpComplianceTarget{
		checkGET: true,
		handler:  srv.routes(),
		tool:     "dolly_current_user",
		header:   map[string]string{"Authorization": "Bearer test-token"},
	})
}
