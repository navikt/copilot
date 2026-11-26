package api

import (
	"context"
	"strings"
	"testing"
	"time"
)

// age moves a generation's submission back by d.
func age(t *testing.T, e *apisEnv, table, name string, d time.Duration) {
	t.Helper()
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE `+table+` SET created_at = created_at - $2::interval WHERE name = $1`, name, d); err != nil {
		t.Fatal(err)
	}
}

func expiredAudits(t *testing.T, e *apisEnv, name string) int {
	t.Helper()
	var n int
	if err := e.db.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'approval_expired' AND name = $1 AND detail->>'outcome' = 'expired'`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A pending server generation is approvable for 30 days, then refused with
// 410 Expired and an audit row; the same content can be submitted again (#2731).
func TestRealPostgresServerPendingExpires(t *testing.T) {
	e := newApisEnv(t)
	base := nsBase + "team-a/mcpservers"
	m := decodeServer(t, e.want(201, "POST", base, person("A111111"), serverJSON("team-a", "old", "", "t")))
	if p := m.Status.Pending; p == nil || p.ExpiresAt == nil || time.Until(*p.ExpiresAt) < 29*24*time.Hour {
		t.Fatalf("pending must carry expiresAt about 30 days ahead: %+v", m.Status.Pending)
	}
	n := decodeServer(t, e.want(201, "POST", base, person("A111111"), serverJSON("team-a", "fresh", "", "t")))
	age(t, e, "server_versions", "old", 31*24*time.Hour)
	age(t, e, "server_versions", "fresh", 29*24*time.Hour)

	code, out := e.do("POST", base+"/old/approval", person("A222222"), approval(1, m.Status.Pending.ContentHash))
	if code != 410 || !strings.Contains(out, `"reason":"Expired"`) {
		t.Fatalf("expired approval: want 410 Expired, got %d %s", code, out)
	}
	if got := expiredAudits(t, e, "old"); got != 1 {
		t.Fatalf("want one approval_expired audit row, got %d", got)
	}
	e.want(200, "POST", base+"/fresh/approval", person("A222222"), approval(1, n.Status.Pending.ContentHash))

	cur := decodeServer(t, e.want(200, "GET", base+"/old", person("A111111"), ""))
	re := decodeServer(t, e.want(200, "PUT", base+"/old", person("A111111"), serverJSON("team-a", "old", cur.Metadata.ResourceVersion, "t")))
	if re.Status.Pending == nil || re.Status.Pending.Generation != 2 {
		t.Fatalf("resubmitting the same content must make a new generation: %+v", re.Status.Pending)
	}
	e.want(200, "POST", base+"/old/approval", person("A222222"), approval(2, re.Status.Pending.ContentHash))
}

// A partly granted profile lapses with its generation; another profile is
// unaffected, and the six months run from the oldest grant (#2731).
func TestRealPostgresProfilePartialGrantExpires(t *testing.T) {
	e := newApisEnv(t)
	e.publish("copilot", "dolly", "C222222", "A111111", "hent")
	e.publish("team-b", "kodeverk", "B444444", "B555555", "lookup")
	base := nsBase + "aksel-studio/agentprofiles"
	owner := map[string]string{"copilot/dolly": "C222222", "team-b/kodeverk": "B444444"}
	hashes := func(p AgentProfileV1) map[string]string {
		h := map[string]string{}
		for _, g := range p.Status.Grants {
			h[g.ServerRef.key()] = g.ContentHash
		}
		return h
	}
	old := decodeProfile(t, e.want(201, "POST", base, person("A111111"), profileJSON("aksel-studio", "old", "", "copilot/dolly=hent", "team-b/kodeverk=lookup")))
	if old.Status.Pending == nil || old.Status.Pending.ExpiresAt == nil {
		t.Fatalf("pending profile must carry expiresAt: %+v", old.Status.Pending)
	}
	other := decodeProfile(t, e.want(201, "POST", base, person("A111111"), profileJSON("aksel-studio", "other", "", "copilot/dolly=hent", "team-b/kodeverk=lookup")))
	h, ho := hashes(old), hashes(other)
	e.want(200, "POST", base+"/old/approval", person(owner["team-b/kodeverk"]), grantBody(1, "team-b/kodeverk", h["team-b/kodeverk"]))
	age(t, e, "agent_profile_generations", "old", 31*24*time.Hour)
	age(t, e, "agent_profile_generations", "other", 29*24*time.Hour)

	code, out := e.do("POST", base+"/old/approval", person(owner["copilot/dolly"]), grantBody(1, "copilot/dolly", h["copilot/dolly"]))
	if code != 410 || !strings.Contains(out, `"reason":"Expired"`) {
		t.Fatalf("last grant on an expired generation: want 410 Expired, got %d %s", code, out)
	}
	if got := expiredAudits(t, e, "old"); got != 1 {
		t.Fatalf("want one approval_expired audit row, got %d", got)
	}
	// Another profile inside its window still takes grants.
	e.want(200, "POST", base+"/other/approval", person(owner["copilot/dolly"]), grantBody(1, "copilot/dolly", ho["copilot/dolly"]))

	cur := decodeProfile(t, e.want(200, "GET", base+"/old", person("A111111"), ""))
	re := decodeProfile(t, e.want(200, "PUT", base+"/old", person("A111111"), profileJSON("aksel-studio", "old", cur.Metadata.ResourceVersion, "copilot/dolly=hent", "team-b/kodeverk=lookup")))
	if re.Status.Pending == nil || re.Status.Pending.Generation != 2 {
		t.Fatalf("resubmitting must make a new generation: %+v", re.Status.Pending)
	}
	h = hashes(re)
	e.want(200, "POST", base+"/old/approval", person(owner["team-b/kodeverk"]), grantBody(2, "team-b/kodeverk", h["team-b/kodeverk"]))
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE agent_profile_grants SET granted_at = granted_at - interval '10 days' WHERE name = 'old' AND generation = 2`); err != nil {
		t.Fatal(err)
	}
	e.want(200, "POST", base+"/old/approval", person(owner["copilot/dolly"]), grantBody(2, "copilot/dolly", h["copilot/dolly"]))
	pub := decodeProfile(t, e.want(200, "GET", base+"/old", person("A111111"), ""))
	if pub.Status.ExpiresAt == nil || pub.Status.ExpiresAt.After(time.Now().AddDate(0, 6, -9)) {
		t.Fatalf("expiry must run from the oldest grant: %v", pub.Status.ExpiresAt)
	}
}
