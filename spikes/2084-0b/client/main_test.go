package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveConfigDefault(t *testing.T) {
	cfg, err := resolveConfig(httpClient, "https://fleet-gw-0b.intern.dev.nav.no/", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.targetURL != "https://fleet-gw-0b.intern.dev.nav.no/mcp" ||
		cfg.resource != "https://fleet-gw-0b.intern.dev.nav.no/mcp" ||
		cfg.clientID != "https://fleet-gw-0b.intern.dev.nav.no/oauth-client.json" ||
		cfg.issuer != "https://fleet-gw-0b.intern.dev.nav.no" ||
		cfg.authzEndpoint != "https://fleet-gw-0b.intern.dev.nav.no/authorize" ||
		cfg.tokenEndpoint != "https://fleet-gw-0b.intern.dev.nav.no/token" ||
		!cfg.checkIss {
		t.Fatalf("unexpected default config: %+v", cfg)
	}
}

func TestDiscoverWellKnownPRMAndAS(t *testing.T) {
	var asSrv, rsSrv *httptest.Server
	asSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-authorization-server" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                         asSrv.URL,
			"authorization_endpoint":                         asSrv.URL + "/authorize",
			"token_endpoint":                                 asSrv.URL + "/token",
			"authorization_response_iss_parameter_supported": true,
		})
	}))
	defer asSrv.Close()

	rsSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-protected-resource/mcp/test" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resource":              rsSrv.URL + "/mcp/test",
			"authorization_servers": []string{asSrv.URL},
		})
	}))
	defer rsSrv.Close()

	cfg, err := resolveConfig(rsSrv.Client(), "https://fleet-gw-0b.intern.dev.nav.no", rsSrv.URL+"/mcp/test", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.targetURL != rsSrv.URL+"/mcp/test" ||
		cfg.resource != rsSrv.URL+"/mcp/test" ||
		cfg.issuer != asSrv.URL ||
		cfg.authzEndpoint != asSrv.URL+"/authorize" ||
		cfg.tokenEndpoint != asSrv.URL+"/token" ||
		cfg.clientID != "https://fleet-gw-0b.intern.dev.nav.no/oauth-client.json" ||
		!cfg.checkIss {
		t.Fatalf("unexpected discovered config: %+v", cfg)
	}
}

func TestDiscoverVia401WWWAuthenticate(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp/test":
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", resource_metadata="`+srv.URL+`/custom-prm"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/custom-prm":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"resource":              srv.URL + "/mcp/test",
				"authorization_servers": []string{srv.URL + "/tenant"},
			})
		case "/.well-known/oauth-authorization-server/tenant":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 srv.URL + "/tenant",
				"authorization_endpoint": srv.URL + "/tenant/auth",
				"token_endpoint":         srv.URL + "/tenant/tok",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg, err := resolveConfig(srv.Client(), "https://fleet-gw-0b.intern.dev.nav.no", srv.URL+"/mcp/test", "https://example.test/client.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.authzEndpoint != srv.URL+"/tenant/auth" ||
		cfg.tokenEndpoint != srv.URL+"/tenant/tok" ||
		cfg.issuer != srv.URL+"/tenant" ||
		cfg.clientID != "https://example.test/client.json" ||
		cfg.checkIss {
		t.Fatalf("unexpected 401-discovered config: %+v", cfg)
	}
}

func TestExtractResourceMetadata(t *testing.T) {
	cases := []struct {
		hdr  string
		want string
	}{
		{`Bearer resource_metadata="https://a.test/.well-known/oauth-protected-resource/mcp", error="invalid_token"`, "https://a.test/.well-known/oauth-protected-resource/mcp"},
		{`Bearer error="invalid_token", Resource_Metadata="https://b.test/prm"`, "https://b.test/prm"},
		{`Bearer resource_metadata=https://c.test/prm, error="invalid_token"`, "https://c.test/prm"},
		{`Bearer error="invalid_token"`, ""},
	}
	for _, tc := range cases {
		if got := extractResourceMetadata(tc.hdr); got != tc.want {
			t.Errorf("extractResourceMetadata(%q) = %q, want %q", tc.hdr, got, tc.want)
		}
	}
}
