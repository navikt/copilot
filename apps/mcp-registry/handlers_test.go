package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func testConfig() *Config {
	return &Config{
		DomainInternal: "intern.dev.nav.no",
		DomainExternal: "ekstern.dev.nav.no",
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", result["status"])
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	readyHandler(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestServersListHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers", nil)
	w := httptest.NewRecorder()

	serversListHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response as ServerListResponse: %v", err)
	}

	if len(response.Servers) == 0 {
		t.Error("registry must contain at least one server")
	}

	if response.Metadata.Count != len(response.Servers) {
		t.Errorf("metadata.count (%d) does not match servers length (%d)", response.Metadata.Count, len(response.Servers))
	}

	for i, sr := range response.Servers {
		if sr.Server.Name == "" {
			t.Errorf("server[%d]: name is required", i)
		}
		if sr.Server.Description == "" {
			t.Errorf("server[%d]: description is required", i)
		}
		if sr.Server.Version == "" {
			t.Errorf("server[%d]: version is required", i)
		}
		if sr.Meta.Official == nil {
			t.Errorf("server[%d]: _meta.io.modelcontextprotocol.registry/official is required", i)
		} else {
			if sr.Meta.Official.Status == "" {
				t.Errorf("server[%d]: status is required", i)
			}
			if sr.Meta.Official.PublishedAt.IsZero() {
				t.Errorf("server[%d]: publishedAt is required", i)
			}
			if sr.Meta.Official.UpdatedAt.IsZero() {
				t.Errorf("server[%d]: updatedAt is required", i)
			}
		}
	}
}

func TestServersListHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v0.1/servers", nil)
	w := httptest.NewRecorder()

	serversListHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}
}

func TestServersListHandler_Options(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v0.1/servers", nil)
	w := httptest.NewRecorder()

	serversListHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}

	if cors := resp.Header.Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("expected CORS header *, got %s", cors)
	}
}

func TestServerVersionHandler_Latest(t *testing.T) {
	serverName := "io.github.navikt/github-mcp"
	encodedName := url.PathEscape(serverName)
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers/"+encodedName+"/versions/latest", nil)
	w := httptest.NewRecorder()

	serverVersionHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Server.Name != serverName {
		t.Errorf("expected server name '%s', got '%s'", serverName, response.Server.Name)
	}

	if response.Meta.Official == nil {
		t.Error("expected _meta.io.modelcontextprotocol.registry/official to be present")
	}
}

func TestServerVersionHandler_SpecificVersion(t *testing.T) {
	serverName := "io.github.navikt/github-mcp"
	encodedName := url.PathEscape(serverName)
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers/"+encodedName+"/versions/1.0.0", nil)
	w := httptest.NewRecorder()

	serverVersionHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Server.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", response.Server.Version)
	}
}

func TestServerVersionHandler_LatestShortPath(t *testing.T) {
	serverName := "io.github.navikt/github-mcp"
	encodedName := url.PathEscape(serverName)
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers/"+encodedName+"/latest", nil)
	w := httptest.NewRecorder()

	serverVersionHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Server.Name != serverName {
		t.Errorf("expected server name '%s', got '%s'", serverName, response.Server.Name)
	}

	if response.Meta.Official == nil {
		t.Error("expected _meta.io.modelcontextprotocol.registry/official to be present")
	}
}

func TestServerVersionHandler_NotFound(t *testing.T) {
	serverName := "io.github.nonexistent/server"
	encodedName := url.PathEscape(serverName)
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers/"+encodedName+"/versions/latest", nil)
	w := httptest.NewRecorder()

	serverVersionHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestServerVersionHandler_InvalidPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers/invalid-path", nil)
	w := httptest.NewRecorder()

	serverVersionHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	rootHandler(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("failed to parse root response: %v", err)
	}

	if info["service"] == nil {
		t.Error("service field is missing")
	}

	if endpoints, ok := info["endpoints"].(map[string]interface{}); !ok || len(endpoints) == 0 {
		t.Error("endpoints field is missing or empty")
	}
}

func TestCORSHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers", nil)
	w := httptest.NewRecorder()

	serversListHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if cors := resp.Header.Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("expected Access-Control-Allow-Origin *, got %s", cors)
	}

	if methods := resp.Header.Get("Access-Control-Allow-Methods"); methods != "GET, OPTIONS" {
		t.Errorf("expected Access-Control-Allow-Methods 'GET, OPTIONS', got %s", methods)
	}

	if headers := resp.Header.Get("Access-Control-Allow-Headers"); headers != "Authorization, Content-Type" {
		t.Errorf("expected Access-Control-Allow-Headers 'Authorization, Content-Type', got %s", headers)
	}
}

func withTempAllowlist(t *testing.T, allowlistJSON string) func() {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/allowlist.json", []byte(allowlistJSON), 0600); err != nil {
		t.Fatalf("failed to write temp allowlist.json: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	return func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}
}

func TestServersListHandler_PackageServers(t *testing.T) {
	allowlist := `{
		"servers": [
			{
				"name": "io.github.test/remote-server",
				"description": "A remote HTTP server.",
				"version": "1.0.0",
				"remotes": [{"type": "streamable-http", "url": "https://example.com/mcp"}]
			},
			{
				"name": "io.github.test/stdio-server",
				"description": "A local stdio server.",
				"version": "0.1.0",
				"packages": [{"registryType": "npm", "identifier": "@test/mcp-server", "transport": {"type": "stdio"}}]
			},
			{
				"name": "io.github.test/dual-server",
				"description": "A server with both remotes and packages.",
				"version": "2.0.0",
				"remotes": [{"type": "streamable-http", "url": "https://dual.example.com/mcp"}],
				"packages": [{"registryType": "pypi", "identifier": "mcp-dual-server", "transport": {"type": "stdio"}}]
			}
		]
	}`
	cleanup := withTempAllowlist(t, allowlist)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers", nil)
	w := httptest.NewRecorder()

	serversListHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Metadata.Count != 3 {
		t.Fatalf("expected 3 servers, got %d", response.Metadata.Count)
	}

	for _, sr := range response.Servers {
		switch sr.Server.Name {
		case "io.github.test/remote-server":
			if len(sr.Server.Remotes) != 1 {
				t.Errorf("remote-server: expected 1 remote, got %d", len(sr.Server.Remotes))
			}
			if len(sr.Server.Packages) != 0 {
				t.Errorf("remote-server: expected 0 packages, got %d", len(sr.Server.Packages))
			}
		case "io.github.test/stdio-server":
			if len(sr.Server.Remotes) != 0 {
				t.Errorf("stdio-server: expected 0 remotes, got %d", len(sr.Server.Remotes))
			}
			if len(sr.Server.Packages) != 1 {
				t.Fatalf("stdio-server: expected 1 package, got %d", len(sr.Server.Packages))
			}
			pkg := sr.Server.Packages[0]
			if pkg.RegistryType != "npm" {
				t.Errorf("stdio-server: expected registryType 'npm', got '%s'", pkg.RegistryType)
			}
			if pkg.Identifier != "@test/mcp-server" {
				t.Errorf("stdio-server: expected identifier '@test/mcp-server', got '%s'", pkg.Identifier)
			}
			if pkg.Transport.Type != "stdio" {
				t.Errorf("stdio-server: expected transport 'stdio', got '%s'", pkg.Transport.Type)
			}
		case "io.github.test/dual-server":
			if len(sr.Server.Remotes) != 1 {
				t.Errorf("dual-server: expected 1 remote, got %d", len(sr.Server.Remotes))
			}
			if len(sr.Server.Packages) != 1 {
				t.Fatalf("dual-server: expected 1 package, got %d", len(sr.Server.Packages))
			}
			if sr.Server.Packages[0].RegistryType != "pypi" {
				t.Errorf("dual-server: expected registryType 'pypi', got '%s'", sr.Server.Packages[0].RegistryType)
			}
		}
	}
}

func TestServerVersionHandler_PackageServer(t *testing.T) {
	allowlist := `{
		"servers": [
			{
				"name": "io.github.test/stdio-server",
				"description": "A local stdio server.",
				"version": "0.1.0",
				"packages": [{
					"registryType": "npm",
					"identifier": "@test/mcp-server",
					"version": "1.2.3",
					"runtimeHint": "npx",
					"transport": {"type": "stdio"},
					"environmentVariables": [
						{"name": "API_KEY", "description": "API key", "isRequired": true, "isSecret": true}
					]
				}]
			}
		]
	}`
	cleanup := withTempAllowlist(t, allowlist)
	defer cleanup()

	serverName := "io.github.test/stdio-server"
	encodedName := url.PathEscape(serverName)
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers/"+encodedName+"/versions/latest", nil)
	w := httptest.NewRecorder()

	serverVersionHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Server.Name != serverName {
		t.Errorf("expected name '%s', got '%s'", serverName, response.Server.Name)
	}

	if len(response.Server.Remotes) != 0 {
		t.Errorf("expected no remotes, got %d", len(response.Server.Remotes))
	}

	if len(response.Server.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(response.Server.Packages))
	}

	pkg := response.Server.Packages[0]
	if pkg.RegistryType != "npm" {
		t.Errorf("expected registryType 'npm', got '%s'", pkg.RegistryType)
	}
	if pkg.Identifier != "@test/mcp-server" {
		t.Errorf("expected identifier '@test/mcp-server', got '%s'", pkg.Identifier)
	}
	if pkg.Version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got '%s'", pkg.Version)
	}
	if pkg.RuntimeHint != "npx" {
		t.Errorf("expected runtimeHint 'npx', got '%s'", pkg.RuntimeHint)
	}
	if pkg.Transport.Type != "stdio" {
		t.Errorf("expected transport type 'stdio', got '%s'", pkg.Transport.Type)
	}
	if len(pkg.EnvironmentVariables) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(pkg.EnvironmentVariables))
	}
	env := pkg.EnvironmentVariables[0]
	if env.Name != "API_KEY" {
		t.Errorf("expected env name 'API_KEY', got '%s'", env.Name)
	}
	if !env.IsRequired {
		t.Error("expected env isRequired true")
	}
	if !env.IsSecret {
		t.Error("expected env isSecret true")
	}
}

func TestServerVersionHandler_PackageArguments(t *testing.T) {
	allowlist := `{
		"servers": [
			{
				"name": "io.github.test/browser-mcp",
				"description": "Browser automation server.",
				"version": "0.1.0",
				"packages": [{
					"registryType": "npm",
					"identifier": "@test/browser-mcp",
					"runtimeHint": "node",
					"transport": {"type": "stdio"},
					"packageArguments": [
						{"type": "named", "name": "--isolated"},
						{"type": "named", "name": "--caps", "value": "core"},
						{"type": "named", "name": "--blocked-origins", "value": "*.internal.example.com"}
					],
					"runtimeArguments": [
						{"type": "named", "name": "--max-old-space-size", "value": "512"}
					]
				}]
			}
		]
	}`
	cleanup := withTempAllowlist(t, allowlist)
	defer cleanup()

	serverName := "io.github.test/browser-mcp"
	encodedName := url.PathEscape(serverName)
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers/"+encodedName+"/versions/latest", nil)
	w := httptest.NewRecorder()

	serverVersionHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Server.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(response.Server.Packages))
	}

	pkg := response.Server.Packages[0]

	if len(pkg.PackageArguments) != 3 {
		t.Fatalf("expected 3 packageArguments, got %d", len(pkg.PackageArguments))
	}

	expectedArgs := []struct {
		argType string
		name    string
		value   string
	}{
		{"named", "--isolated", ""},
		{"named", "--caps", "core"},
		{"named", "--blocked-origins", "*.internal.example.com"},
	}

	for i, expected := range expectedArgs {
		arg := pkg.PackageArguments[i]
		if arg.Type != expected.argType {
			t.Errorf("packageArguments[%d]: expected type '%s', got '%s'", i, expected.argType, arg.Type)
		}
		if arg.Name != expected.name {
			t.Errorf("packageArguments[%d]: expected name '%s', got '%s'", i, expected.name, arg.Name)
		}
		if arg.Value != expected.value {
			t.Errorf("packageArguments[%d]: expected value '%s', got '%s'", i, expected.value, arg.Value)
		}
	}

	if len(pkg.RuntimeArguments) != 1 {
		t.Fatalf("expected 1 runtimeArgument, got %d", len(pkg.RuntimeArguments))
	}

	if pkg.RuntimeArguments[0].Name != "--max-old-space-size" {
		t.Errorf("expected runtimeArgument name '--max-old-space-size', got '%s'", pkg.RuntimeArguments[0].Name)
	}
	if pkg.RuntimeArguments[0].Value != "512" {
		t.Errorf("expected runtimeArgument value '512', got '%s'", pkg.RuntimeArguments[0].Value)
	}
}

func TestServersListHandler_PlaywrightSecurityArgs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v0.1/servers", nil)
	w := httptest.NewRecorder()

	serversListHandler(w, req, testConfig())

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var response ServerListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var playwright *ServerResponse
	for i, sr := range response.Servers {
		if sr.Server.Name == "com.microsoft/playwright-mcp" {
			playwright = &response.Servers[i]
			break
		}
	}

	if playwright == nil {
		t.Fatal("playwright-mcp not found in registry")
	}

	if len(playwright.Server.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(playwright.Server.Packages))
	}

	pkg := playwright.Server.Packages[0]

	requiredFlags := map[string]string{
		"--isolated":              "",
		"--caps":                  "core",
		"--block-service-workers": "",
		"--save-trace":            "",
	}

	for _, arg := range pkg.PackageArguments {
		if expected, ok := requiredFlags[arg.Name]; ok {
			if expected != "" && arg.Value != expected {
				t.Errorf("flag %s: expected value '%s', got '%s'", arg.Name, expected, arg.Value)
			}
			delete(requiredFlags, arg.Name)
		}
	}

	for flag := range requiredFlags {
		t.Errorf("missing required security flag: %s", flag)
	}

	hasBlockedOrigins := false
	for _, arg := range pkg.PackageArguments {
		if arg.Name == "--blocked-origins" {
			hasBlockedOrigins = true
			if arg.Value == "" {
				t.Error("--blocked-origins must have a value")
			}
		}
	}
	if !hasBlockedOrigins {
		t.Error("missing required security flag: --blocked-origins")
	}
}
