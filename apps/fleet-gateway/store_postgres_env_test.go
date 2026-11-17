package main

import (
	"context"
	"net/url"
	"os"
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
	if s.pool.Config().ConnConfig.DefaultQueryExecMode.String() == "" {
		t.Fatal("exec mode unset")
	}
	if err := s.Put(ctx, kindSession, "env-k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := s.Pop(ctx, kindSession, "env-k"); err != nil || !ok || string(v) != "v" {
		t.Fatalf("pop = %q %v %v", v, ok, err)
	}
}

func TestPGEnvPrefix(t *testing.T) {
	t.Setenv("PGUSER", "plain")
	t.Setenv("READWRITE_PGUSER", "rw")
	applyPGEnvPrefix("READWRITE_")
	if os.Getenv("PGUSER") != "rw" {
		t.Fatal("prefixed var not applied")
	}
}
