package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

var updateRouteGolden = flag.Bool("update-route-golden", false, "rewrite testdata/routetable_golden.txt")

// TestRouteDispatchGolden pins the status, key headers and body of route
// dispatch, PRM, /authorize and /token resource checks. The golden file was
// recorded on origin/main before RouteTable (#2334), so it proves the move
// changed nothing a client can see.
func TestRouteDispatchGolden(t *testing.T) {
	te := newTestEnv(t)
	te.gw.store.Close()

	off := false
	cfg := *te.gw.cfg
	cfg.Routes = []PilotRoute{
		te.gw.cfg.Routes[0],
		{Route: "/mcp/team-a/shared", Team: "team-a", Target: "dev-gcp:team-a:shared", UpstreamURL: "http://shared.team-a"},
		{Route: "/mcp/team-b/shared", Team: "team-b", Target: "dev-gcp:team-b:shared", UpstreamURL: "http://shared.team-b"},
		{Route: "/mcp/team-c/off", Team: "team-c", Target: "dev-gcp:team-c:off", UpstreamURL: "http://off.team-c", Enabled: &off},
	}
	cfg.RTKeys = "k1:" + testKeyBase64(t, false)
	cfg.RTStorage = rtStorageClientEncrypted
	cfg.EntraScope = "openid offline_access api://azure-client-id/.default"
	cfg.AllowedResources, cfg.RoutesByPath, cfg.PrimaryResource = nil, nil, ""
	if err := validateConfig(&cfg); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
	upstream, err := url.Parse(te.upstreamServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	for i := range cfg.Routes {
		cfg.Routes[i].ParsedURL = upstream
	}
	gw, err := newGateway(&cfg, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway: %v", err)
	}
	t.Cleanup(gw.store.Close)
	te.gw = gw
	te.handler = gw.routes()

	var out strings.Builder
	record := func(name string, rec *httptest.ResponseRecorder) {
		res := rec.Result()
		body, _ := io.ReadAll(res.Body)
		fmt.Fprintf(&out, "== %s\nstatus: %d\n", name, res.StatusCode)
		for _, h := range []string{"Content-Type", "Allow", "Retry-After", "WWW-Authenticate", "Cache-Control"} {
			if v := res.Header.Get(h); v != "" {
				fmt.Fprintf(&out, "%s: %s\n", h, v)
			}
		}
		if loc := res.Header.Get("Location"); loc != "" {
			// State and nonce are random: keep only where it goes and any error.
			u, _ := url.Parse(loc)
			host := strings.NewReplacer(strings.TrimPrefix(te.entraServer.URL, "http://"), "ENTRA").Replace(u.Host)
			fmt.Fprintf(&out, "Location: %s://%s%s error=%q\n", u.Scheme, host, u.Path, u.Query().Get("error")) //nolint:gosec // strings.Builder, not HTTP
		} else {
			fmt.Fprintf(&out, "body: %s\n", strings.TrimSpace(string(body)))
		}
	}
	do := func(method, target, bearer string) *httptest.ResponseRecorder {
		var body io.Reader
		if method == http.MethodPost {
			body = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
		}
		req := httptest.NewRequest(method, target, body)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		return rec
	}

	paths := []string{
		"/mcp", "/mcp/",
		"/mcp/etterlevelse", "/mcp/etterlevelse/", "/mcp/shared", "/mcp/off", "/mcp/unknown",
		"/mcp/team-etterlevelse/etterlevelse", "/mcp/team-etterlevelse/etterlevelse/",
		"/mcp/team-a/shared", "/mcp/team-c/off", "/mcp/team-x/unknown", "/mcp/team-a/unknown",
		"/mcp/a/b/c",
	}
	for _, p := range paths {
		for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut} {
			record(m+" "+p, do(m, p, ""))
		}
	}

	at, rt, _ := te.obtainTokensForResource(te.resource)
	for _, p := range []string{"/mcp/team-etterlevelse/etterlevelse", "/mcp/team-a/shared", "/mcp/team-c/off"} {
		for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut} {
			record("bearer "+m+" "+p, do(m, p, at))
		}
	}

	prm := "/.well-known/oauth-protected-resource"
	for _, p := range []string{"", "/mcp/team-etterlevelse/etterlevelse", "/mcp/team-a/shared", "/mcp/team-b/shared", "/mcp/team-c/off", "/mcp/team-x/unknown", "/mcp/etterlevelse"} {
		record("GET "+prm+p, do(http.MethodGet, prm+p, ""))
	}

	for _, p := range []string{"/mcp/team-etterlevelse/etterlevelse", "/mcp/team-a/shared", "/mcp/team-c/off", "/mcp/team-x/unknown", "/mcp/etterlevelse"} {
		target := te.authorizeURL(map[string]string{"resource": cfg.BaseURL + p}, "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
		record("GET /authorize resource="+p, do(http.MethodGet, target, ""))
	}

	for _, res := range []string{"", te.resource, cfg.BaseURL + "/mcp/team-a/shared"} {
		form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {rt}, "client_id": {te.clientID}}
		if res != "" {
			form.Set("resource", res)
		}
		status, body := te.postToken(form)
		if status == http.StatusOK {
			rt, _ = body["refresh_token"].(string)
			body = map[string]any{"token_type": body["token_type"]}
		}
		fmt.Fprintf(&out, "== POST /token refresh resource=%q\nstatus: %d\nbody: %v\n", res, status, body) //nolint:gosec // strings.Builder, not HTTP
	}

	const golden = "testdata/routetable_golden.txt"
	if *updateRouteGolden {
		if err = os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(golden, []byte(out.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update-route-golden on origin/main): %v", err)
	}
	if got := out.String(); got != string(want) {
		t.Errorf("route dispatch drifted from %s:\n%s", golden, firstDiff(string(want), got))
	}
}

func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d\nwant: %s\ngot:  %s", i+1, wl, gl)
		}
	}
	return "length differs"
}
