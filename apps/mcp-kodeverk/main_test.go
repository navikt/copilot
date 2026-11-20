package main

import (
	"bytes"
	"context"
	"encoding/base64"
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

func newTestServer(t *testing.T, texasHandler, kodeverkHandler http.HandlerFunc) *server {
	t.Helper()
	texasTS := httptest.NewServer(texasHandler)
	t.Cleanup(texasTS.Close)

	var kodeverkURL string
	if kodeverkHandler != nil {
		kodeverkTS := httptest.NewServer(kodeverkHandler)
		t.Cleanup(kodeverkTS.Close)
		kodeverkURL = kodeverkTS.URL
	} else {
		kodeverkURL = "http://kodeverk.invalid"
	}

	cfg := &config{
		Port:                  "8080",
		IntrospectionEndpoint: texasTS.URL + "/introspect",
		ExchangeEndpoint:      texasTS.URL + "/exchange",
		GatewayClientID:       "gw-client-123",
		KodeverkURL:           kodeverkURL,
		KodeverkScope:         "api://dev-gcp.team-rocket.kodeverk-api/.default",
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
		{"missing KodeverkScope", func(c *config) { c.KodeverkScope = "" }},
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
	t.Setenv("KODEVERK_URL", "ftp://bad.example")
	if _, err := loadConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid KODEVERK_URL scheme")
	}

	t.Setenv("KODEVERK_URL", "http://kodeverk-api.team-rocket")
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
	if cfg.KodeverkURL != "http://kodeverk-api.team-rocket" {
		t.Fatalf("KodeverkURL = %q", cfg.KodeverkURL)
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
	exchangedToken := strings.Join([]string{"obo", "kodeverk", "fixture"}, "-")

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
			"kodeverksnavn": []string{"Postnummer", "Landkoder"},
		})
	})

	rec, out := callToolModern(t, srv.routes(), inboundToken, "list_kodeverk", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), inboundToken) || strings.Contains(rec.Body.String(), exchangedToken) {
		t.Fatal("response leaked token")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotExchangeReq["identity_provider"] != "entra_id" ||
		gotExchangeReq["target"] != "api://dev-gcp.team-rocket.kodeverk-api/.default" ||
		gotExchangeReq["user_token"] != inboundToken {
		t.Fatalf("unexpected OBO request: %+v", gotExchangeReq)
	}
	if gotUpstreamHdr.Get("Authorization") != "Bearer "+exchangedToken {
		t.Fatalf("upstream Authorization = %q, want Bearer %s", gotUpstreamHdr.Get("Authorization"), exchangedToken)
	}
	if gotUpstreamHdr.Get("Nav-Consumer-Id") != "mcp-kodeverk" {
		t.Fatalf("Nav-Consumer-Id = %q", gotUpstreamHdr.Get("Nav-Consumer-Id"))
	}
	if gotUpstreamHdr.Get("Nav-Call-Id") == "" {
		t.Fatal("expected non-empty Nav-Call-Id header")
	}

	text, isErr := extractToolText(t, out)
	if isErr {
		t.Fatalf("unexpected tool error: %s", text)
	}
	if !strings.Contains(text, "Landkoder") {
		t.Fatalf("unexpected tool text: %s", text)
	}
}

func TestToolsAgainstRealisticKodeverkFake(t *testing.T) {
	var (
		mu       sync.Mutex
		lastPath string
		lastRawQ string
	)

	kodeverkFake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		lastPath = r.URL.Path
		lastRawQ = r.URL.RawQuery
		mu.Unlock()

		switch r.URL.Path {
		case "/api/v1/kodeverk":
			writeJSON(w, http.StatusOK, map[string]any{
				"kodeverksnavn": []string{"Yrker", "Landkoder", "LandkoderISO2", "Postnummer"},
			})
		case "/api/v1/kodeverk/Landkoder/koder":
			writeJSON(w, http.StatusOK, map[string]any{
				"koder": []string{"SWE", "NOR", "DNK", "DEU"},
			})
		case "/api/v1/kodeverk/Særområder/koder":
			writeJSON(w, http.StatusOK, map[string]any{
				"koder": []string{"A1", "B2"},
			})
		case "/api/v1/kodeverk/Landkoder/koder/betydninger":
			writeJSON(w, http.StatusOK, map[string]any{
				"betydninger": map[string]any{
					"SWE": []map[string]any{
						{"gyldigFra": "1900-01-01", "gyldigTil": "9999-12-31", "beskrivelser": map[string]any{"nb": map[string]any{"term": "Sverige", "tekst": "Kongeriket Sverige"}}},
					},
					"NOR": []map[string]any{
						{"gyldigFra": "1900-01-01", "gyldigTil": "9999-12-31", "beskrivelser": map[string]any{"nb": map[string]any{"term": "Norge", "tekst": "Kongeriket Norge"}}},
					},
					"DNK": []map[string]any{
						{"gyldigFra": "1900-01-01", "gyldigTil": "9999-12-31", "beskrivelser": map[string]any{"nb": map[string]any{"term": "Danmark", "tekst": "Kongeriket Danmark"}}},
					},
				},
			})
		case "/api/v1/hierarki":
			writeJSON(w, http.StatusOK, map[string]any{
				"hierarkinavn": []string{"Geografi", "Organisasjon"},
			})
		case "/api/v1/hierarki/Geografi/noder":
			writeJSON(w, http.StatusOK, map[string]any{
				"hierarkinivaaer": []string{"Verdensdel", "Land", "Fylke"},
				"noder": map[string]any{
					"EUR": map[string]any{
						"kode":   "EUR",
						"termer": map[string]any{"nb": "Europa"},
						"undernoder": map[string]any{
							"NOR": map[string]any{
								"kode":   "NOR",
								"termer": map[string]any{"nb": "Norge"},
								"undernoder": map[string]any{
									"03": map[string]any{
										"kode":   "03",
										"termer": map[string]any{"nb": "Oslo"},
									},
								},
							},
						},
					},
					"ASI": map[string]any{
						"kode":   "ASI",
						"termer": map[string]any{"nb": "Asia"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	srv := newTestServer(t, defaultTexasHandler("obo-tok"), kodeverkFake)
	h := srv.routes()

	t.Run("list_kodeverk filter and pagination", func(t *testing.T) {
		rec, out := callToolModern(t, h, "in", "list_kodeverk", map[string]any{"filter": "land", "limit": 1, "offset": 1})
		if rec.Code != 200 {
			t.Fatalf("status=%d", rec.Code)
		}
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		var res struct {
			Total         int      `json:"total"`
			Returned      int      `json:"returned"`
			Offset        int      `json:"offset"`
			Kodeverksnavn []string `json:"kodeverksnavn"`
		}
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		if res.Total != 2 || res.Returned != 1 || res.Offset != 1 || len(res.Kodeverksnavn) != 1 || res.Kodeverksnavn[0] != "LandkoderISO2" {
			t.Fatalf("unexpected list_kodeverk result: %+v", res)
		}
	})

	t.Run("get_kodeverk_koder including Norwegian path escaping", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "Særområder"})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		mu.Lock()
		gotPath := lastPath
		mu.Unlock()
		if gotPath != "/api/v1/kodeverk/Særområder/koder" {
			t.Fatalf("path = %q", gotPath)
		}
		if !strings.Contains(txt, `"A1"`) {
			t.Fatalf("unexpected koder text: %s", txt)
		}
	})

	t.Run("get_kodeverk_betydninger sorted pagination and query params", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "get_kodeverk_betydninger", map[string]any{
			"kodeverksnavn":     "Landkoder",
			"ekskluderUgyldige": false,
			"oppslagsdato":      "2025-05-17",
			"spraak":            []string{"nb", "nn"},
			"limit":             2,
			"offset":            1,
		})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		mu.Lock()
		rawQ := lastRawQ
		mu.Unlock()
		for _, want := range []string{"ekskluderUgyldige=false", "oppslagsdato=2025-05-17", "spraak=nb", "spraak=nn"} {
			if !strings.Contains(rawQ, want) {
				t.Fatalf("query %q missing %q", rawQ, want)
			}
		}
		var res struct {
			Total       int                    `json:"total"`
			Returned    int                    `json:"returned"`
			Offset      int                    `json:"offset"`
			Betydninger map[string][]betydning `json:"betydninger"`
		}
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		// Sorted keys: DNK, NOR, SWE -> offset 1, limit 2 => NOR, SWE
		if res.Total != 3 || res.Returned != 2 || len(res.Betydninger) != 2 {
			t.Fatalf("unexpected betydninger pagination: %+v", res)
		}
		if _, ok := res.Betydninger["NOR"]; !ok {
			t.Fatalf("expected NOR in page: %+v", res.Betydninger)
		}
		if _, ok := res.Betydninger["SWE"]; !ok {
			t.Fatalf("expected SWE in page: %+v", res.Betydninger)
		}
	})

	t.Run("get_kodeverk_betydninger exact kode filter", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "get_kodeverk_betydninger", map[string]any{
			"kodeverksnavn": "Landkoder",
			"kode":          "NOR",
		})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		var res struct {
			Total       int                    `json:"total"`
			Returned    int                    `json:"returned"`
			Betydninger map[string][]betydning `json:"betydninger"`
		}
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		if res.Total != 1 || res.Returned != 1 || res.Betydninger["NOR"][0].Beskrivelser["nb"].Term != "Norge" {
			t.Fatalf("unexpected exact filter result: %+v", res)
		}
	})

	t.Run("list_hierarkier", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "list_hierarkier", map[string]any{"filter": "geo"})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		if !strings.Contains(txt, "Geografi") {
			t.Fatalf("unexpected list_hierarkier text: %s", txt)
		}
	})

	t.Run("get_hierarki_noder maxDepth truncation and limit", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "get_hierarki_noder", map[string]any{
			"hierarkinavn": "Geografi",
			"spraak":       []string{"nb"},
			"maxDepth":     2,
			"limit":        2,
		})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		var res struct {
			Total     int                     `json:"total"`
			Returned  int                     `json:"returned"`
			Truncated bool                    `json:"truncated"`
			Noder     map[string]hierarkinode `json:"noder"`
		}
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		if !res.Truncated || res.Total != 2 || res.Returned != 2 {
			t.Fatalf("expected truncated=true at maxDepth=2: %+v", res)
		}
		if len(res.Noder["EUR"].Undernoder["NOR"].Undernoder) != 0 {
			t.Fatalf("expected depth-3 child to be pruned: %+v", res.Noder["EUR"])
		}
	})

	t.Run("search_kodeverk without kodeverksnavn", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "search_kodeverk", map[string]any{"query": "land"})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		var res struct {
			Total    int      `json:"total"`
			Kodeverk []string `json:"kodeverk"`
		}
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		if res.Total != 2 || res.Kodeverk[0] != "Landkoder" {
			t.Fatalf("unexpected search names result: %+v", res)
		}
	})

	t.Run("search_kodeverk with kodeverksnavn matches code and term", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "search_kodeverk", map[string]any{
			"query":         "norg",
			"kodeverksnavn": "Landkoder",
		})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		var res struct {
			Total   int               `json:"total"`
			Matches []searchCodeMatch `json:"matches"`
		}
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		if res.Total != 1 || res.Matches[0].Kode != "NOR" || res.Matches[0].Term != "Norge" {
			t.Fatalf("unexpected search codes result: %+v", res)
		}
	})
}

func TestInputValidationTable(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("kodeverk-api must not be called on invalid inputs")
	})
	h := srv.routes()

	tests := []struct {
		name string
		tool string
		args any
	}{
		{"unknown tool", "delete_kodeverk", map[string]any{}},
		{"arguments array instead of object", "list_kodeverk", []string{"a"}},
		{"unknown key", "list_kodeverk", map[string]any{"unknown": "x"}},
		{"case-variant key", "list_kodeverk", map[string]any{"Filter": "x"}},
		{"wrong limit type string", "list_kodeverk", map[string]any{"limit": "10"}},
		{"wrong limit type float", "list_kodeverk", map[string]any{"limit": 10.5}},
		{"limit zero", "list_kodeverk", map[string]any{"limit": 0}},
		{"limit over 200", "list_kodeverk", map[string]any{"limit": 201}},
		{"negative offset", "list_kodeverk", map[string]any{"offset": -1}},
		{"list_kodeverk filter over max", "list_kodeverk", map[string]any{"filter": strings.Repeat("a", maxFilterLen+1)}},
		{"get_kodeverk_koder filter over max", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "Land", "filter": strings.Repeat("ø", maxFilterLen+1)}},
		{"list_hierarkier filter over max", "list_hierarkier", map[string]any{"filter": strings.Repeat("a", maxFilterLen+1)}},
		{"missing kodeverksnavn", "get_kodeverk_koder", map[string]any{}},
		{"bad kodeverksnavn path traversal", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "../web/api"}},
		{"bad kodeverksnavn slash", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "a/b"}},
		{"bad kodeverksnavn spaces", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "a b"}},
		{"bad kodeverksnavn too long", "get_kodeverk_koder", map[string]any{"kodeverksnavn": strings.Repeat("a", 101)}},
		{"betydninger kode over max", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "kode": strings.Repeat("ø", maxKodeLen+1)}},
		{"betydninger limit over 100", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "limit": 101}},
		{"betydninger bad date", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "oppslagsdato": "2025-02-30"}},
		{"betydninger bad date format", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "oppslagsdato": "17-05-2025"}},
		{"betydninger bad spraak uppercase", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "spraak": []string{"NB"}}},
		{"betydninger too many spraak", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "spraak": []string{"nb", "nn", "en", "se", "da", "sv"}}},
		{"betydninger wrong spraak type", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "spraak": "nb"}},
		{"betydninger wrong ekskluderUgyldige type", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Land", "ekskluderUgyldige": "true"}},
		{"hierarki bad name", "get_hierarki_noder", map[string]any{"hierarkinavn": "bad;name"}},
		{"hierarki maxDepth 0", "get_hierarki_noder", map[string]any{"hierarkinavn": "Geo", "maxDepth": 0}},
		{"hierarki maxDepth 11", "get_hierarki_noder", map[string]any{"hierarkinavn": "Geo", "maxDepth": 11}},
		{"hierarki limit 101", "get_hierarki_noder", map[string]any{"hierarkinavn": "Geo", "limit": 101}},
		{"search query too short", "search_kodeverk", map[string]any{"query": "a"}},
		{"search query whitespace only", "search_kodeverk", map[string]any{"query": "   "}},
		{"search query too long", "search_kodeverk", map[string]any{"query": strings.Repeat("a", 65)}},
		{"search query regex metachar dot star", "search_kodeverk", map[string]any{"query": "land.*"}},
		{"search query regex metachar parens", "search_kodeverk", map[string]any{"query": "(nor|swe)"}},
		{"search query regex metachar slash", "search_kodeverk", map[string]any{"query": "a/b"}},
		{"search bad kodeverksnavn", "search_kodeverk", map[string]any{"query": "norge", "kodeverksnavn": "../secret"}},
		{"search limit over 50", "search_kodeverk", map[string]any{"query": "norge", "limit": 51}},
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

func TestFilterAndKodeLengthLimits(t *testing.T) {
	kodeverkFake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/kodeverk":
			writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder"}})
		case "/api/v1/kodeverk/Landkoder/koder":
			writeJSON(w, http.StatusOK, map[string]any{"koder": []string{"NOR"}})
		case "/api/v1/kodeverk/Landkoder/koder/betydninger":
			writeJSON(w, http.StatusOK, map[string]any{"betydninger": map[string][]betydning{}})
		case "/api/v1/hierarki":
			writeJSON(w, http.StatusOK, map[string]any{"hierarkinavn": []string{"Geografi"}})
		default:
			http.NotFound(w, r)
		}
	})
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), kodeverkFake)
	h := srv.routes()

	accepted := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"list_kodeverk filter at limit (multibyte runes)", "list_kodeverk", map[string]any{"filter": strings.Repeat("ø", maxFilterLen)}},
		{"get_kodeverk_koder filter at limit", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "Landkoder", "filter": strings.Repeat("a", maxFilterLen)}},
		{"list_hierarkier filter at limit", "list_hierarkier", map[string]any{"filter": strings.Repeat("a", maxFilterLen)}},
		{"get_kodeverk_betydninger kode at limit (multibyte runes)", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Landkoder", "kode": strings.Repeat("ø", maxKodeLen)}},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			rec, out := callToolModern(t, h, "in", tt.tool, tt.args)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if got := errCode(out); got != 0 {
				t.Fatalf("unexpected rpc error %d: %s", got, rec.Body.String())
			}
			_, isErr := extractToolText(t, out)
			if isErr {
				t.Fatalf("unexpected tool error: %s", rec.Body.String())
			}
		})
	}

	rejected := []struct {
		name    string
		tool    string
		args    map[string]any
		wantMsg string
	}{
		{"list_kodeverk filter over limit", "list_kodeverk", map[string]any{"filter": strings.Repeat("ø", maxFilterLen+1)}, "filter must be at most 64 characters"},
		{"get_kodeverk_koder filter over limit", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "Landkoder", "filter": strings.Repeat("a", maxFilterLen+1)}, "filter must be at most 64 characters"},
		{"list_hierarkier filter over limit", "list_hierarkier", map[string]any{"filter": strings.Repeat("a", maxFilterLen+1)}, "filter must be at most 64 characters"},
		{"get_kodeverk_betydninger kode over limit", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Landkoder", "kode": strings.Repeat("ø", maxKodeLen+1)}, "kode must be at most 128 characters"},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			rec, out := callToolModern(t, h, "in", tt.tool, tt.args)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if got := errCode(out); got != mcpCodeInvalidParams {
				t.Fatalf("error code = %d, want %d; body=%s", got, mcpCodeInvalidParams, rec.Body.String())
			}
			errObj, _ := out["error"].(map[string]any)
			if msg, _ := errObj["message"].(string); msg != tt.wantMsg {
				t.Fatalf("error message = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestSizeCap32KiB(t *testing.T) {
	bigText := strings.Repeat("A", 3000)
	betydninger := make(map[string][]betydning, 100)
	for i := range 100 {
		code := fmt.Sprintf("CODE_%03d", i)
		betydninger[code] = []betydning{{
			GyldigFra: "2000-01-01",
			GyldigTil: "9999-12-31",
			Beskrivelser: map[string]beskrivelse{
				"nb": {Term: code, Tekst: bigText},
			},
		}}
	}

	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"betydninger": betydninger})
	})

	_, out := callToolModern(t, srv.routes(), "in", "get_kodeverk_betydninger", map[string]any{
		"kodeverksnavn": "Stor",
		"limit":         100,
	})
	txt, isErr := extractToolText(t, out)
	if isErr {
		t.Fatalf("unexpected tool error: %s", txt)
	}
	if len(txt) > maxToolResultBytes {
		t.Fatalf("serialized tool text length %d exceeds 32 KiB (%d)", len(txt), maxToolResultBytes)
	}
	var res struct {
		Total       int                    `json:"total"`
		Returned    int                    `json:"returned"`
		Truncated   bool                   `json:"truncated"`
		Betydninger map[string][]betydning `json:"betydninger"`
	}
	if err := json.Unmarshal([]byte(txt), &res); err != nil {
		t.Fatalf("capped output is not valid JSON: %v", err)
	}
	if !res.Truncated || res.Total != 100 || res.Returned == 0 || res.Returned >= 100 || len(res.Betydninger) != res.Returned {
		t.Fatalf("unexpected capped metadata: total=%d returned=%d truncated=%v len=%d", res.Total, res.Returned, res.Truncated, len(res.Betydninger))
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

	rec, out := callToolModern(t, srv.routes(), inboundToken, "list_kodeverk", map[string]any{})
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

	rec, out := callToolModern(t, srv.routes(), inboundToken, "list_kodeverk", map[string]any{})
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

func TestUpstreamLimitReader4MiB(t *testing.T) {
	if maxUpstreamBodyBytes != 4<<20 {
		t.Fatalf("maxUpstreamBodyBytes = %d, want %d", maxUpstreamBodyBytes, 4<<20)
	}
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"kodeverksnavn":["`))
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxUpstreamBodyBytes+1024))
		_, _ = w.Write([]byte(`"]}`))
	})

	_, out := callToolModern(t, srv.routes(), "in", "list_kodeverk", map[string]any{})
	txt, isErr := extractToolText(t, out)
	if !isErr {
		t.Fatalf("expected error for >4 MiB upstream body, got %s", txt)
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
		writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder"}})
	})
	h := srv.routes()

	const totalRequests = 6
	var wg sync.WaitGroup
	for range totalRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec, out := callToolModern(t, h, "in", "list_kodeverk", map[string]any{})
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
	b, _ := json.Marshal(modernBody("tools/call", map[string]any{"name": "list_kodeverk", "arguments": map[string]any{}}))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer in")
	for k, v := range modernHeaders("tools/call", "list_kodeverk") {
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

func TestSearchKodeverkLanguageFallback(t *testing.T) {
	betydninger := map[string][]betydning{
		"K1": {{
			GyldigFra: "2000-01-01",
			GyldigTil: "9999-12-31",
			Beskrivelser: map[string]beskrivelse{
				"en": {Term: "English term", Tekst: "English text"},
				"nn": {Term: "Nynorsk term", Tekst: "Nynorsk text"},
			},
		}},
		"K2": {{
			GyldigFra: "2000-01-01",
			GyldigTil: "9999-12-31",
			Beskrivelser: map[string]beskrivelse{
				"sv": {Term: "Svensk term", Tekst: "Svensk text"},
				"da": {Term: "Dansk term", Tekst: "Dansk text"},
			},
		}},
	}

	// Run repeatedly to catch map-iteration nondeterminism.
	for range 20 {
		txt, err := buildSearchKodeverkCodesResult("K1", "Test", 25, betydninger)
		if err != nil {
			t.Fatal(err)
		}
		var res struct {
			Matches []searchCodeMatch `json:"matches"`
		}
		if err = json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		if len(res.Matches) != 1 || res.Matches[0].Term != "Nynorsk term" || res.Matches[0].Tekst != "Nynorsk text" {
			t.Fatalf("expected deterministic nn fallback over en, got %+v", res.Matches)
		}

		txt2, err := buildSearchKodeverkCodesResult("K2", "Test", 25, betydninger)
		if err != nil {
			t.Fatal(err)
		}
		var res2 struct {
			Matches []searchCodeMatch `json:"matches"`
		}
		if err = json.Unmarshal([]byte(txt2), &res2); err != nil {
			t.Fatal(err)
		}
		if len(res2.Matches) != 1 || res2.Matches[0].Term != "Dansk term" {
			t.Fatalf("expected lexicographically smallest key da over sv, got %+v", res2.Matches)
		}
	}
}

func TestHierarkiNoderDepthFallbackMultipleRoots(t *testing.T) {
	hugeText := strings.Repeat("Z", maxToolResultBytes+4096)
	noder := make(map[string]hierarkinode, 10)
	for i := range 10 {
		code := fmt.Sprintf("ROOT_%02d", i)
		noder[code] = hierarkinode{
			Kode:   code,
			Termer: map[string]string{"nb": code},
			Undernoder: map[string]hierarkinode{
				"L2": {
					Kode:   "L2",
					Termer: map[string]string{"nb": "Level 2"},
					Undernoder: map[string]hierarkinode{
						"L3": {
							Kode:   "L3",
							Termer: map[string]string{"nb": "Level 3"},
							Undernoder: map[string]hierarkinode{
								"L4": {
									Kode:    "L4",
									Termer:  map[string]string{"nb": "Level 4"},
									Tekster: map[string]string{"nb": hugeText},
								},
							},
						},
					},
				},
			},
		}
	}

	txt, err := buildHierarkiNoderResult("Stor", 4, 50, hierarkiNoderResponse{
		Hierarkinivaaer: []string{"1", "2", "3", "4"},
		Noder:           noder,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(txt) > maxToolResultBytes {
		t.Fatalf("result size %d exceeds %d", len(txt), maxToolResultBytes)
	}
	var res struct {
		Total     int                     `json:"total"`
		Returned  int                     `json:"returned"`
		Truncated bool                    `json:"truncated"`
		Noder     map[string]hierarkinode `json:"noder"`
	}
	if err := json.Unmarshal([]byte(txt), &res); err != nil {
		t.Fatal(err)
	}
	if res.Total != 10 || res.Returned != 10 || !res.Truncated || len(res.Noder) != 10 {
		t.Fatalf("got total=%d returned=%d truncated=%v len=%d; want total=10 returned=10 truncated=true",
			res.Total, res.Returned, res.Truncated, len(res.Noder))
	}
	if len(res.Noder["ROOT_00"].Undernoder["L2"].Undernoder["L3"].Undernoder) != 0 {
		t.Fatal("expected depth 4 nodes to be pruned at fallback depth 3")
	}
}

func TestJSONRPCIDValidation(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder"}})
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
		b := modernBody("tools/call", map[string]any{"name": "list_kodeverk", "arguments": map[string]any{}})
		b["id"] = nil
		raw, _ := json.Marshal(b)
		rec, out := sendRaw(t, "", string(raw), modernHeaders("tools/call", "list_kodeverk"))
		assertInvalidRequest(t, rec, out)
	})

	t.Run("tools/call with id null legacy", func(t *testing.T) {
		rec, out := sendRaw(t, "2999-01-01", `{"jsonrpc":"2.0","id":null,"method":"tools/call","params":{"name":"list_kodeverk","arguments":{}}}`, nil)
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
		tool:    "list_kodeverk",
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
		if r.URL.Path == "/api/v1/kodeverk" {
			writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder"}})
			return
		}
		http.NotFound(w, r)
	})
	runMCP2026Compliance(t, mcpComplianceTarget{
		checkGET: true,
		handler:  srv.routes(),
		tool:     "list_kodeverk",
		header:   map[string]string{"Authorization": "Bearer test-token"},
	})
}
