package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
		{"missing kodeverksnavn", "get_kodeverk_koder", map[string]any{}},
		{"bad kodeverksnavn path traversal", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "../web/api"}},
		{"bad kodeverksnavn slash", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "a/b"}},
		{"bad kodeverksnavn spaces", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "a b"}},
		{"bad kodeverksnavn too long", "get_kodeverk_koder", map[string]any{"kodeverksnavn": strings.Repeat("a", 101)}},
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

func TestSizeCap128KiB(t *testing.T) {
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
		t.Fatalf("serialized tool text length %d exceeds 128 KiB (%d)", len(txt), maxToolResultBytes)
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

func TestUpstreamLimitReader8MiB(t *testing.T) {
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
		t.Fatalf("expected error for >8 MiB upstream body, got %s", txt)
	}
	if !strings.Contains(txt, "exceeded") {
		t.Fatalf("unexpected error text: %q", txt)
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
