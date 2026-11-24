package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const appClientID = "55555555-5555-4555-8555-555555555555"

// delegated is a person's OBO token obtained by the app appClientID.
func delegated(ident string) string { return "user:" + appClientID + ":" + ident }

// liveProfile stores a published, verified, unexpired profile for clientID.
func liveProfile(t *testing.T, db *DB, ns, name, clientID, risk string) {
	t.Helper()
	spec := `{"identity":{"app":"studio-api","cluster":"dev-gcp","entraClientID":"` + clientID + `"},"mode":"Delegated","grants":[{"serverRef":{"group":"fleet.nav.no","kind":"MCPServer","namespace":"copilot","name":"dolly"},"allowedTools":["hent"],"maxRisk":"` + risk + `"}],"quota":{}}`
	ctx := context.Background()
	if _, err := db.pool.Exec(ctx, `INSERT INTO agent_profile_resources (namespace, name, published_generation, published_client_id) VALUES ($1, $2, 1, $3::uuid)`, ns, name, clientID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO agent_profile_generations (namespace, name, generation, spec, spec_hash, entra_client_id, submitted_by, submitted_by_kind,
		approved_at, expires_at, identity_verified_at, grant_hashes) VALUES ($1, $2, 1, $3::jsonb, 'v1:x', $4::uuid, 'A111111', 'User', now(), now() + interval '1 day', now(), '{}')`,
		ns, name, spec, clientID); err != nil {
		t.Fatal(err)
	}
}

func jobJSON(ns, name, extra string) string {
	return `{"apiVersion":"fleet.nav.no/v1alpha1","kind":"AgentJob","metadata":{"name":"` + name + `","namespace":"` + ns + `"},"spec":{"type":"kodeprototype","params":{"repo":"navikt/eksempel"}` + extra + `}}`
}

const studioRef = `,"agentProfile":{"namespace":"team-a","name":"studio"}`

func decodeJob(t *testing.T, body string) AgentJob {
	t.Helper()
	var j AgentJob
	if err := json.Unmarshal([]byte(body), &j); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return j
}

func TestRealPostgresAgentJobPerson(t *testing.T) {
	e := newApisEnv(t)
	liveProfile(t, e.db, "team-a", "studio", appClientID, "Read")
	liveProfile(t, e.db, "team-b", "other", "66666666-6666-4666-8666-666666666666", "Read")
	base := nsBase + "team-a/agentjobs"
	body := jobJSON("team-a", "first", studioRef)

	e.want(404, "POST", base, person("B444444"), body)
	e.want(404, "GET", base, person("B444444"), "")
	e.want(400, "POST", base, person("A111111"), strings.Replace(body, `"type"`, `"bogus":1,"type"`, 1))
	e.want(422, "POST", base, person("A111111"), jobJSON("team-a", "job-7", studioRef))
	e.want(422, "POST", base, person("A111111"), jobJSON("team-a", "first", ""))
	e.want(422, "POST", base, person("A111111"), jobJSON("team-a", "first", `,"agentProfile":{"namespace":"team-b","name":"other"}`))
	e.want(422, "POST", base, person("A111111"), jobJSON("team-a", "first", `,"agentProfile":{"namespace":"team-a","name":"nope"}`))
	e.want(422, "POST", base, person("A111111"), jobJSON("team-a", "first", studioRef+`,"maxAttempts":11`))

	j := decodeJob(t, e.want(201, "POST", base, person("A111111"), body))
	if j.Spec.MaxAttempts != 3 || j.Spec.OnExpiry != "Requeue" || j.Status.CreatedBy != (Actor{Kind: "User", Name: "A111111"}) || j.Status.Orderer != nil ||
		j.Status.AgentProfile == nil || j.Status.AgentProfile.Generation != 1 || condOf(j.Status.Conditions, "Running").Reason != "Queued" {
		t.Fatalf("created: %+v", j)
	}
	// Defaults written out give the same job; another spec is 409.
	e.want(200, "POST", base, person("A222222"), jobJSON("team-a", "first", studioRef+`,"maxAttempts":3,"onExpiry":"Requeue"`))
	e.want(409, "POST", base, person("A111111"), jobJSON("team-a", "first", studioRef+`,"maxAttempts":2`))

	// The name check comes before the kill switch.
	if _, err := e.db.pool.Exec(context.Background(), `INSERT INTO runtime_kill_switches (scope, reason, set_by) VALUES ('team:team-a', 'x', 'A111111')`); err != nil {
		t.Fatal(err)
	}
	e.want(200, "POST", base, person("A111111"), body)
	e.want(503, "POST", base, person("A111111"), jobJSON("team-a", "second", studioRef))
	if _, err := e.db.pool.Exec(context.Background(), `DELETE FROM runtime_kill_switches`); err != nil {
		t.Fatal(err)
	}

	got := decodeJob(t, e.want(200, "GET", base+"/first", person("A222222"), ""))
	if got.Metadata.ResourceVersion != j.Metadata.ResourceVersion {
		t.Fatalf("get: %+v", got)
	}
	if list := e.want(200, "GET", base, person("A111111"), ""); !strings.Contains(list, `"first"`) {
		t.Fatalf("list: %s", list)
	}
	e.want(400, "POST", base+"/first/termination", person("A111111"), `{"mode":"Stop"}`)
	e.want(404, "POST", base+"/nope/termination", person("A111111"), `{"mode":"Cancel"}`)
	done := decodeJob(t, e.want(200, "POST", base+"/first/termination", person("A111111"), `{"mode":"Cancel","reason":"test"}`))
	if condOf(done.Status.Conditions, "Failed").Status != "True" || done.Status.CompletionTime == nil || done.Metadata.ResourceVersion == j.Metadata.ResourceVersion {
		t.Fatalf("cancelled: %+v", done)
	}
	e.want(409, "POST", base+"/first/termination", person("A111111"), `{"mode":"Kill"}`)
	e.noOidLeak()
}

func TestRealPostgresAgentJobWriteProfile(t *testing.T) {
	e := newApisEnv(t)
	liveProfile(t, e.db, "team-a", "studio", appClientID, "Write")
	base := nsBase + "team-a/agentjobs"
	e.want(422, "POST", base, person("A111111"), jobJSON("team-a", "w", studioRef+`,"maxAttempts":2`))
	if j := decodeJob(t, e.want(201, "POST", base, person("A111111"), jobJSON("team-a", "w", studioRef))); j.Spec.MaxAttempts != 1 {
		t.Fatalf("a write profile defaults to 1 attempt: %+v", j.Spec)
	}
	e.want(200, "POST", base, person("A111111"), jobJSON("team-a", "w", studioRef+`,"maxAttempts":1`))
}

func TestRealPostgresAgentJobDelegated(t *testing.T) {
	e := newApisEnv(t)
	base := nsBase + "team-a/agentjobs"
	// No published profile for the app: 401, also before it is published.
	e.want(401, "POST", base, delegated("B444444"), jobJSON("team-a", "first", ""))
	liveProfile(t, e.db, "team-a", "studio", appClientID, "Read")
	// Delegated tokens reach only agentjobs.
	e.want(401, "GET", nsBase+"team-a/mcpservers", delegated("B444444"), "")
	e.want(401, "GET", apisPrefix+"/whoami", delegated("B444444"), "")
	// The app's profile is in team-a, not team-b.
	e.want(404, "GET", nsBase+"team-b/agentjobs", delegated("B444444"), "")
	e.want(422, "POST", base, delegated("B444444"), jobJSON("team-a", "first", `,"agentProfile":{"namespace":"team-a","name":"other"}`))

	// A person outside the team orders through the app.
	j := decodeJob(t, e.want(201, "POST", base, delegated("B444444"), jobJSON("team-a", "first", "")))
	if j.Status.Orderer == nil || j.Status.Orderer.ClientID != appClientID || j.Status.Orderer.Kind != "Application" || j.Status.CreatedBy.Name != "B444444" || j.Spec.AgentProfile == nil {
		t.Fatalf("delegated: %+v", j.Status)
	}
	e.want(200, "POST", base, delegated("B444444"), jobJSON("team-a", "first", studioRef))
	// Same name from another orderer is another job, and neither sees the other's.
	e.want(201, "POST", base, person("A111111"), jobJSON("team-a", "first", studioRef+`,"maxAttempts":2`))
	e.want(201, "POST", base, delegated("A222222"), jobJSON("team-a", "first", ""))
	if got := decodeJob(t, e.want(200, "GET", base+"/first", person("A111111"), "")); got.Status.Orderer != nil || got.Spec.MaxAttempts != 2 {
		t.Fatalf("person sees a delegated job: %+v", got)
	}
	e.want(404, "GET", base+"/second", delegated("B444444"), "")
	if list := e.want(200, "GET", base, delegated("B444444"), ""); strings.Count(list, `"first"`) != 1 || strings.Contains(list, "A222222") {
		t.Fatalf("list: %s", list)
	}
	e.want(200, "POST", base+"/first/termination", delegated("A222222"), `{"mode":"Kill"}`)
	if got := decodeJob(t, e.want(200, "GET", base+"/first", delegated("B444444"), "")); condOf(got.Status.Conditions, "Running").Reason != "Queued" {
		t.Fatalf("another person's kill reached this job: %+v", got.Status)
	}
	// Revoking the profile shuts the app out.
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE agent_profile_resources SET revoked = true`); err != nil {
		t.Fatal(err)
	}
	e.want(401, "GET", base, delegated("B444444"), "")
	e.noOidLeak()
}

// An image from before 007 keeps inserting jobs, and its rows show up for
// persons as job-<id>.
func TestRealPostgresMigration007OldImage(t *testing.T) {
	e := newApisEnv(t)
	ctx := context.Background()
	if _, err := e.db.pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err := e.db.migrateTo(ctx, "005_v1alpha1.sql"); err != nil {
		t.Fatal(err)
	}
	oldInsert := `INSERT INTO runtime_jobs (team, kind, params, on_expiry, max_attempts, created_by) VALUES ('team-a', 'k', '{}', 'requeue', 3, 'A111111') RETURNING id`
	var before, after int64
	if err := e.db.pool.QueryRow(ctx, oldInsert).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := e.db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := e.db.pool.QueryRow(ctx, oldInsert).Scan(&after); err != nil {
		t.Fatalf("old-image insert after 007: %v", err)
	}
	if _, err := e.db.CreateJob(ctx, "A111111", "team-a", CreateJobRequest{Kind: "k", Params: json.RawMessage("{}"), OnExpiry: "requeue", MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	j := decodeJob(t, e.want(200, "GET", nsBase+"team-a/agentjobs/job-1", person("A111111"), ""))
	if j.Spec.Type != "k" || j.Spec.OnExpiry != "Requeue" || j.Status.CreatedBy.Name != "A111111" {
		t.Fatalf("old row: %+v", j)
	}
	if list := e.want(200, "GET", nsBase+"team-a/agentjobs", person("A111111"), ""); strings.Count(list, `"job-`) != 3 {
		t.Fatalf("list: %s", list)
	}
}

// An expired profile binds neither its app nor a person's job.
func TestRealPostgresAgentJobExpiredProfile(t *testing.T) {
	e := newApisEnv(t)
	liveProfile(t, e.db, "team-a", "studio", appClientID, "Read")
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE agent_profile_generations SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	base := nsBase + "team-a/agentjobs"
	e.want(401, "POST", base, delegated("B444444"), jobJSON("team-a", "first", ""))
	e.want(422, "POST", base, person("A111111"), jobJSON("team-a", "first", studioRef))
}

// The app must be the published generation's identity, not only the
// resource's published client.
func TestRealPostgresAgentJobClientMismatch(t *testing.T) {
	e := newApisEnv(t)
	liveProfile(t, e.db, "team-a", "studio", appClientID, "Read")
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE agent_profile_generations SET entra_client_id = '66666666-6666-4666-8666-666666666666'`); err != nil {
		t.Fatal(err)
	}
	e.want(401, "POST", nsBase+"team-a/agentjobs", delegated("B444444"), jobJSON("team-a", "first", ""))
}
