package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

const nsBase = apisPrefix + "/namespaces/"

// Every person below has an oid from testOid. A111111 is in two teams, so
// it can be both submitter and server owner.
var apisTeams = staticTeams{
	"team-a":       {"A111111", "A222222"},
	"copilot":      {"A111111", "C222222"},
	"aksel-studio": {"A111111", "A555555"},
	"team-b":       {"B444444", "B555555"},
	"platform":     {"P999999"},
}

func person(ident string) string { return "user:" + adminID + ":" + ident }

type apisEnv struct {
	t   *testing.T
	db  *DB
	srv *server
	url string
	// bodies collects every answer, so a test can check that no oid leaks.
	bodies []string
}

func newApisEnv(t *testing.T) *apisEnv {
	t.Helper()
	db := openTestDB(t)
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := newServer()
	s.cluster = "dev-gcp"
	s.operatorTeam = "platform"
	s.verifier = fakeVerifier{}
	s.store.Store(&storeBox{db})
	ts := httptest.NewServer(s.routes(testAuthz(t), adminID, apisTeams, nil))
	t.Cleanup(ts.Close)
	return &apisEnv{t: t, db: db, srv: s, url: ts.URL}
}

func (e *apisEnv) do(method, path, tok, body string) (int, string) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.url+path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // httptest server URL
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	e.bodies = append(e.bodies, string(raw))
	return resp.StatusCode, string(raw)
}

func (e *apisEnv) want(code int, method, path, tok, body string) string {
	e.t.Helper()
	got, out := e.do(method, path, tok, body)
	if got != code {
		e.t.Fatalf("%s %s: want %d, got %d %s", method, path, code, got, out)
	}
	return out
}

// noOidLeak fails when any answer except whoami holds an oid.
func (e *apisEnv) noOidLeak() {
	e.t.Helper()
	for _, b := range e.bodies {
		if strings.Contains(b, "00000000-0000-4000-8000-") && !strings.Contains(b, `"teams"`) {
			e.t.Fatalf("an answer holds an oid: %s", b)
		}
	}
}

func serverJSON(ns, name, rv string, tools ...string) string {
	meta := map[string]any{"name": name, "namespace": ns}
	if rv != "" {
		meta["resourceVersion"] = rv
	}
	raw, _ := json.Marshal(map[string]any{
		"apiVersion": apiGroupVersion, "kind": "MCPServer", "metadata": meta,
		"spec": map[string]any{"dataCategory": "Intern", "upstream": map[string]any{"app": "svc", "url": "http://svc." + ns + "/mcp"}, "allowedTools": tools},
	})
	return string(raw)
}

func decodeServer(t *testing.T, body string) MCPServer {
	t.Helper()
	var m MCPServer
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return m
}

func condOf(conds []Condition, typ string) Condition {
	for _, c := range conds {
		if c.Type == typ {
			return c
		}
	}
	return Condition{}
}

func approval(gen int, hash string) string {
	return fmt.Sprintf(`{"generation":%d,"contentHash":%q}`, gen, hash)
}

// publish creates ns/name with tools and has a second member approve it.
func (e *apisEnv) publish(ns, name, submitter, approver string, tools ...string) MCPServer {
	e.t.Helper()
	m := decodeServer(e.t, e.want(201, "POST", nsBase+ns+"/mcpservers", person(submitter), serverJSON(ns, name, "", tools...)))
	return decodeServer(e.t, e.want(200, "POST", nsBase+ns+"/mcpservers/"+name+"/approval", person(approver), approval(1, m.Status.Pending.ContentHash)))
}

func TestApisGuardsFailClosed(t *testing.T) {
	s := newServer()
	h := s.routes(testAuthz(t), adminID, failingTeams{}, nil)
	ok := newServer()
	hNoDB := ok.routes(testAuthz(t), adminID, apisTeams, nil)
	path := nsBase + "team-a/mcpservers"
	for _, tc := range []struct {
		name string
		h    http.Handler
		tok  string
		want int
	}{
		{"no token", hNoDB, "", 401},
		{"app token", hNoDB, "app-with-ident", 401},
		{"person without oid", hNoDB, person("A111111") + ":A111111:", 401},
		{"person with a malformed oid", hNoDB, person("A111111") + ":A111111:not-a-uuid", 401},
		{"token from another client", hNoDB, "user:" + gatewayID + ":A111111", 401},
		{"Texas down", hNoDB, "texas-500", 503},
		{"team lookup down", h, person("A111111"), 503},
		{"database down", hNoDB, person("A111111"), 503},
	} {
		rec := do(t, tc.h, "GET", path, tc.tok, "")
		if rec.Code != tc.want {
			t.Errorf("%s: want %d, got %d %s", tc.name, tc.want, rec.Code, rec.Body)
		}
		var st map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || st["kind"] != "Status" {
			t.Errorf("%s: answer is not metav1.Status: %s", tc.name, rec.Body)
		}
	}
	// No admin client (prod): /apis is not mounted.
	if rec := do(t, newServer().routes(testAuthz(t), "", apisTeams, nil), "GET", path, person("A111111"), ""); rec.Code != 404 {
		t.Errorf("without the admin client /apis must not exist: %d", rec.Code)
	}
}

// With a database present, a failed team lookup is still 503, never an answer.
func TestRealPostgresApisTeamLookupDown(t *testing.T) {
	db := openTestDB(t)
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := newServer()
	s.store.Store(&storeBox{db})
	h := s.routes(testAuthz(t), adminID, failingTeams{}, nil)
	for _, path := range []string{apisPrefix + "/mcpservercatalogentries", apisPrefix + "/whoami", nsBase + "team-a/mcpservers"} {
		if rec := do(t, h, "GET", path, person("A111111"), ""); rec.Code != 503 {
			t.Errorf("%s: want 503, got %d %s", path, rec.Code, rec.Body)
		}
	}
}

func TestRealPostgresMCPServerLifecycle(t *testing.T) {
	e := newApisEnv(t)
	base := nsBase + "team-a/mcpservers"
	// Non-members get 404 whether or not the server exists.
	e.want(404, "POST", base, person("B444444"), serverJSON("team-a", "svc", "", "search"))
	e.want(404, "GET", base+"/svc", person("B444444"), "")
	e.want(400, "POST", base, person("A111111"), serverJSON("team-b", "svc", "", "search"))
	e.want(400, "POST", base, person("A111111"), strings.Replace(serverJSON("team-a", "svc", "", "search"), `"spec"`, `"bogus":1,"spec"`, 1))
	for name, body := range map[string]string{
		"strengt fortrolig": strings.Replace(serverJSON("team-a", "svc", "", "search"), "Intern", "StrengtFortrolig", 1),
		"no dataCategory":   strings.Replace(serverJSON("team-a", "svc", "", "search"), `"dataCategory":"Intern",`, "", 1),
		"authorization":     strings.Replace(serverJSON("team-a", "svc", "", "search"), `"dataCategory"`, `"authorization":{"action":"Allow"},"dataCategory"`, 1),
		"wildcard tool":     serverJSON("team-a", "svc", "", "*"),
		"foreign host":      strings.Replace(serverJSON("team-a", "svc", "", "search"), "svc.team-a", "svc.team-b", 1),
	} {
		if code, out := e.do("POST", base, person("A111111"), body); code != 422 || !strings.Contains(out, `"reason":"Invalid"`) {
			t.Errorf("%s: want 422 Invalid, got %d %s", name, code, out)
		}
	}
	m := decodeServer(t, e.want(201, "POST", base, person("A111111"), serverJSON("team-a", "svc", "", "search", "fetch")))
	if m.Metadata.Generation != 1 || m.Status.Pending == nil || condOf(m.Status.Conditions, "PendingApproval").Status != "True" || condOf(m.Status.Conditions, "Ready").Reason != "PendingApproval" {
		t.Fatalf("created: %+v", m)
	}
	if !slices.Equal(m.Spec.AllowedTools, []string{"fetch", "search"}) || !strings.HasPrefix(m.Status.Pending.ContentHash, "v1:sha256:") {
		t.Fatalf("spec must be normalised and hashed: %+v", m.Spec)
	}
	e.want(409, "POST", base, person("A222222"), serverJSON("team-a", "svc", "", "search"))
	hash := m.Status.Pending.ContentHash
	e.want(403, "POST", base+"/svc/approval", person("A111111"), approval(1, hash))
	// Same person under another NAVident, and another oid behind the same NAVident.
	e.want(403, "POST", base+"/svc/approval", person("A222222")+":A222222:"+testOid("A111111"), approval(1, hash))
	e.want(403, "POST", base+"/svc/approval", person("A111111")+":A111111:"+testOid("A222222"), approval(1, hash))
	e.want(409, "POST", base+"/svc/approval", person("A222222"), approval(1, "v1:sha256:00"))
	e.want(409, "POST", base+"/svc/approval", person("A222222"), approval(2, hash))
	m = decodeServer(t, e.want(200, "POST", base+"/svc/approval", person("A222222"), approval(1, hash)))
	if m.Status.PublishedGeneration != 1 || m.Status.Pending != nil || condOf(m.Status.Conditions, "Ready").Status != "True" || m.Status.Route.Path != "/mcp/team-a/svc" {
		t.Fatalf("approved: %+v", m.Status)
	}
	if m.Status.ApprovedBy.Name != "A222222" || m.Status.SubmittedBy.Kind != "User" {
		t.Fatalf("actors: %+v %+v", m.Status.SubmittedBy, m.Status.ApprovedBy)
	}
	servers, _ := e.db.Servers(context.Background())
	if i := slices.IndexFunc(servers, func(s Server) bool { return s.Key() == "team-a/svc" }); i < 0 || servers[i].Route.Target != "dev-gcp:team-a:svc" {
		t.Fatalf("snapshot columns: %+v", servers)
	}

	// PUT: resourceVersion is required and must be fresh.
	rv := m.Metadata.ResourceVersion
	e.want(409, "PUT", base+"/svc", person("A111111"), serverJSON("team-a", "svc", "", "search"))
	e.want(409, "PUT", base+"/svc", person("A111111"), serverJSON("team-a", "svc", "1", "search"))
	e.want(400, "PUT", base+"/svc", person("A111111"), serverJSON("team-a", "other", rv, "search"))
	m = decodeServer(t, e.want(200, "PUT", base+"/svc", person("A111111"), serverJSON("team-a", "svc", rv, "search")))
	if m.Metadata.Generation != 2 || m.Status.Pending.Generation != 2 || m.Status.PublishedGeneration != 1 || len(m.Spec.AllowedTools) != 2 {
		t.Fatalf("new generation: spec must stay the published one, pending 2: %+v", m)
	}
	// A label change with the pending spec sent back keeps the colleague's proposal.
	labelled := strings.Replace(serverJSON("team-a", "svc", m.Metadata.ResourceVersion, "search"), `"name":"svc"`, `"name":"svc","labels":{"owner":"a"}`, 1)
	m2 := decodeServer(t, e.want(200, "PUT", base+"/svc", person("A222222"), labelled))
	if m2.Metadata.Generation != 2 || m2.Status.Pending == nil || m2.Status.Pending.ContentHash != m.Status.Pending.ContentHash || m2.Metadata.Labels["owner"] != "a" {
		t.Fatalf("label change must not touch the pending generation: %+v", m2)
	}
	if m2.Metadata.ResourceVersion == m.Metadata.ResourceVersion {
		t.Fatal("resourceVersion must move on every write")
	}
	e.want(422, "PUT", base+"/svc", person("A222222"), strings.Replace(labelled, `"owner":"a"`, `"fleet.nav.no/x":"a"`, 1))
	// Two writers with the same resourceVersion: exactly one wins.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i, tools := range [][]string{{"a1"}, {"b1"}} {
		wg.Go(func() {
			codes[i], _ = e.do("PUT", base+"/svc", person("A111111"), serverJSON("team-a", "svc", m2.Metadata.ResourceVersion, tools...))
		})
	}
	wg.Wait()
	slices.Sort(codes)
	if codes[0] != 200 || codes[1] != 409 {
		t.Fatalf("concurrent PUT with one resourceVersion: %v", codes)
	}
	e.want(405, "DELETE", base+"/svc", person("A111111"), "")
	if code, out := e.do("GET", base+"/svc/history", person("A111111"), ""); code != 200 || !strings.Contains(out, "approved_published") {
		t.Fatalf("history: %d %s", code, out)
	}
	e.want(404, "GET", base+"/svc/history", person("B444444"), "")
	e.want(404, "GET", base+"/nope/history", person("A111111"), "")
	g := decodeServer(t, e.want(200, "GET", base+"/svc/generations/1", person("A111111"), ""))
	if g.Status.ContentHash != hash || len(g.Spec.AllowedTools) != 2 {
		t.Fatalf("generation 1: %+v", g)
	}
	e.want(404, "GET", base+"/svc/generations/1", person("B444444"), "")
	who := e.want(200, "GET", apisPrefix+"/whoami", person("A111111"), "")
	if !strings.Contains(who, testOid("A111111")) {
		t.Fatalf("whoami must show the caller's own oid: %s", who)
	}
	e.noOidLeak()
}

func TestRealPostgresMCPServerRevokeRollbackBlock(t *testing.T) {
	e := newApisEnv(t)
	base := nsBase + "team-a/mcpservers/svc"
	e.publish("team-a", "svc", "A111111", "A222222", "one")
	submit := func(tools ...string) MCPServer {
		cur := decodeServer(t, e.want(200, "GET", base, person("A111111"), ""))
		m := decodeServer(t, e.want(200, "PUT", base, person("A111111"), serverJSON("team-a", "svc", cur.Metadata.ResourceVersion, tools...)))
		return decodeServer(t, e.want(200, "POST", base+"/approval", person("A222222"), approval(int(m.Metadata.Generation), m.Status.Pending.ContentHash)))
	}
	submit("two")   // gen 2
	submit("three") // gen 3
	revoke := func(who, body string) (int, string) { return e.do("POST", base+"/revocation", person(who), body) }
	rollback := func(body string) (int, string) { return e.do("POST", base+"/rollback", person("A111111"), body) }

	if code, _ := rollback(`{"fromGeneration":2,"reason":"x"}`); code != 409 {
		t.Fatal("fromGeneration must be the published generation")
	}
	if code, _ := rollback(`{"fromGeneration":3,"toGeneration":4,"reason":"x"}`); code != 409 {
		t.Fatal("rollback never goes forward")
	}
	if code, _ := rollback(`{"fromGeneration":3,"reason":""}`); code != 400 {
		t.Fatal("reason is required")
	}
	if code, out := rollback(`{"fromGeneration":3,"reason":"bad deploy"}`); code != 200 || decodeServer(t, out).Status.PublishedGeneration != 2 {
		t.Fatalf("rollback to 2: %d %s", code, out)
	}
	if code, out := rollback(`{"fromGeneration":2,"toGeneration":3,"reason":"x"}`); code != 409 || !strings.Contains(out, "GenerationNotEligible") {
		t.Fatalf("rollback never goes forward to a newer generation: %d %s", code, out)
	}
	submit("three") // gen 4: same content as gen 3
	if code, _ := revoke("B444444", `{"reason":"x"}`); code != 404 {
		t.Fatal("another team cannot revoke, and gets 404")
	}
	// The platform team may stop it, and its answer holds no owner-only parts.
	code, out := revoke("P999999", `{"reason":"incident","block":true}`)
	if m := decodeServer(t, out); code != 200 || condOf(m.Status.Conditions, "Revoked").Reason != "Blocked" || m.Spec.Upstream != nil || m.Status.SubmittedBy != nil {
		t.Fatalf("platform revoke: %d %s", code, out)
	}
	if code, out := rollback(`{"fromGeneration":4,"reason":"x"}`); code != 409 || !strings.Contains(out, "InvalidState") {
		t.Fatalf("rollback must never lift a revocation: %d %s", code, out)
	}
	cur := decodeServer(t, e.want(200, "GET", base, person("A111111"), ""))
	// Blocked content can never come back.
	if code, out := e.do("PUT", base, person("A111111"), serverJSON("team-a", "svc", cur.Metadata.ResourceVersion, "three")); code != 422 {
		t.Fatalf("blocked content resubmitted: %d %s", code, out)
	}
	// Restart: the same spec as a non-blocked generation gives a new one on a revoked server.
	m := decodeServer(t, e.want(200, "PUT", base, person("A111111"), serverJSON("team-a", "svc", cur.Metadata.ResourceVersion, "two")))
	if m.Metadata.Generation != 5 {
		t.Fatalf("restart must give a new generation: %+v", m.Metadata)
	}
	m = decodeServer(t, e.want(200, "POST", base+"/approval", person("A222222"), approval(5, m.Status.Pending.ContentHash)))
	if condOf(m.Status.Conditions, "Revoked").Status != "False" || m.Status.PublishedGeneration != 5 {
		t.Fatalf("a new approved generation lifts Revoked: %+v", m.Status)
	}
	// Generation 4 was revoked, generation 3 has its content: both refused, no skip to 2.
	if code, out := rollback(`{"fromGeneration":5,"toGeneration":4,"reason":"x"}`); code != 409 || !strings.Contains(out, "GenerationBlocked") {
		t.Fatalf("rollback to a blocked generation: %d %s", code, out)
	}
	if code, out := rollback(`{"fromGeneration":5,"toGeneration":3,"reason":"x"}`); code != 409 || !strings.Contains(out, "ContentRefused") {
		t.Fatalf("rollback to blocked content: %d %s", code, out)
	}
	if code, out := rollback(`{"fromGeneration":5,"reason":"x"}`); code != 409 || !strings.Contains(out, "GenerationBlocked") {
		t.Fatalf("default rollback target is refused, never skipped: %d %s", code, out)
	}
	// Revoke without block, then restart and try to roll back to the revoked generation.
	if code, _ := revoke("A111111", `{"reason":"pause"}`); code != 200 {
		t.Fatal("owner revoke")
	}
	cur = decodeServer(t, e.want(200, "GET", base, person("A111111"), ""))
	m = decodeServer(t, e.want(200, "PUT", base, person("A111111"), serverJSON("team-a", "svc", cur.Metadata.ResourceVersion, "six")))
	e.want(200, "POST", base+"/approval", person("A222222"), approval(6, m.Status.Pending.ContentHash))
	if code, out := rollback(`{"fromGeneration":6,"reason":"x"}`); code != 409 || !strings.Contains(out, "GenerationRevoked") {
		t.Fatalf("rollback to a revoked generation: %d %s", code, out)
	}
	if code, out := rollback(`{"fromGeneration":6,"toGeneration":1,"reason":"x"}`); code != 200 || decodeServer(t, out).Status.PublishedGeneration != 1 {
		t.Fatalf("explicit rollback to an allowed older generation: %d %s", code, out)
	}
	e.noOidLeak()
}

func TestRealPostgresCatalogShowsOnlyPublished(t *testing.T) {
	e := newApisEnv(t)
	e.publish("team-a", "live", "A111111", "A222222", "search")
	e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), serverJSON("team-a", "draft", "", "search"))
	e.publish("team-a", "gone", "A111111", "A222222", "search")
	e.want(200, "POST", nsBase+"team-a/mcpservers/gone/revocation", person("A111111"), `{"reason":"x"}`)
	// Any signed-in person, member or not.
	out := e.want(200, "GET", apisPrefix+"/mcpservercatalogentries", person("B444444"), "")
	var list struct{ Items []map[string]any }
	_ = json.Unmarshal([]byte(out), &list)
	if len(list.Items) != 1 || list.Items[0]["metadata"].(map[string]any)["name"] != "live" {
		t.Fatalf("catalog must hold only the published, unrevoked server: %s", out)
	}
	keys := func(m map[string]any) []string {
		return slices.Sorted(func(yield func(string) bool) {
			for k := range m {
				yield(k)
			}
		})
	}
	item := list.Items[0]
	for path, want := range map[string][]string{
		"":         {"apiVersion", "kind", "metadata", "spec", "status"},
		"metadata": {"name", "namespace"},
		"spec":     {"allowedTools", "dataCategory"},
		"status":   {"approvedAt", "publishedGeneration", "route"},
	} {
		m := item
		if path != "" {
			m = item[path].(map[string]any)
		}
		if got := keys(m); !slices.Equal(got, want) {
			t.Errorf("catalog %s fields: %v, want %v", path, got, want)
		}
	}
	e.want(200, "GET", nsBase+"team-a/mcpservercatalogentries/live", person("B444444"), "")
	e.want(404, "GET", nsBase+"team-a/mcpservercatalogentries/draft", person("B444444"), "")
	e.want(404, "GET", nsBase+"team-a/mcpservercatalogentries/gone", person("B444444"), "")
	e.noOidLeak()
}

// Data written by /admin/v1 (and so by an image from before 005) has no
// v1alpha1 hash or oid: it cannot be approved through /apis.
func TestRealPostgresLegacyDraftNeedsResubmit(t *testing.T) {
	e := newApisEnv(t)
	e.want(200, "PUT", "/admin/v1/servers/team-a/old/draft", person("A111111"), `{"route":{"target":"dev-gcp:team-a:old","upstream_url":"http://old.team-a/mcp","tools":["x"]}}`)
	m := decodeServer(t, e.want(200, "GET", nsBase+"team-a/mcpservers/old", person("A222222"), ""))
	if m.Status.Pending == nil || m.Status.Pending.ContentHash != "" || m.Status.Pending.SubmittedBy.Kind != "" {
		t.Fatalf("a legacy draft shows without hash or guessed kind: %+v", m.Status.Pending)
	}
	e.want(409, "POST", nsBase+"team-a/mcpservers/old/approval", person("A222222"), approval(1, ""))
	// Even with a hash forged into the row, a submitter without oid is refused.
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE server_versions SET spec_hash = 'v1:sha256:ab', submitted_by_kind = 'User' WHERE name = 'old'`); err != nil {
		t.Fatal(err)
	}
	e.want(403, "POST", nsBase+"team-a/mcpservers/old/approval", person("A222222"), approval(1, "v1:sha256:ab"))
}

func profileJSON(ns, name, rv string, grants ...string) string {
	gs := []map[string]any{}
	for _, g := range grants {
		ref, tools, _ := strings.Cut(g, "=")
		sns, sname, _ := strings.Cut(ref, "/")
		gs = append(gs, map[string]any{"serverRef": map[string]any{"group": apiGroup, "kind": "MCPServer", "namespace": sns, "name": sname},
			"allowedTools": strings.Split(tools, ","), "maxRisk": "Read"})
	}
	meta := map[string]any{"name": name, "namespace": ns}
	if rv != "" {
		meta["resourceVersion"] = rv
	}
	raw, _ := json.Marshal(map[string]any{"apiVersion": apiGroupVersion, "kind": "AgentProfile", "metadata": meta, "spec": map[string]any{
		"identity": map[string]any{"app": "studio-api", "cluster": "dev-gcp", "entraClientID": seedClientID},
		"mode":     "Delegated", "grants": gs,
		"quota": map[string]any{"maxInFlight": 4, "requestsPerMinute": 30, "dailyCallsPerUser": 200, "dailyCalls": 2000},
		"dpia":  map[string]any{"owner": ns, "reference": "PVK"},
	}})
	return string(raw)
}

func decodeProfile(t *testing.T, body string) AgentProfileV1 {
	t.Helper()
	var p AgentProfileV1
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return p
}

func grantBody(gen int, ref, hash string) string {
	ns, name, _ := strings.Cut(ref, "/")
	return fmt.Sprintf(`{"generation":%d,"serverRef":{"group":"fleet.nav.no","kind":"MCPServer","namespace":%q,"name":%q},"contentHash":%q}`, gen, ns, name, hash)
}

func TestRealPostgresAgentProfileV1(t *testing.T) {
	e := newApisEnv(t)
	e.publish("copilot", "dolly", "C222222", "A111111", "hent", "opprett")
	e.publish("team-b", "kodeverk", "B444444", "B555555", "lookup")
	base := nsBase + "aksel-studio/agentprofiles"
	body := profileJSON("aksel-studio", "assistant", "", "copilot/dolly=hent", "team-b/kodeverk=lookup")
	e.want(404, "POST", base, person("C222222"), body)
	for name, b := range map[string]string{
		"tool not on server": profileJSON("aksel-studio", "assistant", "", "copilot/dolly=slett"),
		"unpublished server": profileJSON("aksel-studio", "assistant", "", "copilot/nope=hent"),
		"write risk":         strings.Replace(body, `"Read"`, `"Write"`, 1),
		"agent principal":    strings.Replace(body, `"Delegated"`, `"Autonomous"`, 1),
		"wildcard":           profileJSON("aksel-studio", "assistant", "", "copilot/dolly=*"),
		"sts subject":        strings.Replace(body, `"cluster"`, `"stsSubject":"x","cluster"`, 1),
		"foreign client id":  strings.Replace(body, seedClientID, "11111111-2222-4333-8444-555555555555", 1),
	} {
		if code, out := e.do("POST", base, person("A111111"), b); code != 422 {
			t.Errorf("%s: want 422, got %d %s", name, code, out)
		}
	}
	p := decodeProfile(t, e.want(201, "POST", base, person("A111111"), body))
	if p.Status.Pending == nil || len(p.Status.Grants) != 2 || condOf(p.Status.Conditions, "PendingApproval").Reason != "AwaitingGrant" || condOf(p.Status.Conditions, "IdentityVerified").Status != "True" {
		t.Fatalf("created: %+v", p.Status)
	}
	hashFor := map[string]string{}
	for _, g := range p.Status.Grants {
		hashFor[g.ServerRef.key()] = g.ContentHash
	}
	// The server's owner team sees an excerpt: only its own grant, no actors.
	ex := e.want(200, "GET", base+"/assistant", person("B444444"), "")
	if strings.Contains(ex, "copilot") || strings.Contains(ex, "submittedBy") || strings.Contains(ex, "dpia") || strings.Contains(ex, "A111111") {
		t.Fatalf("excerpt leaks another team's grant or actors: %s", ex)
	}
	e.want(404, "GET", base+"/assistant", person("P999999"), "")
	e.want(404, "GET", base+"/assistant/history", person("B444444"), "")
	e.want(404, "GET", base+"/assistant/generations/1", person("B444444"), "")
	inbox := e.want(200, "GET", apisPrefix+"/agentprofiles?awaitingApprovalBy=me", person("B444444"), "")
	if !strings.Contains(inbox, "kodeverk") || strings.Contains(inbox, "dolly") {
		t.Fatalf("inbox: %s", inbox)
	}
	// Only the server's owner team may grant; the profile team gets 403, others 404.
	e.want(403, "POST", base+"/assistant/approval", person("A555555"), grantBody(1, "team-b/kodeverk", hashFor["team-b/kodeverk"]))
	e.want(404, "POST", base+"/assistant/approval", person("P999999"), grantBody(1, "team-b/kodeverk", hashFor["team-b/kodeverk"]))
	// A hash over another team's grant, or the whole spec, does not grant.
	e.want(409, "POST", base+"/assistant/approval", person("B444444"), grantBody(1, "team-b/kodeverk", hashFor["copilot/dolly"]))
	e.want(409, "POST", base+"/assistant/approval", person("B444444"), grantBody(1, "team-b/kodeverk", p.Status.Pending.ContentHash))
	// The submitter is also in copilot and cannot grant their own profile.
	e.want(403, "POST", base+"/assistant/approval", person("A111111"), grantBody(1, "copilot/dolly", hashFor["copilot/dolly"]))
	e.want(200, "POST", base+"/assistant/approval", person("B444444"), grantBody(1, "team-b/kodeverk", hashFor["team-b/kodeverk"]))
	e.want(409, "POST", base+"/assistant/approval", person("B555555"), grantBody(1, "team-b/kodeverk", hashFor["team-b/kodeverk"]))
	if ex2 := e.want(200, "GET", base+"/assistant", person("B555555"), ""); strings.Contains(ex2, "grantedBy") || strings.Contains(ex2, "B444444") {
		t.Fatalf("excerpt shows who granted: %s", ex2)
	}
	ex = e.want(200, "POST", base+"/assistant/approval", person("C222222"), grantBody(1, "copilot/dolly", hashFor["copilot/dolly"]))
	if strings.Contains(ex, "kodeverk") {
		t.Fatalf("the granting team's answer must be an excerpt: %s", ex)
	}
	p = decodeProfile(t, e.want(200, "GET", base+"/assistant", person("A555555"), ""))
	if p.Status.PublishedGeneration != 1 || condOf(p.Status.Conditions, "Ready").Status != "True" || p.Status.ExpiresAt == nil {
		t.Fatalf("all grants given must publish: %+v", p.Status)
	}
	// One live profile per client id.
	e.want(409, "POST", base, person("A111111"), profileJSON("aksel-studio", "copy", "", "copilot/dolly=hent"))
	// A label change keeps the generation.
	full := decodeProfile(t, e.want(200, "GET", base+"/assistant", person("A555555"), ""))
	same := strings.Replace(profileJSON("aksel-studio", "assistant", full.Metadata.ResourceVersion, "team-b/kodeverk=lookup", "copilot/dolly=hent"), `"name":"assistant"`, `"name":"assistant","labels":{"x":"y"}`, 1)
	if q := decodeProfile(t, e.want(200, "PUT", base+"/assistant", person("A555555"), same)); q.Metadata.Generation != 1 || q.Status.Pending != nil {
		t.Fatalf("same spec must not make a generation: %+v", q.Metadata)
	}
	e.want(409, "PUT", base+"/assistant", person("A555555"), same)
	// A server owner revokes only its own grant.
	e.want(200, "POST", base+"/assistant/revocation", person("B444444"), `{"reason":"x","serverRef":{"group":"fleet.nav.no","kind":"MCPServer","namespace":"team-b","name":"kodeverk"}}`)
	e.want(404, "POST", base+"/assistant/revocation", person("B444444"), `{"reason":"x"}`)
	p = decodeProfile(t, e.want(200, "POST", base+"/assistant/revocation", person("P999999"), `{"reason":"incident"}`))
	if condOf(p.Status.Conditions, "Revoked").Reason != "RevokedByPlatform" {
		t.Fatalf("platform revoke: %+v", p.Status.Conditions)
	}
	// Revocation freed the client id.
	e.want(201, "POST", base, person("A111111"), profileJSON("aksel-studio", "copy", "", "copilot/dolly=hent"))
	e.noOidLeak()
}

func TestRealPostgresSeededProfileNeedsPerson(t *testing.T) {
	e := newApisEnv(t)
	e.publish("copilot", "dolly", "C222222", "A111111", "hent")
	var p AgentProfile
	if err := json.Unmarshal([]byte(fixture(t)), &p); err != nil {
		t.Fatal(err)
	}
	p.Grants[0].Tools = []string{"hent"}
	if err := e.db.seedProfileV1(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := e.db.seedProfileV1(context.Background(), p); err != nil {
		t.Fatalf("seeding twice must be a no-op: %v", err)
	}
	got := decodeProfile(t, e.want(200, "GET", nsBase+"aksel-studio/agentprofiles/studio-assistant", person("A555555"), ""))
	if got.Status.Pending.SubmittedBy.Kind != "System" {
		t.Fatalf("seeded submitter: %+v", got.Status.Pending.SubmittedBy)
	}
	e.want(403, "POST", nsBase+"aksel-studio/agentprofiles/studio-assistant/approval", person("C222222"), grantBody(1, "copilot/dolly", got.Status.Grants[0].ContentHash))
}

// TestRealPostgresMigration005 stands where an image from before 005 left
// the database, then migrates. The gateway snapshot must be byte for byte
// the same, live agent profiles are copied with v0 hashes nobody can
// approve, and the backfill fills only published versions, once.
func TestRealPostgresMigration005(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "004_runtime_jobs.sql"); err != nil {
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
	// A pending draft and a revoked server, as the old image writes them.
	if _, err = db.SaveDraft(ctx, "A111111", Server{Namespace: servers[0].Namespace, Name: servers[0].Name, Route: servers[0].Route, Registry: servers[0].Registry}, "h"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.pool.Exec(ctx, `UPDATE servers SET status = 'revoked' WHERE namespace = $1 AND name = $2`, servers[1].Namespace, servers[1].Name); err != nil {
		t.Fatal(err)
	}
	var p AgentProfile
	_ = json.Unmarshal([]byte(fixture(t)), &p)
	id, err := db.CreateAgentProfile(ctx, p, "A111111", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.pool.Exec(ctx, `INSERT INTO agent_profile_approvals (profile_id, route, approved_by) VALUES ($1, 'copilot/dolly', 'C222222')`, id); err != nil {
		t.Fatal(err)
	}
	snap := func() []byte {
		all, qErr := queryServers(ctx, db.pool)
		if qErr != nil {
			t.Fatal(qErr)
		}
		s, bErr := buildSnapshot("routes", all, nil)
		if bErr != nil {
			t.Fatal(bErr)
		}
		return append([]byte(s.ETag+"\n"), s.Items...)
	}
	before := snap()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := db.BackfillSpecs(ctx)
	if err != nil || n != len(servers) {
		t.Fatalf("backfill: %d %v, want %d published versions", n, err, len(servers))
	}
	if again, _ := db.BackfillSpecs(ctx); again != 0 {
		t.Fatalf("backfill must be idempotent: %d", again)
	}
	if after := snap(); string(after) != string(before) {
		t.Fatalf("routes snapshot changed:\nbefore %s\nafter  %s", before, after)
	}
	var pendingHash *string
	_ = db.pool.QueryRow(ctx, `SELECT spec_hash FROM server_versions WHERE version = 2`).Scan(&pendingHash)
	if pendingHash != nil {
		t.Fatal("a pending version must never get a v1alpha1 hash from the backfill")
	}
	var revokedAt *string
	_ = db.pool.QueryRow(ctx, `SELECT revoked_at::text FROM server_versions WHERE namespace = $1 AND name = $2`, servers[1].Namespace, servers[1].Name).Scan(&revokedAt)
	if revokedAt == nil {
		t.Fatal("a generation revoked before 005 must be marked")
	}
	var specHash, grantHash, server string
	if err := db.pool.QueryRow(ctx, `SELECT g.spec_hash, g.grant_hashes->>'copilot/dolly', a.server FROM agent_profile_generations g
		JOIN agent_profile_grants a USING (namespace, name, generation)`).Scan(&specHash, &grantHash, &server); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(specHash, "v0:sha256:") || !strings.HasPrefix(grantHash, "v0:sha256:") || server != "copilot/dolly" {
		t.Fatalf("copied profile: %s %s %s", specHash, grantHash, server)
	}
	// The old image keeps working against the new schema.
	if _, err := db.SaveDraft(ctx, "A111111", Server{Namespace: "team-a", Name: "late", Route: &Route{Path: "/mcp/team-a/late", Target: "dev-gcp:team-a:late", UpstreamURL: "http://late.team-a", Enabled: true, Tools: []string{"x"}}}, "h2"); err != nil {
		t.Fatalf("old SaveDraft on the new schema: %v", err)
	}
	if err := db.Approve(ctx, "A222222", "team-a", "late", 1, "h2"); err != nil {
		t.Fatalf("old Approve on the new schema: %v", err)
	}
}
