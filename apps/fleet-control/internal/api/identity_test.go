package api

import (
	"context"
	"encoding/json"
	"testing"
)

// New writes carry {issuer, subject}, and a person's job keeps NULL, not ”
// (#2896). The backfill of rows from before 012 is covered in internal/store.
func TestRealPostgresIdentityIssuerSubject(t *testing.T) {
	e := newApisEnv(t)
	ctx := context.Background()
	liveProfile(t, e.db, "team-a", "studio", appClientID, "Read")
	var iss, sub string
	var withOrderer, persons int

	// New writes: refused without an issuer, both forms with one.
	entraIssuer = ""
	e.want(503, "POST", nsBase+"team-a/agentjobs", delegated("B444444"), jobJSON("team-a", "refused", ""))
	entraIssuer = testIssuer
	e.want(201, "POST", nsBase+"team-a/agentjobs", delegated("B444444"), jobJSON("team-a", "first", ""))
	e.want(201, "POST", nsBase+"team-a/agentjobs", person("A111111"), jobJSON("team-a", "mine", studioRef))
	if err := e.db.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE name = 'first' AND orderer_issuer = $1 AND orderer_client = orderer_client_id AND orderer_sub = orderer_subject),
		count(*) FILTER (WHERE name = 'mine' AND orderer_issuer IS NULL AND orderer_client IS NULL AND orderer_sub IS NULL) FROM runtime_jobs`, testIssuer).Scan(&withOrderer, &persons); err != nil || withOrderer != 1 || persons != 1 {
		t.Fatalf("new jobs: delegated %d person %d %v", withOrderer, persons, err)
	}
	entraIssuer = ""
	if err := e.db.insertProfileGeneration(ctx, caller{Ident: "seed", Kind: "System"}, ObjectMeta{Namespace: "team-a", Name: "other"}, studioSpec(t), false, true); err == nil {
		t.Fatal("profile written without an issuer")
	}
	entraIssuer = testIssuer
	if err := e.db.insertProfileGeneration(ctx, caller{Ident: "seed", Kind: "System"}, ObjectMeta{Namespace: "team-a", Name: "other"}, studioSpec(t), false, true); err != nil {
		t.Fatal(err)
	}
	if err := e.db.pool.QueryRow(ctx, `SELECT identity_issuer, identity_subject FROM agent_profile_generations WHERE name = 'other'`).Scan(&iss, &sub); err != nil || iss != testIssuer || sub != otherClientID {
		t.Fatalf("new profile identity: %q %q %v", iss, sub, err)
	}

	// The schema refuses a half identity and a row with none.
	for _, q := range []string{
		`UPDATE agent_profile_generations SET identity_issuer = NULL WHERE name = 'other'`,
		`UPDATE agent_profile_generations SET entra_client_id = NULL, identity_issuer = NULL, identity_subject = NULL WHERE name = 'other'`,
		`UPDATE runtime_jobs SET orderer_sub = NULL WHERE name = 'first'`,
		`UPDATE runtime_jobs SET orderer_issuer = 'x' WHERE name = 'mine'`,
	} {
		if _, err := e.db.pool.Exec(ctx, q); err == nil {
			t.Errorf("accepted: %s", q)
		}
	}
	// A non-Entra identity needs no entra_client_id.
	if _, err := e.db.pool.Exec(ctx, `UPDATE agent_profile_generations SET entra_client_id = NULL WHERE name = 'other'`); err != nil {
		t.Fatal(err)
	}
}

const otherClientID = "77777777-7777-4777-8777-777777777777"

func studioSpec(t *testing.T) AgentProfileSpec {
	t.Helper()
	var s AgentProfileSpec
	if err := json.Unmarshal([]byte(`{"identity":{"app":"other","cluster":"dev-gcp","entraClientID":"`+otherClientID+`"},"mode":"Delegated","grants":[{"serverRef":{"group":"fleet.nav.no","kind":"MCPServer","namespace":"copilot","name":"dolly"},"allowedTools":["hent"],"maxRisk":"Read"}],"quota":{}}`), &s); err != nil {
		t.Fatal(err)
	}
	return s
}
