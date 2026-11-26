package probe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/ratelimit"
	"github.com/navikt/copilot/fleet-gateway/internal/teamtoken"
)

const (
	testIssuer = "https://login.microsoftonline.com/tenant/v2.0"
	testAud    = "gateway-client-id"
	testAzp    = "my-copilot-client-id"
)

var testNow = time.Unix(1_800_000_000, 0)

type fakeMembers struct {
	teams map[string]bool
	err   error
}

func (f fakeMembers) IsMember(_ context.Context, team, email string) (bool, error) {
	return f.teams[team] && email == "ola@nav.no", f.err
}

type fakeUpstream struct {
	*httptest.Server
	mu      sync.Mutex
	methods []string
	session []string
	auth    []string
	// requireAuth answers 401 to a request without Authorization, as an Entra-protected server does.
	requireAuth bool
}

// newUpstream answers initialize and tools/list; tools is the tools/list result.
func newUpstream(t *testing.T, tools string) *fakeUpstream {
	f := &fakeUpstream{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&msg)
		f.mu.Lock()
		f.methods = append(f.methods, msg.Method)
		f.session = append(f.session, r.Header.Get("Mcp-Session-Id"))
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		if f.requireAuth && r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if msg.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		result := `{"protocolVersion":"2025-06-18","capabilities":{"tools":{}}}`
		if msg.Method == "tools/list" {
			result = tools
		} else {
			w.Header().Set("Mcp-Session-Id", "sess-1")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+itoa(msg.ID)+`,"result":`+result+`}`)
	}))
	t.Cleanup(f.Close)
	return f
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func (f *fakeUpstream) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.methods...)
}

func goodClaims() map[string]any {
	return map[string]any{
		"active": true, "iss": testIssuer, "aud": testAud, "azp": testAzp,
		"exp": testNow.Add(time.Hour).Unix(), "NAVident": "Z123456", "preferred_username": "Ola@nav.no",
	}
}

func newTexas(t *testing.T, claims map[string]any, status int) string {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(claims)
	}))
	t.Cleanup(s.Close)
	return s.URL
}

type opts struct {
	claims  map[string]any
	texas   int
	members Members
	limit   int
	targets *[]string // records each exchange target
	xerr    error     // exchange error
}

// fakeExchange returns "obo:<target>" so tests can see which token went upstream.
func fakeExchange(seen *[]string, xerr error) Exchange {
	return func(_ context.Context, userToken, target, _ string) (string, error) {
		if xerr != nil {
			return "", xerr
		}
		if userToken != "tok" {
			return "", errors.New("wrong user token")
		}
		if seen != nil {
			*seen = append(*seen, target)
		}
		return "obo:" + target, nil
	}
}

// clientTo is the probe's upstream client with every dial sent to addr (the
// httptest server), so the real validator and host rules run on the URL.
func clientTo(addr string) *http.Client {
	c := newUpstreamClient(func(netip.Addr) bool { return true })
	d := &net.Dialer{}
	c.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return d.DialContext(ctx, network, addr)
	}
	return c
}

// newHandler builds a handler whose upstream client dials the httptest server.
func newHandler(t *testing.T, up *fakeUpstream, o opts) *Handler {
	t.Helper()
	if o.claims == nil {
		o.claims = goodClaims()
	}
	if o.texas == 0 {
		o.texas = http.StatusOK
	}
	if o.members == nil {
		o.members = fakeMembers{teams: map[string]bool{"team-a": true}}
	}
	if o.limit == 0 {
		o.limit = 100
	}
	h, err := New(Config{
		IntrospectURL: newTexas(t, o.claims, o.texas), Issuer: testIssuer, Audience: testAud, AllowedAzp: testAzp,
		Members: o.members, Limiter: ratelimit.New(o.limit, 1000, time.Minute, 100, func() time.Time { return testNow }),
		Cluster: "dev-gcp", Exchange: fakeExchange(o.targets, o.xerr),
		Now: func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatal(err)
	}
	if up != nil {
		h.upstream = clientTo(up.Listener.Addr().String())
	}
	return h
}

func do(h http.Handler, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/probe/v1/tools", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const okBody = `{"team":"team-a","upstream_url":"http://app.team-a"}`

// kind returns the error kind of a failure answer.
func kind(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct{ Error string }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("failure body %q is not {error}: %v", rec.Body, err)
	}
	return out.Error
}

const twoTools = `{"tools":[{"name":"search","description":"d","annotations":{"readOnlyHint":true}},{"name":"get.item"}]}`

func TestProbeListsTools(t *testing.T) {
	up := newUpstream(t, twoTools)
	var targets []string
	rec := do(newHandler(t, up, opts{targets: &targets}), "tok", okBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out struct{ Tools []Tool }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Tools) != 2 || out.Tools[0].Name != "search" {
		t.Fatalf("tools = %+v, err %v", out.Tools, err)
	}
	if got := strings.Join(up.seen(), ","); got != "initialize,notifications/initialized,tools/list" {
		t.Fatalf("upstream saw %q, want initialize,notifications/initialized,tools/list only", got)
	}
	if up.session[1] != "sess-1" || up.session[2] != "sess-1" {
		t.Fatalf("sessions %q, want sess-1 after initialize", up.session)
	}
	if len(targets) != 0 {
		t.Fatalf("exchange targets %q, want none: the target did not ask for login", targets)
	}
	for _, a := range up.auth {
		if a != "" {
			t.Fatalf("upstream Authorization %q, want none", a)
		}
	}
}

// Q2: a 401 from the target makes the probe retry as the user with an OBO token for that app.
func TestProbeExchangesOBOOn401(t *testing.T) {
	up := newUpstream(t, twoTools)
	up.requireAuth = true
	var targets []string
	rec := do(newHandler(t, up, opts{targets: &targets}), "tok", okBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if strings.Join(targets, ",") != "dev-gcp:team-a:app" {
		t.Fatalf("exchange targets %q, want dev-gcp:team-a:app", targets)
	}
	want := []string{"", "Bearer obo:dev-gcp:team-a:app", "Bearer obo:dev-gcp:team-a:app", "Bearer obo:dev-gcp:team-a:app"}
	if strings.Join(up.auth, "|") != strings.Join(want, "|") {
		t.Fatalf("upstream Authorization %q, want %q: never the caller's own token", up.auth, want)
	}
}

func TestProbeOBOFailures(t *testing.T) {
	t.Run("still 401", func(t *testing.T) {
		up := &fakeUpstream{Server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))}
		t.Cleanup(up.Close)
		rec := do(newHandler(t, up, opts{}), "tok", okBody)
		if rec.Code != http.StatusBadGateway || kind(t, rec) != "access" {
			t.Fatalf("status %d body %s, want 502 access", rec.Code, rec.Body)
		}
	})
	for name, tc := range map[string]struct {
		err  error
		kind string
	}{
		"access missing": {&teamtoken.AccessMissingError{App: "app"}, "access"},
		"texas down":     {errors.New("texas exchange request failed"), "unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			up := newUpstream(t, twoTools)
			up.requireAuth = true
			rec := do(newHandler(t, up, opts{xerr: tc.err}), "tok", okBody)
			if rec.Code != http.StatusBadGateway || kind(t, rec) != tc.kind {
				t.Fatalf("status %d body %s, want 502 %s", rec.Code, rec.Body, tc.kind)
			}
			if got := up.seen(); len(got) != 1 {
				t.Fatalf("upstream saw %v, want only the first initialize", got)
			}
		})
	}
}

// Every upstream failure answers "access" (#3078): distinct kinds would tell a
// caller which services in other namespaces exist and answer.
func TestProbeUpstreamKinds(t *testing.T) {
	for code, want := range map[int]string{
		http.StatusForbidden: "access", http.StatusNotFound: "access",
		http.StatusInternalServerError: "access", http.StatusTeapot: "access",
	} {
		up := &fakeUpstream{Server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))}
		t.Cleanup(up.Close)
		if rec := do(newHandler(t, up, opts{}), "tok", okBody); kind(t, rec) != want {
			t.Errorf("upstream %d: body %s, want %s", code, rec.Body, want)
		}
	}
	h := newHandler(t, nil, opts{})
	h.upstream = &http.Client{Transport: errTransport{&net.DNSError{Err: "no such host", Name: "app", IsNotFound: true}}}
	if rec := do(h, "tok", okBody); kind(t, rec) != "access" {
		t.Errorf("no such app: body %s, want access", rec.Body)
	}
	h.upstream = &http.Client{Transport: errTransport{errors.New("connection refused")}}
	if rec := do(h, "tok", okBody); kind(t, rec) != "access" {
		t.Errorf("refused: body %s, want access", rec.Body)
	}
}

func TestProbeSlowUpstream(t *testing.T) {
	release := make(chan struct{})
	up := &fakeUpstream{Server: httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))}
	t.Cleanup(func() { close(release); up.Close() })
	h := newHandler(t, up, opts{})
	h.timeout = 100 * time.Millisecond
	rec := do(h, "tok", okBody)
	if rec.Code != http.StatusBadGateway || kind(t, rec) != "access" {
		t.Fatalf("status %d body %s, want 502 access", rec.Code, rec.Body)
	}
}

func TestProbeTokenGuards(t *testing.T) {
	cases := map[string]func(map[string]any){
		"inactive":  func(c map[string]any) { c["active"] = false },
		"wrong iss": func(c map[string]any) { c["iss"] = "https://login.microsoftonline.com/other/v2.0" },
		"wrong aud": func(c map[string]any) { c["aud"] = "other-app" },
		"aud array": func(c map[string]any) { c["aud"] = []string{"other-app"} },
		"wrong azp": func(c map[string]any) { c["azp"] = "nav-pilot-client-id" },
		"expired":   func(c map[string]any) { c["exp"] = testNow.Unix() },
		"app token": func(c map[string]any) { c["idtyp"] = "app" },
		"no ident":  func(c map[string]any) { delete(c, "NAVident") },
		"no email":  func(c map[string]any) { delete(c, "preferred_username") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			up := newUpstream(t, twoTools)
			c := goodClaims()
			mutate(c)
			if rec := do(newHandler(t, up, opts{claims: c}), "tok", okBody); rec.Code != http.StatusUnauthorized || kind(t, rec) != "auth" {
				t.Fatalf("status %d body %s, want 401 auth", rec.Code, rec.Body)
			}
			if len(up.seen()) != 0 {
				t.Fatal("upstream reached with a bad token")
			}
		})
	}
	t.Run("aud array with gateway", func(t *testing.T) {
		c := goodClaims()
		c["aud"] = []string{"x", testAud}
		if rec := do(newHandler(t, newUpstream(t, twoTools), opts{claims: c}), "tok", okBody); rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
	})
	t.Run("no bearer", func(t *testing.T) {
		if rec := do(newHandler(t, nil, opts{}), "", okBody); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d, want 401", rec.Code)
		}
	})
	t.Run("texas down", func(t *testing.T) {
		if rec := do(newHandler(t, nil, opts{texas: http.StatusBadGateway}), "tok", okBody); rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status %d, want 503", rec.Code)
		}
	})
}

func TestProbeMembership(t *testing.T) {
	up := newUpstream(t, twoTools)
	body := `{"team":"team-b","upstream_url":"http://app.team-b"}`
	if rec := do(newHandler(t, up, opts{}), "tok", body); rec.Code != http.StatusForbidden || kind(t, rec) != "access" {
		t.Fatalf("other team: status %d, want 403", rec.Code)
	}
	failing := fakeMembers{teams: map[string]bool{"team-a": true}, err: errors.New("console down")}
	if rec := do(newHandler(t, up, opts{members: failing}), "tok", okBody); rec.Code != http.StatusServiceUnavailable || kind(t, rec) != "unavailable" {
		t.Fatalf("console down: status %d, want 503", rec.Code)
	}
	if len(up.seen()) != 0 {
		t.Fatal("upstream reached without membership")
	}
}

func TestProbeRateLimitPerUser(t *testing.T) {
	h := newHandler(t, newUpstream(t, twoTools), opts{limit: 2})
	for i := range 2 {
		if rec := do(h, "tok", okBody); rec.Code != http.StatusOK {
			t.Fatalf("probe %d: status %d", i, rec.Code)
		}
	}
	if rec := do(h, "tok", okBody); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third probe: status %d, want 429", rec.Code)
	}
}

// SSRF: the real registration validator refuses these before any dial.
func TestProbeRefusesTargets(t *testing.T) {
	for name, target := range map[string]string{
		"external host":    "https://example.com/mcp",
		"other namespace":  "http://app.team-b",
		"other ns fqdn":    "http://app.team-b.svc.cluster.local",
		"http needs https": "http://app.intern.nav.no/mcp",
		"metadata ip":      "http://169.254.169.254/",
		"private ip":       "http://10.0.0.1/",
		"localhost":        "http://localhost:8080/",
		"bad team (path)":  "",
		"ekstern ingress":  "https://app.ekstern.dev.nav.no/mcp",
		// Internal ingresses pass registration but are not tied to the team's namespace.
		"intern ingress": "https://app.intern.dev.nav.no/mcp",
		"non-ascii host": "http://\u0430pp.team-a",
		"punycode":       "http://xn--pp-8cd.team-a",
		"kelvin sign":    "http://Kpp.team-a", // ToLower folds U+212A to "k"
		"nested label":   "http://a.b.team-a",
		"suffix trick":   "http://app.team-a.evil.com",
	} {
		t.Run(name, func(t *testing.T) {
			up := newUpstream(t, twoTools)
			body := `{"team":"team-a","upstream_url":"` + target + `"}`
			want := http.StatusUnprocessableEntity
			if target == "" {
				body, want = `{"team":"team-a/x","upstream_url":"http://app.team-a"}`, http.StatusBadRequest
			}
			if rec := do(newHandler(t, up, opts{}), "tok", body); rec.Code != want || kind(t, rec) != "invalid" {
				t.Fatalf("status %d, want %d invalid: %s", rec.Code, want, rec.Body)
			}
			if len(up.seen()) != 0 {
				t.Fatal("upstream reached")
			}
		})
	}
}

func TestProbeDoesNotFollowRedirects(t *testing.T) {
	inner := newUpstream(t, twoTools)
	redirector := &fakeUpstream{Server: httptest.NewServer(http.RedirectHandler(inner.URL, http.StatusTemporaryRedirect))}
	t.Cleanup(redirector.Close)
	if rec := do(newHandler(t, redirector, opts{}), "tok", okBody); rec.Code != http.StatusBadGateway || kind(t, rec) != "access" {
		t.Fatalf("status %d, want 502 access", rec.Code)
	}
	if len(inner.seen()) != 0 {
		t.Fatal("redirect followed")
	}
}

// A name that resolves to loopback, link-local or a public address is
// refused at dial time, which covers DNS rebinding.
func TestProbeDialGuard(t *testing.T) {
	for ip, want := range map[string]bool{
		"10.1.2.3": true, "172.16.0.1": true, "192.168.1.1": true, "fd00::1": true,
		"127.0.0.1": false, "::1": false, "169.254.169.254": false, "8.8.8.8": false, "0.0.0.0": false, "::ffff:127.0.0.1": false,
	} {
		if got := privateOnly(netip.MustParseAddr(ip).Unmap()); got != want {
			t.Errorf("privateOnly(%s) = %v, want %v", ip, got, want)
		}
	}
	up := newUpstream(t, twoTools)
	resp, err := newUpstreamClient(privateOnly).Get(up.URL)
	if err == nil {
		_ = resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("dial to loopback: err %v, want refused", err)
	}
	if len(up.seen()) != 0 {
		t.Fatal("loopback reached")
	}
}

type errTransport struct{ err error }

func (e errTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, e.err }

// The body takes no app: the token's app always comes from the host.
func TestProbeRefusesAppField(t *testing.T) {
	var targets []string
	rec := do(newHandler(t, nil, opts{targets: &targets}), "tok", `{"team":"team-a","upstream_url":"http://mcp.team-a","app":"other"}`)
	if rec.Code != http.StatusBadRequest || len(targets) != 0 {
		t.Fatalf("status %d, exchanges %q, want 400 and none", rec.Code, targets)
	}
}

func TestProbeForwardsOnlyInitializeAndToolsList(t *testing.T) {
	up := newUpstream(t, twoTools)
	h := newHandler(t, up, opts{})
	for _, m := range []string{"tools/call", "resources/read", "notifications/cancelled"} {
		if _, _, err := h.call(context.Background(), up.URL, "t", "", 1, m, nil); err == nil {
			t.Fatalf("%s forwarded", m)
		}
	}
	if len(up.seen()) != 0 {
		t.Fatalf("upstream saw %v", up.seen())
	}
}

func TestProbeRefusesOversizedResponse(t *testing.T) {
	big := `{"tools":[{"name":"x","description":"` + strings.Repeat("a", MaxResponseBytes) + `"}]}`
	up := newUpstream(t, big)
	h := newHandler(t, up, opts{})
	if rec := do(h, "tok", okBody); rec.Code != http.StatusBadGateway || kind(t, rec) != "access" {
		t.Fatalf("status %d body %s, want 502 access", rec.Code, rec.Body)
	}
	// The cap itself, not a JSON error from a truncated body.
	if _, _, err := h.call(context.Background(), up.URL, "t", "", 1, "tools/list", nil); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("err %v, want the size cap", err)
	}
}

func TestProbeRefusesBadToolName(t *testing.T) {
	bad := `{"tools":[{"name":"ok"},{"name":"bad name"}]}`
	if rec := do(newHandler(t, newUpstream(t, bad), opts{}), "tok", okBody); rec.Code != http.StatusBadGateway || kind(t, rec) != "access" {
		t.Fatalf("status %d, want 502 access", rec.Code)
	}
}

func TestProbeReadsSSEAnswer(t *testing.T) {
	up := &fakeUpstream{}
	up.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&msg)
		w.Header().Set("Content-Type", "text/event-stream")
		result := `{}`
		if msg.Method == "tools/list" {
			result = twoTools
		}
		_, _ = io.WriteString(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":"+itoa(msg.ID)+",\"result\":"+result+"}\n\n")
	}))
	t.Cleanup(up.Close)
	if rec := do(newHandler(t, up, opts{}), "tok", okBody); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestNewRequiresConfig(t *testing.T) {
	if _, err := New(Config{IntrospectURL: "http://texas", Issuer: testIssuer, Audience: testAud}); err == nil {
		t.Fatal("New without azp, members and limiter must fail")
	}
}

// A server that keeps the SSE stream open after its answer must not stall the probe.
func TestProbeSSEStreamLeftOpen(t *testing.T) {
	up := &fakeUpstream{}
	up.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&msg)
		if msg.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		result := `{}`
		if msg.Method == "tools/list" {
			result = twoTools
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// An unrelated event first, then the answer, then the stream stays open.
		_, _ = io.WriteString(w, "data: {\"jsonrpc\":\"2.0\",\"id\":99,\"result\":{}}\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":"+itoa(msg.ID)+",\"result\":"+result+"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(up.Close)
	h := newHandler(t, up, opts{})
	h.timeout = 3 * time.Second
	start := time.Now()
	if rec := do(h, "tok", okBody); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("probe waited for the stream to end")
	}
}

func TestProbeRefusesOversizedSSE(t *testing.T) {
	up := &fakeUpstream{}
	up.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: "+strings.Repeat("a", MaxResponseBytes+10)+"\n\n")
	}))
	t.Cleanup(up.Close)
	if rec := do(newHandler(t, up, opts{}), "tok", okBody); rec.Code == http.StatusOK {
		t.Fatalf("oversized SSE answer accepted: %s", rec.Body)
	}
}

func TestProbeRefusesHugeResponseHeaders(t *testing.T) {
	up := &fakeUpstream{}
	inner := newUpstream(t, twoTools) // a valid server, only with oversized headers
	up.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Big", strings.Repeat("a", 100<<10))
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(up.Close)
	if rec := do(newHandler(t, up, opts{}), "tok", okBody); rec.Code == http.StatusOK {
		t.Fatalf("100 KiB response headers accepted: %s", rec.Body)
	}
}

// Console answering partial data plus errors is a failure (503), not membership.
func TestConsoleTeamsGraphQLErrorsAreFailure(t *testing.T) {
	console := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"user":{"teams":{"nodes":[{"team":{"slug":"team-a"}}],"pageInfo":{"hasNextPage":false}}}},"errors":[{"message":"partial failure"}]}`)
	}))
	t.Cleanup(console.Close)
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("sa-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewConsoleTeams(tokenPath)
	m.URL = console.URL
	rec := do(newHandler(t, newUpstream(t, twoTools), opts{members: m}), "tok", okBody)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s, want 503", rec.Code, rec.Body)
	}
}
