package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	h := legacyRoutes(srv, testAuthz(t), adminID, testTeams)
	path := "/admin/v1/servers/team-a/svc/draft"
	cases := map[string]struct {
		h    http.Handler
		path string
		tok  string
		want int
	}{
		"admin routes off":   {legacyRoutes(srv, testAuthz(t), "", testTeams), path, userTok("A111111"), 404},
		"no token":           {h, path, "", 401},
		"app token":          {h, path, adminID, 401},
		"app token w/ ident": {h, path, "app-with-ident", 401},
		"other client":       {h, path, "user:" + gatewayID + ":A111111", 401},
		"bad NAVident":       {h, path, userTok("a111111"), 401},
		"texas fails":        {h, path, "texas-500", 503},
		"bad namespace":      {h, "/admin/v1/servers/Team_A/svc/draft", userTok("A111111"), 400},
		"not a member":       {h, path, userTok("B111111"), 403},
		"team lookup fails":  {legacyRoutes(srv, testAuthz(t), adminID, failingTeams{}), path, userTok("A111111"), 503},
		"database down":      {h, path, userTok("A111111"), 503},
		"unset static teams": {legacyRoutes(srv, testAuthz(t), adminID, staticTeams{}), path, userTok("A111111"), 403},
	}
	for name, c := range cases {
		if rec := do(t, c.h, http.MethodPut, c.path, c.tok, routeDraft); rec.Code != c.want {
			t.Errorf("%s: got %d, want %d (%s)", name, rec.Code, c.want, rec.Body)
		}
	}
}

func TestTeamsFromEnv(t *testing.T) {
	// Emails are compared trimmed and case-insensitively.
	t.Setenv("FLEET_CONTROL_STATIC_TEAMS", `{"team-a":[" Kari.Nordmann@Example.com "],"team-b":["kari.nordmann@example.com"]}`)
	if d, err := teamsFromEnv("dev-gcp"); err != nil {
		t.Fatal(err)
	} else if ok, _ := d.IsMember(context.Background(), "team-a", "KARI.nordmann@example.COM"); !ok {
		t.Error("want member")
	} else if ok, _ := d.IsMember(context.Background(), "team-a", ""); ok {
		t.Error("empty email must not be a member")
	} else if list, _ := d.Teams(context.Background(), "kari.nordmann@example.com"); strings.Join(list, ",") != "team-a,team-b" {
		t.Errorf("teams: %v", list)
	}
	// The static list must never run outside dev, and an unknown cluster is not dev.
	for _, cluster := range []string{"prod-gcp", "prod-fss", "management", "dev", ""} {
		if _, err := teamsFromEnv(cluster); err == nil {
			t.Errorf("static teams in cluster %q: want error", cluster)
		}
		if got := adminClientFor(cluster, adminID); got != "" {
			t.Errorf("write routes mounted in cluster %q", cluster)
		}
	}
	ident, list, err := resolvePerson(context.Background(), testAuthz(t), "Bearer "+userTok("A111111"), adminID, testTeams)
	if err != nil || ident != "A111111" || strings.Join(list, ",") != "team-a" {
		t.Errorf("resolvePerson: %q %v %v", ident, list, err)
	}
	if _, _, err = resolvePerson(context.Background(), testAuthz(t), "Bearer "+adminID, adminID, testTeams); !errors.Is(err, errUnauthorized) {
		t.Errorf("resolvePerson with an app token: %v", err)
	}
	if adminClientFor("dev-gcp", adminID) != adminID {
		t.Error("dev-gcp must mount the write routes")
	}
	// A mixed-case preferred_username in the token resolves to its teams.
	_, list, err = resolvePerson(context.Background(), testAuthz(t), "Bearer "+userTok("A111111")+":Kari.Nordmann@Example.com", adminID, staticTeams{"team-a": {"kari.nordmann@example.com"}})
	if err != nil || strings.Join(list, ",") != "team-a" {
		t.Errorf("resolvePerson by email: %v %v", list, err)
	}
	// No preferred_username: refused, not resolved to an empty team list.
	if _, _, err = resolvePerson(context.Background(), testAuthz(t), "Bearer "+userTok("A111111")+":", adminID, testTeams); !errors.Is(err, errUnauthorized) {
		t.Errorf("resolvePerson without preferred_username: %v", err)
	}
	for _, bad := range []string{`{"Team":["a@example.com"]}`, `{"team-a":["x"]}`, `{"team-a":["@example.com"]}`, `[]`} {
		t.Setenv("FLEET_CONTROL_STATIC_TEAMS", bad)
		if _, err := teamsFromEnv("dev-gcp"); err == nil {
			t.Errorf("%s: want error", bad)
		}
	}
}

func TestRealPostgresWriteFlow(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	routes, allowlist := readSeedFiles(t, "prod")
	servers, err := seedServers(routes, allowlist)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Seed(ctx, servers, "test"); err != nil {
		t.Fatal(err)
	}
	srv := newServer()
	srv.cluster = "dev-gcp"
	srv.store.Store(&storeBox{db})
	h := legacyRoutes(srv, testAuthz(t), adminID, testTeams)
	base := "/admin/v1/servers/team-a/svc"
	a, b := userTok("A111111"), userTok("A222222")

	if rec := do(t, h, http.MethodPut, base+"/draft", a, `{"route":{"target":"dev-gcp:team-b:svc"}}`); rec.Code != 400 {
		t.Fatalf("invalid draft: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPut, base+"/draft", a, `{"passAllTools":true,`+routeDraft[1:]); rec.Code != 400 {
		t.Fatalf("unknown field: %d", rec.Code)
	}
	draft := func(tok, body string) (int, string) {
		rec := do(t, h, http.MethodPut, base+"/draft", tok, body)
		if rec.Code != 200 {
			t.Fatalf("draft: %d %s", rec.Code, rec.Body)
		}
		var out struct {
			Version int    `json:"version"`
			Hash    string `json:"content_hash"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out.Version, out.Hash
	}
	approve := func(tok string, v int, hash string) int {
		body, _ := json.Marshal(map[string]any{"version": v, "content_hash": hash})
		return do(t, h, http.MethodPost, base+"/approve", tok, string(body)).Code
	}

	v1, h1 := draft(a, routeDraft)
	if snap, _ := srv.resolve(ctx, "routes"); strings.Contains(string(snap.Items), "team-a") {
		t.Fatal("a draft must not be published")
	}
	if code := approve(a, v1, h1); code != 403 {
		t.Errorf("submitter approving own draft: %d, want 403", code)
	}
	if code := approve(b, v1, strings.Repeat("0", 64)); code != 409 {
		t.Errorf("approve with wrong hash: %d, want 409", code)
	}
	if code := approve(userTok("B111111"), v1, h1); code != 403 {
		t.Errorf("approve from another team: %d, want 403", code)
	}
	if code := approve(b, v1+1, h1); code != 409 {
		t.Errorf("approve of a version that is not pending: %d, want 409", code)
	}
	if code := approve(b, v1, h1); code != 200 {
		t.Fatalf("approve: %d", code)
	}
	if snap, _ := srv.resolve(ctx, "routes"); !strings.Contains(string(snap.Items), "/mcp/team-a/svc") {
		t.Fatal("approved draft must be in the routes snapshot")
	}

	// Second version, then concurrent approvals: exactly one publishes.
	v2, h2 := draft(b, strings.Replace(routeDraft, `"search"`, `"search","fetch"`, 1))
	// Open the pool's connections first, so the approvals really overlap.
	var conns []*pgxpool.Conn
	// Stay within MaxConns, which is the CPU count (min 4): more would block.
	for range min(10, int(db.pool.Config().MaxConns)) {
		c, acqErr := db.pool.Acquire(ctx)
		if acqErr != nil {
			t.Fatal(acqErr)
		}
		conns = append(conns, c)
	}
	for _, c := range conns {
		c.Release()
	}
	start := make(chan struct{})
	errs := make(chan error, 40)
	var wg sync.WaitGroup
	for range 40 {
		wg.Go(func() { <-start; errs <- db.Approve(ctx, "A111111", "team-a", "svc", v2, h2) })
	}
	close(start)
	wg.Wait()
	close(errs)
	okCount := 0
	for e := range errs {
		if e == nil {
			okCount++
		} else if apiErr, ok := errors.AsType[*apiError](e); !ok || apiErr.code != 409 {
			t.Errorf("concurrent approve: %v", e)
		}
	}
	if okCount != 1 {
		t.Errorf("concurrent approvals: %d succeeded, want 1", okCount)
	}

	// A newer draft replaces the pending one; the old hash no longer approves.
	v3, h3 := draft(a, routeDraft)
	v4, _ := draft(a, strings.Replace(routeDraft, `"search"`, `"other"`, 1))
	if code := approve(b, v3, h3); code != 409 {
		t.Errorf("approve of a replaced draft: %d, want 409", code)
	}
	_ = v4

	if rec := do(t, h, http.MethodPost, base+"/rollback", a, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"published_version":1`) {
		t.Errorf("rollback: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, http.MethodPost, base+"/rollback", a, ""); rec.Code != 409 {
		t.Errorf("rollback past the first version: %d, want 409", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/revoke", a, `{"reason":""}`); rec.Code != 400 {
		t.Errorf("revoke without reason: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/revoke", a, `{"reason":"`+strings.Repeat("x", 501)+`"}`); rec.Code != 400 {
		t.Errorf("revoke with a too long reason: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/revoke", a, `{"reason":"incident"}`); rec.Code != 200 {
		t.Errorf("revoke: %d", rec.Code)
	}
	if snap, _ := srv.resolve(ctx, "routes"); !strings.Contains(string(snap.Items), `"upstream_url":"http://svc.team-a","enabled":false`) {
		t.Errorf("revoked route must stay in the snapshot as disabled: %s", snap.Items)
	}
	if rec := do(t, h, http.MethodPost, base+"/revoke", a, `{"reason":"again"}`); rec.Code != 409 {
		t.Errorf("second revoke: %d, want 409", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, base, b, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"revoked"`) {
		t.Errorf("get: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, http.MethodPost, "/admin/v1/servers/team-a/missing/revoke", a, `{"reason":"x"}`); rec.Code != 404 {
		t.Errorf("revoke of unknown server: %d", rec.Code)
	}

	var actions []string
	rows, err := db.pool.Query(ctx, `SELECT actor || ':' || action FROM audit_events WHERE namespace = 'team-a' AND detail ? 'before' AND detail ? 'after' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		actions = append(actions, s)
	}
	want := "A111111:draft_submitted A222222:approved_published A222222:draft_submitted A111111:approved_published A111111:draft_submitted A111111:draft_submitted A111111:rolled_back A111111:revoked"
	if got := strings.Join(actions, " "); got != want {
		t.Errorf("audit trail:\n got %s\nwant %s", got, want)
	}

	// The database itself refuses self-approval.
	if _, err = db.pool.Exec(ctx, `UPDATE server_versions SET approved_by = submitted_by WHERE namespace = 'team-a'`); err == nil {
		t.Error("server_versions must refuse approved_by = submitted_by")
	}

	db.Close()
	if rec := do(t, h, http.MethodPut, base+"/draft", a, routeDraft); rec.Code != 503 {
		t.Errorf("Postgres down: %d, want 503", rec.Code)
	}
}

// Rollback skips versions that were never approved and versions blocked by
// a revoke: one person must not be able to publish either.
func TestRealPostgresRollbackSkipsUnapprovedAndBlocked(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	sv, err := validateDraft("dev-gcp", "team-a", "rb", draftSpec{Route: &struct {
		Target      string   `json:"target"`
		UpstreamURL string   `json:"upstream_url"`
		Enabled     *bool    `json:"enabled"`
		Tools       []string `json:"tools"`
	}{Target: "dev-gcp:team-a:rb", UpstreamURL: "http://rb.team-a/mcp", Tools: []string{"search"}}})
	if err != nil {
		t.Fatal(err)
	}
	publish := func() int {
		v, saveErr := db.SaveDraft(ctx, "A111111", sv, "h")
		if saveErr != nil {
			t.Fatal(saveErr)
		}
		if approveErr := db.Approve(ctx, "A222222", "team-a", "rb", v, "h"); approveErr != nil {
			t.Fatal(approveErr)
		}
		return v
	}
	publish()                                                       // 1
	if _, err = db.SaveDraft(ctx, "A111111", sv, "h"); err != nil { // 2, never approved
		t.Fatal(err)
	}
	publish() // 3
	if err = db.Revoke(ctx, "A111111", "team-a", "rb", "leaked credentials", true); err != nil {
		t.Fatal(err)
	}
	publish() // 4
	got, err := db.Rollback(ctx, "A111111", "team-a", "rb")
	if err != nil || got != 1 {
		t.Fatalf("rollback: got version %d (%v), want 1: 2 was never approved, 3 is blocked", got, err)
	}
	if _, err = db.Rollback(ctx, "A111111", "team-a", "rb"); err == nil {
		t.Error("rollback below the first version must fail")
	}
}

// A seeded version counts as published, so rollback can return to it.
func TestRealPostgresRollbackToSeed(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	routes, allowlist := readSeedFiles(t, "prod")
	servers, err := seedServers(routes, allowlist)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Seed(ctx, servers, "test"); err != nil {
		t.Fatal(err)
	}
	var sv Server
	for _, s := range servers {
		if s.Route != nil {
			sv = s
			break
		}
	}
	v, err := db.SaveDraft(ctx, "A111111", sv, "h")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Approve(ctx, "A222222", sv.Namespace, sv.Name, v, "h"); err != nil {
		t.Fatal(err)
	}
	if got, rbErr := db.Rollback(ctx, "A111111", sv.Namespace, sv.Name); rbErr != nil || got != 1 {
		t.Fatalf("rollback to the seeded version: %d %v", got, rbErr)
	}
}

func TestHTTPServerBoundsBodyReads(t *testing.T) {
	if s := newHTTPServer("0", http.NotFoundHandler()); s.ReadTimeout <= 0 || s.ReadHeaderTimeout <= 0 {
		t.Errorf("read timeouts must be set: %v %v", s.ReadTimeout, s.ReadHeaderTimeout)
	}
}

// The size check runs inside Approve and Rollback, and the routes snapshot
// lock stops two approvals of different servers that each fit alone from
// passing the limit together (#2687).
func TestRealPostgresRoutesSizeLimit(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	routes, allowlist := readSeedFiles(t, "prod")
	servers, err := seedServers(routes, allowlist)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Seed(ctx, servers, "test"); err != nil {
		t.Fatal(err)
	}
	srv := newServer()
	srv.cluster = "dev-gcp"
	srv.store.Store(&storeBox{db})
	h := legacyRoutes(srv, testAuthz(t), adminID, testTeams)
	a, b := userTok("A111111"), userTok("A222222")
	draft := func(name string) []byte {
		body := strings.NewReplacer("svc", name).Replace(routeDraft)
		rec := do(t, h, http.MethodPut, "/admin/v1/servers/team-a/"+name+"/draft", a, body)
		if rec.Code != 200 {
			t.Fatalf("draft %s: %d %s", name, rec.Code, rec.Body)
		}
		var out struct {
			Version int    `json:"version"`
			Hash    string `json:"content_hash"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		approval, _ := json.Marshal(map[string]any{"version": out.Version, "content_hash": out.Hash})
		return approval
	}
	approve := func(name string, body []byte) int {
		return do(t, h, http.MethodPost, "/admin/v1/servers/team-a/"+name+"/approve", b, string(body)).Code
	}
	audits := func() (n int) {
		_ = db.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&n)
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
	big := draft("big")
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
	bodies := map[string][]byte{"s1": draft("s1"), "s2": draft("s2")}
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
	if code := approve(winner, draft(winner)); code != 200 {
		t.Fatalf("second version: %d", code)
	}
	maxRoutesSnapshotBytes = len(snap.Items)
	before = audits()
	if rec := do(t, h, http.MethodPost, "/admin/v1/servers/team-a/"+winner+"/rollback", a, ""); rec.Code != 409 {
		t.Errorf("oversized rollback: %d %s, want 409", rec.Code, rec.Body)
	}
	if got := audits(); got != before {
		t.Errorf("refused rollback wrote %d audit events", got-before)
	}
}

// legacyRoutes is routes plus the /admin/v1/servers write handlers that
// production no longer mounts (#2852).
func legacyRoutes(s *server, a *authz, adminClientID string, teams teamDirectory) http.Handler {
	h := s.routes(a, adminClientID, teams, nil)
	if adminClientID == "" {
		return h
	}
	legacy := http.NewServeMux()
	s.legacyWriteRoutes(legacy, a, adminClientID, teams)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := legacy.Handler(r); pattern != "" {
			legacy.ServeHTTP(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func TestAdminServerWritesAreRemoved(t *testing.T) {
	h := newServer().routes(testAuthz(t), adminID, testTeams, nil)
	for _, c := range []struct{ method, path string }{
		{http.MethodPut, "/admin/v1/servers/team-a/svc/draft"},
		{http.MethodPost, "/admin/v1/servers/team-a/svc/approve"},
		{http.MethodPost, "/admin/v1/servers/team-a/svc/revoke"},
		{http.MethodPost, "/admin/v1/servers/team-a/svc/rollback"},
		{http.MethodPost, "/admin/v1/jobs/team-a"}, // no agent profile (#2854)
	} {
		if rec := do(t, h, c.method, c.path, userTok("A111111"), "{}"); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: got %d, want 404 or 405", c.method, c.path, rec.Code)
		}
	}
}
