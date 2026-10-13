package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

func TestReadOpenCodeUsage(t *testing.T) {
	since := time.UnixMilli(1000)
	old := openCodeCommand
	t.Cleanup(func() { openCodeCommand = old })
	openCodeCommand = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "session" {
			return []byte(`[{"id":"new","updated":2000,"directory":"/repo"},{"id":"old","updated":500,"directory":"/repo"},{"id":"other","updated":2000,"directory":"/elsewhere"}]`), nil
		}
		if args[len(args)-1] != "new" || strings.Contains(strings.Join(args, " "), "--sanitize") {
			t.Fatalf("unexpected export %v", args)
		}
		return []byte(`{"messages":[
{"info":{"role":"user","time":{"created":1500}}},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":900},"tokens":{"input":999,"output":999,"reasoning":0,"cache":{"read":0,"write":0}}},"parts":[{"type":"tool","tool":"bash"}]},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":1500},"tokens":{"input":10,"output":5,"reasoning":2,"cache":{"read":100,"write":1}}},"parts":[{"type":"text"},{"type":"tool","tool":"bash"},{"type":"tool","tool":"read"},{"type":"tool","tool":"my_mcp_secret"}]},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":1600},"tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}},"parts":[{"type":"tool","tool":"bash"}]}]}`), nil
	}
	u, _ := readOpenCodeUsage("/repo", since)
	want := telemetry.ModelUsage{Calls: 2, Input: 11, Output: 6, Reasoning: 2, CacheRead: 100, CacheWrite: 1}
	if got := u.Models[telemetry.ModelKey{Provider: "github-copilot", Model: "m"}]; got != want || len(u.Models) != 1 {
		t.Fatalf("models = %+v, want %+v", u.Models, want)
	}
	if p, m := openCodeTelemetryModel("my-own", "secret-name"); p != "custom" || m != "custom" {
		t.Fatalf("own provider = %q/%q, want custom/custom", p, m)
	}
	if u.Tools["bash"] != 2 || u.Tools["read"] != 1 || u.Tools["other"] != 1 || len(u.Tools) != 3 {
		t.Fatalf("tools = %v", u.Tools)
	}
}

func TestReadOpenCodeUsageResolvesSymlinks(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	old := openCodeCommand
	t.Cleanup(func() { openCodeCommand = old })
	openCodeCommand = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[len(args)-1] != "s1" {
			return []byte(`[{"id":"s1","updated":2000,"directory":"` + real + `"}]`), nil
		}
		return []byte(`{"messages":[{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":1500}}}]}`), nil
	}
	u, _ := readOpenCodeUsage(link, time.UnixMilli(1000))
	if u.Models[telemetry.ModelKey{Provider: "github-copilot", Model: "m"}].Calls != 1 {
		t.Fatalf("symlinked dir not matched: %+v", u.Models)
	}
}

func TestOpenCodeUsageDirResolvesRelative(t *testing.T) {
	wd, _ := os.Getwd()
	if got, want := openCodeUsageDir("sub"), filepath.Join(wd, "sub"); got != want {
		t.Fatalf("openCodeUsageDir(sub) = %q, want %q", got, want)
	}
}

// openCodeCanaryExport is a raw export with a canary in every free-text field
// opencode has, including the input fields next to the skill and subagent
// names. Only allowlisted names may come out.
const openCodeCanaryExport = `{"info":{"title":"CANARY-TITLE"},"messages":[
{"info":{"role":"user","time":{"created":1000}},"parts":[{"type":"text","text":"CANARY-PROMPT"}]},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":2000,"completed":5000}},"parts":[
 {"type":"text","text":"CANARY-TEXT"},
 {"type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"CANARY-CMD"},"output":"CANARY-OUT","title":"CANARY-T"}},
 {"type":"tool","tool":"skill","state":{"status":"completed","input":{"name":"nav-skill","arguments":"CANARY-ARGS"},"output":"CANARY-OUT","metadata":{"name":"nav-skill","dir":"CANARY-DIR"}}},
 {"type":"tool","tool":"skill","state":{"status":"completed","input":{"name":"CANARY-OWN-SKILL"}}},
 {"type":"tool","tool":"task","state":{"status":"error","input":{"subagent_type":"nav-agent","prompt":"CANARY-TASK-PROMPT","description":"CANARY-DESC"},"metadata":{"sessionId":"CANARY-SID"}}},
 {"type":"tool","tool":"task","state":{"status":"completed","input":{"subagent_type":"CANARY-OWN-AGENT","prompt":"CANARY-TASK-PROMPT"}}},
 {"type":"tool","tool":"CANARY-MCP","state":{"status":"running","input":{"name":"CANARY-MCP-ARG"}}}]},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":62000,"completed":63500}},"parts":[]}]}`

func sessionLeaks(s telemetry.CopilotSession, canary string) bool {
	return strings.Contains(fmt.Sprintf("%+v", s), canary)
}

func TestOpenCodeSessionMetrics(t *testing.T) {
	skills, agents := map[string]bool{"nav-skill": true}, map[string]bool{"nav-agent": true}
	u := telemetry.ClientUsage{Models: map[telemetry.ModelKey]telemetry.ModelUsage{}, Tools: map[string]int64{}}
	s, ok := addOpenCodeExport(&u, []byte(openCodeCanaryExport), 1000, skills, agents)
	if !ok {
		t.Fatal("no session")
	}
	if sessionLeaks(s, "CANARY") || strings.Contains(fmt.Sprintf("%+v", u), "CANARY") {
		t.Fatalf("content reached the metrics: %+v", s)
	}
	if s.Client != "opencode" || s.Turns != 2 || s.DurationSeconds != 62 || len(s.Models) != 0 {
		t.Fatalf("session = %+v", s)
	}
	want := map[telemetry.ToolKey]int64{
		{Tool: "bash", Outcome: "success"}: 1, {Tool: "skill", Outcome: "success"}: 2,
		{Tool: "task", Outcome: "error"}: 1, {Tool: "task", Outcome: "success"}: 1, {Tool: "other", Outcome: "unset"}: 1,
	}
	if fmt.Sprint(s.Tools) != fmt.Sprint(want) {
		t.Fatalf("tools = %v", s.Tools)
	}
	sk := func(n string) telemetry.SkillKey {
		return telemetry.SkillKey{Skill: n, Source: "unset", Trigger: "unset"}
	}
	if s.Skills[sk("nav-skill")] != 1 || s.Skills[sk("other")] != 1 || len(s.Skills) != 2 ||
		s.Agents["nav-agent"] != 1 || s.Agents["other"] != 1 || len(s.Agents) != 2 {
		t.Fatalf("skills = %v, agents = %v", s.Skills, s.Agents)
	}
	if _, ok := addOpenCodeExport(&u, []byte(openCodeCanaryExport), 99000, skills, agents); ok {
		t.Fatal("a session with nothing since the launch was returned")
	}
}

// The control: decoding one non-name input field (the task prompt) into a
// label is caught by sessionLeaks, so the check above has teeth.
func TestOpenCodeSessionCanaryControl(t *testing.T) {
	var e struct {
		Messages []struct {
			Parts []struct {
				State struct {
					Input struct{ Prompt string } `json:"input"`
				} `json:"state"`
			} `json:"parts"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(openCodeCanaryExport), &e); err != nil {
		t.Fatal(err)
	}
	leaky := telemetry.CopilotSession{Agents: map[string]int64{}}
	for _, m := range e.Messages {
		for _, p := range m.Parts {
			if p.State.Input.Prompt != "" {
				leaky.Agents[p.State.Input.Prompt]++
			}
		}
	}
	if !sessionLeaks(leaky, "CANARY-TASK-PROMPT") {
		t.Fatal("sessionLeaks misses a leaked prompt")
	}
}

// A fake opencode writes 300 KiB through the temp file (the 128 KiB pipe cut
// is Bun's, measured on the real binary), and the file is gone afterwards.
func TestOpenCodeCommandReadsPast128KiB(t *testing.T) {
	bin, tmp := t.TempDir(), t.TempDir()
	script := "#!/bin/sh\nhead -c 307200 /dev/zero | tr '\\0' x\n"
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMPDIR", tmp)
	out, err := openCodeCommand(context.Background(), t.TempDir(), "export")
	if err != nil || len(out) != 307200 {
		t.Fatalf("len = %d, err = %v", len(out), err)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("temp file left behind: %v", left)
	}
}

// The allowlist is resolved for the launch directory, not the process cwd.
func TestReadOpenCodeUsageAllowlistForLaunchDir(t *testing.T) {
	old, oldNames := openCodeCommand, OpenCodeNavNames
	t.Cleanup(func() { openCodeCommand, OpenCodeNavNames = old, oldNames })
	var asked string
	OpenCodeNavNames = func(dir string) (map[string]bool, map[string]bool) {
		asked = dir
		return map[string]bool{"nav-skill": true}, map[string]bool{}
	}
	openCodeCommand = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "session" {
			return []byte(`[{"id":"s1","updated":2000,"directory":"/repo"}]`), nil
		}
		return []byte(openCodeCanaryExport), nil
	}
	_, sessions := readOpenCodeUsage("/repo", time.UnixMilli(1000))
	if asked != "/repo" || len(sessions) != 1 || sessions[0].Skills[telemetry.SkillKey{Skill: "nav-skill", Source: "unset", Trigger: "unset"}] != 1 {
		t.Fatalf("asked = %q, sessions = %+v", asked, sessions)
	}
}
