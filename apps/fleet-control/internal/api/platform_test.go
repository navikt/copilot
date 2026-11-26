package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/navikt/copilot/fleet-control/internal/auth"
)

// readServerJSON is a read-only server, so prod needs no ROS for it.
func readServerJSON(ns, name, rv, readme string, tools ...string) string {
	meta := map[string]any{"name": name, "namespace": ns}
	if rv != "" {
		meta["resourceVersion"] = rv
	}
	raw, _ := json.Marshal(map[string]any{
		"apiVersion": apiGroupVersion, "kind": "MCPServer", "metadata": meta,
		"spec": map[string]any{"dataCategory": "Intern", "upstream": map[string]any{"app": "svc", "url": "http://svc." + ns + "/mcp"},
			"allowedTools": tools, "defaultRisk": "read", "readme": readme},
	})
	return string(raw)
}

// Console down: every /apis write answers 503 before any rule runs.
func TestApprovalModelConsoleDown(t *testing.T) {
	s := newServer()
	s.cluster = "prod-gcp"
	s.operatorTeam = "copilot"
	h := s.routes(testAuthz(t), adminID, auth.UnboundTeams{}, nil)
	for _, p := range []string{"team-b/mcpservers", "team-b/mcpservers/svc/approval", "team-b/mcpservers/svc/rollback", "team-b/mcpservers/svc/revocation"} {
		if rec := do(t, h, "POST", nsBase+p, person("A111111"), `{}`); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: want 503, got %d", p, rec.Code)
		}
	}
}

// Owner decision 2026-11-26 (#2634, #2752): any team submits in its own
// namespace, only the platform team approves, owners roll back and revoke.
func TestRealPostgresApprovalModel(t *testing.T) {
	e := newApisEnv(t)
	e.srv.cluster = "prod-gcp"
	e.srv.operatorTeam = "copilot" // A111111 and C222222
	ctx := context.Background()
	base := nsBase + "team-b/mcpservers"

	// Submit: any team, own namespace only, no pilot list.
	e.want(404, "POST", base, person("A555555"), readServerJSON("team-b", "svc", "", "", "t1"))
	e.want(404, "POST", base, person("C222222"), readServerJSON("team-b", "svc", "", "", "t1"))
	e.want(400, "POST", base, person("B444444"), readServerJSON("team-a", "svc", "", "", "t1"))
	m := decodeServer(t, e.want(201, "POST", base, person("B444444"), readServerJSON("team-b", "svc", "", "", "t1")))
	gen1 := approval(1, m.Status.Pending.ContentHash)

	// The platform team reads what it approves; outsiders do not.
	e.want(200, "GET", base+"/svc", person("C222222"), "")
	e.want(200, "GET", base+"/svc/generations/1", person("C222222"), "")
	e.want(404, "GET", base+"/svc", person("A555555"), "")
	if out := e.want(200, "GET", apisPrefix+"/mcpservers", person("C222222"), ""); !strings.Contains(out, `"namespace":"team-b"`) {
		t.Errorf("platform list must show team-b's pending server: %s", out)
	}

	// Approve: owners and the submitter cannot, outsiders get 404.
	e.want(403, "POST", base+"/svc/approval", person("B444444"), gen1)
	e.want(403, "POST", base+"/svc/approval", person("B555555"), gen1)
	e.want(404, "POST", base+"/svc/approval", person("A555555"), gen1)
	e.srv.operatorTeam = ""
	e.want(403, "POST", base+"/svc/approval", person("B555555"), gen1)
	e.want(404, "POST", base+"/svc/approval", person("C222222"), gen1)
	e.srv.operatorTeam = "copilot"
	m = decodeServer(t, e.want(200, "POST", base+"/svc/approval", person("C222222"), gen1))
	if m.Status.PublishedGeneration != 1 || m.Status.ApprovedBy.Name != "C222222" {
		t.Fatalf("platform approval: %+v", m.Status)
	}
	outcome := func(ns, name string) (o, sub string) {
		_ = e.db.pool.QueryRow(ctx, `SELECT detail->>'outcome', detail->>'submittedBy' FROM audit_events
			WHERE action = 'approved_published' AND namespace = $1 AND name = $2 ORDER BY id DESC LIMIT 1`, ns, name).Scan(&o, &sub)
		return o, sub
	}
	if o, sub := outcome("team-b", "svc"); o != "approved" || sub != "B444444" {
		t.Errorf("approval audit: %q %q", o, sub)
	}

	// A change that adds no access publishes at once, audited with its fields.
	put := func(who, readme string, tools ...string) MCPServer {
		cur := decodeServer(t, e.want(200, "GET", base+"/svc", person(who), ""))
		return decodeServer(t, e.want(200, "PUT", base+"/svc", person(who), readServerJSON("team-b", "svc", cur.Metadata.ResourceVersion, readme, tools...)))
	}
	m = put("B444444", "new readme", "t1")
	if m.Status.PublishedGeneration != 2 || m.Status.Pending != nil {
		t.Fatalf("readme change must auto-publish: %+v", m.Status)
	}
	var by, changed string
	if err := e.db.pool.QueryRow(ctx, `SELECT detail->>'approvedBy', detail->>'changed' FROM audit_events
		WHERE action = 'auto_published' AND namespace = 'team-b'`).Scan(&by, &changed); err != nil || by != "auto" || changed != `["readme"]` {
		t.Errorf("auto audit: %q %q %v", by, changed, err)
	}
	// A new tool waits for the platform team.
	m = put("B444444", "new readme", "t1", "t2")
	if m.Status.PublishedGeneration != 2 || m.Status.Pending == nil {
		t.Fatalf("new tool must wait: %+v", m.Status)
	}
	e.want(200, "POST", base+"/svc/approval", person("C222222"), approval(3, m.Status.Pending.ContentHash))

	// Rollback: owner team only, only to a generation the platform team approved.
	rb := `{"fromGeneration":3,"toGeneration":1,"reason":"x"}`
	e.want(404, "POST", base+"/svc/rollback", person("C222222"), rb)
	if _, err := e.db.pool.Exec(ctx, `UPDATE server_versions SET approved_via = NULL WHERE namespace = 'team-b' AND name = 'svc' AND version = 1`); err != nil {
		t.Fatal(err)
	}
	if code, out := e.do("POST", base+"/svc/rollback", person("B555555"), rb); code != 409 || !strings.Contains(out, "not approved by the platform team") {
		t.Errorf("rollback to a generation the platform team did not approve: %d %s", code, out)
	}
	e.want(200, "POST", base+"/svc/rollback", person("B555555"), `{"fromGeneration":3,"toGeneration":2,"reason":"x"}`)
	// No auto-publish on top of a generation the platform team never approved.
	if _, err := e.db.pool.Exec(ctx, `UPDATE server_versions SET approved_via = NULL, approved_by = NULL WHERE namespace = 'team-b' AND name = 'svc' AND version = 2`); err != nil {
		t.Fatal(err)
	}
	if m = put("B444444", "another readme", "t1"); m.Status.Pending == nil {
		t.Errorf("change on an unapproved base must wait: %+v", m.Status)
	}

	// Revoke: the platform team anywhere, with a reason; outsiders 404.
	e.want(404, "POST", base+"/svc/revocation", person("A555555"), `{"reason":"x"}`)
	e.want(400, "POST", base+"/svc/revocation", person("C222222"), `{"reason":"  "}`)
	e.want(200, "POST", base+"/svc/revocation", person("C222222"), `{"reason":"leaks data"}`)
	var reason, role string
	if err := e.db.pool.QueryRow(ctx, `SELECT detail->>'reason', detail->>'by' FROM audit_events
		WHERE action = 'revoked' AND namespace = 'team-b'`).Scan(&reason, &role); err != nil || reason != "leaks data" || role != "platform" {
		t.Errorf("revoke audit: %q %q %v", reason, role, err)
	}

	// Self-approval: a platform member approves what they submitted, in any namespace.
	for _, ns := range []string{"copilot", "team-a"} {
		m := decodeServer(t, e.want(201, "POST", nsBase+ns+"/mcpservers", person("A111111"), readServerJSON(ns, "own", "", "", "t1")))
		e.want(200, "POST", nsBase+ns+"/mcpservers/own/approval", person("A111111"), approval(1, m.Status.Pending.ContentHash))
		if o, _ := outcome(ns, "own"); o != "self_approved" {
			t.Errorf("%s: self-approval audit %q", ns, o)
		}
	}

	// The database keeps the self_approved mark and auto rows consistent.
	for _, q := range []string{
		`UPDATE server_versions SET self_approved = false WHERE namespace = 'copilot' AND name = 'own'`,
		`UPDATE server_versions SET self_approved = true WHERE namespace = 'team-b' AND version = 1`,
		`UPDATE server_versions SET approved_by_oid = submitted_by_oid WHERE namespace = 'team-b' AND version = 2`,
	} {
		if _, err := e.db.pool.Exec(ctx, q); err == nil {
			t.Errorf("database accepted: %s", q)
		}
	}
}

// Lowering a write tool to read hands it to every read-capped agent and drops
// prod's ROS requirement, so it must wait for the platform team.
func TestRealPostgresRiskLoweringWaits(t *testing.T) {
	e := newApisEnv(t)
	e.srv.cluster = "prod-gcp"
	e.srv.operatorTeam = "copilot"
	f := &fakeSlack{code: 200}
	n, wg := f.start(t)
	e.srv.notifier = n
	base := nsBase + "team-b/mcpservers"
	body := func(rv string, spec map[string]any) string {
		meta := map[string]any{"name": "svc", "namespace": "team-b"}
		if rv != "" {
			meta["resourceVersion"] = rv
		}
		spec["upstream"] = map[string]any{"app": "svc", "url": "http://svc.team-b/mcp"}
		spec["allowedTools"] = []string{"delete_all"}
		raw, _ := json.Marshal(map[string]any{"apiVersion": apiGroupVersion, "kind": "MCPServer", "metadata": meta, "spec": spec})
		return string(raw)
	}
	m := decodeServer(t, e.want(201, "POST", base, person("B444444"), body("", map[string]any{"rosRef": "ros-1", "dataCategory": "Fortrolig"})))
	wg.Add(1)
	m = decodeServer(t, e.want(200, "POST", base+"/svc/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash)))
	m = decodeServer(t, e.want(200, "PUT", base+"/svc", person("B444444"),
		body(m.Metadata.ResourceVersion, map[string]any{"toolRisk": map[string]string{"delete_all": "read"}, "dataCategory": "Aapen"})))
	if m.Status.PublishedGeneration != 1 || m.Status.Pending == nil {
		t.Fatalf("risk lowering must wait for the platform team: %+v", m.Status)
	}
	// A readme change still auto-publishes, with a Slack notice naming the field.
	wg.Add(1)
	m = decodeServer(t, e.want(200, "PUT", base+"/svc", person("B444444"),
		body(m.Metadata.ResourceVersion, map[string]any{"rosRef": "ros-1", "dataCategory": "Fortrolig", "readme": "x"})))
	if m.Status.PublishedGeneration != 3 {
		t.Fatalf("readme change must auto-publish: %+v", m.Status)
	}
	wg.Wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.texts) != 2 || !strings.Contains(f.texts[1], "uten godkjenning") || !strings.Contains(f.texts[1], "endret: readme") {
		t.Errorf("auto-publish notice: %q", f.texts)
	}
}

// The DB layer refuses a non-platform approver even when the handler guard is skipped.
func TestRealPostgresDBApproveRefusesNonPlatform(t *testing.T) {
	e := newApisEnv(t)
	m := decodeServer(t, e.want(201, "POST", nsBase+"team-b/mcpservers", person("B444444"), readServerJSON("team-b", "svc", "", "", "t1")))
	c := caller{Ident: "B444444", Oid: "00000000-0000-0000-0000-000000000444", Teams: []string{"team-b"}}
	err := e.db.approveServer(context.Background(), c, "copilot", "team-b", "svc", 1, m.Status.Pending.ContentHash)
	if err == nil || !strings.Contains(err.Error(), "only the platform team") {
		t.Fatalf("want refusal, got %v", err)
	}
}
