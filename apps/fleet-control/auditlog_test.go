package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func captureAuditLines(t *testing.T) func() []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return func() []map[string]any {
		var out []map[string]any
		for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			var m map[string]any
			if l == "" || json.Unmarshal([]byte(l), &m) != nil || m["msg"] != "audit fleet-control" {
				continue
			}
			out = append(out, m)
		}
		buf.Reset()
		return out
	}
}

func TestRealPostgresAuditLogOnlyAfterCommit(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	lines := captureAuditLines(t)
	gen := 3
	row := auditRow{Actor: "Z990001", ActorKind: "User", ActorOid: "00000000-0000-0000-0000-000000000001", ActorClientID: "client-a",
		Action: "approve", ResourceKind: "MCPServer", Namespace: "team-a", Name: "srv", Version: &gen, Generation: &gen,
		Detail: map[string]any{"reason": "do not log me"}}

	boom := errors.New("boom")
	if err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		if err := insertAudit(ctx, tx, row); err != nil {
			return err
		}
		return boom
	}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	// A failed statement after the insert aborts the transaction; COMMIT then rolls back.
	if err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		if err := insertAudit(ctx, tx, row); err != nil {
			return err
		}
		_, _ = tx.Exec(ctx, `SELECT 1/0`)
		return nil
	}); err == nil {
		t.Fatal("commit of aborted transaction succeeded")
	}
	if got := lines(); len(got) != 0 {
		t.Fatalf("rolled-back writes logged %d lines: %v", len(got), got)
	}

	if err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error { return insertAudit(ctx, tx, row) }); err != nil {
		t.Fatal(err)
	}
	got := lines()
	if len(got) != 1 {
		t.Fatalf("committed write logged %d lines, want 1: %v", len(got), got)
	}
	var n int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE actor = 'Z990001'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows = %d, %v", n, err)
	}
	l := got[0]
	sub := l["subject"].(map[string]any)
	props := sub["properties"].(map[string]any)
	res := l["resource"].(map[string]any)
	rp := res["properties"].(map[string]any)
	for k, want := range map[string]any{
		"subject.type": sub["type"], "subject.id": sub["id"], "oid": props["oid"], "client_id": props["client_id"],
		"action": l["action"].(map[string]any)["name"], "resource.type": res["type"], "resource.id": res["id"],
		"namespace": rp["namespace"], "name": rp["name"], "generation": rp["generation"], "outcome": l["context"].(map[string]any)["outcome"],
	} {
		exp := map[string]any{"subject.type": "user", "subject.id": "Z990001", "oid": row.ActorOid, "client_id": "client-a",
			"action": "approve", "resource.type": "MCPServer", "resource.id": "team-a/srv",
			"namespace": "team-a", "name": "srv", "generation": float64(3), "outcome": "committed"}[k]
		if want != exp {
			t.Errorf("%s = %v, want %v", k, want, exp)
		}
	}
	if l["at"] == nil || strings.Contains(fmtJSON(l), "do not log me") {
		t.Errorf("line missing at or leaks detail: %v", l)
	}
}

func TestAuditTracerForget(t *testing.T) {
	c := &pgx.Conn{}
	pendingAudit.add(c, loggedAudit{})
	pendingAudit.forget(c)
	pendingAudit.mu.Lock()
	defer pendingAudit.mu.Unlock()
	if _, ok := pendingAudit.pending[c]; ok {
		t.Fatal("rows kept for a closed connection")
	}
}

func fmtJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// Each writer records who acted: a person with the token's azp, the seed as
// System, a runtime as Application.
func TestRealPostgresAuditActorKindAndClientID(t *testing.T) {
	e := newApisEnv(t)
	ctx := context.Background()
	routes, allowlist := readSeedFiles(t, "prod")
	servers, err := seedServers(routes, allowlist)
	if err != nil {
		t.Fatal(err)
	}
	lines := captureAuditLines(t)
	if _, err := e.db.Seed(ctx, servers, "test"); err != nil {
		t.Fatal(err)
	}
	e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), serverJSON("team-a", "draft", "", "search"))
	if _, err := e.db.CreateJob(ctx, "A111111", "team-a", CreateJobRequest{Kind: "k", Params: json.RawMessage("{}"), OnExpiry: "requeue", MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	if l, err := e.db.Claim(ctx, "rt-1", "tok-1", time.Minute); err != nil || l == nil {
		t.Fatalf("claim: %v %v", l, err)
	}
	want := map[string][2]string{
		"seed_published": {"system", ""},
		"job_created":    {"user", ""},
		"job_claimed":    {"application", ""},
	}
	got := map[string][2]string{}
	for _, l := range lines() {
		s := l["subject"].(map[string]any)
		got[l["action"].(map[string]any)["name"].(string)] = [2]string{s["type"].(string), s["properties"].(map[string]any)["client_id"].(string)}
	}
	for a, w := range want {
		if got[a] != w {
			t.Errorf("%s: subject type, client_id = %v, want %v", a, got[a], w)
		}
	}
	var created [2]string
	for a, v := range got {
		if a != "seed_published" && a != "job_created" && a != "job_claimed" {
			created = v
		}
	}
	if created != [2]string{"user", adminID} {
		t.Errorf("v1 create: subject type, client_id = %v, want user, %s", created, adminID)
	}
	var stored string
	if err := e.db.pool.QueryRow(ctx, `SELECT actor_client_id FROM audit_events WHERE actor = 'A111111' AND resource_kind IS NOT NULL`).Scan(&stored); err != nil || stored != adminID {
		t.Errorf("stored actor_client_id = %q, %v", stored, err)
	}
}
