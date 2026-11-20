package provider

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The post-hook payload names the MCP server a tool belongs to only when the
// server is listed in the registry and the tool name matches its prefix; the
// redaction hook trusts that name to let Dolly's synthetic idents through.
func TestBridgePostHookMCPServer(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node")
	}
	drivers := map[string]string{
		"hooks-bridge.js": `import { NavPilotHooks } from "./bridge.mjs"
const h = await NavPilotHooks({ directory: process.cwd(), worktree: process.cwd() })
await h["tool.execute.after"]({ tool: process.argv[2], sessionID: "s", args: {} }, { output: "x" })`,
		"hooks-bridge-v2.js": `import plugin from "./bridge.mjs"
const hooks = {}
await plugin.setup({ location: { directory: process.cwd() }, session: { hook: async () => {} }, tool: { hook: async (n, f) => (hooks[n] = f) } })
await hooks["execute.after"]({ tool: process.argv[2], sessionID: "s", id: "c1", status: "ok", input: {}, result: { content: [{ type: "text", text: "x" }] } })`,
	}
	const dolly = "io.github.navikt/dolly"
	env := `{"listed":["io.github.navikt/dolly","io.github.navikt/dolly_x","other"],"blocked":["io.github.navikt/dolly_evil"]}`
	tests := []struct{ tool, want string }{
		{"io_github_navikt_dolly_search", dolly},
		{"x_io_github_navikt_dolly_search", ""},                         // name merely contains it
		{"io_github_navikt_dolly-evil_search", ""},                      // different server
		{"io_github_navikt_dolly_x_search", "io.github.navikt/dolly_x"}, // longest prefix wins
		{"io_github_navikt_dolly_evil_search", ""},                      // blocked server
		{"bash", ""},
	}
	for file, driver := range drivers {
		for _, tt := range tests {
			dir := t.TempDir()
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for name, b := range map[string][]byte{"bridge.mjs": src, "driver.mjs": []byte(driver)} {
				if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cfg, _ := json.Marshal(HookBridge{Post: []BridgeHook{{Name: "cap", Argv: []string{"/bin/sh", "-c", "cat > payload.json; echo '{}'"}, Timeout: 3}}})
			cmd := exec.Command(node, "driver.mjs", tt.tool)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "NAV_PILOT_OPENCODE_HOOKS="+string(cfg), MCPBlockedEnv+"="+env,
				"NAV_PILOT_DISPATCH_GATE=", "NAV_PILOT_HOOK_STATE_DIR="+dir)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s %s: %v\n%s", file, tt.tool, err, out)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "payload.json"))
			if err != nil {
				t.Fatalf("%s %s: hook not run: %v", file, tt.tool, err)
			}
			var p struct {
				MCPServer string `json:"mcpServer"`
			}
			if err := json.Unmarshal(raw, &p); err != nil {
				t.Fatal(err)
			}
			if p.MCPServer != tt.want {
				t.Errorf("%s %q: mcpServer %q, want %q", file, tt.tool, p.MCPServer, strings.TrimSpace(tt.want))
			}
		}
	}
}
