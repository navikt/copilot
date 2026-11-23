package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

var dollyTools = []string{"dolly_current_user", "dolly_list_groups", "dolly_get_group", "dolly_search",
	"dolly_get_order", "dolly_list_group_orders", "create_order",
	// Wildcards on the route itself, so only the tool-name guard can stop them.
	"*", "dolly_*"}

// fakeVerifier accepts only the seed's client id for aksel-studio/studio-api.
type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, id AgentIdentity) (bool, error) {
	return id.EntraClientID == seedClientID && id.Namespace == "aksel-studio" && id.App == "studio-api", nil
}

const seedClientID = "7009f359-1451-429f-94d7-535d5c0c73c0"

type agentEnv struct {
	t   *testing.T
	db  *DB
	srv *server
	url string
}

func newAgentEnv(t *testing.T) *agentEnv {
	t.Helper()
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	servers := []Server{
		{Namespace: "copilot", Name: "dolly", Route: &Route{Path: "/mcp/copilot/dolly", Target: "t", UpstreamURL: "http://mcp-dolly", Enabled: true, Tools: dollyTools}},
		{Namespace: "team-b", Name: "kodeverk", Route: &Route{Path: "/mcp/team-b/kodeverk", Target: "t", UpstreamURL: "http://kodeverk", Enabled: true, Tools: []string{"lookup"}}},
		{Namespace: "team-c", Name: "registry-only", Registry: json.RawMessage(`{"name":"x"}`), RegistryName: "x"},
	}
	if _, err := db.Seed(ctx, servers, "test"); err != nil {
		t.Fatal(err)
	}
	texas := fakeTexas(t)
	a, err := newAuthz(texas.URL, gatewayID, registryID)
	if err != nil {
		t.Fatal(err)
	}
	a.homeTenant = testTenant
	s := newServer()
	s.verifier = fakeVerifier{}
	s.store.Store(&storeBox{db})
	// The real user-token path from #2678: Texas plus the static team list.
	teams := staticTeams{"aksel-studio": {"A111111", "A555555"}, "copilot": {"A111111", "C222222"}, "team-b": {"B444444"}, "team-x": {"X333333"}}
	ts := httptest.NewServer(s.routes(a, adminID, teams, nil))
	t.Cleanup(ts.Close)
	return &agentEnv{t, db, s, ts.URL}
}

func (e *agentEnv) do(method, path, person, body string) (int, string) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.url+path, strings.NewReader(body))
	ident, _, _ := strings.Cut(person, ":")
	switch {
	case strings.HasPrefix(path, "/internal/"):
		req.Header.Set("Authorization", "Bearer "+person)
	case ident != "":
		req.Header.Set("Authorization", "Bearer "+userTok(ident))
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // httptest server URL
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// ab is an approval body for route on the current live profile.
func (e *agentEnv) ab(route string) string {
	e.t.Helper()
	var id int64
	_ = e.db.pool.QueryRow(context.Background(), `SELECT COALESCE(max(id), 0) FROM agent_profiles WHERE status IN ('draft', 'approved')`).Scan(&id)
	return fmt.Sprintf(`{"route":%q,"id":%d}`, route, id)
}

func (e *agentEnv) active() []AgentProfile {
	e.t.Helper()
	code, body := e.do("GET", "/internal/v1/agents", gatewayID, "")
	if code != 200 {
		e.t.Fatalf("snapshot: %d %s", code, body)
	}
	var out struct{ Agents []AgentProfile }
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		e.t.Fatal(err)
	}
	return out.Agents
}

func fixture(t *testing.T) string {
	t.Helper()
	// The dev seed holds one profile; tests use it as the request body.
	raw, err := os.ReadFile("seed/agents.dev.json")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(raw)), "["), "]")
}

const (
	author   = "A111111:aksel-studio"
	approver = "C222222:copilot"
	stranger = "X333333:team-x"
	// The second member of the owner team (styringsmodell v0.3).
	ownerApprover = "A555555:aksel-studio"
)

// ownerApprove gives the owner team's approval, keyed by namespace.
func (e *agentEnv) ownerApprove() (int, string) {
	e.t.Helper()
	return e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", ownerApprover, e.ab("aksel-studio"))
}

func TestRealPostgresAgentProfileLifecycle(t *testing.T) {
	e := newAgentEnv(t)
	profile := fixture(t)

	if code, body := e.do("POST", "/admin/v1/agents", stranger, profile); code != 403 {
		t.Fatalf("create by another team: %d %s", code, body)
	}
	if code, body := e.do("POST", "/admin/v1/agents", author, profile); code != 201 {
		t.Fatalf("create: %d %s", code, body)
	}
	e.srv.verifier = nil
	sameName := strings.Replace(profile, seedClientID, "00000000-0000-4000-8000-0000000000b6", 1)
	if code, _ := e.do("POST", "/admin/v1/agents", author, sameName); code != 409 {
		t.Fatalf("second live profile with the same name: %d", code)
	}
	e.srv.verifier = fakeVerifier{}
	if got := e.active(); len(got) != 0 {
		t.Fatalf("draft must not be in the snapshot: %+v", got)
	}

	approve := e.ab("copilot/dolly")
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", stranger, approve); code != 403 {
		t.Fatalf("approval outside the route team: %d", code)
	}
	// The author is also in the route team (copilot); four eyes still applies.
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", author, approve); code != 403 {
		t.Fatalf("self-approval: %d", code)
	}
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("team-b/kodeverk")); code != 403 {
		t.Fatalf("approval of a route the approver does not own: %d", code)
	}
	// A route the profile does not grant must not count towards approval.
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", "B444444:team-b", e.ab("team-b/kodeverk")); code != 400 || len(e.active()) != 0 {
		t.Fatalf("approval of an ungranted route: %d", code)
	}
	code, body := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, approve)
	if code != 200 || !strings.Contains(body, `"draft"`) || len(e.active()) != 0 {
		t.Fatalf("route owner alone must not approve: %d %s", code, body)
	}
	if c, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", author, e.ab("aksel-studio")); c != 403 {
		t.Fatalf("owner-team self-approval: %d", c)
	}
	if c, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("aksel-studio")); c != 403 {
		t.Fatalf("owner-team approval from outside the team: %d", c)
	}
	code, body = e.ownerApprove()
	if code != 200 || !strings.Contains(body, `"approved"`) {
		t.Fatalf("approve: %d %s", code, body)
	}
	got := e.active()
	if len(got) != 1 || got[0].Identity.App != "studio-api" || len(got[0].Grants[0].Tools) != 6 || got[0].ExpiresAt == nil {
		t.Fatalf("snapshot after approval: %+v", got)
	}
	if d := time.Until(*got[0].ExpiresAt); d < 180*24*time.Hour || d > 185*24*time.Hour {
		t.Fatalf("expiry not six months: %v", d)
	}
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, approve); code != 409 {
		t.Fatalf("second approval: %d", code)
	}
	if code, _ := e.do("GET", "/internal/v1/agents", registryID, ""); code != 403 {
		t.Fatalf("mcp-registry must not read agent profiles: %d", code)
	}

	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/revoke", stranger, ""); code != 403 {
		t.Fatalf("revoke by stranger: %d", code)
	}
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/revoke", approver, ""); code != 204 {
		t.Fatalf("revoke by route owner: %d", code)
	}
	if got := e.active(); len(got) != 0 {
		t.Fatalf("revoked profile in snapshot: %+v", got)
	}

	rows, err := e.db.pool.Query(context.Background(), `SELECT action FROM audit_events WHERE namespace = 'aksel-studio' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		actions = append(actions, a)
	}
	want := "agent_profile_drafted agent_profile_route_approved agent_profile_route_approved agent_profile_approved agent_profile_revoked"
	if strings.Join(actions, " ") != want {
		t.Fatalf("audit: %v", actions)
	}
}

func TestRealPostgresAgentProfileExpiryAtRead(t *testing.T) {
	e := newAgentEnv(t)
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly"))
	e.ownerApprove()
	if len(e.active()) != 1 {
		t.Fatal("approved profile missing")
	}
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE agent_profiles SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if got := e.active(); len(got) != 0 {
		t.Fatalf("expired profile in snapshot: %+v", got)
	}
}

func TestRealPostgresAgentProfileNeedsEveryRouteOwner(t *testing.T) {
	e := newAgentEnv(t)
	p := strings.Replace(fixture(t), `"max_risk": "read"
    }`, `"max_risk": "read"
    },
    {"route": "team-b/kodeverk", "tools": ["lookup"], "max_risk": "read"}`, 1)
	if code, body := e.do("POST", "/admin/v1/agents", author, p); code != 201 {
		t.Fatalf("create: %d %s", code, body)
	}
	e.ownerApprove()
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly"))
	if len(e.active()) != 0 {
		t.Fatal("approved before every route owner approved")
	}
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", "B444444:team-b", e.ab("team-b/kodeverk"))
	if len(e.active()) != 1 {
		t.Fatal("not approved after every route owner approved")
	}
}

func TestRealPostgresAgentProfileRejectsHostileInput(t *testing.T) {
	e := newAgentEnv(t)
	base := fixture(t)
	cases := map[string][2]string{
		"wildcard tool":   {`"dolly_search"`, `"*"`},
		"glob tool":       {`"dolly_search"`, `"dolly_*"`},
		"agent principal": {`"delegated"`, `"agent-principal"`},
		"autonomous":      {`"delegated"`, `"autonomous"`},
		"app in other ns": {`"namespace": "aksel-studio",
    "app"`, `"namespace": "copilot",
    "app"`},
		"sts subject":         {`"app": "studio-api",`, `"app": "studio-api", "sts_subject": "x",`},
		"bad client id":       {seedClientID, `not-a-uuid`},
		"unknown field":       {`"mode"`, `"enabled": true, "mode"`},
		"status set":          {`"mode"`, `"status": "approved", "mode"`},
		"tool not on route":   {`"dolly_search"`, `"dolly_delete_everything"`},
		"write tool risk":     {`"max_risk": "read"`, `"max_risk": "write"`},
		"duplicate tool":      {`"dolly_get_group"`, `"dolly_search"`},
		"unknown route":       {`"copilot/dolly"`, `"copilot/nope"`},
		"registry-only route": {`"copilot/dolly"`, `"team-c/registry-only"`},
		"zero quota":          {`"max_in_flight": 4`, `"max_in_flight": 0`},
		"huge quota":          {`"daily_calls": 2000`, `"daily_calls": 99999999`},
		"empty dpia":          {`"reference": "PVK for KI-assistenten i Aksel Studio (KI_FLEET_DOLLY), #2582"`, `"reference": ""`},
		"control in dpia":     {`"reference": "PVK`, `"reference": "\u0000PVK`},
		"bad route":           {`"copilot/dolly"`, `"copilot/dolly/../x"`},
		"trailing": {`}
  ]`, `}
  ], "a": 1`},
	}
	for name, c := range cases {
		body := strings.Replace(base, c[0], c[1], 1)
		if body == base {
			t.Fatalf("%s: replacement did not apply", name)
		}
		if code, resp := e.do("POST", "/admin/v1/agents", author, body); code != 400 {
			t.Errorf("%s: %d %s", name, code, resp)
		}
	}
	for _, tail := range []string{"{}", "}", "]"} {
		if code, _ := e.do("POST", "/admin/v1/agents", author, base+tail); code != 400 {
			t.Errorf("trailing %q: %d", tail, code)
		}
	}
	if code, _ := e.do("POST", "/admin/v1/agents", "", base); code != 401 {
		t.Errorf("no person: %d", code)
	}
	var n int
	_ = e.db.pool.QueryRow(context.Background(), `SELECT count(*) FROM agent_profiles`).Scan(&n)
	if n != 0 {
		t.Fatalf("rejected input stored %d profiles", n)
	}
}

// The database refuses an agent principal even if the API check is bypassed.
func TestRealPostgresAgentProfileSchemaRejectsAgentPrincipal(t *testing.T) {
	e := newAgentEnv(t)
	_, err := e.db.pool.Exec(context.Background(), `INSERT INTO agent_profiles (namespace, name, cluster, app, entra_client_id, mode, grants,
		max_in_flight, rpm, daily_calls_per_user, daily_calls, dpia_owner, dpia_ref, status, created_by)
		VALUES ('a', 'b', 'dev-gcp', 'c', gen_random_uuid(), 'autonomous', '[{}]', 1, 1, 1, 1, 'a', 'r', 'draft', 'x')`)
	if err == nil {
		t.Fatal("schema accepted mode autonomous")
	}
}

func TestAgentWriteRoutesNeedAdminClient(t *testing.T) {
	texas := fakeTexas(t)
	a, err := newAuthz(texas.URL, gatewayID, registryID)
	if err != nil {
		t.Fatal(err)
	}
	a.homeTenant = testTenant
	ts := httptest.NewServer(newServer().routes(a, "", staticTeams{"aksel-studio": {"A111111"}}, nil))
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/admin/v1/agents", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("write route mounted without the admin client: %d", resp.StatusCode)
	}
}

func TestRealPostgresAgentProfileUnverifiedStaysOut(t *testing.T) {
	e := newAgentEnv(t)
	e.srv.verifier = nil
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly"))
	code, body := e.ownerApprove()
	if code != 200 || !strings.Contains(body, `"approved"`) {
		t.Fatalf("approve: %d %s", code, body)
	}
	if got := e.active(); len(got) != 0 {
		t.Fatalf("unverified profile in snapshot: %+v", got)
	}
}

func TestRealPostgresAgentProfileClientIDBinding(t *testing.T) {
	e := newAgentEnv(t)
	// Team B claims aksel-studio's client id for its own app.
	stolen := strings.NewReplacer(`"aksel-studio"`, `"team-b"`, `"studio-api"`, `"thief"`).Replace(fixture(t))
	if code, body := e.do("POST", "/admin/v1/agents", "B444444:team-b", stolen); code != 400 {
		t.Fatalf("verifier must refuse another app's client id: %d %s", code, body)
	}
	// Without a verifier the claim is stored, but unverified it reserves nothing.
	e.srv.verifier = nil
	if code, _ := e.do("POST", "/admin/v1/agents", "B444444:team-b", stolen); code != 201 {
		t.Fatalf("unverified claim: %d", code)
	}
	e.srv.verifier = fakeVerifier{}
	if code, body := e.do("POST", "/admin/v1/agents", author, fixture(t)); code != 201 {
		t.Fatalf("an unverified claim blocked the verified owner: %d %s", code, body)
	}
	other := strings.Replace(fixture(t), `"studio-assistant"`, `"studio-assistant-2"`, 1)
	if code, _ := e.do("POST", "/admin/v1/agents", author, other); code != 409 {
		t.Fatalf("client id in two verified live profiles: %d", code)
	}
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/revoke", author, "")
	if code, _ := e.do("POST", "/admin/v1/agents", author, other); code != 201 {
		t.Fatalf("client id after revoke: %d", code)
	}
}

func TestRealPostgresAgentApprovalBindsToDraft(t *testing.T) {
	e := newAgentEnv(t)
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	stale := e.ab("copilot/dolly")
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/revoke", author, "")
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, stale); code != 409 {
		t.Fatalf("approval for a revoked draft applied to its replacement: %d", code)
	}
}

// A route revoked in a transaction that commits during the approval must
// fail the approval, not be checked against a stale read.
func TestRealPostgresAgentApprovalWaitsForRouteRevoke(t *testing.T) {
	e := newAgentEnv(t)
	ctx := context.Background()
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	body := e.ab("copilot/dolly")
	tx, err := e.db.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM servers WHERE name = 'dolly' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE servers SET status = 'revoked' WHERE name = 'dolly'`); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() {
		code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, body)
		done <- code
	}()
	// Wait until the approval is blocked on the row lock.
	for waiting := 0; waiting == 0; {
		_ = e.db.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock'`).Scan(&waiting)
		select {
		case code := <-done:
			t.Fatalf("approval did not wait for the route lock: %d", code)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 400 {
		t.Fatalf("approval against a route revoked meanwhile: %d", code)
	}
}

func TestRealPostgresAgentProfileExpiredCanBeReplaced(t *testing.T) {
	e := newAgentEnv(t)
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly"))
	e.ownerApprove()
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE agent_profiles SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if code, body := e.do("POST", "/admin/v1/agents", author, fixture(t)); code != 201 {
		t.Fatalf("re-create after expiry: %d %s", code, body)
	}
	var statuses string
	_ = e.db.pool.QueryRow(context.Background(), `SELECT string_agg(status, ' ' ORDER BY id) FROM agent_profiles`).Scan(&statuses)
	var audited int
	_ = e.db.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'agent_profile_expired'`).Scan(&audited)
	if statuses != "expired draft" || audited != 1 {
		t.Fatalf("statuses %q, expiry audits %d", statuses, audited)
	}
}

func TestRealPostgresAgentProfileApprovalRechecksRoute(t *testing.T) {
	e := newAgentEnv(t)
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE servers SET status = 'revoked' WHERE name = 'dolly'`); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly")); code != 400 {
		t.Fatalf("approval of a revoked route: %d", code)
	}
}

func TestRealPostgresAgentSeed(t *testing.T) {
	e := newAgentEnv(t)
	ctx := context.Background()
	t.Setenv("SEED_AGENTS_FILE", "seed/agents.dev.json")
	t.Setenv("NAIS_CLUSTER_NAME", "prod-gcp")
	if err := seedAgentsFromEnv(ctx, e.db); err == nil {
		t.Fatal("agent seed ran in prod")
	}
	t.Setenv("NAIS_CLUSTER_NAME", "dev-gcp")
	for range 2 {
		if err := seedAgentsFromEnv(ctx, e.db); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	var status, by string
	var verified bool
	_ = e.db.pool.QueryRow(ctx, `SELECT count(*), min(status), min(created_by), bool_or(identity_verified_at IS NOT NULL) FROM agent_profiles`).Scan(&n, &status, &by, &verified)
	if n != 1 || status != "draft" || by != "seed" || verified {
		t.Fatalf("seed: n=%d status=%s by=%s verified=%v", n, status, by, verified)
	}
}

func TestRealPostgresAgentDetailAndSeededApproval(t *testing.T) {
	e := newAgentEnv(t)
	t.Setenv("SEED_AGENTS_FILE", "seed/agents.dev.json")
	t.Setenv("NAIS_CLUSTER_NAME", "dev-gcp")
	if err := seedAgentsFromEnv(context.Background(), e.db); err != nil {
		t.Fatal(err)
	}
	path := "/admin/v1/agents/aksel-studio/studio-assistant"
	for _, who := range []string{author, approver} {
		if code, body := e.do("GET", path, who, ""); code != 200 || !strings.Contains(body, `"copilot/dolly":null`) || !strings.Contains(body, `"aksel-studio":null`) {
			t.Fatalf("%s: %d %s", who, code, body)
		}
	}
	if code, _ := e.do("GET", path, stranger, ""); code != 404 {
		t.Fatalf("another team must get 404: %d", code)
	}
	if code, _ := e.do("GET", path, "", ""); code != 401 {
		t.Fatalf("unauthenticated: %d", code)
	}
	_, body := e.do("GET", path, approver, "")
	var got struct {
		ID      int64
		Status  string
		Profile AgentProfile
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil || got.ID == 0 || got.Status != "draft" || got.Profile.Identity.App != "studio-api" {
		t.Fatalf("detail: %v %s", err, body)
	}
	// "seed" is not a person: one owner-team member must not complete it.
	if code, _ := e.do("POST", path+"/approvals", ownerApprover, fmt.Sprintf(`{"route":"aksel-studio","id":%d}`, got.ID)); code != 403 {
		t.Fatalf("seeded draft approved by one owner-team person: %d", code)
	}
	e.do("POST", path+"/revoke", author, "")
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	_, body = e.do("GET", path, approver, "")
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	e.do("POST", path+"/approvals", approver, fmt.Sprintf(`{"route":"copilot/dolly","id":%d}`, got.ID))
	code, resp := e.do("POST", path+"/approvals", ownerApprover, fmt.Sprintf(`{"route":"aksel-studio","id":%d}`, got.ID))
	if code != 200 || !strings.Contains(resp, `"approved"`) {
		t.Fatalf("approve re-submitted draft: %d %s", code, resp)
	}
	if _, body = e.do("GET", path, author, ""); !strings.Contains(body, `"approved_by":"C222222"`) {
		t.Fatalf("approval state: %s", body)
	}
}

// A grant must accept every tool name a route can register (#2677).
func TestAgentGrantToolMatchesRegisteredNames(t *testing.T) {
	for _, tool := range []string{".internal", "-dash"} {
		if err := validateTools([]string{tool}); err != nil {
			t.Fatalf("validateTools(%q): %v", tool, err)
		}
		var p AgentProfile
		if err := json.Unmarshal([]byte(fixture(t)), &p); err != nil {
			t.Fatal(err)
		}
		p.Grants[0].Tools = []string{tool}
		if err := p.validate(); err != nil {
			t.Fatalf("grant tool %q: %v", tool, err)
		}
	}
}
