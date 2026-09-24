package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

const loopPayload = `{"sessionId":"s1","toolName":"bash","toolArgs":{"command":"gh run view 1"},"toolResult":{"resultType":"success","textResultForLlm":"queued"}}`

// runLoopGuard feeds the same payload to `nav-pilot hook loop-guard` n times
// and returns the last answer.
func runLoopGuard(t *testing.T, n int, payload string) string {
	t.Helper()
	var out bytes.Buffer
	for range n {
		out.Reset()
		runHookCommand([]string{"loop-guard"}, strings.NewReader(payload), &out)
	}
	return strings.TrimSpace(out.String())
}

func TestHookLoopGuardCommand(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		apiKey  string
		payload string
		wantHit bool
	}{
		{"on by default, trips at half of local_loop_guard", "version = 1\n", "", loopPayload, true},
		{"follows local_loop_guard", "version = 1\nlocal_loop_guard = 20\n", "", loopPayload, false},
		{"off in config", "version = 1\nhook_loop_guard = false\n", "", loopPayload, false},
		{"a local session is the local guard's", "version = 1\n", "nav-pilot", loopPayload, false},
		{"another BYOK key is not a local session", "version = 1\n", "sk-other", loopPayload, true},
		{"a broken config passes", "version = [\n", "", loopPayload, false},
		{"an unreadable payload passes", "version = 1\n", "", "not json", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeTestConfig(t, tt.config)
			t.Setenv("COPILOT_PROVIDER_API_KEY", tt.apiKey)
			out := runLoopGuard(t, 4, tt.payload)
			if hit := strings.Contains(out, "modifiedResult"); hit != tt.wantHit {
				t.Errorf("4 identical calls: %s, want hit=%v", out, tt.wantHit)
			}
			if !tt.wantHit && out != "{}" {
				t.Errorf("a pass must answer {}: %s", out)
			}
		})
	}
}

func TestSyncBuiltinHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".copilot", "hooks", "nav-pilot-loop-guard.json")

	syncBuiltinHooks(ResolvedConfig{Client: "opencode", HookLoopGuard: true})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("an opencode launch wrote a Copilot hook: %v", err)
	}

	syncBuiltinHooks(ResolvedConfig{Client: "copilot", HookLoopGuard: true})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("hook not written: %v", err)
	}
	for _, want := range []string{`"PostToolUse"`, `nav-pilot hook loop-guard`, `"navPilot": "nav-pilot-loop-guard"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("hook file lacks %s:\n%s", want, data)
		}
	}
	if names := source.HookNamesIn(path); len(names) != 1 {
		t.Errorf("doctor would not list it: %v", names)
	}

	syncBuiltinHooks(ResolvedConfig{Client: "copilot", HookLoopGuard: false})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("hook_loop_guard = false left the hook in place: %v", err)
	}
}
