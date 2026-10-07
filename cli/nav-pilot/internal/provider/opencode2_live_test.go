package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// TestOpenCode2LiveBridge runs a real opencode 2 session, outside cplt, with
// the plugin and environment a launch stages for it, and real `nav-pilot hook`
// commands behind it. opencode 2 cannot start under cplt yet (navikt/cplt#710),
// and nav-pilot refuses to launch it (CheckOpenCodeMajor), so this is the only
// end-to-end proof the v2 bridge has. Opt-in: set NAV_PILOT_OPENCODE2 to an
// opencode 2 binary; it needs node for the MCP server.
//
//	NAV_PILOT_OPENCODE2=$(which opencode) go test ./internal/provider -run OpenCode2Live -v
func TestOpenCode2LiveBridge(t *testing.T) {
	oc := os.Getenv("NAV_PILOT_OPENCODE2")
	if oc == "" {
		t.Skip("set NAV_PILOT_OPENCODE2 to an opencode 2 binary")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("needs node for the MCP server")
	}
	work := t.TempDir()
	navPilot := filepath.Join(work, "nav-pilot")
	if out, err := exec.Command("go", "build", "-o", navPilot, "../..").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	mcp := filepath.Join(work, "mcp.mjs")
	gate := filepath.Join(work, "gate.sh")
	mustWrite(t, mcp, mcpServerJS)
	mustWrite(t, gate, `grep -q forbidden && echo '{"permissionDecision":"deny","permissionDecisionReason":"vakt says no"}'; true`)

	// Built at run time, so no token-shaped string sits in the repository.
	token := strings.Join([]string{"ghp", "0123456789abcdefghijABCDEFGHIJ012345"}, "_")
	llm := &fakeLLM{calls: []fakeCall{
		{"shell", map[string]any{"command": "echo forbidden", "description": "x"}},
		{"shell", map[string]any{"command": "echo token=" + token, "description": "x"}},
		{"probe_ping", map[string]any{}},
		{"reenabled_ping", map[string]any{}},
	}}
	srv := httptest.NewServer(llm)
	defer srv.Close()

	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24\n", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev })
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge {
		return HookBridge{Bin: navPilot,
			Post: []BridgeHook{{Name: "nav-pilot-redact-tool-output", Argv: []string{navPilot, "hook", "redact",
				"hook_redact_secrets=true", "hook_redact_fnr=true", "hook_injection_note=false"}, Timeout: 10, FailClosed: true}},
			Pre: []BridgeHook{{Name: "vakt", Command: "/bin/sh " + gate, Matcher: "bash", Timeout: 5}}}
	}
	// The MCP server sits in the user's config in opencode 2's own shape, and
	// the registry does not list it. Not the project's: opencode 2.0.24 did
	// not start a project's MCP server in `run --standalone` in any shape.
	proj := filepath.Join(work, "proj")
	started, restarted := filepath.Join(work, "probe-started"), filepath.Join(work, "reenabled-started")
	_ = os.MkdirAll(proj, 0o755)
	_ = exec.Command("git", "-C", proj, "init", "-q").Run()
	pcfg, _ := json.Marshal(map[string]any{"mcp": map[string]any{"servers": map[string]any{
		"probe": map[string]any{"type": "local", "command": []string{node, mcp, started}, "codemode": false}}}})
	mustWrite(t, filepath.Join(openCodeConfigDir(), "opencode.json"), string(pcfg))
	origPolicy, origReg := fetchMCPPolicy, fetchMCPRegistry
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry = origPolicy, origReg })
	fetchMCPPolicy = func() (string, error) { return "https://registry/", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) { return mcpRegistry{}, nil }
	env := applyOpenCodeMCPPolicy(nil, proj)
	// A blocked server that is running anyway, as after /mcp turns it back
	// on: the bridge refuses its tools.
	env = append(env, MCPBlockedEnv+`={"blocked":["probe","reenabled"],"listed":[]}`)
	env = withOpenCodeConfigContent(env, map[string]any{
		"mcp": map[string]any{"reenabled": map[string]any{"type": "local", "command": []string{node, mcp, restarted}, "codemode": false}},
		"provider": map[string]any{"fake": map[string]any{"npm": "@ai-sdk/openai-compatible", "name": "Fake",
			"options": map[string]any{"baseURL": srv.URL + "/v1", "apiKey": "x"},
			"models":  map[string]any{"m": map[string]any{"name": "m", "tool_call": true}}}},
	})
	env, _ = applyOpenCodeHooks(domain.ResolvedConfig{}, env, nil)
	if !strings.Contains(strings.Join(env, "\n"), `"plugins":["`) {
		t.Fatalf("no plugins dir staged: %v", env)
	}

	// The arguments a `nav-pilot -- run go` launch builds, rewritten for v2.
	args, env := openCodeV2Args(openCodeClientArgs([]string{"--agent", "build", "--auto", "--model", "fake/m", "--log-level", "WARN"}, []string{"run", "go"}, ""), env)
	t.Logf("opencode %s", strings.Join(args, " "))
	cmd := exec.Command(oc, args...)
	cmd.Dir = proj
	cmd.Env = append(os.Environ(), env...)
	done := make(chan struct{})
	time.AfterFunc(150*time.Second, func() {
		select {
		case <-done:
		default:
			_ = cmd.Process.Kill()
		}
	})
	out, err := cmd.CombinedOutput()
	close(done)
	t.Logf("opencode run: %v\n%s", err, tail(string(out), 2000))

	read := llm.toolResults()
	t.Logf("tool results the model read:\n%s", strings.Join(read, "\n"))
	all := strings.Join(read, "\n")
	for _, want := range []string{"vakt says no", "[REDACTED:github-token]", "not in Nav's MCP registry"} {
		if !strings.Contains(all, want) {
			t.Errorf("the model never read %q", want)
		}
	}
	if strings.Contains(all, token) {
		t.Error("the model read the raw token")
	}
	if _, err := os.Stat(started); err == nil {
		t.Error("the MCP server the registry does not list was started")
	}
	// The control: a server not turned off does start, so the check above can fail.
	if _, err := os.Stat(restarted); err != nil {
		t.Error("the re-enabled MCP server never started, so its block proves nothing")
	}

	if strings.Contains(all, "PONG-FROM-MCP") {
		t.Error("the blocked MCP server's tool ran")
	}
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

const mcpServerJS = `import readline from "node:readline"
import fs from "node:fs"
fs.writeFileSync(process.argv[2], "started")
const rl = readline.createInterface({ input: process.stdin })
const send = (o) => process.stdout.write(JSON.stringify(o) + "\n")
rl.on("line", (l) => {
  let m; try { m = JSON.parse(l) } catch { return }
  if (m.id === undefined) return
  if (m.method === "initialize") return send({ jsonrpc: "2.0", id: m.id, result: { protocolVersion: m.params?.protocolVersion ?? "2024-11-05", capabilities: { tools: {} }, serverInfo: { name: "probe", version: "1" } } })
  if (m.method === "tools/list") return send({ jsonrpc: "2.0", id: m.id, result: { tools: [{ name: "ping", description: "ping", inputSchema: { type: "object", properties: {} } }] } })
  if (m.method === "tools/call") return send({ jsonrpc: "2.0", id: m.id, result: { content: [{ type: "text", text: "PONG-FROM-MCP" }] } })
  send({ jsonrpc: "2.0", id: m.id, result: {} })
})
`

type fakeCall struct {
	name string
	args map[string]any
}

// fakeLLM is an OpenAI chat-completions server: each request that offers
// tools gets the next scripted tool call, then plain text. It keeps the tool
// results every request carried, which is what the model read.
type fakeLLM struct {
	mu      sync.Mutex
	calls   []fakeCall
	n       int
	results []string
}

func (f *fakeLLM) toolResults() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.results
}

func (f *fakeLLM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.URL.Path, "chat/completions") {
		_, _ = io.WriteString(w, `{"data":[]}`)
		return
	}
	var body struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
		Tools []any `json:"tools"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	var call *fakeCall
	if len(body.Tools) > 0 {
		for _, m := range body.Messages {
			if m.Role == "tool" {
				b, _ := json.Marshal(m.Content)
				if s := string(b); !contains(f.results, s) {
					f.results = append(f.results, s)
				}
			}
		}
		if f.n < len(f.calls) {
			call = &f.calls[f.n]
			f.n++
		}
	}
	n := f.n
	f.mu.Unlock()

	w.Header().Set("content-type", "text/event-stream")
	send := func(delta map[string]any, finish any) {
		b, _ := json.Marshal(map[string]any{"id": "x", "object": "chat.completion.chunk", "created": 1, "model": "fake",
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	if call != nil {
		args, _ := json.Marshal(call.args)
		send(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call_%d", n),
			"type": "function", "function": map[string]any{"name": call.name, "arguments": string(args)}}}}, nil)
		send(map[string]any{}, "tool_calls")
	} else {
		send(map[string]any{"role": "assistant", "content": "DONE"}, nil)
		send(map[string]any{}, "stop")
	}
	fmt.Fprint(w, `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"fake","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`+"\n\ndata: [DONE]\n\n")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
