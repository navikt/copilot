package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/oauth/server"
	"github.com/navikt/copilot/fleet-gateway/internal/ratelimit"
)

// enableDCR turns on DCR for a test env with a fresh store holding capDCR clients.
func enableDCR(te *testEnv, capDCR, perIP int) {
	te.gw.cfg.DCREnabled = true
	te.gw.cfg.DCRClientTTL = 24 * time.Hour
	te.gw.cfg.DCRClientMaxAge = 72 * time.Hour
	te.gw.cfg.DCRRateLimitRequests = perIP
	te.gw.cfg.DCRRateLimitGlobal = 1000
	te.gw.store = newMemoryStore(te.gw.now, map[string]int{kindDCRClient: capDCR})
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
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(page), "uverifisert verktøy") || !strings.Contains(string(page), "OpenCode") {
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
	if known, _, _ := te.gw.oauth().ClientGrants(t.Context(), clientID); known {
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
		"refresh only":          {"redirect_uris": []string{"http://127.0.0.1/cb"}, "grant_types": []string{"refresh_token"}},
		"code plus implicit":    {"redirect_uris": []string{"http://127.0.0.1/cb"}, "grant_types": []string{"authorization_code", "implicit"}},
		"token response type":   {"redirect_uris": []string{"http://127.0.0.1/cb"}, "response_types": []string{"token"}},
		"web app loopback":      {"redirect_uris": []string{"http://127.0.0.1/cb"}, "application_type": "web"},
		"web app https":         {"redirect_uris": []string{"https://evil.example/cb"}, "application_type": "web"},
		"empty app type":        {"redirect_uris": []string{"http://127.0.0.1/cb"}, "application_type": ""},
		"unknown app type":      {"redirect_uris": []string{"http://127.0.0.1/cb"}, "application_type": "Native"},
		"non-string app type":   {"redirect_uris": []string{"http://127.0.0.1/cb"}, "application_type": 1},
		"native custom scheme":  {"redirect_uris": []string{"myapp://cb"}, "application_type": "native"},
		"native non-loopback":   {"redirect_uris": []string{"https://evil.example/cb"}, "application_type": "native"},
	}
	for name, body := range cases {
		if code, out := postRegister(te, body, "10.0.0.2"); code != http.StatusBadRequest {
			t.Errorf("%s: got %d %v, want 400", name, code, out)
		}
	}
}

func TestDCR_ApplicationTypeNativeOrAbsent(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 10)
	for name, body := range map[string]map[string]any{
		"native": {"redirect_uris": []string{"http://127.0.0.1:5000/cb"}, "application_type": "native"},
		"absent": {"redirect_uris": []string{"http://127.0.0.1:5000/cb"}},
	} {
		code, out := postRegister(te, body, "10.0.0.4")
		if code != http.StatusCreated || out["application_type"] != "native" {
			t.Errorf("%s: got %d %v, want 201 with application_type native", name, code, out)
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
	if strings.ContainsRune(name, '\x1b') || len([]rune(name)) > server.MaxDCRClientName {
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

}

func registerID(t *testing.T, te *testEnv, ip string) string {
	t.Helper()
	code, out := postRegister(te, opencodeRegistration(), ip)
	if code != http.StatusCreated {
		t.Fatalf("register from %s: got %d %v", ip, code, out)
	}
	id, _ := out["client_id"].(string)
	return id
}

func clientKnown(t *testing.T, te *testEnv, id string) bool {
	t.Helper()
	known, _, err := te.gw.oauth().ClientGrants(t.Context(), id)
	if err != nil {
		t.Fatalf("clientGrants: %v", err)
	}
	return known
}

// A full store evicts the least recently renewed client instead of refusing.
func TestDCR_FullCapEvictsLeastRecentlyRenewed(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 2, 100)
	a := registerID(t, te, "10.0.0.6")
	te.advance(time.Hour)
	b := registerID(t, te, "10.0.0.6")
	te.advance(time.Hour)
	te.gw.oauth().RenewClient(t.Context(), a) // a token exchange renews a past b
	c := registerID(t, te, "10.0.0.7")
	if !clientKnown(t, te, a) || !clientKnown(t, te, c) {
		t.Fatalf("a and c should be live after eviction")
	}
	if clientKnown(t, te, b) {
		t.Fatalf("b was least recently renewed and should be evicted")
	}
}

// A token exchange slides the TTL; DCRClientMaxAge still ends the client.
func TestDCR_TTLRenewedOnUseBoundedByMaxAge(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 10)
	id := registerID(t, te, "10.0.0.8")
	for elapsed := 20 * time.Hour; elapsed < 72*time.Hour; elapsed += 20 * time.Hour {
		te.advance(20 * time.Hour)
		if !clientKnown(t, te, id) {
			t.Fatalf("client renewed every 20h expired after %s; TTL is 24h", elapsed)
		}
		te.gw.oauth().RenewClient(t.Context(), id)
	}
	te.advance(20 * time.Hour) // 80h > 72h max age
	if clientKnown(t, te, id) {
		t.Fatalf("client must expire at max age despite use")
	}
}

// Unauthenticated /authorize hits must not keep a registered client alive.
func TestDCR_AuthorizeDoesNotRenew(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 10)
	id := registerID(t, te, "10.0.0.9")
	for range 2 {
		te.advance(20 * time.Hour)
		u := te.authorizeURL(map[string]string{"client_id": id, "redirect_uri": "http://127.0.0.1:40001/mcp/oauth/callback"}, "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
		te.handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, u, nil))
	}
	if clientKnown(t, te, id) {
		t.Fatalf("client kept alive by /authorize past its 24h TTL")
	}
}

// /register sits behind the in-memory per-IP limiter before the store-backed one.
func TestDCR_RegisterBehindInMemoryRateLimit(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 100, 100)
	te.gw.limiters[metrics.EndpointRegister] = ratelimit.New(1, 100, time.Minute, 10, func() time.Time { return te.now })
	te.handler = te.gw.routes()
	registerID(t, te, "10.0.2.1")
	code, out := postRegister(te, opencodeRegistration(), "10.0.2.1")
	if code != http.StatusTooManyRequests || out["error"] != "temporarily_unavailable" {
		t.Fatalf("second register from same IP: got %d %v, want 429 temporarily_unavailable", code, out)
	}
}

func TestDCR_AllowRegistrationRejectionIncrementsMetric(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 100, 1)
	registerID(t, te, "10.0.2.2")

	before := testutil.ToFloat64(rateLimitedSeries(metrics.EndpointRegister))
	code, out := postRegister(te, opencodeRegistration(), "10.0.2.2")
	if code != http.StatusTooManyRequests || out["error"] != "temporarily_unavailable" {
		t.Fatalf("second register from same IP: got %d %v, want 429 temporarily_unavailable", code, out)
	}
	if got := testutil.ToFloat64(rateLimitedSeries(metrics.EndpointRegister)); got != before+1 {
		t.Fatalf("register rate_limited_total = %v, want %v", got, before+1)
	}
}

// Two replicas sharing one store share the limits, and one IP cannot use up
// the global budget.
func TestDCR_RateLimitSharedAcrossReplicas(t *testing.T) {
	te1 := newTestEnv(t)
	enableDCR(te1, 100, 2)
	te2 := newTestEnv(t)
	enableDCR(te2, 100, 2)
	te2.gw.store = te1.gw.store
	te2.handler = te2.gw.routes()
	te1.gw.cfg.DCRRateLimitGlobal = 3
	te2.gw.cfg.DCRRateLimitGlobal = 3

	registerID(t, te1, "10.0.1.1")
	registerID(t, te2, "10.0.1.1")
	for i := 0; i < 10; i++ {
		for _, te := range []*testEnv{te1, te2} {
			if code, _ := postRegister(te, opencodeRegistration(), "10.0.1.1"); code != http.StatusTooManyRequests {
				t.Fatalf("per-IP limit must hold across replicas: got %d", code)
			}
		}
	}
	registerID(t, te2, "10.0.1.2") // global 3: one IP took only 2
	if code, _ := postRegister(te1, opencodeRegistration(), "10.0.1.3"); code != http.StatusTooManyRequests {
		t.Fatalf("global limit must hold across replicas: got %d", code)
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
	mayRefresh := func(id string) bool {
		_, ok, err := te.gw.oauth().ClientGrants(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !mayRefresh(reg([]string{"authorization_code", "refresh_token"})) {
		t.Fatal("OpenCode-style registration must be allowed to refresh")
	}
	if mayRefresh(reg([]string{"authorization_code"})) {
		t.Fatal("authorization_code-only client must not refresh")
	}
	if mayRefresh(reg(nil)) {
		t.Fatal("omitted grant_types defaults to authorization_code only")
	}

	// The code grant hands out a gateway refresh token only to clients registered for it.
	te.redirectURI = "http://127.0.0.1:19876/mcp/oauth/callback"
	te.clientID = reg([]string{"authorization_code"})
	if _, rt, _ := te.obtainTokensForResource(te.resource); rt != "" {
		t.Fatal("authorization_code-only DCR client got a refresh token")
	}
	te.clientID = reg([]string{"authorization_code", "refresh_token"})
	if _, rt, _ := te.obtainTokensForResource(te.resource); rt == "" {
		t.Fatal("refresh-registered DCR client got no refresh token")
	}

	// End to end: the token endpoint refuses the grant before touching the token.
	id := reg([]string{"authorization_code"})
	code, out := te.postToken(url.Values{"grant_type": {"refresh_token"}, "client_id": {id}, "refresh_token": {"opaque"}})
	if code != http.StatusBadRequest || out["error"] != "unauthorized_client" {
		t.Fatalf("got %d %v, want 400 unauthorized_client", code, out)
	}
}

func TestDCR_AuthorizeRequiresRegisteredCodeGrant(t *testing.T) {
	te := newTestEnv(t)
	enableDCR(te, 10, 10)
	// A registration stored before grant_types were enforced.
	id := dcrClientIDPrefix + "legacy"
	raw, _ := json.Marshal(server.ClientMetadata{ClientID: id, RedirectURIs: []string{"http://127.0.0.1/mcp/oauth/callback"}, GrantTypes: []string{"refresh_token"}})
	if err := te.gw.store.Put(t.Context(), kindDCRClient, id, raw, time.Hour); err != nil {
		t.Fatal(err)
	}
	u := te.authorizeURL(map[string]string{"client_id": id, "redirect_uri": "http://127.0.0.1:40001/mcp/oauth/callback"}, "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unauthorized_client") {
		t.Fatalf("got %d %s, want 400 unauthorized_client", rec.Code, rec.Body.String())
	}
}

func TestDCR_StoreErrorIsTemporary(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	enableDCR(te, 10, 10)
	_, out := postRegister(te, opencodeRegistration(), "10.0.0.9")
	id, _ := out["client_id"].(string)
	te.redirectURI = "http://127.0.0.1:19876/mcp/oauth/callback"
	te.clientID = id
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	code := te.obtainAuthCode(verifier)

	te.gw.store = &failingStore{SessionStore: te.gw.store, failGet: errors.New("db connection refused")}
	te.handler = te.gw.routes()
	check := func(name string, form url.Values) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "temporarily_unavailable") || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("%s: got %d %s, want 503 temporarily_unavailable with Retry-After", name, rec.Code, rec.Body.String())
		}
	}
	check("refresh", url.Values{"grant_type": {"refresh_token"}, "client_id": {id}, "refresh_token": {"opaque"}})
	check("code", url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {id}, "redirect_uri": {te.redirectURI}, "resource": {te.resource}, "code_verifier": {verifier}})

	// The 503 must not have consumed the code: the retry the client was invited to make succeeds.
	te.gw.store = te.gw.store.(*failingStore).SessionStore
	te.handler = te.gw.routes()
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {id}, "redirect_uri": {te.redirectURI}, "resource": {te.resource}, "code_verifier": {verifier}}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code retry after store recovery: got %d %s, want 200", rec.Code, rec.Body.String())
	}
}

func TestDCR_CodeGrantRejectsUnknownAndUnreadableClient(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	t.Run("evicted or disabled client cannot redeem code", func(t *testing.T) {
		te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
		enableDCR(te, 1, 10)
		te.redirectURI = "http://127.0.0.1:19876/mcp/oauth/callback"

		id1 := registerID(t, te, "10.0.0.10")
		te.clientID = id1
		code1 := te.obtainAuthCode(verifier)

		// Evict id1 by registering another client when cap=1.
		id2 := registerID(t, te, "10.0.0.11")
		te.clientID = id2
		code2 := te.obtainAuthCode(verifier)

		status, body := te.postToken(url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code1},
			"client_id":     {id1},
			"redirect_uri":  {te.redirectURI},
			"resource":      {te.resource},
			"code_verifier": {verifier},
		})
		if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Fatalf("evicted DCR client code exchange: got %d %v, want 400 invalid_grant", status, body)
		}

		// Disable DCR between /authorize and /token.
		te.gw.cfg.DCREnabled = false
		status, body = te.postToken(url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code2},
			"client_id":     {id2},
			"redirect_uri":  {te.redirectURI},
			"resource":      {te.resource},
			"code_verifier": {verifier},
		})
		if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Fatalf("disabled DCR client code exchange: got %d %v, want 400 invalid_grant", status, body)
		}
	})

	t.Run("unreadable dcr_client row from removed RT_KEYS returns 400 invalid_grant", func(t *testing.T) {
		k1 := testKeyBase64(t, false)
		k2 := testKeyBase64(t, false)
		te := newClientEncryptedTestEnv(t, "k1:"+k1)
		te.gw.cfg.DCREnabled = true
		te.gw.cfg.DCRClientTTL = 24 * time.Hour
		te.gw.cfg.DCRClientMaxAge = 72 * time.Hour
		te.gw.cfg.DCRRateLimitRequests = 10
		te.gw.cfg.DCRRateLimitGlobal = 100
		te.handler = te.gw.routes()

		id := registerID(t, te, "10.0.0.12")
		te.redirectURI = "http://127.0.0.1:19876/mcp/oauth/callback"
		te.clientID = id
		code := te.obtainAuthCode(verifier)

		// Rotate RT_KEYS to k2 only: dcr_client row sealed with k1 becomes errSealUnknownKID.
		kr2, err := parseKeyRing("k2:" + k2)
		if err != nil {
			t.Fatal(err)
		}
		te.gw.rtKeys = kr2
		if ss, ok := te.gw.store.(*sealedStore); ok {
			*ss = *ss.WithKeys(kr2)
		} else {
			t.Fatalf("expected sealedStore, got %T", te.gw.store)
		}

		status, body := te.postToken(url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {id},
			"redirect_uri":  {te.redirectURI},
			"resource":      {te.resource},
			"code_verifier": {verifier},
		})
		if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Fatalf("unreadable dcr_client code exchange: got %d %v, want 400 invalid_grant", status, body)
		}

		status, body = te.postToken(url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {"opaque"},
			"client_id":     {id},
		})
		if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Fatalf("unreadable dcr_client refresh: got %d %v, want 400 invalid_grant", status, body)
		}
	})
}
