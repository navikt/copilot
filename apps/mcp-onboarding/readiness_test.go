package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/mcp-onboarding/internal/discovery"
)

func newGitHubMock(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	return httptest.NewServer(mux)
}

func newTestGitHubClient(serverURL string) *GitHubClient {
	return &GitHubClient{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   http.DefaultClient,
		APIBaseURL:   serverURL,
	}
}

// --- GitHub API method tests ---

func TestGetRepoFile_Exists(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/my-app/contents/.github/copilot-instructions.md": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Error("expected Authorization header")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"copilot-instructions.md","type":"file"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	exists, err := client.GetRepoFile("test-token", "navikt", "my-app", ".github/copilot-instructions.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected file to exist")
	}
}

func TestGetRepoFile_NotFound(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/my-app/contents/AGENTS.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	exists, err := client.GetRepoFile("test-token", "navikt", "my-app", "AGENTS.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected file to not exist")
	}
}

func TestGetDirectoryCount(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/my-app/contents/.github/instructions": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]RepoContent{
				{Name: "kotlin.instructions.md", Type: "file"},
				{Name: "testing.instructions.md", Type: "file"},
				{Name: "database.instructions.md", Type: "file"},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	count, err := client.GetDirectoryCount("test-token", "navikt", "my-app", ".github/instructions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestGetDirectoryCount_NotFound(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/my-app/contents/.github/agents": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	count, err := client.GetDirectoryCount("test-token", "navikt", "my-app", ".github/agents")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestGetRepoLanguages(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/my-app/languages": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Kotlin":50000,"Dockerfile":1200}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	langs, err := client.GetRepoLanguages("test-token", "navikt", "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(langs) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(langs))
	}
	langSet := map[string]bool{}
	for _, l := range langs {
		langSet[l] = true
	}
	if !langSet["Kotlin"] || !langSet["Dockerfile"] {
		t.Errorf("expected Kotlin and Dockerfile, got %v", langs)
	}
}

func TestGetRepoLanguages_Error(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/private-repo/languages": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	_, err := client.GetRepoLanguages("test-token", "navikt", "private-repo")
	if err == nil {
		t.Error("expected error for 403 response")
	}
}

func TestGetRepoFileContent_Found(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/my-app/contents/package.json": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "application/vnd.github.raw+json" {
				t.Error("expected raw content accept header")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name": "my-app", "version": "1.0.0"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	content, err := client.GetRepoFileContent("test-token", "navikt", "my-app", "package.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "my-app") {
		t.Errorf("expected content to contain 'my-app', got %q", content)
	}
}

func TestGetRepoFileContent_NotFound(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	content, err := client.GetRepoFileContent("test-token", "navikt", "my-app", "missing.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string for 404, got %q", content)
	}
}

func TestDetectRepoInfo_FullStack(t *testing.T) {
	server := newRepoInfoMock(t, `{"Go":50000,"Dockerfile":1000}`, map[string]bool{
		"go.mod":     true,
		"Dockerfile": true,
		".nais":      true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)

	info, err := handler.detectRepoInfo("tok", "navikt", "my-api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.HasGoMod {
		t.Error("expected HasGoMod")
	}
	if !info.HasDockerfile {
		t.Error("expected HasDockerfile")
	}
	if !info.HasNais {
		t.Error("expected HasNais")
	}
	if info.HasPackageJSON {
		t.Error("unexpected HasPackageJSON")
	}
	if info.Owner != "navikt" || info.Repo != "my-api" {
		t.Errorf("expected navikt/my-api, got %s/%s", info.Owner, info.Repo)
	}
}

func TestDetectRepoInfo_GradleProject(t *testing.T) {
	server := newRepoInfoMock(t, `{"Kotlin":60000}`, map[string]bool{
		"build.gradle.kts": true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)

	info, err := handler.detectRepoInfo("tok", "navikt", "kotlin-svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.HasGradleKts {
		t.Error("expected HasGradleKts")
	}
	if info.HasPomXML || info.HasGoMod {
		t.Error("unexpected build tool flags")
	}
}

func TestDetectRepoInfo_MavenProject(t *testing.T) {
	server := newRepoInfoMock(t, `{"Java":40000}`, map[string]bool{
		"pom.xml": true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)

	info, err := handler.detectRepoInfo("tok", "navikt", "legacy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.HasPomXML {
		t.Error("expected HasPomXML")
	}
}

// --- MCP tool handler tests ---

func newTestMCPHandler(githubClient *GitHubClient) *MCPHandler {
	ds := discovery.NewService("navikt", "copilot", "main", "http://localhost:8080")
	if err := ds.LoadManifest(); err != nil {
		panic("failed to load manifest: " + err.Error())
	}
	return NewMCPHandler(githubClient, ds)
}

func callTool(h *MCPHandler, toolName string, args map[string]interface{}, user *UserContext) *JSONRPCResponse {
	argsJSON, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolParams{Name: toolName, Arguments: args})
	_ = argsJSON

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  params,
	}

	return h.processRequest(req, user)
}

func TestCheckAgentReadiness_FullRepo(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/my-app/contents/.github/copilot-instructions.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
		"GET /repos/navikt/my-app/contents/.github/instructions": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]RepoContent{{Name: "test.instructions.md"}})
		},
		"GET /repos/navikt/my-app/contents/.github/agents": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]RepoContent{{Name: "review.agent.md"}})
		},
		"GET /repos/navikt/my-app/contents/.github/prompts": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/my-app/contents/.github/skills": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/my-app/contents/.github/workflows/copilot-setup-steps.yml": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/my-app/contents/.github/hooks/copilot-hooks.json": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/my-app/contents/AGENTS.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
		"GET /repos/navikt/my-app/languages": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Kotlin":50000}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 123, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "check_agent_readiness", map[string]interface{}{
		"owner": "navikt",
		"repo":  "my-app",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("expected CallToolResult")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "navikt/my-app") {
		t.Error("expected repo name in output")
	}
	if !strings.Contains(text, "Intermediate") {
		t.Errorf("expected Intermediate level (3 checks pass), got output:\n%s", text)
	}
	if !strings.Contains(text, "✅ .github/copilot-instructions.md") {
		t.Error("expected checkmark for copilot-instructions.md")
	}
	if !strings.Contains(text, "❌ .github/prompts/") {
		t.Error("expected X for missing prompts dir")
	}
	if !strings.Contains(text, "Kotlin") {
		t.Error("expected Kotlin language in output")
	}
}

func TestCheckAgentReadiness_EmptyRepo(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/empty-repo/contents/.github/copilot-instructions.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/contents/.github/instructions": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/contents/.github/agents": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/contents/.github/prompts": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/contents/.github/skills": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/contents/.github/workflows/copilot-setup-steps.yml": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/contents/.github/hooks/copilot-hooks.json": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/contents/AGENTS.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/empty-repo/languages": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 123, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "check_agent_readiness", map[string]interface{}{
		"owner": "navikt",
		"repo":  "empty-repo",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result := resp.Result.(CallToolResult)
	text := result.Content[0].Text

	if !strings.Contains(text, "None") {
		t.Error("expected None level for empty repo")
	}
	if !strings.Contains(text, "0/8") {
		t.Error("expected 0/8 score")
	}
	if !strings.Contains(text, "copilot-instructions.md") {
		t.Error("expected recommendation about copilot-instructions.md")
	}
}

func TestCheckAgentReadiness_MissingParams(t *testing.T) {
	client := newTestGitHubClient("http://unused")
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 123, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "check_agent_readiness", map[string]interface{}{
		"owner": "navikt",
	}, user)

	if resp.Error == nil {
		t.Fatal("expected error for missing repo param")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestSuggestCustomizations_KotlinRepo(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/kotlin-app/languages": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Kotlin":80000,"Dockerfile":500}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 123, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "suggest_customizations", map[string]interface{}{
		"owner": "navikt",
		"repo":  "kotlin-app",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result := resp.Result.(CallToolResult)
	text := result.Content[0].Text

	if !strings.Contains(text, "navikt/kotlin-app") {
		t.Error("expected repo name in output")
	}
	if !strings.Contains(text, "Kotlin") {
		t.Error("expected Kotlin language in output")
	}
}

func TestSuggestCustomizations_MissingParams(t *testing.T) {
	client := newTestGitHubClient("http://unused")
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 123, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "suggest_customizations", map[string]interface{}{}, user)

	if resp.Error == nil {
		t.Fatal("expected error for missing params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestToolsList_IncludesReadinessTools(t *testing.T) {
	client := newTestGitHubClient("http://unused")
	handler := newTestMCPHandler(client)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	}

	resp := handler.processRequest(req, &UserContext{Login: "test"})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(ListToolsResult)
	if !ok {
		t.Fatal("expected ListToolsResult")
	}

	toolNames := map[string]bool{}
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range []string{"check_agent_readiness", "suggest_customizations"} {
		if !toolNames[expected] {
			t.Errorf("expected tool %q in tools/list response", expected)
		}
	}

	if len(result.Tools) != 16 {
		t.Errorf("expected 16 tools total, got %d", len(result.Tools))
	}
}

func TestInspectRepo_PassesAuthToken(t *testing.T) {
	var receivedTokens []string
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, r *http.Request) {
			receivedTokens = append(receivedTokens, r.Header.Get("Authorization"))
			if strings.Contains(r.URL.Path, "languages") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			} else if strings.Contains(r.URL.Path, "contents") {
				w.WriteHeader(http.StatusNotFound)
			}
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 123, GitHubAccessToken: "my-secret-token"}

	_ = callTool(handler, "check_agent_readiness", map[string]interface{}{
		"owner": "navikt",
		"repo":  "test-repo",
	}, user)

	for _, token := range receivedTokens {
		if token != "Bearer my-secret-token" {
			t.Errorf("expected 'Bearer my-secret-token', got %q", token)
		}
	}
	if len(receivedTokens) == 0 {
		t.Error("expected at least one API call to GitHub")
	}
}

func TestProcessRequest_UserContextPassedToTools(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "languages") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "hans", ID: 42, GitHubAccessToken: "token-123"}

	params, _ := json.Marshal(CallToolParams{
		Name:      "check_agent_readiness",
		Arguments: map[string]interface{}{"owner": "navikt", "repo": "test"},
	})
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  params,
	}

	resp := handler.processRequest(req, user)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	ctx := context.WithValue(context.Background(), userContextKey, user)
	userFromCtx := GetUserFromContext(ctx)
	if userFromCtx.Login != "hans" {
		t.Errorf("expected login 'hans', got %q", userFromCtx.Login)
	}
}

// --- Phase 2: generate_copilot_instructions & generate_setup_steps ---

func newRepoInfoMock(t *testing.T, langs string, files map[string]bool) *httptest.Server {
	t.Helper()
	return newGitHubMock(t, map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if strings.HasSuffix(path, "/languages") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(langs))
				return
			}

			if strings.Contains(path, "/contents/") {
				parts := strings.SplitN(path, "/contents/", 2)
				filePath := ""
				if len(parts) == 2 {
					filePath = parts[1]
				}
				if files[filePath] {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
				return
			}

			w.WriteHeader(http.StatusNotFound)
		},
	})
}

func TestGenerateAgentsMD_GoRepo(t *testing.T) {
	server := newRepoInfoMock(t, `{"Go":50000}`, map[string]bool{
		"go.mod":     true,
		"Dockerfile": true,
		".nais":      true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "tok"}

	resp := callTool(handler, "generate_agents_md", map[string]interface{}{
		"owner": "navikt", "repo": "my-api",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	text := resp.Result.(CallToolResult).Content[0].Text
	if !strings.Contains(text, "go build") {
		t.Error("expected Go build commands")
	}
	if !strings.Contains(text, "Nais") {
		t.Error("expected Nais deployment section")
	}
	if !strings.Contains(text, "AGENTS.md") {
		t.Error("expected save instruction in output")
	}
}

func TestGenerateAgentsMD_PnpmRepo(t *testing.T) {
	server := newRepoInfoMock(t, `{"TypeScript":80000}`, map[string]bool{
		"package.json":   true,
		"pnpm-lock.yaml": true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "tok"}

	resp := callTool(handler, "generate_agents_md", map[string]interface{}{
		"owner": "navikt", "repo": "frontend",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	text := resp.Result.(CallToolResult).Content[0].Text
	if !strings.Contains(text, "pnpm install") {
		t.Error("expected pnpm commands")
	}
}

func TestGenerateAgentsMD_MissingParams(t *testing.T) {
	client := newTestGitHubClient("http://unused")
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "tok"}

	resp := callTool(handler, "generate_agents_md", map[string]interface{}{}, user)
	if resp.Error == nil {
		t.Fatal("expected error for missing params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected -32602, got %d", resp.Error.Code)
	}
}

func TestGenerateSetupSteps_GoRepo(t *testing.T) {
	server := newRepoInfoMock(t, `{"Go":50000}`, map[string]bool{
		"go.mod": true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "tok"}

	resp := callTool(handler, "generate_setup_steps", map[string]interface{}{
		"owner": "navikt", "repo": "my-api",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	text := resp.Result.(CallToolResult).Content[0].Text
	if !strings.Contains(text, "setup-go") {
		t.Error("expected Go setup action")
	}
	if !strings.Contains(text, "go mod download") {
		t.Error("expected go mod download")
	}
	if !strings.Contains(text, "copilot-setup-steps.yml") {
		t.Error("expected save instruction in output")
	}
}

func TestGenerateSetupSteps_GradleRepo(t *testing.T) {
	server := newRepoInfoMock(t, `{"Kotlin":60000}`, map[string]bool{
		"build.gradle.kts": true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "tok"}

	resp := callTool(handler, "generate_setup_steps", map[string]interface{}{
		"owner": "navikt", "repo": "kotlin-svc",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	text := resp.Result.(CallToolResult).Content[0].Text
	if !strings.Contains(text, "setup-java") {
		t.Error("expected Java setup action")
	}
	if !strings.Contains(text, "gradlew") {
		t.Error("expected Gradle build")
	}
}

func TestGenerateSetupSteps_MissingParams(t *testing.T) {
	client := newTestGitHubClient("http://unused")
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "tok"}

	resp := callTool(handler, "generate_setup_steps", map[string]interface{}{"owner": "navikt"}, user)
	if resp.Error == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestToolsList_IncludesPhase2Tools(t *testing.T) {
	client := newTestGitHubClient("http://unused")
	handler := newTestMCPHandler(client)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	}

	resp := handler.processRequest(req, &UserContext{Login: "test"})
	result := resp.Result.(ListToolsResult)

	toolNames := map[string]bool{}
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range []string{"generate_agents_md", "generate_setup_steps"} {
		if !toolNames[expected] {
			t.Errorf("expected tool %q in tools/list", expected)
		}
	}

	if len(result.Tools) != 16 {
		t.Errorf("expected 16 tools total, got %d", len(result.Tools))
	}
}

func TestDetectPackageManager_Pnpm(t *testing.T) {
	server := newRepoInfoMock(t, `{"TypeScript":1}`, map[string]bool{
		"package.json":   true,
		"pnpm-lock.yaml": true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)

	info, err := handler.detectRepoInfo("tok", "navikt", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PackageManager != "pnpm" {
		t.Errorf("expected pnpm, got %q", info.PackageManager)
	}
}

func TestDetectPackageManager_Yarn(t *testing.T) {
	server := newRepoInfoMock(t, `{"TypeScript":1}`, map[string]bool{
		"package.json": true,
		"yarn.lock":    true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)

	info, err := handler.detectRepoInfo("tok", "navikt", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PackageManager != "yarn" {
		t.Errorf("expected yarn, got %q", info.PackageManager)
	}
}

func TestDetectPackageManager_NpmDefault(t *testing.T) {
	server := newRepoInfoMock(t, `{"JavaScript":1}`, map[string]bool{
		"package.json": true,
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)

	info, err := handler.detectRepoInfo("tok", "navikt", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PackageManager != "npm" {
		t.Errorf("expected npm, got %q", info.PackageManager)
	}
}

// --- ListTeamRepos tests ---

func TestListTeamRepos(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /orgs/navikt/teams/dagpenger/repos": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]GitHubRepo{
				{Name: "dp-inntekt", FullName: "navikt/dp-inntekt"},
				{Name: "dp-soknad", FullName: "navikt/dp-soknad"},
				{Name: "old-lib", FullName: "navikt/old-lib", Archived: true},
				{Name: "dp-fork", FullName: "navikt/dp-fork", Fork: true},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	repos, err := client.ListTeamRepos("test-token", "navikt", "dagpenger")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "dp-inntekt" {
		t.Errorf("expected dp-inntekt, got %s", repos[0].Name)
	}
	if repos[1].Name != "dp-soknad" {
		t.Errorf("expected dp-soknad, got %s", repos[1].Name)
	}
}

func TestListTeamRepos_NotFound(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /orgs/navikt/teams/nonexistent/repos": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	_, err := client.ListTeamRepos("test-token", "navikt", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent team")
	}
}

// --- SearchReposByPrefix tests ---

func TestSearchReposByPrefix(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /search/repositories": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query().Get("q")
			if !strings.Contains(q, "dp-") || !strings.Contains(q, "org:navikt") {
				t.Errorf("unexpected query: %s", q)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"name":"dp-inntekt","full_name":"navikt/dp-inntekt"},{"name":"dp-soknad","full_name":"navikt/dp-soknad"}]}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	repos, err := client.SearchReposByPrefix("test-token", "navikt", "dp-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "dp-inntekt" {
		t.Errorf("expected dp-inntekt, got %s", repos[0].Name)
	}
}

func TestSearchReposByPrefix_Empty(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /search/repositories": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	repos, err := client.SearchReposByPrefix("test-token", "navikt", "zzz-nonexistent-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

// --- team_readiness tool tests ---

func TestTeamReadiness_ByPrefix(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /search/repositories": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"name":"dp-inntekt","full_name":"navikt/dp-inntekt"},{"name":"dp-soknad","full_name":"navikt/dp-soknad"}]}`))
		},
		"GET /repos/navikt/dp-inntekt/contents/AGENTS.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"AGENTS.md"}`))
		},
		"GET /repos/navikt/dp-inntekt/contents/.github/copilot-instructions.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"copilot-instructions.md"}`))
		},
		"GET /repos/navikt/dp-inntekt/contents/.github/workflows/copilot-setup-steps.yml": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/dp-soknad/contents/AGENTS.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/dp-soknad/contents/.github/copilot-instructions.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"GET /repos/navikt/dp-soknad/contents/.github/workflows/copilot-setup-steps.yml": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "team_readiness", map[string]interface{}{
		"org":    "navikt",
		"prefix": "dp-",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("expected CallToolResult")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "dp-inntekt") {
		t.Error("expected dp-inntekt in output")
	}
	if !strings.Contains(text, "dp-soknad") {
		t.Error("expected dp-soknad in output")
	}
	if !strings.Contains(text, "2 repos") {
		t.Error("expected '2 repos' in output")
	}
	if !strings.Contains(text, "dp-*") {
		t.Error("expected 'dp-*' label in output")
	}
}

func TestTeamReadiness_ByTeam(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /orgs/navikt/teams/dagpenger/repos": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]GitHubRepo{
				{Name: "dp-inntekt", FullName: "navikt/dp-inntekt"},
			})
		},
		"GET /repos/navikt/dp-inntekt/contents/AGENTS.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"AGENTS.md"}`))
		},
		"GET /repos/navikt/dp-inntekt/contents/.github/copilot-instructions.md": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"copilot-instructions.md"}`))
		},
		"GET /repos/navikt/dp-inntekt/contents/.github/workflows/copilot-setup-steps.yml": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"copilot-setup-steps.yml"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server.URL)
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "team_readiness", map[string]interface{}{
		"org":  "navikt",
		"team": "dagpenger",
	}, user)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result := resp.Result.(CallToolResult)
	text := result.Content[0].Text
	if !strings.Contains(text, "🟢 Advanced") {
		t.Error("expected Advanced level for repo with all 3 files")
	}
	if !strings.Contains(text, "dagpenger") {
		t.Error("expected team name in output")
	}
}

func TestTeamReadiness_MissingParams(t *testing.T) {
	client := newTestGitHubClient("http://unused")
	handler := newTestMCPHandler(client)
	user := &UserContext{Login: "testuser", ID: 1, GitHubAccessToken: "test-token"}

	resp := callTool(handler, "team_readiness", map[string]interface{}{
		"org": "navikt",
	}, user)
	if resp.Error == nil {
		t.Fatal("expected error when neither team nor prefix provided")
	}
	if !strings.Contains(resp.Error.Message, "either team or prefix") {
		t.Errorf("expected helpful error message, got: %s", resp.Error.Message)
	}
}
