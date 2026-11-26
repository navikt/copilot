package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const adminID = "44444444-4444-4444-4444-444444444444"

var testTeams = staticTeams{"team-a": {"A111111", "A222222"}, "team-b": {"B111111"}}

func userTok(ident string) string { return "user:" + adminID + ":" + ident }

func do(t *testing.T, h http.Handler, method, path, tok, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const routeDraft = `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://svc.team-a/mcp","tools":["search"]}}`

func TestValidateDraftRejectsHostileInput(t *testing.T) {
	gw := `{"name":"io.github.navikt/svc","description":"d","version":"1","remotes":[{"type":"streamable-http","url":"https://fleet-gateway.{{domain_internal}}/mcp/team-a/svc"}]}`
	ok := map[string]string{
		"route":            routeDraft,
		"registry only":    `{"registry":{"name":"io.github.navikt/svc","description":"d","version":"1"}}`,
		"route + registry": `{"registry":` + gw + `,` + routeDraft[1:],
	}
	for name, body := range ok {
		var spec draftSpec
		if err := json.Unmarshal([]byte(body), &spec); err != nil {
			t.Fatal(err)
		}
		if _, err := validateDraft("dev-gcp", "team-a", "svc", spec); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := map[string]string{
		"empty":                 `{}`,
		"target in other ns":    `{"route":{"target":"dev-gcp:team-b:svc","upstream_url":"http://svc.team-a/mcp","tools":["search"]}}`,
		"upstream in other ns":  `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://svc.team-b/mcp","tools":["search"]}}`,
		"upstream external":     `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"https://evil.example.com/mcp","tools":["search"]}}`,
		"upstream ip":           `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://169.254.169.254/","tools":["search"]}}`,
		"target other cluster":  `{"route":{"target":"prod-gcp:team-a:svc","upstream_url":"http://svc.team-a/mcp","tools":["search"]}}`,
		"registry no desc":      `{"registry":{"name":"io.github.navikt/svc","version":"1"}}`,
		"registry no version":   `{"registry":{"name":"io.github.navikt/svc","description":"d"}}`,
		"registry num version":  `{"registry":{"name":"io.github.navikt/svc","description":"d","version":1}}`,
		"registry long desc":    `{"registry":{"name":"io.github.navikt/svc","version":"1","description":"` + strings.Repeat("x", 101) + `"}}`,
		"upstream ingress":      `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"https://collector.intern.dev.nav.no/mcp","tools":["search"]}}`,
		"upstream other app":    `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://other.team-a/mcp","tools":["search"]}}`,
		"upstream query":        `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://svc.team-a/mcp?x=1","tools":["search"]}}`,
		"upstream fragment":     `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://svc.team-a/mcp#x","tools":["search"]}}`,
		"bad tool":              `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://svc.team-a/mcp","tools":["a b"]}}`,
		"no tools":              `{"route":{"target":"dev-gcp:team-a:svc","upstream_url":"http://svc.team-a/mcp","tools":[]}}`,
		"registry elsewhere":    `{"registry":{"name":"io.github.navikt/svc","description":"d","version":"1","remotes":[{"url":"https://fleet-gateway.{{domain_internal}}/mcp/team-b/svc"}]},` + routeDraft[1:],
		"gateway without route": `{"registry":` + gw + `}`,
		"route without gateway": `{"registry":{"name":"io.github.navikt/svc","description":"d","version":"1"},` + routeDraft[1:],
		"registry bad url":      `{"registry":{"name":"io.github.navikt/svc","description":"d","version":"1","websiteUrl":"http://x.example.com"},` + routeDraft[1:],
	}
	for name, body := range bad {
		var spec draftSpec
		if err := json.Unmarshal([]byte(body), &spec); err != nil {
			t.Fatal(err)
		}
		if _, err := validateDraft("dev-gcp", "team-a", "svc", spec); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

type failingTeams struct{}

func (failingTeams) Teams(context.Context, string) ([]string, error) {
	return nil, errors.New("teams API down")
}

func (failingTeams) IsMember(context.Context, string, string) (bool, error) {
	return false, errors.New("teams API down")
}

func TestAdminGuardsFailClosed(t *testing.T) {
	srv := newServer() // no database
	srv.cluster = "dev-gcp"
	h := srv.routes(testAuthz(t), adminID, testTeams, nil)
	path := "/admin/v1/jobs/team-a/kill-switch"
	cases := map[string]struct {
		h    http.Handler
		path string
		tok  string
		want int
	}{
		"admin routes off":   {srv.routes(testAuthz(t), "", testTeams, nil), path, userTok("A111111"), 404},
		"no token":           {h, path, "", 401},
		"app token":          {h, path, adminID, 401},
		"app token w/ ident": {h, path, "app-with-ident", 401},
		"other client":       {h, path, "user:" + gatewayID + ":A111111", 401},
		"bad NAVident":       {h, path, userTok("a111111"), 401},
		"texas fails":        {h, path, "texas-500", 503},
		"bad namespace":      {h, "/admin/v1/jobs/Team_A/kill-switch", userTok("A111111"), 400},
		"not a member":       {h, path, userTok("B111111"), 403},
		"team lookup fails":  {srv.routes(testAuthz(t), adminID, failingTeams{}, nil), path, userTok("A111111"), 503},
		"database down":      {h, path, userTok("A111111"), 503},
		"unset static teams": {srv.routes(testAuthz(t), adminID, staticTeams{}, nil), path, userTok("A111111"), 403},
	}
	for name, c := range cases {
		if rec := do(t, c.h, http.MethodPut, c.path, c.tok, `{"scope":"team","on":false}`); rec.Code != c.want {
			t.Errorf("%s: got %d, want %d (%s)", name, rec.Code, c.want, rec.Body)
		}
	}
}

func TestHTTPServerBoundsBodyReads(t *testing.T) {
	if s := newHTTPServer("0", http.NotFoundHandler()); s.ReadTimeout <= 0 || s.ReadHeaderTimeout <= 0 {
		t.Errorf("read timeouts must be set: %v %v", s.ReadTimeout, s.ReadHeaderTimeout)
	}
}

// The size check runs inside approval and rollback, and the routes snapshot
// lock stops two approvals of different servers that each fit alone from
// passing the limit together (#2687).
func TestRealPostgresRoutesSizeLimit(t *testing.T) {
	e := newApisEnv(t)
	ctx := context.Background()
	routes, allowlist := readSeedFiles(t, "prod")
	servers, err := seedServers("prod-gcp", routes, allowlist)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.db.Seed(ctx, servers, "test"); err != nil {
		t.Fatal(err)
	}
	srv := e.srv
	a, b := person("A111111"), person("P999999")
	base := nsBase + "team-a/mcpservers/"
	// draft submits name and returns the approval body for its pending generation.
	draft := func(name string, tools ...string) string {
		code, out := e.do("GET", base+name, a, "")
		var m MCPServer
		if code == 404 {
			m = decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", a, serverJSON("team-a", name, "", tools...)))
		} else {
			cur := decodeServer(t, out)
			m = decodeServer(t, e.want(200, "PUT", base+name, a, serverJSON("team-a", name, cur.Metadata.ResourceVersion, tools...)))
		}
		return approval(int(m.Metadata.Generation), m.Status.Pending.ContentHash)
	}
	approve := func(name, body string) int {
		code, _ := e.do("POST", base+name+"/approval", b, body)
		return code
	}
	audits := func() (n int) {
		_ = e.db.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&n)
		return n
	}
	snap, err := srv.resolve(ctx, "routes")
	if err != nil {
		t.Fatal(err)
	}
	old := maxRoutesSnapshotBytes
	t.Cleanup(func() { maxRoutesSnapshotBytes = old })

	// Too big alone: refused, nothing published or audited.
	maxRoutesSnapshotBytes = len(snap.Items)
	big := draft("big", "search")
	before := audits()
	if code := approve("big", big); code != 409 {
		t.Errorf("oversized approve: %d, want 409", code)
	}
	if got := audits(); got != before {
		t.Errorf("refused approve wrote %d audit events", got-before)
	}
	if s, _ := srv.resolve(ctx, "routes"); strings.Contains(string(s.Items), "team-a") {
		t.Error("refused approve must not publish")
	}

	// Room for one more route, not two: exactly one concurrent approval wins.
	maxRoutesSnapshotBytes = len(snap.Items) + 250
	bodies := map[string]string{"s1": draft("s1", "search"), "s2": draft("s2", "search")}
	routesFitPause = func() { time.Sleep(100 * time.Millisecond) }
	t.Cleanup(func() { routesFitPause = nil })
	codes := make(chan int, 2)
	for name, body := range bodies {
		go func() { codes <- approve(name, body) }()
	}
	if c1, c2 := <-codes, <-codes; c1+c2 != 200+409 {
		t.Errorf("concurrent approvals: %d and %d, want one 200 and one 409", c1, c2)
	}

	routesFitPause = nil

	// Rollback runs the same check.
	winner := "s1"
	if s, _ := srv.resolve(ctx, "routes"); !strings.Contains(string(s.Items), "/mcp/team-a/s1") {
		winner = "s2"
	}
	maxRoutesSnapshotBytes = old
	if code := approve(winner, draft(winner, "search", "two")); code != 200 {
		t.Fatalf("second generation: %d", code)
	}
	// Rolling back to a smaller snapshot while over budget is a repair: allowed.
	maxRoutesSnapshotBytes = len(snap.Items)
	before = audits()
	if code, out := e.do("POST", base+winner+"/rollback", a, `{"fromGeneration":2,"reason":"x"}`); code != 200 {
		t.Errorf("shrinking rollback: %d %s, want 200", code, out)
	}
	if got := audits(); got != before+1 {
		t.Errorf("shrinking rollback wrote %d audit events, want 1", got-before)
	}

	// Rolling back to a larger snapshot over the limit is refused, with no audit row.
	maxRoutesSnapshotBytes = old
	if code := approve(winner, draft(winner, "a")); code != 200 {
		t.Fatalf("third generation: %d", code)
	}
	cur, err := srv.resolve(ctx, "routes")
	if err != nil {
		t.Fatal(err)
	}
	maxRoutesSnapshotBytes = len(cur.Items)
	before = audits()
	if code, out := e.do("POST", base+winner+"/rollback", a, `{"fromGeneration":3,"toGeneration":2,"reason":"x"}`); code != 409 {
		t.Errorf("growing rollback: %d %s, want 409", code, out)
	}
	if got := audits(); got != before {
		t.Errorf("refused rollback wrote %d audit events", got-before)
	}
	// Compare ETag and generation, not Items: a repeat resolve reads jsonb back from Postgres, so bytes differ in key order and whitespace.
	if s, err := srv.resolve(ctx, "routes"); err != nil || s.ETag != cur.ETag || s.Generation != cur.Generation {
		t.Errorf("refused rollback changed the published snapshot: err=%v", err)
	}
}

// Only the kill switch is left of /admin/v1 (#2895); the rest is under /apis.
func TestLegacyRoutesAreRemoved(t *testing.T) {
	h := newServer().routes(testAuthz(t), adminID, testTeams, nil)
	for _, c := range []struct{ method, path, tok string }{
		{http.MethodPut, "/admin/v1/servers/team-a/svc/draft", userTok("A111111")},
		{http.MethodPost, "/admin/v1/servers/team-a/svc/approve", userTok("A111111")},
		{http.MethodPost, "/admin/v1/servers/team-a/svc/revoke", userTok("A111111")},
		{http.MethodPost, "/admin/v1/servers/team-a/svc/rollback", userTok("A111111")},
		{http.MethodGet, "/admin/v1/servers/team-a/svc", userTok("A111111")},
		{http.MethodGet, "/admin/v1/servers", userTok("A111111")},
		{http.MethodGet, "/admin/v1/servers/team-a/svc/audit", userTok("A111111")},
		{http.MethodGet, "/admin/v1/me", userTok("A111111")},
		{http.MethodPost, "/admin/v1/jobs/team-a", userTok("A111111")},
		{http.MethodPost, "/admin/v1/jobs/team-a/1/cancel", userTok("A111111")},
		{http.MethodPost, "/admin/v1/jobs/team-a/1/kill", userTok("A111111")},
		{http.MethodPost, "/admin/v1/agents", userTok("A111111")},
		{http.MethodGet, "/admin/v1/agents/a/b", userTok("A111111")},
		{http.MethodGet, "/admin/v1/agents/pending-approval", userTok("A111111")},
		{http.MethodGet, "/internal/v1/servers", registryID},
		{http.MethodGet, "/internal/v1/routes", gatewayID},
		{http.MethodGet, "/internal/v1/agents", gatewayID},
	} {
		if rec := do(t, h, c.method, c.path, c.tok, "{}"); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: got %d, want 404 or 405", c.method, c.path, rec.Code)
		}
	}
}
