package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/navikt/copilot/mcp-onboarding/internal/discovery"
	"github.com/navikt/copilot/mcp-onboarding/internal/readiness"
	"github.com/navikt/copilot/mcp-onboarding/internal/templates"
)

type MCPHandler struct {
	githubClient     *GitHubClient
	discoveryService *discovery.Service
}

func NewMCPHandler(githubClient *GitHubClient, discoveryService *discovery.Service) *MCPHandler {
	return &MCPHandler{
		githubClient:     githubClient,
		discoveryService: discoveryService,
	}
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

func (h *MCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")

	if r.Method == "GET" {
		h.handleSSE(w, r)
		return
	}

	if r.Method == "POST" {
		if accept == "text/event-stream" || accept == "application/json, text/event-stream" {
			h.handleStreamableHTTP(w, r)
		} else {
			h.handleJSONRPC(w, r)
		}
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *MCPHandler) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, nil, -32700, "Parse error", nil)
		return
	}

	user := GetUserFromContext(r.Context())
	response := h.processRequest(&req, user)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *MCPHandler) handleStreamableHTTP(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, nil, -32700, "Parse error", nil)
		return
	}

	user := GetUserFromContext(r.Context())
	response := h.processRequest(&req, user)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	data, _ := json.Marshal(response)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *MCPHandler) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	user := GetUserFromContext(ctx)

	slog.Info("SSE connection opened", "user", user.Login)

	var mu sync.Mutex
	sendEvent := func(event, data string) {
		mu.Lock()
		defer mu.Unlock()
		if event != "" {
			_, _ = fmt.Fprintf(w, "event: %s\n", event)
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	reader := bufio.NewReader(r.Body)
	messages := make(chan []byte)

	go func() {
		defer close(messages)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					slog.Error("error reading SSE message", "error", err)
				}
				return
			}
			if len(line) > 0 {
				messages <- line
			}
		}
	}()

	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("SSE connection closed", "user", user.Login)
			return
		case <-keepalive.C:
			sendEvent("", `{"type":"keepalive"}`)
		case msg, ok := <-messages:
			if !ok {
				return
			}
			var req JSONRPCRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			response := h.processRequest(&req, user)
			data, _ := json.Marshal(response)
			sendEvent("message", string(data))
		}
	}
}

func (h *MCPHandler) processRequest(req *JSONRPCRequest, user *UserContext) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(req)
	case "initialized":
		return nil
	case "tools/list":
		return h.handleListTools(req)
	case "tools/call":
		return h.handleCallTool(req, user)
	case "ping":
		return h.handlePing(req)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (h *MCPHandler) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "mcp-onboarding",
			Version: "2.0.0",
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (h *MCPHandler) handleListTools(req *JSONRPCRequest) *JSONRPCResponse {
	tools := []Tool{
		{
			Name:        "hello_world",
			Description: "Returns a friendly hello world greeting with the authenticated user's GitHub username",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			Name:        "greet",
			Description: "Returns a personalized greeting message",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "The name to greet"
					}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "whoami",
			Description: "Returns information about the authenticated GitHub user",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			Name:        "echo",
			Description: "Echoes back the provided message",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"message": {
						"type": "string",
						"description": "The message to echo back"
					}
				},
				"required": ["message"]
			}`),
		},
		{
			Name:        "get_time",
			Description: "Returns the current server time in various formats",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"format": {
						"type": "string",
						"description": "Time format: 'iso', 'unix', or 'human'",
						"enum": ["iso", "unix", "human"]
					}
				},
				"required": []
			}`),
		},
		{
			Name:        "search_customizations",
			Description: "Search NAV Copilot customizations (agents, instructions, prompts, skills) by query, type, and tags",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query to match against names, descriptions, and tags"
					},
					"type": {
						"type": "string",
						"description": "Filter by customization type",
						"enum": ["agent", "instruction", "prompt", "skill"]
					},
					"tags": {
						"type": "array",
						"description": "Filter by tags",
						"items": {"type": "string"}
					}
				},
				"required": []
			}`),
		},
		{
			Name:        "list_agents",
			Description: "List all NAV Copilot agents with their descriptions and use cases",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"category": {
						"type": "string",
						"description": "Filter by category (platform, security, backend, frontend)"
					}
				},
				"required": []
			}`),
		},
		{
			Name:        "list_instructions",
			Description: "List all NAV Copilot instructions with their descriptions",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			Name:        "list_prompts",
			Description: "List all NAV Copilot prompts with their descriptions",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			Name:        "list_skills",
			Description: "List all NAV Copilot skills with their descriptions",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			Name:        "get_installation_guide",
			Description: "Generate installation instructions for a specific customization",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"type": {
						"type": "string",
						"description": "Customization type",
						"enum": ["agent", "instruction", "prompt", "skill"]
					},
					"name": {
						"type": "string",
						"description": "Customization name (e.g., 'nais-agent', 'kotlin-ktor')"
					}
				},
				"required": ["type", "name"]
			}`),
		},
		{
			Name:        "check_agent_readiness",
			Description: "Assess how ready a GitHub repository is for Copilot agent mode. Checks agent customization files (copilot-instructions.md, scoped instructions, custom agents, prompts, skills, setup steps, hooks, AGENTS.md) AND verification infrastructure (CI/CD workflows, linter config, type checking, test config, Dependabot, README). Returns a readiness scorecard with prioritized recommendations.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner (e.g., 'navikt')"
					},
					"repo": {
						"type": "string",
						"description": "Repository name (e.g., 'my-app')"
					}
				},
				"required": ["owner", "repo"]
			}`),
		},
		{
			Name:        "suggest_customizations",
			Description: "Suggest NAV Copilot customizations (agents, instructions, prompts, skills) tailored to a repository's language and tech stack. Detects languages via GitHub API and maps them to relevant NAV-maintained customizations with one-click install links.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner (e.g., 'navikt')"
					},
					"repo": {
						"type": "string",
						"description": "Repository name (e.g., 'my-app')"
					}
				},
				"required": ["owner", "repo"]
			}`),
		},
		{
			Name:        "generate_agents_md",
			Description: "Generate a tailored AGENTS.md file for a repository. AGENTS.md is a cross-agent standard that works with Copilot, Claude, Codex, and other AI agents. Detects the repo's languages, build tools (package.json, go.mod, build.gradle.kts, pom.xml), and platform (Nais) to produce a ready-to-use file with build commands, code standards, and boundaries.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner (e.g., 'navikt')"
					},
					"repo": {
						"type": "string",
						"description": "Repository name (e.g., 'my-app')"
					}
				},
				"required": ["owner", "repo"]
			}`),
		},
		{
			Name:        "generate_setup_steps",
			Description: "Generate a .github/workflows/copilot-setup-steps.yml file to enable the GitHub Copilot coding agent. Detects the repo's languages and build tools to produce a workflow that installs the correct runtime, package manager, and dependencies.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner (e.g., 'navikt')"
					},
					"repo": {
						"type": "string",
						"description": "Repository name (e.g., 'my-app')"
					}
				},
				"required": ["owner", "repo"]
			}`),
		},
		{
			Name:        "team_readiness",
			Description: "Scan all repositories belonging to a team and produce an agent readiness summary. Identify the team by either its GitHub team slug (uses the teams API) or a repo name prefix (e.g., 'dp-' for dagpenger, 'tms-' for team min side). Returns a table showing which repos have AGENTS.md, copilot-instructions.md, and copilot-setup-steps.yml.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"org": {
						"type": "string",
						"description": "GitHub organization (e.g., 'navikt')"
					},
					"team": {
						"type": "string",
						"description": "GitHub team slug (e.g., 'dagpenger'). Mutually exclusive with prefix."
					},
					"prefix": {
						"type": "string",
						"description": "Repo name prefix to match (e.g., 'dp-'). Mutually exclusive with team."
					}
				},
				"required": ["org"]
			}`),
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListToolsResult{Tools: tools},
	}
}

func (h *MCPHandler) handleCallTool(req *JSONRPCRequest, user *UserContext) *JSONRPCResponse {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	slog.Info("tool called", "tool", params.Name, "user", user.Login)

	var result CallToolResult

	switch params.Name {
	case "hello_world":
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("Hello, World! 👋 Greetings from Nav MCP Hello World server. You are authenticated as @%s.", user.Login)},
			},
		}

	case "greet":
		name, _ := params.Arguments["name"].(string)
		if name == "" {
			name = user.Login
		}
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("Hello, %s! 🎉 Welcome to the Nav MCP Hello World server.", name)},
			},
		}

	case "whoami":
		info := fmt.Sprintf(`GitHub User Information:
- Username: @%s
- User ID: %d
- Authenticated: ✅

This information is from your GitHub OAuth session.`, user.Login, user.ID)
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: info},
			},
		}

	case "echo":
		message, _ := params.Arguments["message"].(string)
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("Echo: %s", message)},
			},
		}

	case "get_time":
		format, _ := params.Arguments["format"].(string)
		now := time.Now()
		var timeStr string
		switch format {
		case "unix":
			timeStr = fmt.Sprintf("%d", now.Unix())
		case "human":
			timeStr = now.Format("Monday, January 2, 2006 at 3:04 PM MST")
		default:
			timeStr = now.Format(time.RFC3339)
		}
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("Current server time: %s", timeStr)},
			},
		}

	case "search_customizations":
		query, _ := params.Arguments["query"].(string)
		customType, _ := params.Arguments["type"].(string)
		tagsRaw, _ := params.Arguments["tags"].([]interface{})

		var tags []string
		for _, t := range tagsRaw {
			if tagStr, ok := t.(string); ok {
				tags = append(tags, tagStr)
			}
		}

		results := h.discoveryService.Search(query, customType, tags)

		jsonBytes, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Internal error",
				},
			}
		}

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("Found %d customizations:\n\n```json\n%s\n```", len(results), string(jsonBytes))},
			},
		}

	case "list_agents":
		category, _ := params.Arguments["category"].(string)
		agents := h.discoveryService.ListByType(discovery.TypeAgent, category)

		jsonBytes, err := json.MarshalIndent(agents, "", "  ")
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Internal error",
				},
			}
		}

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("NAV Copilot Agents (%d total):\n\n```json\n%s\n```", len(agents), string(jsonBytes))},
			},
		}

	case "list_instructions":
		instructions := h.discoveryService.ListByType(discovery.TypeInstruction, "")

		jsonBytes, err := json.MarshalIndent(instructions, "", "  ")
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Internal error",
				},
			}
		}

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("NAV Copilot Instructions (%d total):\n\n```json\n%s\n```", len(instructions), string(jsonBytes))},
			},
		}

	case "list_prompts":
		prompts := h.discoveryService.ListByType(discovery.TypePrompt, "")

		jsonBytes, err := json.MarshalIndent(prompts, "", "  ")
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Internal error",
				},
			}
		}

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("NAV Copilot Prompts (%d total):\n\n```json\n%s\n```", len(prompts), string(jsonBytes))},
			},
		}

	case "list_skills":
		skills := h.discoveryService.ListByType(discovery.TypeSkill, "")

		jsonBytes, err := json.MarshalIndent(skills, "", "  ")
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Internal error",
				},
			}
		}

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("NAV Copilot Skills (%d total):\n\n```json\n%s\n```", len(skills), string(jsonBytes))},
			},
		}

	case "get_installation_guide":
		typeStr, _ := params.Arguments["type"].(string)
		name, _ := params.Arguments["name"].(string)

		var customType discovery.CustomizationType
		switch typeStr {
		case "agent":
			customType = discovery.TypeAgent
		case "instruction":
			customType = discovery.TypeInstruction
		case "prompt":
			customType = discovery.TypePrompt
		case "skill":
			customType = discovery.TypeSkill
		default:
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32602,
					Message: fmt.Sprintf("Invalid type: %s", typeStr),
				},
			}
		}

		guide, err := h.discoveryService.GenerateInstallationGuide(customType, name)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32602,
					Message: err.Error(),
				},
			}
		}

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: guide},
			},
		}

	case "check_agent_readiness":
		owner, _ := params.Arguments["owner"].(string)
		repo, _ := params.Arguments["repo"].(string)
		if owner == "" || repo == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32602, Message: "owner and repo are required"},
			}
		}

		contents, err := h.inspectRepo(user.GitHubAccessToken, owner, repo)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32603, Message: fmt.Sprintf("Failed to inspect repo: %v", err)},
			}
		}

		report := readiness.Assess(contents)
		report.Owner = owner
		report.Repo = repo
		report.Suggestions = readiness.SuggestCustomizations(contents, h.discoveryService.GetManifest())

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: readiness.FormatReport(report)},
			},
		}

	case "suggest_customizations":
		owner, _ := params.Arguments["owner"].(string)
		repo, _ := params.Arguments["repo"].(string)
		if owner == "" || repo == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32602, Message: "owner and repo are required"},
			}
		}

		langs, err := h.githubClient.GetRepoLanguages(user.GitHubAccessToken, owner, repo)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32603, Message: fmt.Sprintf("Failed to get repo languages: %v", err)},
			}
		}

		contents := &readiness.RepoContents{Languages: langs}
		h.detectRepoContentsStack(user.GitHubAccessToken, owner, repo, contents)
		suggestions := readiness.SuggestCustomizations(contents, h.discoveryService.GetManifest())

		var sb fmt.Stringer = &suggestionsFormatter{owner: owner, repo: repo, langs: langs, suggestions: suggestions}
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: sb.String()},
			},
		}

	case "generate_agents_md":
		owner, _ := params.Arguments["owner"].(string)
		repo, _ := params.Arguments["repo"].(string)
		if owner == "" || repo == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32602, Message: "owner and repo are required"},
			}
		}

		info, err := h.detectRepoInfo(user.GitHubAccessToken, owner, repo)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32603, Message: fmt.Sprintf("Failed to detect repo info: %v", err)},
			}
		}

		output := templates.GenerateAgentsMD(info)
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("Generated `AGENTS.md` for %s/%s:\n\n```markdown\n%s\n```\n\nSave this as `AGENTS.md` at the root of your repository. This works across Copilot, Claude, Codex, and other AI agents.", owner, repo, output)},
			},
		}

	case "generate_setup_steps":
		owner, _ := params.Arguments["owner"].(string)
		repo, _ := params.Arguments["repo"].(string)
		if owner == "" || repo == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32602, Message: "owner and repo are required"},
			}
		}

		info, err := h.detectRepoInfo(user.GitHubAccessToken, owner, repo)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32603, Message: fmt.Sprintf("Failed to detect repo info: %v", err)},
			}
		}

		output := templates.GenerateSetupSteps(info)
		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: fmt.Sprintf("Generated `copilot-setup-steps.yml` for %s/%s:\n\n```yaml\n%s\n```\n\nSave this as `.github/workflows/copilot-setup-steps.yml` in your repository.", owner, repo, output)},
			},
		}

	case "team_readiness":
		org, _ := params.Arguments["org"].(string)
		team, _ := params.Arguments["team"].(string)
		prefix, _ := params.Arguments["prefix"].(string)
		if org == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32602, Message: "org is required"},
			}
		}
		if team == "" && prefix == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32602, Message: "either team or prefix is required"},
			}
		}

		var repos []GitHubRepo
		var err error
		if team != "" {
			repos, err = h.githubClient.ListTeamRepos(user.GitHubAccessToken, org, team)
		} else {
			repos, err = h.githubClient.SearchReposByPrefix(user.GitHubAccessToken, org, prefix)
		}
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32603, Message: fmt.Sprintf("Failed to list repos: %v", err)},
			}
		}

		label := team
		if label == "" {
			label = prefix + "*"
		}

		summary := &readiness.TeamSummary{
			Org:  org,
			Team: label,
		}

		for _, r := range repos {
			agentsMD, _ := h.githubClient.GetRepoFile(user.GitHubAccessToken, org, r.Name, "AGENTS.md")
			copilotMD, _ := h.githubClient.GetRepoFile(user.GitHubAccessToken, org, r.Name, ".github/copilot-instructions.md")
			setupSteps, _ := h.githubClient.GetRepoFile(user.GitHubAccessToken, org, r.Name, ".github/workflows/copilot-setup-steps.yml")

			summary.Repos = append(summary.Repos, readiness.RepoReadiness{
				Repo:       r.Name,
				AgentsMD:   agentsMD,
				CopilotMD:  copilotMD,
				SetupSteps: setupSteps,
				Level:      readiness.AssessRepoLight(agentsMD, copilotMD, setupSteps),
			})
		}
		summary.Total = len(summary.Repos)

		result = CallToolResult{
			Content: []TextContent{
				{Type: "text", Text: readiness.FormatTeamSummary(summary)},
			},
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			},
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (h *MCPHandler) inspectRepo(accessToken, owner, repo string) (*readiness.RepoContents, error) {
	contents := &readiness.RepoContents{}

	var err error
	contents.CopilotInstructions, err = h.githubClient.GetRepoFile(accessToken, owner, repo, ".github/copilot-instructions.md")
	if err != nil {
		return nil, fmt.Errorf("checking copilot-instructions.md: %w", err)
	}

	contents.InstructionsCount, err = h.githubClient.GetDirectoryCount(accessToken, owner, repo, ".github/instructions")
	if err != nil {
		return nil, fmt.Errorf("checking instructions dir: %w", err)
	}

	contents.AgentsCount, err = h.githubClient.GetDirectoryCount(accessToken, owner, repo, ".github/agents")
	if err != nil {
		return nil, fmt.Errorf("checking agents dir: %w", err)
	}

	contents.PromptsCount, err = h.githubClient.GetDirectoryCount(accessToken, owner, repo, ".github/prompts")
	if err != nil {
		return nil, fmt.Errorf("checking prompts dir: %w", err)
	}

	contents.SkillsCount, err = h.githubClient.GetDirectoryCount(accessToken, owner, repo, ".github/skills")
	if err != nil {
		return nil, fmt.Errorf("checking skills dir: %w", err)
	}

	contents.SetupSteps, err = h.githubClient.GetRepoFile(accessToken, owner, repo, ".github/workflows/copilot-setup-steps.yml")
	if err != nil {
		return nil, fmt.Errorf("checking copilot-setup-steps.yml: %w", err)
	}

	contents.HooksConfig, err = h.githubClient.GetRepoFile(accessToken, owner, repo, ".github/hooks/copilot-hooks.json")
	if err != nil {
		return nil, fmt.Errorf("checking hooks config: %w", err)
	}

	contents.AgentsMD, err = h.githubClient.GetRepoFile(accessToken, owner, repo, "AGENTS.md")
	if err != nil {
		return nil, fmt.Errorf("checking AGENTS.md: %w", err)
	}

	contents.HasReadme, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "README.md")
	contents.HasDependabot, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, ".github/dependabot.yml")
	if !contents.HasDependabot {
		contents.HasDependabot, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, ".github/dependabot.yaml")
	}
	workflowCount, _ := h.githubClient.GetDirectoryCount(accessToken, owner, repo, ".github/workflows")
	contents.HasCIWorkflows = workflowCount > 0

	contents.Languages, err = h.githubClient.GetRepoLanguages(accessToken, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("getting languages: %w", err)
	}

	h.detectRepoContentsStack(accessToken, owner, repo, contents)
	h.detectVerificationInfra(accessToken, owner, repo, contents)

	return contents, nil
}

func (h *MCPHandler) detectRepoContentsStack(accessToken, owner, repo string, contents *readiness.RepoContents) {
	contents.HasPackageJSON, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "package.json")
	contents.HasGradleKts, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "build.gradle.kts")
	contents.HasPomXML, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "pom.xml")

	if contents.HasPackageJSON {
		contents.HasNextConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "next.config.ts")
		if !contents.HasNextConfig {
			contents.HasNextConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "next.config.js")
		}
		if !contents.HasNextConfig {
			contents.HasNextConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "next.config.mjs")
		}
	}

	if contents.HasGradleKts || contents.HasPomXML {
		contents.HasAppYml, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "src/main/resources/application.yml")
		if !contents.HasAppYml {
			contents.HasAppYml, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "src/main/resources/application.properties")
		}
		if !contents.HasAppYml {
			contents.HasAppYml, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "src/main/resources/application.yaml")
		}
	}
}

func (h *MCPHandler) detectVerificationInfra(accessToken, owner, repo string, contents *readiness.RepoContents) {
	langs := make(map[string]bool, len(contents.Languages))
	for _, l := range contents.Languages {
		langs[l] = true
	}

	if langs["TypeScript"] || langs["JavaScript"] {
		contents.HasLinterConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "eslint.config.mjs")
		if !contents.HasLinterConfig {
			contents.HasLinterConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "eslint.config.js")
		}
		if !contents.HasLinterConfig {
			contents.HasLinterConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, ".eslintrc.json")
		}
	}
	if !contents.HasLinterConfig && langs["Go"] {
		contents.HasLinterConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, ".golangci.yml")
		if !contents.HasLinterConfig {
			contents.HasLinterConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, ".golangci.yaml")
		}
	}
	if !contents.HasLinterConfig && (langs["Kotlin"] || langs["Java"]) {
		contents.HasLinterConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "detekt.yml")
		if !contents.HasLinterConfig {
			contents.HasLinterConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "config/detekt/detekt.yml")
		}
	}

	if langs["TypeScript"] || langs["JavaScript"] {
		contents.HasTypeChecking, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "tsconfig.json")
	} else if langs["Go"] || langs["Kotlin"] || langs["Java"] || langs["Rust"] {
		contents.HasTypeChecking = true
	}

	if langs["TypeScript"] || langs["JavaScript"] {
		contents.HasTestConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "jest.config.js")
		if !contents.HasTestConfig {
			contents.HasTestConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "jest.config.ts")
		}
		if !contents.HasTestConfig {
			contents.HasTestConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "vitest.config.ts")
		}
		if !contents.HasTestConfig {
			contents.HasTestConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "vitest.config.js")
		}
	}
	if !contents.HasTestConfig && (contents.HasGradleKts || contents.HasPomXML) {
		contents.HasTestConfig = true
	}
	if !contents.HasTestConfig && langs["Go"] {
		contents.HasTestConfig = true
	}
}

func (h *MCPHandler) detectRepoInfo(accessToken, owner, repo string) (*templates.RepoInfo, error) {
	info := &templates.RepoInfo{Owner: owner, Repo: repo}

	var err error
	info.Languages, err = h.githubClient.GetRepoLanguages(accessToken, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("getting languages: %w", err)
	}

	info.HasPackageJSON, err = h.githubClient.GetRepoFile(accessToken, owner, repo, "package.json")
	if err != nil {
		return nil, fmt.Errorf("checking package.json: %w", err)
	}

	if info.HasPackageJSON {
		info.PackageManager = detectPackageManager(h.githubClient, accessToken, owner, repo)
	}

	info.HasGoMod, err = h.githubClient.GetRepoFile(accessToken, owner, repo, "go.mod")
	if err != nil {
		return nil, fmt.Errorf("checking go.mod: %w", err)
	}

	info.HasGradleKts, err = h.githubClient.GetRepoFile(accessToken, owner, repo, "build.gradle.kts")
	if err != nil {
		return nil, fmt.Errorf("checking build.gradle.kts: %w", err)
	}

	info.HasPomXML, err = h.githubClient.GetRepoFile(accessToken, owner, repo, "pom.xml")
	if err != nil {
		return nil, fmt.Errorf("checking pom.xml: %w", err)
	}

	info.HasDockerfile, err = h.githubClient.GetRepoFile(accessToken, owner, repo, "Dockerfile")
	if err != nil {
		return nil, fmt.Errorf("checking Dockerfile: %w", err)
	}

	info.HasNais, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, ".nais")

	// Stack-specific detection
	if info.HasPackageJSON {
		info.HasNextConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "next.config.ts")
		if !info.HasNextConfig {
			info.HasNextConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "next.config.js")
		}
		if !info.HasNextConfig {
			info.HasNextConfig, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "next.config.mjs")
		}
	}

	if info.HasGradleKts || info.HasPomXML {
		info.HasAppYml, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "src/main/resources/application.yml")
		if !info.HasAppYml {
			info.HasAppYml, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "src/main/resources/application.properties")
		}
		if !info.HasAppYml {
			info.HasAppYml, _ = h.githubClient.GetRepoFile(accessToken, owner, repo, "src/main/resources/application.yaml")
		}
	}

	return info, nil
}

func detectPackageManager(client *GitHubClient, accessToken, owner, repo string) string {
	if exists, _ := client.GetRepoFile(accessToken, owner, repo, "pnpm-lock.yaml"); exists {
		return "pnpm"
	}
	if exists, _ := client.GetRepoFile(accessToken, owner, repo, "yarn.lock"); exists {
		return "yarn"
	}
	return "npm"
}

type suggestionsFormatter struct {
	owner       string
	repo        string
	langs       []string
	suggestions []readiness.Suggestion
}

func (f *suggestionsFormatter) String() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Suggested Customizations for %s/%s\n\n", f.owner, f.repo)

	if len(f.langs) > 0 {
		fmt.Fprintf(&sb, "**Detected languages**: %s\n\n", strings.Join(f.langs, ", "))
	}

	if len(f.suggestions) == 0 {
		sb.WriteString("No specific customization suggestions for this repository's tech stack.\n")
		return sb.String()
	}

	for i, s := range f.suggestions {
		fmt.Fprintf(&sb, "## %d. %s (%s)\n\n", i+1, s.Name, s.Type)
		fmt.Fprintf(&sb, "%s\n\n", s.Reason)
		fmt.Fprintf(&sb, "**Install**: %s\n\n", s.InstallURL)
	}

	return sb.String()
}

func (h *MCPHandler) handlePing(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]string{},
	}
}

func (h *MCPHandler) writeError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}

type contextKey string

const userContextKey contextKey = "user"

type UserContext struct {
	Login             string
	ID                int64
	GitHubAccessToken string
}

func GetUserFromContext(ctx context.Context) *UserContext {
	user, _ := ctx.Value(userContextKey).(*UserContext)
	if user == nil {
		return &UserContext{Login: "anonymous", ID: 0}
	}
	return user
}
