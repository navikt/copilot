package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// fakeVerifier accepts only the seed's client id for aksel-studio/studio-api.
type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, id AgentIdentity) (bool, error) {
	return id.EntraClientID == seedClientID && id.Namespace == "aksel-studio" && id.App == "studio-api", nil
}

const seedClientID = "7009f359-1451-429f-94d7-535d5c0c73c0"

func fixture(t *testing.T) string {
	t.Helper()
	// The dev seed holds one profile; tests use it as the request body.
	raw, err := os.ReadFile("seed/agents.dev.json")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(raw)), "["), "]")
}

func TestRealPostgresAgentSeed(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SEED_AGENTS_FILE", "seed/agents.dev.json")
	t.Setenv("NAIS_CLUSTER_NAME", "prod-gcp")
	if err := seedAgentsFromEnv(ctx, db); err == nil {
		t.Fatal("agent seed ran in prod")
	}
	t.Setenv("NAIS_CLUSTER_NAME", "dev-gcp")
	for range 2 {
		if err := seedAgentsFromEnv(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	var status, by string
	var verified bool
	_ = db.pool.QueryRow(ctx, `SELECT count(*), min(submitted_by_kind), min(submitted_by), bool_or(identity_verified_at IS NOT NULL) FROM agent_profile_generations`).Scan(&n, &status, &by, &verified)
	if n != 1 || status != "System" || by != "seed" || verified {
		t.Fatalf("seed: n=%d status=%s by=%s verified=%v", n, status, by, verified)
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
