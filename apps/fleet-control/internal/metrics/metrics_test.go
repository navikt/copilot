package metrics //nolint:revive // fleet-control never imports runtime/metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDBTracerCounts(t *testing.T) {
	tr := DBTracer{}
	run := func(sql string, err error) {
		ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sql})
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: err})
	}
	db, audit := testutil.ToFloat64(DBErrors), testutil.ToFloat64(AuditWriteFailures)
	run("SELECT 1", nil)
	run("SELECT 1", context.Canceled)
	run("INSERT INTO servers", &pgconn.PgError{Code: "23505"})
	if testutil.ToFloat64(DBErrors) != db {
		t.Error("success, cancel or constraint violation counted as a DB error")
	}
	run("SELECT 1", errors.New("conn reset"))
	run(`INSERT INTO audit_events (actor) VALUES ($1)`, errors.New("permission denied"))
	if got := testutil.ToFloat64(DBErrors) - db; got != 2 {
		t.Errorf("db errors: %v, want 2", got)
	}
	if got := testutil.ToFloat64(AuditWriteFailures) - audit; got != 1 {
		t.Errorf("audit failures: %v, want 1", got)
	}
}
