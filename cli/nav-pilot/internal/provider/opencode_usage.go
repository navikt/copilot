package provider

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// openCodeCommand runs opencode for its output. A variable so the test can
// answer for it.
var openCodeCommand = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = dir
	// These reads are not a session: no spans from them.
	cmd.Env = append(os.Environ(), "OTEL_SDK_DISABLED=true")
	// opencode 1.18 cuts its output to a pipe at 128 KiB when it exits, which
	// left any longer session unread. A file gets all of it. The file is
	// 0600 and unlinked before anything is written, so the raw export is
	// never on disk by name, not even if nav-pilot is killed; the deferred
	// Remove is for Windows, where the unlink waits for the close.
	f, err := os.CreateTemp("", "nav-pilot-opencode-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	_ = os.Remove(f.Name())
	cmd.Stdout = f
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

// readOpenCodeUsage sums the tokens, model calls and tool calls of the
// sessions in dir that changed since the launch, counting only messages from
// since on, so a resumed session is not counted twice.
//
// opencode sends traces but no metrics, so this is where its usage becomes a
// metric. It reads opencode's own store through its CLI after the session has
// ended. The export is the raw transcript; only fixed fields are decoded.
//
// It also returns one [telemetry.CopilotSession] per session read, for the
// nav_pilot_session_* metrics. Those carry no models: tokens and model calls
// go out once, through u.
//
// ponytail: a second opencode in the same directory at the same time is
// counted by both launches; tag sessions with the launch if that shows up.
func readOpenCodeUsage(dir string, since time.Time) (telemetry.ClientUsage, []telemetry.CopilotSession) {
	var sessions []telemetry.CopilotSession
	u := telemetry.ClientUsage{Client: "opencode", Dir: dir, Models: map[telemetry.ModelKey]telemetry.ModelUsage{}, Tools: map[string]int64{}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// v1 runs in-process with --pure; v2 needs --standalone so it does not
	// start or reuse the background service.
	list := []string{"session", "list", "--format", "json", "-n", "20", "--pure"}
	export := []string{"export", "--pure"}
	if openCodeMajor() >= 2 {
		list = []string{"session", "list", "--format", "json", "-n", "20", "--standalone"}
		export = []string{"session", "export", "--standalone"}
	}
	out, err := openCodeCommand(ctx, dir, list...)
	if err != nil {
		return u, nil
	}
	var listed []struct {
		ID        string `json:"id"`
		Updated   int64  `json:"updated"`
		Directory string `json:"directory"`
	}
	if json.Unmarshal(out, &listed) != nil {
		return u, nil
	}
	sinceMS := since.UnixMilli()
	want := realDir(dir)
	skills, agents := map[string]bool{}, map[string]bool{}
	if OpenCodeNavNames != nil {
		skills, agents = OpenCodeNavNames(dir)
	}
	for _, s := range listed {
		if s.Updated < sinceMS || realDir(s.Directory) != want {
			continue
		}
		out, err := openCodeCommand(ctx, dir, append(export, s.ID)...)
		if err != nil {
			continue
		}
		if sess, ok := addOpenCodeExport(&u, out, sinceMS, skills, agents); ok {
			sessions = append(sessions, sess)
		}
	}
	return u, sessions
}

// realDir resolves symlinks so /tmp and /private/tmp match; the raw path if
// that fails.
func realDir(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// openCodeTools are opencode's built-in tools. Other names come from MCP
// servers and plugins the developer named, so they are "other".
var openCodeTools = map[string]bool{
	"bash": true, "edit": true, "multiedit": true, "write": true, "read": true,
	"grep": true, "glob": true, "list": true, "patch": true, "apply_patch": true,
	"todowrite": true, "todoread": true, "webfetch": true, "websearch": true,
	"task": true, "skill": true, "question": true, "lsp": true, "batch": true,
	"codesearch": true,
}

// addOpenCodeExport adds one exported session's assistant messages from
// sinceMS on to u, and returns what the session did from sinceMS on as a
// [telemetry.CopilotSession]; false when it has no assistant message then.
// Only the fields in the struct below are decoded: of a tool's input, just the
// skill tool's name and the task tool's subagent_type, and those go through
// the skills and agents allowlists or become "other". Prompts, output and
// text never do.
func addOpenCodeExport(u *telemetry.ClientUsage, export []byte, sinceMS int64, skills, agents map[string]bool) (telemetry.CopilotSession, bool) {
	sess := telemetry.CopilotSession{
		Client: "opencode", Skills: map[telemetry.SkillKey]int64{}, Tools: map[telemetry.ToolKey]int64{},
		Agents: map[string]int64{}, DurationSeconds: -1,
	}
	var first, last int64
	var e struct {
		Messages []struct {
			Info struct {
				Role       string `json:"role"`
				ModelID    string `json:"modelID"`
				ProviderID string `json:"providerID"`
				Time       struct {
					Created   int64 `json:"created"`
					Completed int64 `json:"completed"`
				} `json:"time"`
				Tokens *struct {
					Input, Output, Reasoning int64
					Cache                    struct{ Read, Write int64 }
				} `json:"tokens"`
			} `json:"info"`
			Parts []struct {
				Type  string `json:"type"`
				Tool  string `json:"tool"`
				State struct {
					Status string `json:"status"`
					Input  struct {
						Name         string `json:"name"`
						SubagentType string `json:"subagent_type"`
					} `json:"input"`
				} `json:"state"`
			} `json:"parts"`
		} `json:"messages"`
	}
	if json.Unmarshal(export, &e) != nil {
		return sess, false
	}
	for _, m := range e.Messages {
		if m.Info.Time.Created < sinceMS {
			continue
		}
		if first == 0 {
			first = m.Info.Time.Created
		}
		last = max(last, m.Info.Time.Created, m.Info.Time.Completed)
		if m.Info.Role != "assistant" {
			continue
		}
		sess.Turns++
		provider, model := openCodeTelemetryModel(m.Info.ProviderID, m.Info.ModelID)
		k := telemetry.ModelKey{Provider: provider, Model: model}
		mu := u.Models[k]
		mu.Calls++
		if t := m.Info.Tokens; t != nil {
			mu.Input += t.Input
			mu.Output += t.Output
			mu.Reasoning += t.Reasoning
			mu.CacheRead += t.Cache.Read
			mu.CacheWrite += t.Cache.Write
		}
		u.Models[k] = mu
		for _, p := range m.Parts {
			if p.Type == "tool" && p.Tool != "" {
				tool := p.Tool
				if !openCodeTools[tool] {
					tool = "other"
				}
				u.Tools[tool]++
				sess.Tools[telemetry.ToolKey{Tool: tool, Outcome: openCodeOutcome(p.State.Status)}]++
				switch p.Tool {
				case "skill":
					sess.Skills[telemetry.SkillKey{Skill: navName(p.State.Input.Name, skills), Source: "unset", Trigger: "unset"}]++
				case "task":
					sess.Agents[navName(p.State.Input.SubagentType, agents)]++
				}
			}
		}
	}
	if sess.Turns == 0 {
		return sess, false
	}
	sess.DurationSeconds = (last - first) / 1000
	return sess, true
}

// OpenCodeNavNames returns the skill and agent names nav-pilot installed for
// the user and for the repo the launch directory dir is in, the same allowlist
// the Copilot session metrics use. The cli package sets it.
var OpenCodeNavNames func(dir string) (skills, agents map[string]bool)

// navName is name when it is on the allowlist, "unset" when empty, else "other".
func navName(name string, allow map[string]bool) string {
	switch {
	case name == "":
		return "unset"
	case allow[name]:
		return name
	}
	return "other"
}

// openCodeOutcome maps a tool part's state.status to the outcome label the
// Copilot sessions use. A tool still pending or running when opencode exited
// is "unset".
func openCodeOutcome(status string) string {
	switch status {
	case "completed":
		return "success"
	case "error":
		return "error"
	}
	return "unset"
}

// openCodeTelemetryModel keeps model ids bounded. Copilot's are a short public
// list and nav-pilot's local provider goes through [local.TelemetryModel]; a
// provider the developer added has a name and ids they typed, so both are
// "custom".
func openCodeTelemetryModel(provider, model string) (string, string) {
	switch provider {
	case "github-copilot":
		return provider, model
	case LocalProviderID:
		return provider, local.TelemetryModel(model)
	}
	return "custom", "custom"
}
