package provider

import (
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// TestOpenCode2LiveDispatchGate runs the local dispatch gate in a real
// opencode 2 session, through the v2 bridge, against the real gate
// (local.Guard). The scripted orchestrator writes a new file (the gate
// refuses: create-file), overwrites an existing one (the gate allows), sends a
// task to local-worker (the gate appends its check), then writes text with a
// shell call (the gate's reminder reaches the next request). The same session
// without the gate is the control: every assertion flips, so none passes on
// its own. Opt-in, as TestOpenCode2LiveBridge:
//
//	NAV_PILOT_OPENCODE2=<opencode 2> NAV_PILOT_CPLT=<cplt> go test ./internal/provider -run OpenCode2LiveDispatchGate -v
func TestOpenCode2LiveDispatchGate(t *testing.T) {
	oc, _ := liveOpenCode2(t)
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24\n", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev })
	// No other hook: the bridge is staged for the gate alone.
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge { return HookBridge{} }
	mustWrite(t, filepath.Join(openCodeConfigDir(), "opencode.json"), "{}")

	for _, gated := range []bool{true, false} {
		name := map[bool]string{true: "gated", false: "control-ungated"}[gated]
		t.Run(name, func(t *testing.T) {
			// Resolved: opencode reports /private/var where t.TempDir says /var.
			proj := liveDir(t)
			_ = exec.Command("git", "-C", proj, "init", "-q").Run()
			mustWrite(t, filepath.Join(proj, "exists.txt"), "old\n")
			llm := &fakeLLM{calls: []fakeCall{
				{"write", map[string]any{"path": "new.go", "content": "package x\n"}, ""},
				{"write", map[string]any{"path": "exists.txt", "content": "allowed\n"}, ""},
				{"subagent", map[string]any{"agent": "local-worker", "description": "x", "prompt": "Rename foo in exists.txt"}, ""},
				{"", nil, "worker done"}, // the worker's own session
				{"shell", map[string]any{"command": "echo after", "description": "x"}, "Looks good."},
			}}
			srv := newFakeLLMServer(t, llm)

			env := withOpenCodeConfigContent(nil, map[string]any{
				"agent": map[string]any{local.WorkerAgent: map[string]any{"mode": "subagent", "description": "local worker", "model": "fake/m"}},
				"provider": map[string]any{"fake": map[string]any{"npm": "@ai-sdk/openai-compatible", "name": "Fake",
					"options": map[string]any{"baseURL": srv + "/v1", "apiKey": "x"},
					"models":  map[string]any{"m": map[string]any{"name": "m", "tool_call": true}}}},
			})
			var guard *local.Guard
			if gated {
				var err error
				// The fake model's server stands in for the local one: the gate
				// refuses only while that is up.
				if guard, err = local.StartGuard(srv, local.Model{Model: "m"}); err != nil {
					t.Fatal(err)
				}
				defer guard.Close()
				guard.EnableDispatchGate(local.GateRules{Create: true, Multi: true, Root: proj})
				env = append(env, DispatchGateEnv+"="+guard.GateURL())
			}
			env, cpltArgs := applyOpenCodeHooks(domain.ResolvedConfig{}, env, nil)
			local := []string{srv}
			if guard != nil {
				cpltArgs = append(cpltArgs, "--pass-env", DispatchGateEnv)
				local = append(local, guard.GateURL())
			}
			if staged := strings.Contains(strings.Join(env, "\n"), `"plugins":["`); staged != gated {
				t.Fatalf("bridge staged = %v, want %v: %v", staged, gated, env)
			}
			args, env := openCodeV2Args(openCodeClientArgs([]string{"--agent", "build", "--auto", "--model", "fake/m", "--log-level", "WARN"}, []string{"run", "go"}, ""), env)
			out := runOpenCode2(t, oc, proj, args, env, cpltArgs, local...)
			t.Logf("opencode run:\n%s", tail(out, 1500))

			results := strings.Join(llm.toolResults(), "\n")
			users := strings.Join(llm.userMessages(), "\n")
			t.Logf("tool results:\n%s\nuser messages:\n%s", results, users)
			if guard != nil {
				t.Logf("gate counts: %v", guard.GateCounts())
			}
			_, err := os.Stat(filepath.Join(proj, "new.go"))
			checks := []struct {
				what string
				got  bool
			}{
				{"the new file was refused (create-file)", strings.Contains(results, "new files go to `local-worker` first") && os.IsNotExist(err)},
				{"the check was appended to the worker's result", strings.Contains(results, "before you accept this")},
				{"the reminder reached the model", strings.Contains(users, "no build or test command has run")},
			}
			for _, c := range checks {
				if c.got != gated {
					t.Errorf("%s: %v, want %v", c.what, c.got, gated)
				}
			}
			// Allowed with and without the gate: the edit of an existing file.
			if b, _ := os.ReadFile(filepath.Join(proj, "exists.txt")); string(b) != "allowed\n" {
				t.Errorf("the allowed write did not land: %q", b)
			}
		})
	}
}

func newFakeLLMServer(t *testing.T, llm *fakeLLM) string {
	t.Helper()
	srv := httptest.NewServer(llm)
	t.Cleanup(srv.Close)
	return srv.URL
}

// liveOpenCode2 returns the opencode 2 and cplt binaries the live tests run,
// or skips. They run only under cplt, as a launch does: outside it, opencode 2
// sends the session to the shared background service, which ignores the
// launch's environment. Keep both binaries out of /tmp, where cplt refuses
// to run them.
func liveOpenCode2(t *testing.T) (oc, cplt string) {
	t.Helper()
	oc, cplt = os.Getenv("NAV_PILOT_OPENCODE2"), os.Getenv("NAV_PILOT_CPLT")
	if oc == "" || cplt == "" {
		t.Skip("set NAV_PILOT_OPENCODE2 to an opencode 2 binary and NAV_PILOT_CPLT to a cplt binary")
	}
	return oc, cplt
}

// liveDir is a fresh directory for a live session, in the module rather than
// the temp dir: cplt does not let a session execute from /tmp or
// /var/folders, and the session runs nav-pilot from here.
func liveDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("../..", ".live-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	d, _ = filepath.Abs(d)
	return d
}

// runOpenCode2 runs opencode 2 in dir under cplt, with the cplt arguments the
// launch built, and localhost let through for each of local's URLs (the fake
// model, the gate).
func runOpenCode2(t *testing.T, oc, dir string, args, env, cpltArgs []string, local ...string) string {
	t.Helper()
	_, cplt := liveOpenCode2(t)
	argv := append([]string{"--yes", "--quiet", "--no-audit", "--agent", "opencode"}, cpltArgs...)
	for _, u := range local {
		p, _ := url.Parse(u)
		argv = append(argv, "--allow-localhost", p.Port())
	}
	cmd := exec.Command(cplt, append(append(argv, "--"), args...)...)
	// cplt finds opencode on PATH.
	env = append(env, "PATH="+filepath.Dir(oc)+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Dir = dir
	// opencode takes its directory from PWD, not from the process's cwd.
	cmd.Env = append(append(os.Environ(), "PWD="+dir), env...)
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
	if err != nil {
		t.Logf("opencode run: %v", err)
	}
	return string(out)
}
