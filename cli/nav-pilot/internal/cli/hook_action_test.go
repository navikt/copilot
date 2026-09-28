package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/hook"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

const riskyPayload = `{"sessionId":"s1","cwd":"/w","toolName":"bash","toolArgs":{"command":"kubectl delete deployment app -n team --token=ghp_0123456789abcdefghijklmnopqrstuvwxyz","description":"Restart the app"}}`

// runActionHook runs `nav-pilot hook action-check` on payload with config,
// and returns its answer and the session's log and spool, "" when absent.
func runActionHook(t *testing.T, config, payload string) (out, log, spool string) {
	t.Helper()
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "")
	if err := os.MkdirAll(filepath.Dir(os.Getenv("NAV_PILOT_CONFIG")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("NAV_PILOT_CONFIG"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	runHookCommand([]string{"action-check"}, strings.NewReader(payload), &b)
	l, _ := os.ReadFile(filepath.Join(hookStateDir(), "s1", hook.ActionLogName))
	s, _ := os.ReadFile(filepath.Join(hookStateDir(), "s1", "nav-pilot-hook-events"))
	return strings.TrimSpace(b.String()), string(l), string(s)
}

const localOn = "version = 1\nlocal_enabled = true\n"

func TestActionCheckOutcomes(t *testing.T) {
	answerB := func(p float64) func(string) []fakeTok {
		return func(string) []fakeTok { return []fakeTok{{"A", 1 - p}, {"B", p}} }
	}
	tests := []struct {
		name   string
		answer func(string) []fakeTok
		want   string
	}{
		{"a risky answer flags", answerB(0.9), "action_check flagged kubectl"},
		{"safe answers pass", answerB(0.1), "action_check passed kubectl"},
		// Only the destructive question comes back risky.
		{"one question is enough", func(prompt string) []fakeTok {
			if strings.Contains(prompt, "destructive or irreversible") {
				return []fakeTok{{"B", 0.8}, {"A", 0.2}}
			}
			return []fakeTok{{"A", 0.9}, {"B", 0.1}}
		}, "action_check flagged kubectl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeDecideServer(t, tt.answer)
			out, log, spool := runActionHook(t, localOn, riskyPayload)
			if out != "{}" {
				t.Errorf("answer %s, want {} (allow)", out)
			}
			if strings.TrimSpace(spool) != tt.want {
				t.Errorf("spooled %q, want %q", spool, tt.want)
			}
			if !strings.Contains(log, `"category":"kubectl"`) || !strings.Contains(log, `"destructive":`) {
				t.Errorf("log lacks the verdict: %s", log)
			}
			if strings.Contains(log, "ghp_0123") || !strings.Contains(log, "kubectl delete deployment app") {
				t.Errorf("log must hold the command, redacted: %s", log)
			}
		})
	}
}

func TestActionCheckTimeoutAllows(t *testing.T) {
	localTestHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body, or the server never notices the client hang up.
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)
	orig := decideServer
	decideServer = func(context.Context) (string, string, func(), error) { return srv.URL, "fake", func() {}, nil }
	t.Cleanup(func() { decideServer = orig })

	started := time.Now()
	out, _, spool := runActionHook(t, localOn, riskyPayload)
	if took := time.Since(started); took > 2*actionCheckBudget {
		t.Errorf("took %s, the budget is %s", took, actionCheckBudget)
	}
	if out != "{}" || strings.TrimSpace(spool) != "action_check skipped_timeout kubectl" {
		t.Errorf("slow server: %s, spooled %q", out, spool)
	}
}

// A server that cannot be found fast, too: the lock or the server's checks
// hanging past the budget without watching ctx.
func TestActionCheckStuckServerAllows(t *testing.T) {
	localTestHome(t)
	orig := decideServer
	decideServer = func(context.Context) (string, string, func(), error) {
		time.Sleep(3 * actionCheckBudget)
		return "", "", nil, context.DeadlineExceeded
	}
	t.Cleanup(func() { decideServer = orig })
	out, _, spool := runActionHook(t, localOn, riskyPayload)
	if out != "{}" || strings.TrimSpace(spool) != "action_check skipped_timeout kubectl" {
		t.Errorf("stuck lookup: %s, spooled %q", out, spool)
	}
}

func TestActionCheckNoServerAllows(t *testing.T) {
	localTestHome(t)
	orig := decideServer
	decideServer = func(context.Context) (string, string, func(), error) {
		return "", "", nil, local.ErrNoServerRecorded
	}
	t.Cleanup(func() { decideServer = orig })
	out, _, spool := runActionHook(t, localOn, riskyPayload)
	if out != "{}" || strings.TrimSpace(spool) != "action_check skipped_no_server kubectl" {
		t.Errorf("no server: %s, spooled %q", out, spool)
	}
}

// Off, no local model, or nothing risky: no decide call, no log, no spool.
func TestActionCheckAsksNothing(t *testing.T) {
	tests := []struct{ name, config, payload string }{
		{"key off", localOn + "hook_action_check = \"off\"\n", riskyPayload},
		{"no local model", "version = 1\n", riskyPayload},
		{"not risky", localOn, `{"sessionId":"s1","toolName":"bash","toolArgs":{"command":"kubectl get pods -n team"}}`},
		{"not a shell call", localOn, `{"sessionId":"s1","toolName":"edit","toolArgs":{"command":"rm -rf /"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asked := false
			fakeDecideServer(t, func(string) []fakeTok { asked = true; return []fakeTok{{"B", 1}} })
			out, log, spool := runActionHook(t, tt.config, tt.payload)
			if out != "{}" || asked || log != "" || spool != "" {
				t.Errorf("out %s, asked %v, log %q, spool %q", out, asked, log, spool)
			}
		})
	}
}

func TestActionCheckRegistration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".copilot", "hooks", "nav-pilot-action-check.json")

	syncBuiltinHooks(ResolvedConfig{Client: "copilot", HookActionCheck: "log"})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("written without a local model: %v", err)
	}
	on := ResolvedConfig{Client: "copilot", HookActionCheck: "log", LocalEnabled: true, HookLoopGuard: true, HookRedactSecrets: true}
	syncBuiltinHooks(on)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"PreToolUse"`, `"matcher": "bash|shell|execute|powershell"`, `nav-pilot hook action-check local_enabled=true ||`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("hook file lacks %s:\n%s", want, data)
		}
	}

	// OpenCode: a pre hook, once (not again from the Copilot file), and the
	// post hooks keep their order with redaction last.
	on.ProjectDir = t.TempDir() // no repo gates
	b := openCodeHookBridge(on)
	var pre, post []string
	for _, h := range b.Pre {
		pre = append(pre, h.Name)
	}
	for _, h := range b.Post {
		post = append(post, h.Name)
	}
	if strings.Join(pre, ",") != "nav-pilot-action-check" || !strings.Contains(b.Pre[0].Command, "hook action-check 'local_enabled=true'") {
		t.Errorf("pre = %v %+v", pre, b.Pre)
	}
	if strings.Join(post, ",") != "nav-pilot-loop-guard,nav-pilot-redact-tool-output" {
		t.Errorf("post = %v, want redaction last", post)
	}

	on.HookActionCheck = "off"
	syncBuiltinHooks(on)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("off left the hook in place: %v", err)
	}
}

// testdata/action-check.jsonl is the labelled case set for the action check,
// in `alpha decide --eval` format: one line per command and question. It is
// measured against the models in mlx-workspace, not here; this checks that it
// asks what the hook asks, with the evidence the hook builds, and that the
// classifier asks about every command marked risky_command.
func TestActionCheckCaseSet(t *testing.T) {
	data, err := os.ReadFile("testdata/action-check.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	type line struct {
		evalCase
		Meta struct {
			Case, Question, Command, Description, Cwd, Class, Category string
			RiskyCommand                                               bool `json:"risky_command"`
		} `json:"meta"`
	}
	questions := map[string]actionQuestion{}
	for _, q := range actionQuestions {
		questions[q.key] = q
	}
	perCase := map[string]int{}
	classes := map[string]int{}
	for n, raw := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var c line
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			t.Fatalf("line %d: %v", n+1, err)
		}
		q, ok := questions[c.Meta.Question]
		if !ok || c.Question != q.question || !slices.Equal(c.Options, q.options) || !slices.Contains(c.Options, c.Expect) {
			t.Errorf("line %d: question %q does not match the hook's, or expect %q is not an option", n+1, c.Meta.Question, c.Expect)
		}
		if c.Evidence == nil || *c.Evidence != actionEvidence(c.Meta.Command, c.Meta.Description, c.Meta.Cwd) {
			t.Errorf("line %d: evidence is not what the hook would send", n+1)
		}
		if got := hook.RiskyCommand(c.Meta.Command); c.Meta.RiskyCommand != (got != "") || got != c.Meta.Category {
			t.Errorf("line %d: RiskyCommand(%q) = %q, the set says %v %q", n+1, c.Meta.Command, got, c.Meta.RiskyCommand, c.Meta.Category)
		}
		if perCase[c.Meta.Case]++; perCase[c.Meta.Case] == 1 {
			classes[c.Meta.Class]++
		}
	}
	for id, n := range perCase {
		if n != len(actionQuestions) {
			t.Errorf("case %s has %d questions, want %d", id, n, len(actionQuestions))
		}
	}
	if len(perCase) < 40 || classes["risky"] < 15 || classes["harmless"] < 15 {
		t.Errorf("%d cases, classes %v: want at least 40, and both classes", len(perCase), classes)
	}
}
