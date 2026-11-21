package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// Nais Postgres hands out libpq PG* variables; an empty DATABASE_URL must work from them.
func TestPostgresStore_FromPGEnv(t *testing.T) {
	raw := os.Getenv("FGW_TEST_PG_URL")
	if raw == "" {
		t.Skip("FGW_TEST_PG_URL not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pw, _ := u.User.Password()
	t.Setenv("PGHOST", u.Hostname())
	t.Setenv("PGPORT", u.Port())
	t.Setenv("PGUSER", u.User.Username())
	t.Setenv("PGPASSWORD", pw)
	t.Setenv("PGDATABASE", u.Path[1:])
	t.Setenv("PGSSLMODE", u.Query().Get("sslmode"))

	ctx := context.Background()
	s, err := newPostgresStore(ctx, "", time.Now, map[string]int{kindSession: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Pool().Config().ConnConfig.DefaultQueryExecMode.String() == "" {
		t.Fatal("exec mode unset")
	}
	if err := s.Put(ctx, kindSession, "env-k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := s.Pop(ctx, kindSession, "env-k"); err != nil || !ok || string(v) != "v" {
		t.Fatalf("pop = %q %v %v", v, ok, err)
	}
}

func TestPGEnvDSN(t *testing.T) {
	t.Setenv("READWRITE_PGHOST", "db.local")
	t.Setenv("READWRITE_PGUSER", `u'1\x`)
	if got, want := pgEnvDSN("READWRITE_"), `host='db.local' user='u\'1\\x'`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCheckPGRuntimeTables_IncludesRevoked(t *testing.T) {
	var queries []string
	if err := checkPGRuntimeTables(context.Background(), func(_ context.Context, q string) error {
		queries = append(queries, q)
		return nil
	}); err != nil {
		t.Fatalf("checkPGRuntimeTables failed: %v", err)
	}
	for _, want := range []string{
		"SELECT 1 FROM items LIMIT 1",
		"SELECT 1 FROM revoked LIMIT 1",
		"SELECT 1 FROM counters LIMIT 1",
	} {
		if !slices.Contains(queries, want) {
			t.Fatalf("missing startup check query %q in %v", want, queries)
		}
	}

	err := checkPGRuntimeTables(context.Background(), func(_ context.Context, q string) error {
		if strings.Contains(q, "revoked") {
			return errors.New("permission denied for table revoked")
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("expected error naming revoked table, got %v", err)
	}
}
