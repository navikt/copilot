package provider

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A pre hook that cannot answer in time denies the call when it is marked
// failClosed, and allows it otherwise, in both bridges. The hook sleeps past
// its one-second timeout, so the bridge's own timer fires and run() returns
// null.
func TestBridgePreHookFailClosed(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node")
	}
	// Each case prints the error the hook threw, or "allowed".
	drivers := map[string]string{
		"hooks-bridge.js": `import { NavPilotHooks } from "./bridge.mjs"
const h = await NavPilotHooks({ directory: process.cwd(), worktree: process.cwd() })
try { await h["tool.execute.before"]({ tool: "shell", sessionID: "s" }, { args: { command: "x" } }); console.log("allowed") } catch (e) { console.log(e.message) }`,
		"hooks-bridge-v2.js": `import plugin from "./bridge.mjs"
const hooks = {}
await plugin.setup({ location: { directory: process.cwd() }, session: { hook: async () => {} }, tool: { hook: async (n, f) => (hooks[n] = f) } })
try { await hooks["execute.before"]({ tool: "shell", sessionID: "s", input: { command: "x" } }); console.log("allowed") } catch (e) { console.log(e.message) }`,
	}
	for file, driver := range drivers {
		for _, failClosed := range []bool{true, false} {
			dir := t.TempDir()
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bridge.mjs"), src, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "driver.mjs"), []byte(driver), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, _ := json.Marshal(HookBridge{Pre: []BridgeHook{{Name: "treg", Command: "sleep 5", Matcher: ".*", Timeout: 1, FailClosed: failClosed}}})
			cmd := exec.Command(node, "driver.mjs")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "NAV_PILOT_OPENCODE_HOOKS="+string(cfg), "NAV_PILOT_DISPATCH_GATE=")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v\n%s", file, err, out)
			}
			want := "allowed"
			if failClosed {
				want = "treg svarte ikke innen fristen, så kallet er stoppet"
			}
			if got := strings.TrimSpace(string(out)); got != want {
				t.Errorf("%s failClosed=%v: got %q, want %q", file, failClosed, got, want)
			}
		}
	}
}
