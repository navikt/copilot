package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// auditRow is one audit_events row. Nil pointers and empty actor or
// resource kind, oid and client id are stored as NULL.
type auditRow struct {
	Actor, ActorKind, ActorOid, ActorClientID string
	Action, ResourceKind, Namespace, Name     string
	Version, Generation                       *int
	Detail                                    any
}

// insertAudit is the only writer of audit_events. The `audit fleet-control`
// line is logged by auditTracer once the transaction commits, so Loki holds
// the trail after the database prunes it (#2762).
func insertAudit(ctx context.Context, tx pgx.Tx, r auditRow) error {
	var detail *string
	if r.Detail != nil {
		raw, err := json.Marshal(r.Detail)
		if err != nil {
			return err
		}
		s := string(raw)
		detail = &s
	}
	// Only columns with a value are named, so a writer that sets none of the
	// 005 columns still works against a pre-005 schema (TestRealPostgresMigration005).
	cols := []string{"actor", "action", "namespace", "name", "version", "detail"}
	vals := []string{"$1", "$2", "$3", "$4", "$5", "$6::jsonb"}
	args := []any{r.Actor, r.Action, r.Namespace, r.Name, r.Version, detail}
	for _, c := range []struct {
		col, cast string
		v         any
		set       bool
	}{
		{"actor_kind", "", r.ActorKind, r.ActorKind != ""},
		{"actor_oid", "::uuid", r.ActorOid, r.ActorOid != ""},
		{"actor_client_id", "", r.ActorClientID, r.ActorClientID != ""},
		{"resource_kind", "", r.ResourceKind, r.ResourceKind != ""},
		{"generation", "", r.Generation, r.Generation != nil},
	} {
		if c.set {
			args = append(args, c.v)
			cols = append(cols, c.col)
			vals = append(vals, fmt.Sprintf("$%d%s", len(args), c.cast))
		}
	}
	var at time.Time
	err := tx.QueryRow(ctx, "INSERT INTO audit_events ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(vals, ", ")+") RETURNING at", args...).Scan(&at)
	if err != nil {
		return err
	}
	pendingAudit.add(tx.Conn(), loggedAudit{at, r})
	return nil
}

type loggedAudit struct {
	at time.Time
	r  auditRow
}

// auditTracer holds inserted rows per connection until the transaction ends:
// a COMMIT logs them, anything else that ends the transaction drops them.
// Nested transactions (savepoints) are not used for audited writes.
type auditTracer struct {
	mu      sync.Mutex
	pending map[*pgx.Conn][]loggedAudit
}

var pendingAudit = &auditTracer{pending: map[*pgx.Conn][]loggedAudit{}}

// forget frees rows held on a connection the pool destroys mid-transaction.
func (t *auditTracer) forget(c *pgx.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pending, c)
}

func (t *auditTracer) add(c *pgx.Conn, e loggedAudit) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending[c] = append(t.pending[c], e)
}

type traceSQL struct{}

func (t *auditTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	// pgx ends and starts transactions with exactly these lowercase statements.
	if d.SQL == "commit" || d.SQL == "rollback" || strings.HasPrefix(d.SQL, "begin") {
		return context.WithValue(ctx, traceSQL{}, d.SQL)
	}
	return ctx
}

func (t *auditTracer) TraceQueryEnd(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryEndData) {
	sql, _ := ctx.Value(traceSQL{}).(string)
	if sql == "" {
		return
	}
	t.mu.Lock()
	rows := t.pending[c]
	delete(t.pending, c)
	t.mu.Unlock()
	// A commit of a failed transaction returns tag ROLLBACK without an error.
	if sql != "commit" || d.Err != nil || d.CommandTag.String() != "COMMIT" {
		return
	}
	for _, e := range rows {
		logAudit(e)
	}
}

// logAudit writes the row's fields in the fleet-gateway audit shape
// (AuthZEN subject/action/resource). detail is not logged: it can hold free
// text, and Loki needs only who did what to which resource.
func logAudit(e loggedAudit) {
	r := e.r
	// The seed row leaves actor_kind NULL: Seed can run against a pre-005
	// schema (TestRealPostgresMigration005), so its kind is set here.
	kind := r.ActorKind
	switch {
	case kind == "" && r.Actor == "seed":
		kind = "System"
	case kind == "":
		kind = "User"
	}
	gen := r.Generation
	if gen == nil {
		gen = r.Version
	}
	var g any
	if gen != nil {
		g = *gen
	}
	slog.Info("audit fleet-control",
		"at", e.at.UTC(),
		slog.Group("subject",
			"type", strings.ToLower(kind),
			"id", r.Actor,
			slog.Group("properties", "oid", r.ActorOid, "client_id", r.ActorClientID),
		),
		slog.Group("action", "name", r.Action),
		slog.Group("context", "outcome", "committed"),
		slog.Group("resource",
			"type", r.ResourceKind,
			"id", strings.Trim(r.Namespace+"/"+r.Name, "/"),
			slog.Group("properties", "namespace", r.Namespace, "name", r.Name, "generation", g),
		),
	)
}
