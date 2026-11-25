package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/navikt/copilot/fleet-control/internal/auth"
	"github.com/navikt/copilot/fleet-control/internal/model"
)

const nsBase = apisPrefix + "/namespaces/"

// Every person below has an oid from testOid. A111111 is in two teams, so
// it can be both submitter and server owner.
var apisTeams = auth.StaticTeams{
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
	mu     sync.Mutex
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
	e.mu.Lock()
	e.bodies = append(e.bodies, string(raw))
	e.mu.Unlock()
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
	if i := slices.IndexFunc(servers, func(s model.Server) bool { return s.Key() == "team-a/svc" }); i < 0 || servers[i].Route.Target != "dev-gcp:team-a:svc" || servers[i].DataClass != "Intern" {
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
	// A revocation older than 90 days survives retention (#2762).
	revokedAt := condOf(decodeServer(t, e.want(200, "GET", base, person("A111111"), "")).Status.Conditions, "Revoked").LastTransitionTime
	// A second server revoked by the platform without block shows by_role.
	e.publish("team-a", "svc2", "A111111", "A222222", "one")
	e.want(200, "POST", nsBase+"team-a/mcpservers/svc2/revocation", person("P999999"), `{"reason":"incident"}`)
	ageAudit(t, e.db)
	if m := decodeServer(t, e.want(200, "GET", nsBase+"team-a/mcpservers/svc2", person("A111111"), "")); condOf(m.Status.Conditions, "Revoked").Reason != "RevokedByPlatform" {
		t.Fatalf("svc2 after prune: %+v", m.Status.Conditions)
	}
	if m := decodeServer(t, e.want(200, "GET", base, person("A111111"), "")); condOf(m.Status.Conditions, "Revoked").Reason != "Blocked" || condOf(m.Status.Conditions, "Revoked").Status != "True" ||
		!condOf(m.Status.Conditions, "Revoked").LastTransitionTime.Equal(revokedAt) {
		t.Fatalf("after prune: %+v", m.Status.Conditions)
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
	// Tool descriptions reach other teams through the catalog; upstream and readme do not.
	doc := `{"apiVersion":"` + apiGroupVersion + `","kind":"MCPServer","metadata":{"name":"doc","namespace":"team-a"},"spec":{"dataCategory":"Intern","upstream":{"app":"svc","url":"http://svc.team-a/mcp"},"allowedTools":["search"],"readme":"secret","tools":[{"name":"search","description":"Finds things."}]}}`
	m := decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), doc))
	e.want(200, "POST", nsBase+"team-a/mcpservers/doc/approval", person("A222222"), approval(1, m.Status.Pending.ContentHash))
	got := e.want(200, "GET", nsBase+"team-a/mcpservercatalogentries/doc", person("B444444"), "")
	if !strings.Contains(got, `"tools":[{"name":"search","description":"Finds things."}]`) || strings.Contains(got, "secret") || strings.Contains(got, "upstream") {
		t.Fatalf("catalog entry tools: %s", got)
	}
	e.noOidLeak()
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
	// Two-person rule on stable identity (#2634): the submitter's oid under another
	// NAVident, and the submitter's NAVident with another oid, are both refused.
	e.want(403, "POST", base+"/assistant/approval", person("C222222")+":C222222:"+testOid("A111111"), grantBody(1, "copilot/dolly", hashFor["copilot/dolly"]))
	e.want(403, "POST", base+"/assistant/approval", person("A111111")+":A111111:"+testOid("C222222"), grantBody(1, "copilot/dolly", hashFor["copilot/dolly"]))
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
	// Every signed-in person sees which agent profiles use a server, by name only (#2981).
	if c := e.want(200, "GET", nsBase+"team-b/mcpservercatalogentries/kodeverk", person("P999999"), ""); !strings.Contains(c, `"consumers":[{"namespace":"aksel-studio","name":"assistant"}]`) || strings.Contains(c, seedClientID) {
		t.Fatalf("catalog consumers: %s", c)
	}
	// A server owner sees when the profile expires (#2696).
	if ex := decodeProfile(t, e.want(200, "GET", base+"/assistant", person("B444444"), "")); ex.Status.ExpiresAt == nil {
		t.Fatalf("excerpt must show expiresAt: %+v", ex.Status)
	}
	// A hidden profile costs a non-member the same one query as a missing one
	// (#2696): with audit_events gone, building the profile would fail with 500.
	hideTable(t, e.db, "audit_events", func() {
		e.want(404, "GET", base+"/assistant", person("P999999"), "")
		e.want(404, "GET", base+"/missing", person("P999999"), "")
	})
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
	if c := e.want(200, "GET", nsBase+"team-b/mcpservercatalogentries/kodeverk", person("P999999"), ""); strings.Contains(c, "consumers") {
		t.Fatalf("a revoked grant is not a consumer: %s", c)
	}
	p = decodeProfile(t, e.want(200, "POST", base+"/assistant/revocation", person("P999999"), `{"reason":"incident"}`))
	if condOf(p.Status.Conditions, "Revoked").Reason != "RevokedByPlatform" {
		t.Fatalf("platform revoke: %+v", p.Status.Conditions)
	}
	ageAudit(t, e.db)
	if p = decodeProfile(t, e.want(200, "GET", base+"/assistant", person("A555555"), "")); condOf(p.Status.Conditions, "Revoked").Reason != "RevokedByPlatform" {
		t.Fatalf("platform revoke after prune: %+v", p.Status.Conditions)
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
	if got.Status.Pending.SubmittedBy.Kind != "System" || !got.Status.NeedsResubmit {
		t.Fatalf("seeded submitter: %+v", got.Status.Pending.SubmittedBy)
	}
	// The granting team sees the flag too, on GET and in its inbox.
	if ex := decodeProfile(t, e.want(200, "GET", nsBase+"aksel-studio/agentprofiles/studio-assistant", person("C222222"), "")); !ex.Status.NeedsResubmit {
		t.Fatalf("excerpt drops needsResubmit: %+v", ex.Status)
	}
	if inbox := e.want(200, "GET", apisPrefix+"/agentprofiles?awaitingApprovalBy=me", person("C222222"), ""); !strings.Contains(inbox, `"needsResubmit":true`) {
		t.Fatalf("inbox drops needsResubmit: %s", inbox)
	}
	e.want(403, "POST", nsBase+"aksel-studio/agentprofiles/studio-assistant/approval", person("C222222"), grantBody(1, "copilot/dolly", got.Status.Grants[0].ContentHash))
	// Even with an oid, kind System never counts as a person.
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE agent_profile_generations SET submitted_by_oid = $1::uuid`, testOid("A555555")); err != nil {
		t.Fatal(err)
	}
	e.want(403, "POST", nsBase+"aksel-studio/agentprofiles/studio-assistant/approval", person("C222222"), grantBody(1, "copilot/dolly", got.Status.Grants[0].ContentHash))
}

// readme and tool descriptions come back byte for byte, and a person's
// pending profile does not need resubmitting.
func TestRealPostgresServerReadmeRoundTrip(t *testing.T) {
	e := newApisEnv(t)
	readme := "# Dolly\n\n<script>alert(1)</script> [x](javascript:alert(1))\n"
	body := strings.Replace(serverJSON("team-a", "svc", "", "a", "b"), `"allowedTools"`, `"readme":`+strconv.Quote(readme)+`,"tools":[{"name":"b","description":"Henter <b>b</b>"}],"allowedTools"`, 1)
	e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), body)
	m := decodeServer(t, e.want(200, "GET", nsBase+"team-a/mcpservers/svc", person("A222222"), ""))
	if p := m.Status.Pending; p == nil || p.Spec.Readme != readme || len(p.Spec.Tools) != 1 || p.Spec.Tools[0].Description != "Henter <b>b</b>" {
		t.Fatalf("round trip: %+v", m.Status.Pending)
	}
	e.want(422, "POST", nsBase+"team-a/mcpservers", person("A111111"), strings.Replace(serverJSON("team-a", "big", "", "a"), `"allowedTools"`, `"readme":"`+strings.Repeat("x", maxReadmeBytes+1)+`","allowedTools"`, 1))
	e.publish("copilot", "dolly", "C222222", "A111111", "hent")
	e.want(201, "POST", nsBase+"aksel-studio/agentprofiles", person("A555555"), profileJSON("aksel-studio", "p", "", "copilot/dolly=hent"))
	if got := decodeProfile(t, e.want(200, "GET", nsBase+"aksel-studio/agentprofiles/p", person("A555555"), "")); got.Status.Pending == nil || got.Status.NeedsResubmit {
		t.Fatalf("a person's draft needs no resubmit: %+v", got.Status)
	}
}

// A server team that is not granted anything on a profile gets the same 404
// as for a profile that does not exist, before any state shows.
func TestRealPostgresGrantActionsDoNotLeakProfiles(t *testing.T) {
	e := newApisEnv(t)
	e.publish("copilot", "dolly", "C222222", "A111111", "hent")
	e.publish("team-b", "kodeverk", "B444444", "B555555", "lookup")
	base := nsBase + "aksel-studio/agentprofiles/"
	e.want(201, "POST", nsBase+"aksel-studio/agentprofiles", person("A111111"), profileJSON("aksel-studio", "assistant", "", "copilot/dolly=hent"))
	ref := `{"group":"fleet.nav.no","kind":"MCPServer","namespace":"team-b","name":"kodeverk"}`
	for _, name := range []string{"assistant", "missing"} {
		for path, body := range map[string]string{
			"/approval":   grantBody(9, "team-b/kodeverk", "v1:sha256:00"),
			"/revocation": `{"reason":"x","serverRef":` + ref + `}`,
		} {
			code, out := e.do("POST", base+name+path, person("B444444"), body)
			if code != 404 || !strings.Contains(out, `"reason":"NotFound"`) {
				t.Errorf("%s%s: want 404, got %d %s", name, path, code, out)
			}
		}
	}
}

// Publishing re-checks every grant: one whose server was revoked after it
// was given stops the profile.
func TestRealPostgresPublishRechecksEveryGrant(t *testing.T) {
	e := newApisEnv(t)
	e.publish("copilot", "dolly", "C222222", "A111111", "hent")
	e.publish("team-b", "kodeverk", "B444444", "B555555", "lookup")
	base := nsBase + "aksel-studio/agentprofiles"
	p := decodeProfile(t, e.want(201, "POST", base, person("A111111"), profileJSON("aksel-studio", "assistant", "", "copilot/dolly=hent", "team-b/kodeverk=lookup")))
	h := map[string]string{}
	for _, g := range p.Status.Grants {
		h[g.ServerRef.key()] = g.ContentHash
	}
	e.want(200, "POST", base+"/assistant/approval", person("B444444"), grantBody(1, "team-b/kodeverk", h["team-b/kodeverk"]))
	after := decodeProfile(t, e.want(200, "GET", base+"/assistant", person("A555555"), ""))
	if after.Metadata.ResourceVersion == p.Metadata.ResourceVersion {
		t.Fatal("a grant must move resourceVersion")
	}
	e.want(200, "POST", nsBase+"team-b/mcpservers/kodeverk/revocation", person("B444444"), `{"reason":"x"}`)
	// Republishing the server does not bring the old grant back.
	srv := decodeServer(t, e.want(200, "GET", nsBase+"team-b/mcpservers/kodeverk", person("B444444"), ""))
	srv = decodeServer(t, e.want(200, "PUT", nsBase+"team-b/mcpservers/kodeverk", person("B444444"), serverJSON("team-b", "kodeverk", srv.Metadata.ResourceVersion, "lookup")))
	e.want(200, "POST", nsBase+"team-b/mcpservers/kodeverk/approval", person("B555555"), approval(int(srv.Metadata.Generation), srv.Status.Pending.ContentHash))
	if code, out := e.do("POST", base+"/assistant/approval", person("C222222"), grantBody(1, "copilot/dolly", h["copilot/dolly"])); code == 200 {
		t.Fatalf("published with a grant on a revoked server: %s", out)
	}
}

// hideTable renames table while f runs, so a test can prove a path never reads it.
func hideTable(t *testing.T, db *DB, table string, f func()) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.pool.Exec(ctx, `ALTER TABLE `+table+` RENAME TO hidden_`+table); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.pool.Exec(ctx, `ALTER TABLE hidden_`+table+` RENAME TO `+table); err != nil {
			t.Fatal(err)
		}
	}()
	f()
}

// A pending update that drops a team's grant hides the profile from that team
// at the precheck, as excerpt does, without building it first (#2696).
func TestRealPostgresProfilePrecheckFollowsPending(t *testing.T) {
	e := newApisEnv(t)
	e.publish("copilot", "dolly", "C222222", "A111111", "hent")
	e.publish("team-b", "kodeverk", "B444444", "B555555", "lookup")
	base := nsBase + "aksel-studio/agentprofiles"
	p := decodeProfile(t, e.want(201, "POST", base, person("A111111"), profileJSON("aksel-studio", "assistant", "", "copilot/dolly=hent", "team-b/kodeverk=lookup")))
	for _, g := range p.Status.Grants {
		owner := map[string]string{"copilot/dolly": "C222222", "team-b/kodeverk": "B444444"}[g.ServerRef.key()]
		e.want(200, "POST", base+"/assistant/approval", person(owner), grantBody(1, g.ServerRef.key(), g.ContentHash))
	}
	full := decodeProfile(t, e.want(200, "GET", base+"/assistant", person("A555555"), ""))
	e.want(200, "PUT", base+"/assistant", person("A555555"), profileJSON("aksel-studio", "assistant", full.Metadata.ResourceVersion, "copilot/dolly=hent"))
	hideTable(t, e.db, "audit_events", func() {
		e.want(404, "GET", base+"/assistant", person("B444444"), "")
	})
}

// An approver without an oid is refused before the two-person rule compares
// oids, not left to the uuid cast in the UPDATE (#2879).
func TestRealPostgresApproveNeedsApproverOid(t *testing.T) {
	e := newApisEnv(t)
	m := decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), serverJSON("team-a", "svc", "", "t")))
	err := e.db.approveServer(context.Background(), caller{Ident: "A222222"}, "platform", "team-a", "svc", 1, m.Status.Pending.ContentHash)
	if se, ok := errors.AsType[*statusError](err); !ok || se.code != http.StatusForbidden {
		t.Fatalf("approver without oid: %v", err)
	}
}

// spec.toolRisk survives the Postgres round trip into the routes snapshot. A
// legacy version without it has no tool_risk (every tool counts as write), and
// a malformed stored value fails the build and keeps the previous snapshot.
func TestRealPostgresToolRiskSnapshot(t *testing.T) {
	e := newApisEnv(t)
	ctx := context.Background()
	body := strings.Replace(serverJSON("team-a", "svc", "", "a", "b"), `"allowedTools"`, `"toolRisk":{"a":"read"},"allowedTools"`, 1)
	m := decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), body))
	e.want(200, "POST", nsBase+"team-a/mcpservers/svc/approval", person("A222222"), approval(1, m.Status.Pending.ContentHash))
	e.publish("team-a", "legacy", "A111111", "A222222", "c")
	body = strings.Replace(serverJSON("team-a", "dflt", "", "a", "b"), `"allowedTools"`, `"defaultRisk":"propose","toolRisk":{"a":"write"},"allowedTools"`, 1)
	m = decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), body))
	e.want(200, "POST", nsBase+"team-a/mcpservers/dflt/approval", person("A222222"), approval(1, m.Status.Pending.ContentHash))
	if _, err := e.db.pool.Exec(ctx, `UPDATE server_versions SET spec = spec - 'toolRisk' WHERE name = 'legacy'`); err != nil {
		t.Fatal(err)
	}
	snap, err := e.srv.resolve(ctx, "routes")
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		Path     string            `json:"path"`
		ToolRisk map[string]string `json:"tool_risk"`
	}
	if err = json.Unmarshal(snap.Items, &items); err != nil {
		t.Fatalf("%v: %s", err, snap.Items)
	}
	got := map[string]map[string]string{}
	for _, it := range items {
		got[it.Path] = it.ToolRisk
	}
	if r, ok := got["/mcp/team-a/svc"]; !ok || len(r) != 1 || r["a"] != "read" {
		t.Errorf("svc toolRisk lost: %s", snap.Items)
	}
	if r := got["/mcp/team-a/dflt"]; len(r) != 2 || r["a"] != "write" || r["b"] != "propose" {
		t.Errorf("dflt must carry resolved risk per tool: %s", snap.Items)
	}
	if r, ok := got["/mcp/team-a/legacy"]; !ok || r != nil {
		t.Errorf("legacy row must have no tool_risk: %s", snap.Items)
	}

	if _, err = e.db.pool.Exec(ctx, `UPDATE server_versions SET spec = jsonb_set(spec, '{toolRisk}', '"read"') WHERE name = 'svc'`); err != nil {
		t.Fatal(err)
	}
	if _, err = e.db.Publish(ctx, "routes", buildSnapshot); err == nil || !strings.Contains(err.Error(), "spec.toolRisk") {
		t.Errorf("malformed toolRisk must fail the build: %v", err)
	}
	kept, err := e.srv.resolve(ctx, "routes")
	if err != nil || kept.Generation != snap.Generation || kept.ETag != snap.ETag {
		t.Errorf("previous snapshot must be kept: %v %+v", err, kept)
	}
}

// fakeApps is a team directory that can also list Nais applications.
type fakeApps struct {
	auth.StaticTeams
	apps []auth.TeamApp
	err  error
}

func (f fakeApps) Apps(context.Context, string) ([]auth.TeamApp, error) { return f.apps, f.err }

func TestListTeamApps(t *testing.T) {
	ct := fakeApps{apps: []auth.TeamApp{{Name: "a-mcp", Environment: "dev-gcp"}}}
	call := func(teams auth.TeamDirectory, c caller) (int, string, error) {
		r := httptest.NewRequest("GET", nsBase+"team-a/apps", nil)
		r.SetPathValue("namespace", "team-a")
		w := httptest.NewRecorder()
		err := listTeamApps(teams)(w, r, c, nil)
		return w.Code, w.Body.String(), err
	}
	code, body, err := call(ct, caller{Teams: []string{"team-a"}})
	if err != nil || code != 200 || !strings.Contains(body, `"a-mcp"`) || !strings.Contains(body, `"TeamAppList"`) {
		t.Fatalf("member: %d %s %v", code, body, err)
	}
	if _, _, err = call(ct, caller{Teams: []string{"team-b"}}); err != errV1NotFound {
		t.Fatalf("non-member must get 404, got %v", err)
	}
	_, _, err = call(auth.StaticTeams{"team-a": {"x@nav.no"}}, caller{Teams: []string{"team-a"}})
	if se, ok := err.(*statusError); !ok || se.code != http.StatusServiceUnavailable {
		t.Fatalf("no Console must be 503, got %v", err)
	}
}

// A Console failure is logged, never answered: the 503 body holds only the generic message.
func TestListTeamAppsHidesConsoleError(t *testing.T) {
	const secret = "kari.nordmann@nav.no is not allowed"
	r := httptest.NewRequest("GET", nsBase+"team-a/apps", nil)
	r.SetPathValue("namespace", "team-a")
	w := httptest.NewRecorder()
	err := listTeamApps(fakeApps{err: errors.New("nais console: " + secret)})(w, r, caller{Teams: []string{"team-a"}}, nil)
	if err == nil {
		t.Fatal("want error")
	}
	writeStatus(w, r, "", err)
	if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "kari") || strings.Contains(w.Body.String(), "allowed") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
