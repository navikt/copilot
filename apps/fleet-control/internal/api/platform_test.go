package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/navikt/copilot/fleet-control/internal/auth"
)

// The platform team (#2752) may revoke an MCPServer in any namespace in prod,
// pilot or not, and nothing else there. No database: a request that passes
// the gate ends in 503.
func TestPlatformRevokeGate(t *testing.T) {
	srv := func(operator string, teams auth.TeamDirectory) http.Handler {
		s := newServer()
		s.cluster = "prod-gcp"
		s.pilotTeams = []string{"copilot", "aksel-studio"}
		s.operatorTeam = operator
		return s.routes(testAuthz(t), adminID, teams, nil)
	}
	revoke := nsBase + "team-b/mcpservers/svc/revocation"
	reason := `{"reason":"leaks data"}`
	for _, tc := range []struct {
		name, operator, method, path, who string
		teams                             auth.TeamDirectory
		want                              int
	}{
		{"platform member revokes outside the pilot", "copilot", "POST", revoke, "A111111", apisTeams, 503},
		{"non-platform member cannot revoke elsewhere", "copilot", "POST", revoke, "A555555", apisTeams, 403},
		{"owner of a non-pilot team cannot revoke", "copilot", "POST", revoke, "B444444", apisTeams, 403},
		{"operator team unset", "", "POST", revoke, "A111111", apisTeams, 403},
		{"platform member cannot approve elsewhere", "copilot", "POST", nsBase + "team-b/mcpservers/svc/approval", "A111111", apisTeams, 403},
		{"platform member cannot change elsewhere", "copilot", "PUT", nsBase + "team-b/mcpservers/svc", "A111111", apisTeams, 403},
		{"platform member cannot roll back elsewhere", "copilot", "POST", nsBase + "team-b/mcpservers/svc/rollback", "A111111", apisTeams, 403},
		{"platform member cannot revoke a profile elsewhere", "copilot", "POST", nsBase + "team-b/agentprofiles/p/revocation", "A111111", apisTeams, 403},
		{"Console lookup down", "copilot", "POST", revoke, "A111111", auth.UnboundTeams{}, 503},
	} {
		rec := do(t, srv(tc.operator, tc.teams), tc.method, tc.path, person(tc.who), reason)
		if rec.Code != tc.want {
			t.Errorf("%s: want %d, got %d %s", tc.name, tc.want, rec.Code, rec.Body)
		}
	}
}

func TestRealPostgresPlatformTeamRights(t *testing.T) {
	e := newApisEnv(t)
	e.srv.operatorTeam = "copilot"
	e.publish("team-b", "svc", "B444444", "B555555", "one")
	// Drafts by A111111 (in copilot and aksel-studio), submitted in dev to skip prod's ROS rule.
	hash := map[string]string{}
	for _, k := range [][2]string{{"aksel-studio", "s1"}, {"copilot", "s1"}, {"copilot", "s2"}} {
		m := decodeServer(t, e.want(201, "POST", nsBase+k[0]+"/mcpservers", person("A111111"), serverJSON(k[0], k[1], "", "t")))
		hash[k[0]+"/"+k[1]] = m.Status.Pending.ContentHash
	}
	e.srv.cluster = "prod-gcp"
	e.srv.pilotTeams = []string{"copilot", "aksel-studio"}
	ctx := context.Background()

	// #2752: revoke in a non-pilot namespace.
	base := nsBase + "team-b/mcpservers/svc"
	e.want(403, "POST", base+"/revocation", person("A555555"), `{"reason":"x"}`)
	e.want(400, "POST", base+"/revocation", person("A111111"), `{}`)
	e.want(400, "POST", base+"/revocation", person("A111111"), `{"reason":"  "}`)
	e.want(403, "POST", base+"/rollback", person("A111111"), `{"fromGeneration":1,"reason":"x"}`)
	e.want(200, "POST", base+"/revocation", person("A111111"), `{"reason":"leaks data"}`)
	var reason, by string
	if err := e.db.pool.QueryRow(ctx, `SELECT detail->>'reason', detail->>'by' FROM audit_events
		WHERE action = 'revoked' AND namespace = 'team-b' AND actor = 'A111111'`).Scan(&reason, &by); err != nil || reason != "leaks data" || by != "platform" {
		t.Fatalf("revoke audit: %q %q %v", reason, by, err)
	}

	// #2634: self-approval in copilot only.
	selfApprove := func(ns, name string) int {
		code, _ := e.do("POST", nsBase+ns+"/mcpservers/"+name+"/approval", person("A111111"), approval(1, hash[ns+"/"+name]))
		return code
	}
	if code := selfApprove("aksel-studio", "s1"); code != 403 {
		t.Errorf("self-approval in aksel-studio: want 403, got %d", code)
	}
	if code := selfApprove("copilot", "s1"); code != 200 {
		t.Errorf("self-approval in copilot: want 200, got %d", code)
	}
	var outcome string
	if err := e.db.pool.QueryRow(ctx, `SELECT detail->>'outcome' FROM audit_events
		WHERE action = 'approved_published' AND namespace = 'copilot' AND name = 's1'`).Scan(&outcome); err != nil || outcome != "self_approved" {
		t.Errorf("self-approval audit: %q %v", outcome, err)
	}
	e.srv.operatorTeam = ""
	if code := selfApprove("copilot", "s2"); code != 403 {
		t.Errorf("self-approval with operator team unset: want 403, got %d", code)
	}
	e.srv.operatorTeam = "copilot"
	// A two-person approval elsewhere is still plain "approved".
	if err := e.db.pool.QueryRow(ctx, `SELECT detail->>'outcome' FROM audit_events
		WHERE action = 'approved_published' AND namespace = 'team-b'`).Scan(&outcome); err != nil || outcome != "approved" {
		t.Errorf("two-person audit: %q %v", outcome, err)
	}
	// The database refuses a self-approval outside copilot even if the code would allow it.
	if _, err := e.db.pool.Exec(ctx, `UPDATE server_versions SET self_approved = true, approved_by = submitted_by,
		approved_by_oid = submitted_by_oid WHERE namespace = 'aksel-studio' AND name = 's1'`); err == nil {
		t.Error("self_approved outside copilot passed the database check")
	}
}
