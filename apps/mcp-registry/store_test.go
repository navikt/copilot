package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	for _, bad := range []string{
		"{broken",
		`{"servers":[]}`,
		`{"servers":[{"name":"bad name"}]}`,
		`{"servers":[{"name":"io.github.test/s","description":"d","version":"1","protocolVersion":["2026-07-28"]}]}`,
		`{"servers":[{"name":"io.github.test/s","description":"d","version":"1","sandboxHost":["api.figma.com"]}]}`,
	} {
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := store.Reload(t.Context()); err != nil {
			t.Fatalf("%s: reload with bad static but cached set: %v", bad, err)
		}
		after, _ := store.GetServers()
		if len(after) != len(before) {
			t.Fatalf("%s: server count changed from %d to %d after static load failure", bad, len(before), len(after))
		}
	}
}

func TestStore_StaticPostSubstitutionValidation(t *testing.T) {
	allowlistWithURL := func(url string) []byte {
		return []byte(`{"servers":[{"name":"io.github.navikt/test-mcp","description":"Test MCP server","version":"1.0.0","remotes":[{"type":"streamable-http","url":"` + url + `"}]}]}`)
	}

	tests := []struct {
		name           string
		rawURL         string
		domainInternal string
		domainExternal string
		wantErr        bool
		wantURL        string
	}{
		{
			name:           "(a) template domain_internal with intern.nav.no loads OK",
			rawURL:         "https://mcp.{{domain_internal}}/mcp",
			domainInternal: "intern.nav.no",
			domainExternal: "ekstern.nav.no",
			wantErr:        false,
			wantURL:        "https://mcp.intern.nav.no/mcp",
		},
		{
			name:           "(a) template domain_internal with intern.dev.nav.no loads OK",
			rawURL:         "https://mcp.{{domain_internal}}/mcp",
			domainInternal: "intern.dev.nav.no",
			domainExternal: "ekstern.dev.nav.no",
			wantErr:        false,
			wantURL:        "https://mcp.intern.dev.nav.no/mcp",
		},
		{
			name:           "(b) non-template literal x.intern.nav.no is rejected",
			rawURL:         "https://x.intern.nav.no/mcp",
			domainInternal: "intern.nav.no",
			domainExternal: "ekstern.nav.no",
			wantErr:        true,
		},
		{
			name:           "(c) template with slash in DomainInternal is rejected",
			rawURL:         "https://mcp.{{domain_internal}}/mcp",
			domainInternal: "evil.example.com/x",
			domainExternal: "ekstern.nav.no",
			wantErr:        true,
		},
		{
			name:           "(c) template with spaces in DomainInternal is rejected",
			rawURL:         "https://mcp.{{domain_internal}}/mcp",
			domainInternal: "bad host.nav.no",
			domainExternal: "ekstern.nav.no",
			wantErr:        true,
		},
		{
			name:           "(c) template with scheme in DomainInternal is rejected",
			rawURL:         "https://{{domain_internal}}/mcp",
			domainInternal: "http://evil.example.com",
			domainExternal: "ekstern.nav.no",
			wantErr:        true,
		},
		{
			name:           "(c) template with empty DomainInternal is rejected",
			rawURL:         "https://mcp.{{domain_internal}}/mcp",
			domainInternal: "",
			domainExternal: "ekstern.nav.no",
			wantErr:        true,
		},
		{
			name:           "(d) domain_external resolving to intern.nav.no is rejected",
			rawURL:         "https://mcp.{{domain_external}}/mcp",
			domainInternal: "intern.nav.no",
			domainExternal: "intern.nav.no",
			wantErr:        true,
		},
		{
			name:           "(d) domain_external resolving to intern.dev.nav.no is rejected",
			rawURL:         "https://mcp.{{domain_external}}/mcp",
			domainInternal: "intern.dev.nav.no",
			domainExternal: "x.intern.dev.nav.no",
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "allowlist.json")
			if err := os.WriteFile(path, allowlistWithURL(tc.rawURL), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := &Config{
				DomainInternal: tc.domainInternal,
				DomainExternal: tc.domainExternal,
			}
			store := NewStore(cfg, path, nil)
			servers, err := store.GetServers()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error from GetServers on invalid allowlist, got %d servers", len(servers))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected GetServers error: %v", err)
			}
			if len(servers) != 1 || len(servers[0].Server.Remotes) != 1 {
				t.Fatalf("unexpected servers shape: %+v", servers)
			}
			if got := servers[0].Server.Remotes[0].URL; got != tc.wantURL {
				t.Errorf("remote URL = %q, want %q", got, tc.wantURL)
			}
		})
	}

	t.Run("reload failure after bad substitution keeps last good set", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "allowlist.json")
		if err := os.WriteFile(path, allowlistWithURL("https://mcp.{{domain_internal}}/mcp"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := &Config{
			DomainInternal: "intern.nav.no",
			DomainExternal: "ekstern.nav.no",
		}
		store := NewStore(cfg, path, nil)
		before, err := store.GetServers()
		if err != nil {
			t.Fatalf("initial load failed: %v", err)
		}
		wantGoodURL := "https://mcp.intern.nav.no/mcp"
		if before[0].Server.Remotes[0].URL != wantGoodURL {
			t.Fatalf("initial URL = %q, want %q", before[0].Server.Remotes[0].URL, wantGoodURL)
		}

		for _, badDomain := range []string{"evil.example.com/x", "bad host.nav.no", "http://evil.example.com"} {
			cfg.DomainInternal = badDomain
			if _, _, loadErr := store.loadStatic(); loadErr == nil {
				t.Fatalf("loadStatic(%q) succeeded, want error", badDomain)
			}
			if err := store.Reload(t.Context()); err != nil {
				t.Fatalf("Reload(%q) returned %v, want nil when keeping last good static set", badDomain, err)
			}
			after, err := store.GetServers()
			if err != nil {
				t.Fatalf("GetServers after bad reload failed: %v", err)
			}
			if len(after) != len(before) || after[0].Server.Remotes[0].URL != wantGoodURL {
				t.Fatalf("last good set changed after %q: got %+v", badDomain, after)
			}
		}
	})
}

func TestStore_ReloadBoundsStalledDBLoad(t *testing.T) {
	old := dbLoadTimeout
	dbLoadTimeout = 50 * time.Millisecond
	t.Cleanup(func() { dbLoadTimeout = old })
	loader := func(ctx context.Context) ([]DBEntry, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	store := NewStore(testConfig(), "allowlist.json", loader)
	done := make(chan error, 1)
	go func() { done <- store.Reload(context.Background()) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Reload hung on a stalled DB load")
	}
	if servers, err := store.GetServers(); err != nil || len(servers) == 0 {
		t.Fatalf("static entries not served: %d, %v", len(servers), err)
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

func TestLoadConfig_DatabaseEnabled(t *testing.T) {
	t.Run("disabled when DB_URL and PGHOST are unset", func(t *testing.T) {
		t.Setenv("DB_URL", "")
		t.Setenv("PGHOST", "")
		if cfg := LoadConfig(); cfg.DatabaseEnabled {
			t.Error("expected DatabaseEnabled to be false")
		}
	})

	t.Run("enabled when DB_URL is set", func(t *testing.T) {
		t.Setenv("DB_URL", "postgres://localhost/testdb")
		t.Setenv("PGHOST", "")
		if cfg := LoadConfig(); !cfg.DatabaseEnabled {
			t.Error("expected DatabaseEnabled to be true")
		}
	})

	t.Run("enabled when PGHOST is set", func(t *testing.T) {
		t.Setenv("DB_URL", "")
		t.Setenv("PGHOST", "127.0.0.1")
		if cfg := LoadConfig(); !cfg.DatabaseEnabled {
			t.Error("expected DatabaseEnabled to be true")
		}
	})
}

func TestReloadBeforeDBConnectedIsQuiet(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	loader := func(_ context.Context) ([]DBEntry, error) { return nil, ErrDBNotConnected }
	store := NewStore(testConfig(), "allowlist.json", loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if servers, _ := store.GetServers(); len(servers) == 0 {
		t.Error("static entries not served")
	}
	if strings.Contains(buf.String(), "level=ERROR") {
		t.Errorf("unexpected error log: %s", buf.String())
	}
}
