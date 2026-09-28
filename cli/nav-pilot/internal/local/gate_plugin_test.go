package local

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The plugin's side of the create-file retry, driven through
// NavPilotDispatchGate itself against the real route: a worker's new file,
// the task's return with its session, and a failing check's exit code must
// reach the gate in the shape opencode 1.18.20 hands the hooks (#1156).
func TestGatePluginDrivesTheCreateRetry(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("needs node to load the opencode plugin")
	}
	// Before testGate, which changes directory.
	plugin, err := filepath.Abs(filepath.Join("..", "provider", "dispatch-gate.js"))
	if err != nil {
		t.Fatal(err)
	}
	g := testGate(t, true, multi)
	srv := httptest.NewServer(http.HandlerFunc(g.serve))
	defer srv.Close()
	script := `
const { NavPilotDispatchGate } = await import(process.argv[2])
const client = { session: { get: async ({ path }) => ({ data: { parentID: path.id === "w1" ? "s" : undefined } }), prompt: async () => {} } }
const h = await NavPilotDispatchGate({ client, directory: process.argv[3] })
await h["chat.message"]({ sessionID: "s", agent: "nav-pilot" }, { parts: [] })
await h["chat.message"]({ sessionID: "w1", agent: "local-worker" }, { parts: [] })
await h["tool.execute.before"]({ sessionID: "w1", tool: "write" }, { args: { filePath: "src/New.kt" } })
const task = { output: "done", metadata: { sessionId: "w1" } }
await h["tool.execute.after"]({ sessionID: "s", tool: "task", args: { subagent_type: "local-worker" } }, task)
await h["tool.execute.before"]({ sessionID: "s", tool: "bash" }, { args: { command: "npm test" } })
const check = { output: "1 failing", metadata: { exit: 1 } }
await h["tool.execute.after"]({ sessionID: "s", tool: "bash", args: { command: "npm test" } }, check)
console.log(check.output)
`
	f := filepath.Join(t.TempDir(), "drive.mjs")
	if err := os.WriteFile(f, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, f, "file://"+plugin, root)
	cmd.Env = append(os.Environ(), "NAV_PILOT_DISPATCH_GATE="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), GateRetryText) {
		t.Fatalf("the failing check did not get the retry text:\n%s", out)
	}
	if n := g.snapshot()["create_retry"]; n != 1 {
		t.Errorf("create_retry = %d, want 1", n)
	}
}
