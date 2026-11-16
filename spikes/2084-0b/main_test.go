package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const base = "https://gw.test"
const redirect = "http://127.0.0.1:62471/callback"

func testGW() *gateway {
	g := newGateway(base, []string{redirect})
	g.entraAuthz = "https://entra.test/authorize" // skip well-known fetch
	return g
}

func authzQuery(mut func(url.Values)) string {
	v := url.Values{"client_id": {base + "/oauth-client.json"}, "redirect_uri": {redirect}, "response_type": {"code"},
		"state": {"s1"}, "code_challenge": {s256("verifier")}, "code_challenge_method": {"S256"}, "resource": {base + "/mcp"}}
	if mut != nil {
		mut(v)
	}
	return "/authorize?" + v.Encode()
}

func do(g *gateway, req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	g.routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAuthorizeRejectsUnknownClientAndRedirect(t *testing.T) {
	g := testGW()
	for name, mut := range map[string]func(url.Values){
		"client":         func(v url.Values) { v.Set("client_id", "https://evil.test/oauth-client.json") },
		"redirect port":  func(v url.Values) { v.Set("redirect_uri", "http://127.0.0.1:62472/callback") },
		"redirect path":  func(v url.Values) { v.Set("redirect_uri", redirect+"/x") },
		"redirect host":  func(v url.Values) { v.Set("redirect_uri", "http://localhost:62471/callback") },
		"redirect query": func(v url.Values) { v.Set("redirect_uri", redirect+"?a=1") },
	} {
		rec := do(g, httptest.NewRequest("GET", authzQuery(mut), nil))
		if rec.Code != 400 || rec.Header().Get("Location") != "" {
			t.Errorf("%s: got %d, location %q; want 400 without redirect", name, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestAuthorizeRequiresPKCEAndResource(t *testing.T) {
	g := testGW()
	for name, mut := range map[string]func(url.Values){
		"plain":       func(v url.Values) { v.Set("code_challenge_method", "plain") },
		"no pkce":     func(v url.Values) { v.Del("code_challenge") },
		"no resource": func(v url.Values) { v.Del("resource") },
		"wrong res":   func(v url.Values) { v.Set("resource", base+"/other") },
	} {
		rec := do(g, httptest.NewRequest("GET", authzQuery(mut), nil))
		loc := rec.Header().Get("Location")
		if rec.Code != 302 || !strings.HasPrefix(loc, redirect+"?") || !strings.Contains(loc, "error=") || !strings.Contains(loc, "iss=") {
			t.Errorf("%s: got %d %q; want error redirect with iss", name, rec.Code, loc)
		}
	}
}

// consent renders the page and returns the consent id and CSRF cookie.
func consent(t *testing.T, g *gateway) (string, *http.Cookie) {
	rec := do(g, httptest.NewRequest("GET", authzQuery(nil), nil))
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("consent page: %d, csp %q", rec.Code, rec.Header().Get("Content-Security-Policy"))
	}
	if len(g.pend) != 1 {
		t.Fatalf("pending = %d", len(g.pend))
	}
	for k := range g.pend {
		return strings.TrimPrefix(k, "consent:"), rec.Result().Cookies()[0]
	}
	return "", nil
}

func postConsent(g *gateway, id, csrf string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/consent", strings.NewReader(url.Values{"id": {id}, "csrf": {csrf}, "decision": {"approve"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return do(g, req)
}

func TestConsentCSRF(t *testing.T) {
	g := testGW()
	id, c := consent(t, g)
	if rec := postConsent(g, id, "forged", c); rec.Code != 403 {
		t.Fatalf("forged csrf: %d", rec.Code)
	}
	if rec := postConsent(g, id, c.Value, c); rec.Code != 403 {
		t.Fatalf("consent id must be single use after a failed attempt: %d", rec.Code)
	}
	id, c = consent(t, g)
	if rec := postConsent(g, id, c.Value, nil); rec.Code != 403 {
		t.Fatalf("missing cookie: %d", rec.Code)
	}
	id, c = consent(t, g)
	rec := postConsent(g, id, c.Value, c)
	if rec.Code != 302 || !strings.HasPrefix(rec.Header().Get("Location"), "https://entra.test/authorize?") {
		t.Fatalf("approve: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if !strings.Contains(rec.Header().Get("Location"), "offline_access") {
		t.Errorf("strategy a must request offline_access")
	}
}

func TestTokenPKCEAndRedirectMatch(t *testing.T) {
	g := testGW()
	issue := func() string {
		code := random()
		g.codes[code] = &issued{P: pending{ClientID: g.clientID, RedirectURI: redirect, Challenge: s256("verifier"), Resource: g.mcpResource()},
			Session: &session{Strategy: "b", Resource: g.mcpResource(), Expiry: time.Now().Add(time.Hour)}, Created: time.Now()}
		return code
	}
	post := func(v url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/token", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return do(g, req)
	}
	form := func(code string) url.Values {
		return url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {g.clientID},
			"redirect_uri": {redirect}, "code_verifier": {"verifier"}, "resource": {g.mcpResource()}}
	}
	for name, mut := range map[string]func(url.Values){
		"bad verifier": func(v url.Values) { v.Set("code_verifier", "wrong") },
		"redirect":     func(v url.Values) { v.Set("redirect_uri", "http://127.0.0.1:62471/callback/") },
		"client":       func(v url.Values) { v.Set("client_id", "x") },
		"resource":     func(v url.Values) { v.Set("resource", base+"/other") },
	} {
		f := form(issue())
		mut(f)
		if rec := post(f); rec.Code != 400 {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	code := issue()
	if rec := post(form(code)); rec.Code != 200 || !strings.Contains(rec.Body.String(), "access_token") {
		t.Fatalf("valid: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(form(code)); rec.Code != 400 {
		t.Fatalf("code reuse: %d", rec.Code)
	}
}

func TestTeamCheck(t *testing.T) {
	ok := map[string]any{"aud": "team", "azp": "gw", "NAVident": "Z1", "idtyp": "user"}
	if m := teamCheck(ok, "team", "gw", "gw2"); m != "" {
		t.Fatal(m)
	}
	if m := teamCheck(map[string]any{"aud": "team", "azp": "gw2", "NAVident": "Z1", "idtyp": "user"}, "team", "gw", "gw2"); m != "" {
		t.Fatal(m)
	}
	if m := teamCheck(ok, "team"); m == "" {
		t.Fatal("empty gateway allowlist accepted")
	}
	for _, c := range []struct{ k, v string }{{"idtyp", "app"}, {"azp", "other"}, {"azp", ""}, {"aud", "x"}, {"NAVident", ""}} {
		in := map[string]any{}
		for k, v := range ok {
			in[k] = v
		}
		in[c.k] = c.v
		if teamCheck(in, "team", "gw", "gw2") == "" {
			t.Errorf("%s=%s accepted", c.k, c.v)
		}
	}
}

func TestGatewayClientIDs(t *testing.T) {
	t.Setenv("GATEWAY_NAME", "dev-gcp:copilot:fleet-gw-0b, dev-gcp:copilot:fleet-gateway")
	t.Setenv("AZURE_APP_PRE_AUTHORIZED_APPS", `[{"name":"dev-gcp:copilot:fleet-gw-0b","clientId":"id-0b"},{"name":"dev-gcp:copilot:other","clientId":"id-other"},{"name":"dev-gcp:copilot:fleet-gateway","clientId":"id-gw"}]`)
	got := gatewayClientIDs()
	if len(got) != 2 || got[0] != "id-0b" || got[1] != "id-gw" {
		t.Fatalf("got %v, want [id-0b id-gw]", got)
	}
}
