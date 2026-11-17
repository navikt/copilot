package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// enableDCR turns on DCR for a test env with a fresh store holding capDCR clients.
func enableDCR(te *testEnv, capDCR, perIP int) {
	te.gw.cfg.DCREnabled = true
	te.gw.cfg.DCRClientTTL = 24 * time.Hour
	te.gw.store = newMemoryStore(te.gw.now, map[string]int{kindDCRClient: capDCR})
	te.gw.dcrLimiter = newIPRateLimiter(perIP, 1000, time.Hour, 100, te.gw.now)
	te.handler = te.gw.routes()
}

func postRegister(te *testEnv, body any, ip string) (int, map[string]any) {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func opencodeRegistration() map[string]any {
	return map[string]any{
		"redirect_uris":              []string{"http://127.0.0.1:19876/mcp/oauth/callback"},
		"client_name":                "OpenCode",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	}
}

func TestDCR_DisabledByDefault(t *testing.T) {
	te := newTestEnv(t)
	if code, _ := postRegister(te, opencodeRegistration(), "10.0.0.1"); code == http.StatusCreated {
		t.Fatalf("registration must not be served when DCR is disabled")
	}
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if strings.Contains(rec.Body.String(), "registration_endpoint") {
		t.Fatalf("metadata must not advertise registration_endpoint when disabled")
	}
}

func TestDCR_RegisterAndAuthorize(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 10)

	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if !strings.Contains(rec.Body.String(), te.gw.cfg.BaseURL+"/register") {
		t.Fatalf("metadata missing registration_endpoint: %s", rec.Body.String())
	}

	code, out := postRegister(te, opencodeRegistration(), "10.0.0.1")
	if code != http.StatusCreated {
		t.Fatalf("register status %d: %v", code, out)
	}
	clientID, _ := out["client_id"].(string)
	if !strings.HasPrefix(clientID, dcrClientIDPrefix) || len(clientID) < 40 {
		t.Fatalf("client_id not opaque random: %q", clientID)
	}
	if out["token_endpoint_auth_method"] != "none" {
		t.Fatalf("unexpected auth method: %v", out["token_endpoint_auth_method"])
	}

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	// Any port on the registered loopback path goes to Entra; after the callback
	// the consent page marks the client unverified.
	cb := "http://127.0.0.1:40001/mcp/oauth/callback"
	login := func(id string) *http.Response {
		return te.callback(te.followToEntra(httptest.NewRequest(http.MethodGet, te.authorizeURL(map[string]string{"client_id": id, "redirect_uri": cb}, verifier), nil)))
	}
	resp := login(clientID)
	page, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(page), "Unverified client") || !strings.Contains(string(page), "OpenCode") {
		t.Fatalf("expected unverified consent page, got %d: %s", resp.StatusCode, page)
	}
	resp.Body = io.NopCloser(strings.NewReader(string(page)))
	id, csrf, cookie, _ := te.parseConsentPage(resp)
	_ = te.clientCode(te.postConsent(id, csrf, cookie, "approve")) //nolint:bodyclose // clientCode closes it

	// Consent is remembered for this registered client like any other client.
	status := func(id string) int { r := login(id); _ = r.Body.Close(); return r.StatusCode }
	if got := status(clientID); got != http.StatusFound {
		t.Fatalf("remembered consent not used for DCR client: %d", got)
	}
	// A new registration gets a new random client_id and must ask again.
	_, out2 := postRegister(te, opencodeRegistration(), "10.0.0.1")
	clientID2, _ := out2["client_id"].(string)
	if clientID2 == clientID {
		t.Fatal("registrations share a client_id")
	}
	if got := status(clientID2); got != http.StatusOK {
		t.Fatalf("new registration inherited consent: %d", got)
	}

	// PKCE stays mandatory for registered clients.
	u := te.authorizeURL(map[string]string{"client_id": clientID, "redirect_uri": "http://127.0.0.1:40001/mcp/oauth/callback", "code_challenge": ""}, verifier)
	rec = httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != http.StatusFound || loc.Query().Get("error") != "invalid_request" {
		t.Fatalf("expected PKCE rejection, got %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// A different path is rejected without redirect.
	u = te.authorizeURL(map[string]string{"client_id": clientID, "redirect_uri": "http://127.0.0.1:40001/evil"}, verifier)
	rec = httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unregistered path, got %d", rec.Code)
	}
}

func TestDCR_UnknownOrExpiredClientRejected(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 10)
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	u := te.authorizeURL(map[string]string{"client_id": dcrClientIDPrefix + "made-up", "redirect_uri": "http://127.0.0.1:40001/mcp/oauth/callback"}, verifier)
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown registered client: got %d", rec.Code)
	}

	_, out := postRegister(te, opencodeRegistration(), "10.0.0.1")
	clientID, _ := out["client_id"].(string)
	te.advance(25 * time.Hour)
	u = te.authorizeURL(map[string]string{"client_id": clientID, "redirect_uri": "http://127.0.0.1:40001/mcp/oauth/callback"}, verifier)
	rec = httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expired registered client: got %d", rec.Code)
	}

	// A registered id stops working when DCR is switched off again.
	_, out = postRegister(te, opencodeRegistration(), "10.0.0.1")
	clientID, _ = out["client_id"].(string)
	te.gw.cfg.DCREnabled = false
	if te.gw.clientKnown(t.Context(), clientID) {
		t.Fatalf("registered client must be unknown with DCR disabled")
	}
}

func TestDCR_RejectsUnsafeMetadata(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 100)
	cases := map[string]map[string]any{
		"non-loopback redirect": {"redirect_uris": []string{"https://evil.example/cb"}},
		"https loopback":        {"redirect_uris": []string{"https://127.0.0.1:1/cb"}},
		"only localhost":        {"redirect_uris": []string{"http://localhost:19876/cb"}},
		"loopback plus foreign": {"redirect_uris": []string{"http://127.0.0.1:1/cb", "http://evil.example/cb"}},
		"no redirect":           {"client_name": "x"},
		"custom scheme":         {"redirect_uris": []string{"myapp://cb"}},
		"client secret":         {"redirect_uris": []string{"http://127.0.0.1/cb"}, "token_endpoint_auth_method": "client_secret_post"},
		"implicit grant":        {"redirect_uris": []string{"http://127.0.0.1/cb"}, "grant_types": []string{"implicit"}},
		"token response type":   {"redirect_uris": []string{"http://127.0.0.1/cb"}, "response_types": []string{"token"}},
	}
	for name, body := range cases {
		if code, out := postRegister(te, body, "10.0.0.2"); code != http.StatusBadRequest {
			t.Errorf("%s: got %d %v, want 400", name, code, out)
		}
	}
}

func TestDCR_DropsLocalhostKeepsLoopbackIP(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 10)
	code, out := postRegister(te, map[string]any{
		"redirect_uris": []string{"http://127.0.0.1:5000/callback", "http://localhost:5000/callback"},
		"client_name":   "opencode\x1b[31m with a very long name that goes on and on and on and on and on and on",
	}, "10.0.0.3")
	if code != http.StatusCreated {
		t.Fatalf("got %d %v", code, out)
	}
	uris, _ := out["redirect_uris"].([]any)
	if len(uris) != 1 || uris[0] != "http://127.0.0.1:5000/callback" {
		t.Fatalf("redirect_uris = %v", uris)
	}
	name, _ := out["client_name"].(string)
	if strings.ContainsRune(name, '\x1b') || len([]rune(name)) > maxDCRClientName {
		t.Fatalf("client_name not sanitized: %q", name)
	}
}

func TestDCR_RateLimitAndCap(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 100, 2)
	for i := 0; i < 2; i++ {
		if code, _ := postRegister(te, opencodeRegistration(), "10.0.0.4"); code != http.StatusCreated {
			t.Fatalf("request %d: got %d", i, code)
		}
	}
	if code, _ := postRegister(te, opencodeRegistration(), "10.0.0.4"); code != http.StatusTooManyRequests {
		t.Fatalf("per-IP limit: got %d", code)
	}
	if code, _ := postRegister(te, opencodeRegistration(), "10.0.0.5"); code != http.StatusCreated {
		t.Fatalf("other IP should pass: got %d", code)
	}

	te = newTestEnv(t)
	enableDCR(te, 2, 100)
	for i := 0; i < 2; i++ {
		if code, _ := postRegister(te, opencodeRegistration(), "10.0.0.6"); code != http.StatusCreated {
			t.Fatalf("request %d: got %d", i, code)
		}
	}
	if code, out := postRegister(te, opencodeRegistration(), "10.0.0.7"); code != http.StatusServiceUnavailable {
		t.Fatalf("global cap: got %d %v", code, out)
	}
}

func TestDCR_RefreshHonoursRegisteredGrantTypes(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	enableDCR(te, 10, 10)
	reg := func(grants []string) string {
		body := opencodeRegistration()
		if grants == nil {
			delete(body, "grant_types")
		} else {
			body["grant_types"] = grants
		}
		_, out := postRegister(te, body, "10.0.0.8")
		id, _ := out["client_id"].(string)
		return id
	}
	ctx := t.Context()
	if !te.gw.clientMayRefresh(ctx, reg([]string{"authorization_code", "refresh_token"})) {
		t.Fatal("OpenCode-style registration must be allowed to refresh")
	}
	if te.gw.clientMayRefresh(ctx, reg([]string{"authorization_code"})) {
		t.Fatal("authorization_code-only client must not refresh")
	}
	if te.gw.clientMayRefresh(ctx, reg(nil)) {
		t.Fatal("omitted grant_types defaults to authorization_code only")
	}

	// End to end: the token endpoint refuses the grant before touching the token.
	id := reg([]string{"authorization_code"})
	code, out := te.postToken(url.Values{"grant_type": {"refresh_token"}, "client_id": {id}, "refresh_token": {"opaque"}})
	if code != http.StatusBadRequest || out["error"] != "unauthorized_client" {
		t.Fatalf("got %d %v, want 400 unauthorized_client", code, out)
	}
}
