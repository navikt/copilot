package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func decodeAs[T any](t *testing.T, code int, body string) T {
	t.Helper()
	var v T
	if code != 200 {
		t.Fatalf("status %d: %s", code, body)
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

type nameList []struct{ Namespace, Name string }

func TestRealPostgresUIReadScoping(t *testing.T) {
	e := newAgentEnv(t)
	const routeOwner, teamB = "C222222", "B444444"

	code, body := e.do("GET", "/admin/v1/me", teamB, "")
	me := decodeAs[struct{ Teams []string }](t, code, body)
	if len(me.Teams) != 1 || me.Teams[0] != "team-b" {
		t.Fatalf("me: %v", me.Teams)
	}

	servers := func(who, query string) nameList {
		t.Helper()
		c, b := e.do("GET", "/admin/v1/servers"+query, who, "")
		return decodeAs[struct{ Servers nameList }](t, c, b).Servers
	}
	if got := servers(teamB, ""); len(got) != 1 || got[0].Name != "kodeverk" {
		t.Fatalf("team-b servers: %+v", got)
	}
	if got := servers(teamB, "?namespace=copilot"); len(got) != 0 {
		t.Fatalf("another team's servers leaked: %+v", got)
	}
	if got := servers(routeOwner, "?namespace=copilot"); len(got) != 1 || got[0].Name != "dolly" {
		t.Fatalf("own namespace: %+v", got)
	}
	if c, _ := e.do("GET", "/admin/v1/servers?namespace=Bad_NS", teamB, ""); c != 400 {
		t.Fatalf("invalid namespace: %d", c)
	}

	if c, b := e.do("POST", "/admin/v1/agents", author, fixture(t)); c != 201 {
		t.Fatalf("create: %d %s", c, b)
	}
	agents := func(who, query string) nameList {
		t.Helper()
		c, b := e.do("GET", "/admin/v1/agents"+query, who, "")
		return decodeAs[struct{ Agents nameList }](t, c, b).Agents
	}
	if got := agents(author, ""); len(got) != 1 || got[0].Name != "studio-assistant" {
		t.Fatalf("owner agents: %+v", got)
	}
	// The route owner approves the profile but does not own it.
	if got := agents(routeOwner, ""); len(got) != 0 {
		t.Fatalf("route owner sees the profile in its list: %+v", got)
	}
	if got := agents(stranger, "?namespace=aksel-studio"); len(got) != 0 {
		t.Fatalf("another team's agents leaked: %+v", got)
	}

	inbox := func(who string) []pendingAgent {
		t.Helper()
		c, b := e.do("GET", "/admin/v1/agents/pending-approval", who, "")
		return decodeAs[struct{ Agents []pendingAgent }](t, c, b).Agents
	}
	if got := inbox(routeOwner); len(got) != 1 || len(got[0].Awaiting) != 1 || got[0].Awaiting[0] != "copilot/dolly" {
		t.Fatalf("route owner inbox: %+v", got)
	}
	for _, who := range []string{teamB, stranger} {
		if got := inbox(who); len(got) != 0 {
			t.Fatalf("%s inbox: %+v", who, got)
		}
	}

	// Audit: the owner team sees it; the route owner, another team and a
	// name that does not exist all get the same 404.
	code, body = e.do("GET", "/admin/v1/agents/aksel-studio/studio-assistant/audit", author, "")
	if ev := decodeAs[struct{ Events []auditEvent }](t, code, body).Events; len(ev) != 1 || ev[0].Action != "agent_profile_drafted" {
		t.Fatalf("agent audit: %+v", ev)
	}
	for _, c := range []struct{ who, path string }{
		{routeOwner, "/admin/v1/agents/aksel-studio/studio-assistant/audit"},
		{stranger, "/admin/v1/agents/aksel-studio/studio-assistant/audit"},
		{stranger, "/admin/v1/agents/aksel-studio/does-not-exist/audit"},
		{teamB, "/admin/v1/servers/copilot/dolly/audit"},
		{teamB, "/admin/v1/servers/copilot/does-not-exist/audit"},
	} {
		if got, b := e.do("GET", c.path, c.who, ""); got != 404 || b != "not found\n" {
			t.Fatalf("%s %s: %d %q", c.who, c.path, got, b)
		}
	}

	if c, b := e.do("POST", "/admin/v1/servers/copilot/dolly/revoke", routeOwner, `{"reason":"test"}`); c != 200 {
		t.Fatalf("revoke: %d %s", c, b)
	}
	// Neither a job event on the same name nor a server event on another
	// name in the same namespace may show up in the server's trail.
	if _, err := e.db.pool.Exec(t.Context(), `INSERT INTO audit_events (actor, action, namespace, name) VALUES ('x', 'job_submitted', 'copilot', 'dolly')`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.pool.Exec(t.Context(), `INSERT INTO audit_events (actor, action, namespace, name) VALUES ('x', 'revoked', 'copilot', 'other')`); err != nil {
		t.Fatal(err)
	}
	code, body = e.do("GET", "/admin/v1/servers/copilot/dolly/audit", routeOwner, "")
	if ev := decodeAs[struct{ Events []auditEvent }](t, code, body).Events; len(ev) != 1 || ev[0].Action != "revoked" || ev[0].Actor != routeOwner {
		t.Fatalf("server audit: %+v", ev)
	}
}

func TestRealPostgresUIInboxEmptiesAfterApproval(t *testing.T) {
	e := newAgentEnv(t)
	if code, body := e.do("POST", "/admin/v1/agents", author, fixture(t)); code != 201 {
		t.Fatalf("create: %d %s", code, body)
	}
	if code, body := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly")); code != 200 {
		t.Fatalf("approve: %d %s", code, body)
	}
	code, body := e.do("GET", "/admin/v1/agents/pending-approval", approver, "")
	if got := decodeAs[struct{ Agents []pendingAgent }](t, code, body).Agents; len(got) != 0 {
		t.Fatalf("approved profile still in inbox: %+v", got)
	}
}

func TestUIReadGuardsFailClosed(t *testing.T) {
	srv := newServer() // no database
	h := srv.routes(testAuthz(t), adminID, testTeams, nil)
	cases := map[string]struct {
		h    http.Handler
		path string
		tok  string
		want int
	}{
		"admin routes off":     {srv.routes(testAuthz(t), "", testTeams, nil), "/admin/v1/me", userTok("A111111"), 404},
		"no token":             {h, "/admin/v1/me", "", 401},
		"app token":            {h, "/admin/v1/servers", adminID, 401},
		"wrong azp":            {h, "/admin/v1/agents", "user:" + gatewayID + ":A111111", 401},
		"texas fails":          {h, "/admin/v1/me", "texas-500", 503},
		"team lookup fails":    {srv.routes(testAuthz(t), adminID, failingTeams{}, nil), "/admin/v1/servers", userTok("A111111"), 503},
		"database down":        {h, "/admin/v1/servers", userTok("A111111"), 503},
		"database down, inbox": {h, "/admin/v1/agents/pending-approval", userTok("A111111"), 503},
		// Refused before the database, so the answer cannot depend on it.
		"other team audit": {h, "/admin/v1/servers/team-b/svc/audit", userTok("A111111"), 404},
		"other team list":  {h, "/admin/v1/agents?namespace=team-b", userTok("A111111"), 200},
		"no teams, inbox":  {h, "/admin/v1/agents/pending-approval", userTok("Z999999"), 200},
		"bad name":         {h, "/admin/v1/servers/team-a/Bad_Name/audit", userTok("A111111"), 400},
	}
	for name, c := range cases {
		if rec := do(t, c.h, http.MethodGet, c.path, c.tok, ""); rec.Code != c.want {
			t.Errorf("%s: got %d, want %d (%s)", name, rec.Code, c.want, rec.Body)
		}
	}
}

// With two route owners, approving one route drops it from that approver's
// inbox while the other owner still sees theirs. The profile stays a draft,
// so only the handler's approved-route filter can remove it.
func TestRealPostgresUIInboxTwoRouteOwners(t *testing.T) {
	e := newAgentEnv(t)
	p := strings.Replace(fixture(t), `"max_risk": "read"
    }`, `"max_risk": "read"
    },
    {"route": "team-b/kodeverk", "tools": ["lookup"], "max_risk": "read"}`, 1)
	if code, body := e.do("POST", "/admin/v1/agents", author, p); code != 201 {
		t.Fatalf("create: %d %s", code, body)
	}
	if code, body := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly")); code != 200 {
		t.Fatalf("approve: %d %s", code, body)
	}
	inbox := func(who string) []pendingAgent {
		t.Helper()
		c, b := e.do("GET", "/admin/v1/agents/pending-approval", who, "")
		return decodeAs[struct{ Agents []pendingAgent }](t, c, b).Agents
	}
	if got := inbox(approver); len(got) != 0 {
		t.Fatalf("approved route still in approver inbox: %+v", got)
	}
	if got := inbox("B444444:team-b"); len(got) != 1 || len(got[0].Awaiting) != 1 || got[0].Awaiting[0] != "team-b/kodeverk" {
		t.Fatalf("other route owner inbox: %+v", got)
	}
}

// An approved profile past expires_at is listed as expired, as the gateway
// treats it.
func TestRealPostgresUIAgentsReportExpired(t *testing.T) {
	e := newAgentEnv(t)
	e.do("POST", "/admin/v1/agents", author, fixture(t))
	e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly"))
	e.ownerApprove()
	status := func() string {
		t.Helper()
		c, b := e.do("GET", "/admin/v1/agents", author, "")
		got := decodeAs[struct{ Agents []AgentProfile }](t, c, b).Agents
		if len(got) != 1 {
			t.Fatalf("agents: %+v", got)
		}
		return got[0].Status
	}
	if s := status(); s != "approved" {
		t.Fatalf("status before expiry: %s", s)
	}
	if _, err := e.db.pool.Exec(t.Context(), `UPDATE agent_profiles SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if s := status(); s != "expired" {
		t.Fatalf("status after expiry: %s", s)
	}
}

// The owner-team slot: a second member of the profile's team sees the draft
// until they approve, also after every route has approved. The author, who
// is in the route team too, never sees their own draft.
func TestRealPostgresUIInboxOwnerTeamSlot(t *testing.T) {
	e := newAgentEnv(t)
	if code, body := e.do("POST", "/admin/v1/agents", author, fixture(t)); code != 201 {
		t.Fatalf("create: %d %s", code, body)
	}
	inbox := func(who string) []pendingAgent {
		t.Helper()
		c, b := e.do("GET", "/admin/v1/agents/pending-approval", who, "")
		return decodeAs[struct{ Agents []pendingAgent }](t, c, b).Agents
	}
	owner := func() {
		t.Helper()
		if got := inbox(ownerApprover); len(got) != 1 || len(got[0].Awaiting) != 1 || got[0].Awaiting[0] != "aksel-studio" {
			t.Fatalf("owner-team inbox: %+v", got)
		}
	}
	owner()
	if got := inbox(author); len(got) != 0 {
		t.Fatalf("author sees own draft: %+v", got)
	}
	if code, body := e.do("POST", "/admin/v1/agents/aksel-studio/studio-assistant/approvals", approver, e.ab("copilot/dolly")); code != 200 {
		t.Fatalf("route approve: %d %s", code, body)
	}
	owner()
	if got := inbox(approver); len(got) != 0 {
		t.Fatalf("route owner inbox after approving: %+v", got)
	}
	if code, body := e.ownerApprove(); code != 200 {
		t.Fatalf("owner approve: %d %s", code, body)
	}
	if got := inbox(ownerApprover); len(got) != 0 {
		t.Fatalf("approved profile in owner-team inbox: %+v", got)
	}
}
