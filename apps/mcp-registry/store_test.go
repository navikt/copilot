package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStore_StaticWinsOverDBOnSameName(t *testing.T) {
	registryDBCollisionsTotal.Reset()
	cfg := testConfig()

	// allowlist.json contains "io.github.nais/nais-mcp" with version "6.6.1"
	collidingDBEntry := DBEntry{
		Name: "io.github.nais/nais-mcp",
		Entry: []byte(`{
			"name": "io.github.nais/nais-mcp",
			"description": "DB duplicate with colliding name and version 99.0.0",
			"version": "99.0.0"
		}`),
	}
	validDBEntry := DBEntry{
		Name: "io.github.navikt/test-valid-server",
		Entry: []byte(`{
			"name": "io.github.navikt/test-valid-server",
			"description": "A valid unique test server from DB",
			"version": "1.0.0",
			"status": "active"
		}`),
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

	collisions := testutil.ToFloat64(registryDBCollisionsTotal.WithLabelValues("io.github.nais/nais-mcp"))
	if collisions != 1 {
		t.Errorf("expected 1 collision recorded for io.github.nais/nais-mcp, got %v", collisions)
	}
}

func TestStore_LoaderErrorKeepsLastGoodSet(t *testing.T) {
	cfg := testConfig()
	validDBEntry := DBEntry{
		Name: "io.github.navikt/test-db-server",
		Entry: []byte(`{
			"name": "io.github.navikt/test-db-server",
			"description": "Valid DB server",
			"version": "1.0.0"
		}`),
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
			Name: "io.github.navikt/malformed-json",
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
			Name: "io.github.navikt/valid-server",
			Entry: []byte(`{
				"name": "io.github.navikt/valid-server",
				"description": "Completely valid server",
				"version": "1.0.0",
				"websiteUrl": "https://nav.no"
			}`),
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
}

func TestStore_HTTPHandlersIntegration(t *testing.T) {
	cfg := testConfig()
	validDBEntry := DBEntry{
		Name: "io.github.navikt/integration-db-server",
		Entry: []byte(`{
			"name": "io.github.navikt/integration-db-server",
			"description": "DB server for integration test",
			"version": "2.0.0"
		}`),
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
	cfg := testConfig()
	var mu sync.Mutex
	calls := 0
	loader := func(_ context.Context) ([]DBEntry, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return nil, nil
	}

	store := NewStore(cfg, "allowlist.json", loader)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	store.StartRefresher(ctx, 15*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	cancel()

	mu.Lock()
	count := calls
	mu.Unlock()

	if count < 2 {
		t.Errorf("expected refresher to run at least 2 times, ran %d times", count)
	}
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
