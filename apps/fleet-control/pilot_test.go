package main

import (
	"strings"
	"testing"
)

func TestPilotTeamsFromEnv(t *testing.T) {
	t.Setenv("FLEET_CONTROL_PILOT_TEAMS", " copilot, aksel-studio ,,copilot ")
	if got, err := pilotTeamsFromEnv(); err != nil || strings.Join(got, ",") != "copilot,aksel-studio" {
		t.Errorf("got %v %v", got, err)
	}
	for _, v := range []string{"", " ", " , "} {
		t.Setenv("FLEET_CONTROL_PILOT_TEAMS", v)
		if got, err := pilotTeamsFromEnv(); err != nil || len(got) != 0 {
			t.Errorf("%q: want none, got %v %v", v, got, err)
		}
	}
	// Not folded into a match: a wrong-case or odd entry refuses startup.
	for _, v := range []string{"Copilot", "copilot,team a", "team_a"} {
		t.Setenv("FLEET_CONTROL_PILOT_TEAMS", v)
		if _, err := pilotTeamsFromEnv(); err == nil {
			t.Errorf("%q: want error", v)
		}
	}
}

// Outside dev-* a write needs a pilot team and membership; reads are untouched.
// No database here: a write that passes every guard ends in "database unavailable".
func TestPilotWriteGate(t *testing.T) {
	prod := func(_ teamDirectory, pilot ...string) *server {
		s := newServer()
		s.cluster = "prod-gcp"
		s.pilotTeams = pilot
		return s
	}
	kill := func(team string) string { return "/admin/v1/jobs/" + team + "/kill-switch" }
	body := `{"scope":"team","on":true,"reason":"x"}`
	for _, tc := range []struct {
		name    string
		cluster string
		pilot   []string
		teams   teamDirectory
		team    string
		code    int
		msg     string
	}{
		{"prod, allowlist unset", "prod-gcp", nil, apisTeams, "copilot", 403, "not enabled"},
		{"unset cluster is not dev", "", nil, apisTeams, "copilot", 403, "not enabled"},
		{"prod, member, team not in allowlist", "prod-gcp", []string{"copilot", "aksel-studio"}, apisTeams, "team-a", 403, "not enabled"},
		{"prod, allowlisted, not a member", "prod-gcp", []string{"platform"}, apisTeams, "platform", 403, "not a member"},
		{"prod, allowlisted and member", "prod-gcp", []string{"copilot", "aksel-studio"}, apisTeams, "aksel-studio", 503, "database unavailable"},
		{"prod, Console SA unbound", "prod-gcp", []string{"copilot"}, unboundTeams{}, "copilot", 503, "team lookup unavailable"},
		{"dev, no allowlist", "dev-gcp", nil, apisTeams, "team-a", 503, "database unavailable"},
	} {
		s := prod(tc.teams, tc.pilot...)
		s.cluster = tc.cluster
		h := s.routes(testAuthz(t), adminID, tc.teams, nil)
		rec := do(t, h, "PUT", kill(tc.team), person("A111111"), body)
		if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.msg) {
			t.Errorf("admin %s: want %d %q, got %d %s", tc.name, tc.code, tc.msg, rec.Code, rec.Body)
		}
		// /apis: same verdict on a write; the gate sits before the database.
		want := tc.code
		rec = do(t, h, "POST", nsBase+tc.team+"/mcpservers", person("A111111"), serverJSON(tc.team, "s1", ""))
		if tc.msg == "not a member" {
			want = 503 // /apis checks membership in the handler, after the database
		}
		if rec.Code != want {
			t.Errorf("apis %s: want %d, got %d %s", tc.name, want, rec.Code, rec.Body)
		}
	}
	// Write routes are mounted in prod (403, not 404), and reads are not gated.
	h := prod(apisTeams).routes(testAuthz(t), adminID, apisTeams, nil)
	if rec := do(t, h, "GET", nsBase+"team-a/mcpservers", person("A111111"), ""); rec.Code != 503 {
		t.Errorf("prod read: want 503 (no database), got %d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{nsBase + "team-a/agentprofiles", nsBase + "team-a/agentjobs", nsBase + "team-a/mcpservers/s1/approval"} {
		if rec := do(t, h, "POST", p, person("A111111"), "{}"); rec.Code != 403 {
			t.Errorf("POST %s in prod: want 403, got %d %s", p, rec.Code, rec.Body)
		}
	}
}

// With a database, an allowlisted member's write in prod goes through.
func TestRealPostgresPilotWrite(t *testing.T) {
	e := newApisEnv(t)
	// Prod needs a ROS for write tools; declare the tool read.
	body := func(ns, name string) string {
		return strings.Replace(serverJSON(ns, name, "", "a"), `"allowedTools"`, `"toolRisk":{"a":"read"},"allowedTools"`, 1)
	}
	e.srv.cluster = "prod-gcp"
	e.want(403, "POST", nsBase+"copilot/mcpservers", person("A111111"), body("copilot", "s1"))
	e.srv.pilotTeams = []string{"copilot", "aksel-studio"}
	e.want(403, "POST", nsBase+"team-a/mcpservers", person("A111111"), body("team-a", "s1"))
	e.want(201, "POST", nsBase+"copilot/mcpservers", person("A111111"), body("copilot", "s1"))
	// 404: memberOf hides team names from non-members (apis.go).
	e.want(404, "POST", nsBase+"copilot/mcpservers", person("B444444"), body("copilot", "s2"))
}
