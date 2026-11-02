package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStore_StaticWinsOverDBOnSameName(t *testing.T) {
	registryDBEntriesDroppedTotal.Reset()
	cfg := testConfig()

	// allowlist.json contains "io.github.nais/nais-mcp" with version "6.6.1"
	collidingDBEntry := DBEntry{
		Name:  "io.github.nais/nais-mcp",
		Entry: subJSON("io.github.nais/nais-mcp", "DB duplicate with colliding name and version 99.0.0", "99.0.0"),
	}
	validDBEntry := DBEntry{
		Name:  "io.github.navikt/test-valid-server",
		Entry: subJSON("io.github.navikt/test-valid-server", "A valid unique test server from DB", "1.0.0"),
	}

	loader := func(_ context.Context) ([]DBEntry, error) {
		return []DBEntry{collidingDBEntry, validDBEntry}, nil
	}

	store := NewStore(cfg, "allowlist.json", loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatalf("unexpected reload error: %v", err)
	}

	servers, err := store.GetServers()
	if err != nil {
		t.Fatalf("GetServers error: %v", err)
	}

	var foundStatic, foundValidDB bool
	for _, s := range servers {
		if s.Server.Name == "io.github.nais/nais-mcp" {
			foundStatic = true
			if s.Server.Version == "99.0.0" {
				t.Errorf("expected static version to win, got DB version 99.0.0")
			}
		}
		if s.Server.Name == "io.github.navikt/test-valid-server" {
			foundValidDB = true
		}
	}

	if !foundStatic {
		t.Errorf("expected static server io.github.nais/nais-mcp in servers list")
	}
	if !foundValidDB {
		t.Errorf("expected valid non-colliding DB server in servers list")
	}

	collisions := testutil.ToFloat64(registryDBEntriesDroppedTotal.WithLabelValues("collision"))
	if collisions != 1 {
		t.Errorf("expected 1 collision recorded, got %v", collisions)
	}
}

func TestStore_LoaderErrorKeepsLastGoodSet(t *testing.T) {
	cfg := testConfig()
	validDBEntry := DBEntry{
		Name:  "io.github.navikt/test-db-server",
		Entry: subJSON("io.github.navikt/test-db-server", "Valid DB server", "1.0.0"),
	}

	var shouldFail bool
	loader := func(_ context.Context) ([]DBEntry, error) {
		if shouldFail {
			return nil, errors.New("database connection lost")
		}
		return []DBEntry{validDBEntry}, nil
	}

	store := NewStore(cfg, "allowlist.json", loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatalf("initial reload failed: %v", err)
	}

	serversInitial, err := store.GetServers()
	if err != nil {
		t.Fatalf("GetServers failed: %v", err)
	}
	var found bool
	for _, s := range serversInitial {
		if s.Server.Name == "io.github.navikt/test-db-server" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DB server in initial load")
	}

	// Trigger error on reload
	shouldFail = true
	err = store.Reload(t.Context())
	if err == nil {
		t.Fatalf("expected reload to return error on DB failure")
	}

	// Last good merged set must still be served
	serversAfterErr, err := store.GetServers()
	if err != nil {
		t.Fatalf("GetServers failed after reload error: %v", err)
	}
	foundAfterErr := false
	for _, s := range serversAfterErr {
		if s.Server.Name == "io.github.navikt/test-db-server" {
			foundAfterErr = true
			break
		}
	}
	if !foundAfterErr {
		t.Errorf("expected DB server to still be present in last-good cache after DB failure")
	}
	if len(serversAfterErr) != len(serversInitial) {
		t.Errorf("server count changed: had %d, now %d", len(serversInitial), len(serversAfterErr))
	}
}

func TestStore_LoaderNeverSucceededServesStaticOnly(t *testing.T) {
	cfg := testConfig()
	loader := func(_ context.Context) ([]DBEntry, error) {
		return nil, errors.New("database completely unavailable")
	}

	store := NewStore(cfg, "allowlist.json", loader)
	err := store.Reload(t.Context())
	if err == nil {
		t.Fatalf("expected error from Reload when loader fails")
	}

	servers, err := store.GetServers()
	if err != nil {
		t.Fatalf("GetServers failed: %v", err)
	}

	if len(servers) == 0 {
		t.Fatalf("expected static servers to be served, got 0")
	}

	for _, s := range servers {
		if s.Server.Name == "" {
			t.Errorf("server name is empty")
		}
	}
}

func TestStore_InvalidDBEntryDropped(t *testing.T) {
	registryDBEntriesDroppedTotal.Reset()
	cfg := testConfig()
	entries := []DBEntry{
		{
			Name: "invalid-name-without-slash",
			Entry: []byte(`{
				"name": "invalid-name-without-slash",
				"description": "Invalid name format",
				"version": "1.0.0"
			}`),
		},
		{
			Name: "io.github.navikt/empty-description",
			Entry: []byte(`{
				"name": "io.github.navikt/empty-description",
				"description": "",
				"version": "1.0.0"
			}`),
		},
		{
			Name:  "io.github.navikt/malformed-json",
			Entry: []byte(`{ broken json`),
		},
		{
			Name: "io.github.navikt/invalid-url-remote",
			Entry: []byte(`{
				"name": "io.github.navikt/invalid-url-remote",
				"description": "Remote with internal forbidden host",
				"version": "1.0.0",
				"remotes": [
					{"type": "streamable-http", "url": "https://service.intern.nav.no/mcp"}
				]
			}`),
		},
		{
			Name:  "io.github.navikt/unknown-field",
			Entry: []byte(`{"name":"io.github.navikt/unknown-field","description":"d","version":"1","owner":"team","remotes":[{"type":"sse","url":"https://mcp.example.com"}],"packages":[]}`),
		},
		{
			Name:  "io.github.navikt/stdio-remote",
			Entry: []byte(`{"name":"io.github.navikt/stdio-remote","description":"d","version":"1","owner":"team","remotes":[{"type":"stdio","url":"https://mcp.example.com"}]}`),
		},
		{
			Name:  "io.github.navikt/valid-server",
			Entry: subJSON("io.github.navikt/valid-server", "Completely valid server", "1.0.0"),
		},
	}

	loader := func(_ context.Context) ([]DBEntry, error) {
		return entries, nil
	}

	store := NewStore(cfg, "allowlist.json", loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatalf("unexpected reload error: %v", err)
	}

	servers, err := store.GetServers()
	if err != nil {
		t.Fatalf("GetServers failed: %v", err)
	}

	var foundValid bool
	for _, s := range servers {
		switch s.Server.Name {
		case "invalid-name-without-slash",
			"io.github.navikt/empty-description",
			"io.github.navikt/malformed-json",
			"io.github.navikt/invalid-url-remote":
			t.Errorf("invalid DB entry %q was NOT dropped", s.Server.Name)
		case "io.github.navikt/valid-server":
			foundValid = true
		}
	}

	if !foundValid {
		t.Errorf("expected valid DB server io.github.navikt/valid-server to be included")
	}
	if got := testutil.ToFloat64(registryDBEntriesDroppedTotal.WithLabelValues("parse")); got != 1 {
		t.Errorf("parse drops = %v, want 1", got)
	}
	if got := testutil.ToFloat64(registryDBEntriesDroppedTotal.WithLabelValues("invalid")); got != 5 {
		t.Errorf("invalid drops = %v, want 5", got)
	}
}

func TestStore_DBEntryUsesRowUpdatedAt(t *testing.T) {
	updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	loader := func(_ context.Context) ([]DBEntry, error) {
		return []DBEntry{{Name: "io.github.navikt/dated", Entry: subJSON("io.github.navikt/dated", "d", "1.0.0"), UpdatedAt: updated}}, nil
	}
	store := NewStore(testConfig(), "allowlist.json", loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	sr, _ := store.GetServer("io.github.navikt/dated", "latest")
	if sr == nil {
		t.Fatal("DB entry not served")
	}
	if !sr.Meta.Official.UpdatedAt.Equal(updated) {
		t.Errorf("UpdatedAt = %v, want %v", sr.Meta.Official.UpdatedAt, updated)
	}
	if sr.Meta.NavRegistry == nil || sr.Meta.NavRegistry.Owner != "team-registry" {
		t.Errorf("owner not carried into _meta: %+v", sr.Meta.NavRegistry)
	}
}

func TestStore_StaticLoadFailureKeepsLastGoodSet(t *testing.T) {
	data, err := os.ReadFile("allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "allowlist.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loader := func(_ context.Context) ([]DBEntry, error) {
		return []DBEntry{{Name: "io.github.navikt/db-only", Entry: subJSON("io.github.navikt/db-only", "d", "1.0.0")}}, nil
	}
	store := NewStore(testConfig(), path, loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	before, _ := store.GetServers()

	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(t.Context()); err != nil {
		t.Fatalf("reload with broken static but cached set: %v", err)
	}
	after, _ := store.GetServers()
	if len(after) != len(before) {
		t.Fatalf("server count changed from %d to %d after static load failure", len(before), len(after))
	}
}

func subJSON(name, desc, version string) []byte {
	return []byte(`{"name":"` + name + `","description":"` + desc + `","version":"` + version + `","owner":"team-registry","remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]}`)
}

func TestStore_HTTPHandlersIntegration(t *testing.T) {
	cfg := testConfig()
	validDBEntry := DBEntry{
		Name:  "io.github.navikt/integration-db-server",
		Entry: subJSON("io.github.navikt/integration-db-server", "DB server for integration test", "2.0.0"),
	}

	loader := func(_ context.Context) ([]DBEntry, error) {
		return []DBEntry{validDBEntry}, nil
	}

	store := NewStore(cfg, "allowlist.json", loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatalf("reload error: %v", err)
	}
	cfg.Store = store

	// Test serversListHandler with Store
	listReq := httptest.NewRequest(http.MethodGet, "/v0.1/servers", nil)
	listRec := httptest.NewRecorder()
	serversListHandler(listRec, listReq, cfg)

	if listRec.Code != http.StatusOK {
		t.Errorf("serversListHandler expected 200, got %d", listRec.Code)
	}

	var listResp ServerListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode servers list response: %v", err)
	}

	var foundInList bool
	for _, s := range listResp.Servers {
		if s.Server.Name == "io.github.navikt/integration-db-server" {
			foundInList = true
			if s.Server.Version != "2.0.0" {
				t.Errorf("expected version 2.0.0, got %s", s.Server.Version)
			}
		}
	}
	if !foundInList {
		t.Errorf("expected DB server in /v0.1/servers list response")
	}

	// Test serverVersionHandler with Store for DB server
	verReq := httptest.NewRequest(http.MethodGet, "/v0.1/servers/io.github.navikt%2Fintegration-db-server/latest", nil)
	verRec := httptest.NewRecorder()
	serverVersionHandler(verRec, verReq, cfg)

	if verRec.Code != http.StatusOK {
		body, _ := io.ReadAll(verRec.Body)
		t.Fatalf("serverVersionHandler expected 200, got %d: %s", verRec.Code, string(body))
	}

	var verResp ServerResponse
	if err := json.Unmarshal(verRec.Body.Bytes(), &verResp); err != nil {
		t.Fatalf("failed to decode server version response: %v", err)
	}
	if verResp.Server.Name != "io.github.navikt/integration-db-server" {
		t.Errorf("expected server name io.github.navikt/integration-db-server, got %s", verResp.Server.Name)
	}
}

func TestStore_BackgroundRefresher(t *testing.T) {
	loaded := make(chan struct{})
	loader := func(_ context.Context) ([]DBEntry, error) {
		loaded <- struct{}{}
		return nil, nil
	}
	store := NewStore(testConfig(), "allowlist.json", loader)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ticks := make(chan time.Time)
	done := make(chan struct{})
	go func() {
		store.runRefresher(ctx, ticks)
		close(done)
	}()
	for i := 0; i < 2; i++ {
		ticks <- time.Time{}
		<-loaded
	}
	cancel()
	<-done
}

func TestGetDatabaseURL(t *testing.T) {
	t.Run("returns DB_URL when set", func(t *testing.T) {
		t.Setenv("DB_URL", "postgres://localhost/testdb")
		if got := getDatabaseURL(); got != "postgres://localhost/testdb" {
			t.Errorf("expected postgres://localhost/testdb, got %s", got)
		}
	})

	t.Run("empty when DB_URL not set", func(t *testing.T) {
		t.Setenv("DB_URL", "")
		if got := getDatabaseURL(); got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})

	t.Run("ignores NAIS_DATABASE env vars", func(t *testing.T) {
		t.Setenv("DB_URL", "")
		t.Setenv("NAIS_DATABASE_MCP_REGISTRY_URL", "postgres://cloudsql/naisdb")
		if got := getDatabaseURL(); got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})
}
