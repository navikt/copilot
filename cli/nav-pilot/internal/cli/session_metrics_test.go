package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

const canary = "CANARY-7f3a-secret-prompt"

// sessionFixture has a content, result, arguments or message field holding the
// canary on every event that can carry one, a garbled line, a line with the
// wrong shape, and names inside and outside the allowlists.
var sessionFixture = strings.Join([]string{
	`{"type":"session.start","data":{"context":"` + canary + `"}}`,
	`{"type":"user.message","data":{"content":"` + canary + `"}}`,
	`{"type":"assistant.turn_start","data":{"turnId":"1"}}`,
	`{"type":"skill.invoked","data":{"name":"klarsprak","source":"personal-copilot","trigger":"agent-invoked","model":"gpt-6-sol","content":"` + canary + `","path":"/Users/` + canary + `/SKILL.md"}}`,
	`{"type":"skill.invoked","data":{"name":"` + canary + `","source":"` + canary + `","trigger":"agent-invoked","content":"` + canary + `"}}`,
	`{"type":"tool.execution_start","data":{"toolCallId":"a","toolName":"bash","arguments":{"command":"` + canary + `"}}}`,
	`{"type":"tool.execution_complete","data":{"toolCallId":"a","success":true,"model":"gpt-6-sol","result":{"content":"` + canary + `"},"toolTelemetry":{"properties":{"x":"` + canary + `"}}}}`,
	`{"type":"tool.execution_start","data":{"toolCallId":"b","toolName":"` + canary + `"}}`,
	`{"type":"tool.execution_complete","data":{"toolCallId":"b","success":false,"result":{"content":"` + canary + `"}}}`,
	`{"type":"hook.end","data":{"hookType":"preToolUse","success":true}}`,
	`{"type":"hook.end","data":{"hookType":"` + canary + `","success":false}}`,
	`{"type":"subagent.selected","data":{"agentName":"kafka","tools":["` + canary + `"]}}`,
	`{"type":"subagent.selected","data":{"agentName":"` + canary + `"}}`,
	`{"type":"assistant.message","data":{"content":"` + canary + `"}}`,
	`{"type":"assistant.turn_start","data":{"turnId":"2"}}`,
	`not json ` + canary,
	`{"type":"hook.end","data":"` + canary + `"}`,
	`{"type":"session.shutdown","timestamp":"2026-10-11T10:10:00.000Z","data":{"shutdownType":"routine","sessionStartTime":` +
		fmt.Sprint(time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC).UnixMilli()) +
		`,"totalApiDurationMs":15453,"currentModel":"gpt-6-sol","totalPremiumRequests":1,` +
		`"modelMetrics":{"gpt-6-sol":{"requests":{"count":6,"cost":1},"totalNanoAiu":8511570000,"tokenDetails":{"input":{"tokenCount":18},"output":{"tokenCount":1177},"cache_read":{"tokenCount":106436},"cache_write":{"tokenCount":20809}}},` +
		`"` + canary + `":{"requests":{"cost":0.5},"tokenDetails":{"input":{"tokenCount":5}}}},` +
		`"codeChanges":{"filesModified":["` + canary + `"]}}}`,
	`{"type":"assistant.turn_start","data":{"turnId":"after-shutdown"}}`,
}, "\n") + "\n"

// resumeFixture is the segment Copilot appends when the session is resumed:
// the shutdown reports running totals for the whole session.
var resumeFixture = strings.Join([]string{
	`{"type":"session.resume","timestamp":"2026-10-11T11:00:00.000Z","data":{"context":"` + canary + `"}}`,
	`{"type":"assistant.turn_start","data":{"turnId":"3"}}`,
	`{"type":"session.shutdown","timestamp":"2026-10-11T11:05:00.000Z","data":{"shutdownType":"routine","sessionStartTime":` +
		fmt.Sprint(time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC).UnixMilli()) +
		`,"totalApiDurationMs":20453,"currentModel":"gpt-6-sol",` +
		`"modelMetrics":{"gpt-6-sol":{"requests":{"cost":2},"totalNanoAiu":9000000000,"tokenDetails":{"input":{"tokenCount":30},"output":{"tokenCount":1200},"cache_read":{"tokenCount":106436},"cache_write":{"tokenCount":20809}}}}}}`,
}, "\n") + "\n"

func testModel(m string) string {
	if m == "gpt-6-sol" {
		return m
	}
	return "custom"
}

func TestParseCopilotSession(t *testing.T) {
	s, mark, ok := parseCopilotSession(strings.NewReader(sessionFixture), testModel,
		map[string]bool{"klarsprak": true}, map[string]bool{"kafka": true}, sessionTotals{})
	if !ok {
		t.Fatal("no shutdown found")
	}
	if dump := fmt.Sprintf("%#v %#v", s, mark); strings.Contains(dump, canary) {
		t.Fatalf("canary reached the metrics: %s", dump)
	}
	if want := int64(strings.LastIndex(sessionFixture, `{"type":"assistant.turn_start","data":{"turnId":"after`)); mark.Offset != want {
		t.Errorf("offset %d, want %d (just past the shutdown)", mark.Offset, want)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"skill nav", s.Skills[telemetrypkg.SkillKey{Skill: "klarsprak", Source: "personal-copilot", Trigger: "agent-invoked"}], int64(1)},
		{"skill other", s.Skills[telemetrypkg.SkillKey{Skill: "other", Source: "other", Trigger: "agent-invoked"}], int64(1)},
		{"tool bash", s.Tools[telemetrypkg.ToolKey{Tool: "bash", Outcome: "success"}], int64(1)},
		{"tool other", s.Tools[telemetrypkg.ToolKey{Tool: "other", Outcome: "error"}], int64(1)},
		{"hook", s.Hooks[telemetrypkg.HookKey{HookType: "preToolUse", Outcome: "success"}], int64(1)},
		{"hook other", s.Hooks[telemetrypkg.HookKey{HookType: "other", Outcome: "error"}], int64(1)},
		{"agents", s.Agents, map[string]int64{"kafka": 1, "other": 1}},
		{"turns", s.Turns, int64(2)},
		{"skipped", s.Skipped, int64(2)},
		{"duration", s.DurationSeconds, int64(600)},
		{"shutdown", s.ShutdownType, "routine"},
		{"api", [2]any{s.APIDurationMS, s.APIModel}, [2]any{int64(15453), "gpt-6-sol"}},
		{"model", s.Models["gpt-6-sol"], telemetrypkg.SessionModel{Input: 18, Output: 1177, CacheRead: 106436, CacheWrite: 20809, PremiumRequests: 1, NanoAIU: 8511570000}},
		{"custom model", s.Models["custom"], telemetrypkg.SessionModel{Input: 5, PremiumRequests: 0.5}},
	}
	for _, c := range checks {
		if fmt.Sprint(c.got) != fmt.Sprint(c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestParseCopilotSessionResumed: a resumed segment records what grew since the
// marker's totals, and its duration from the resume, not from the first start.
func TestParseCopilotSessionResumed(t *testing.T) {
	_, mark, _ := parseCopilotSession(strings.NewReader(sessionFixture), testModel, nil, nil, sessionTotals{})
	s, _, ok := parseCopilotSession(strings.NewReader(resumeFixture), testModel, nil, nil, mark.sessionTotals)
	if !ok {
		t.Fatal("no shutdown found")
	}
	want := telemetrypkg.SessionModel{Input: 12, Output: 23, PremiumRequests: 1, NanoAIU: 488430000}
	if s.Models["gpt-6-sol"] != want || len(s.Models) != 1 {
		t.Errorf("models = %+v, want gpt-6-sol %+v", s.Models, want)
	}
	if s.Turns != 1 || s.APIDurationMS != 5000 || s.DurationSeconds != 300 {
		t.Errorf("turns=%d api=%d duration=%d, want 1, 5000, 300", s.Turns, s.APIDurationMS, s.DurationSeconds)
	}
	// Both segments in one read: the last shutdown's totals, once.
	s, _, _ = parseCopilotSession(strings.NewReader(sessionFixture+resumeFixture), testModel, nil, nil, sessionTotals{})
	if s.Models["gpt-6-sol"].Input != 30 || s.APIDurationMS != 20453 || s.Turns != 4 {
		t.Errorf("whole file: input=%d api=%d turns=%d, want 30, 20453, 4", s.Models["gpt-6-sol"].Input, s.APIDurationMS, s.Turns)
	}
}

func TestParseCopilotSessionWithoutShutdown(t *testing.T) {
	for _, in := range []string{"", "garbage\n", `{"type":"assistant.turn_start"}` + "\n", "\x00\xff{{{"} {
		if _, _, ok := parseCopilotSession(strings.NewReader(in), testModel, nil, nil, sessionTotals{}); ok {
			t.Errorf("%q: reported a finished session", in)
		}
	}
}

type sessionTelemetry struct {
	noopTelemetry
	got []telemetrypkg.CopilotSession
}

func (s *sessionTelemetry) RecordCopilotSession(c telemetrypkg.CopilotSession) {
	s.got = append(s.got, c)
}

func TestRecordCopilotSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	root := filepath.Join(home, "session-state")
	dir := filepath.Join(root, "s1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(sessionFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := &sessionTelemetry{}
	orig := telemetry
	telemetry = rec
	t.Cleanup(func() { telemetry = orig })
	since := time.Now().Add(-time.Minute)

	t.Setenv("DO_NOT_TRACK", "1")
	recordCopilotSessions(root, since)
	if len(rec.got) != 0 {
		t.Fatal("DO_NOT_TRACK=1 and a session was recorded")
	}
	if _, err := os.Stat(filepath.Join(dir, sessionMetricsMarker)); !os.IsNotExist(err) {
		t.Fatal("opted out and the session was marked read")
	}
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "false")
	recordCopilotSessions(root, since)
	if len(rec.got) != 0 {
		t.Fatal("NAV_PILOT_TELEMETRY_ENABLED=false and a session was recorded")
	}
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "")

	recordCopilotSessions(root, time.Now().Add(time.Hour))
	if len(rec.got) != 0 {
		t.Fatal("recorded a session not written since the launch")
	}
	recordCopilotSessions(root, since)
	recordCopilotSessions(root, since)
	if len(rec.got) != 1 {
		t.Fatalf("recorded %d times, want once", len(rec.got))
	}
	// No state file in this HOME: no name is on the allowlist.
	if rec.got[0].Skills[telemetrypkg.SkillKey{Skill: "other", Source: "personal-copilot", Trigger: "agent-invoked"}] != 1 {
		t.Errorf("skills = %v", rec.got[0].Skills)
	}
	// Resumed: the marker's totals make the second record a delta.
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(resumeFixture)
	f.Close()
	recordCopilotSessions(root, since)
	// Turns is 2: the turn after the first shutdown lies past the marker too.
	if len(rec.got) != 2 || rec.got[1].Models["gpt-6-sol"].Input != 12 || rec.got[1].Turns != 2 {
		t.Fatalf("after resume: %d records, last %+v", len(rec.got), rec.got[len(rec.got)-1])
	}
}

// TestRealCopilotSession crosses the boundary once: a real events.jsonl from
// this machine. Run with NAV_PILOT_REAL_EVENTS=<path>; prints counts only.
func TestRealCopilotSession(t *testing.T) {
	p := os.Getenv("NAV_PILOT_REAL_EVENTS")
	if p == "" {
		t.Skip("NAV_PILOT_REAL_EVENTS not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, _, ok := parseCopilotSession(f, configModelLabel, map[string]bool{}, map[string]bool{}, sessionTotals{})
	if !ok {
		t.Fatal("no session.shutdown in the file")
	}
	sum := func(m any) (n int64) {
		switch m := m.(type) {
		case map[telemetrypkg.SkillKey]int64:
			for _, v := range m {
				n += v
			}
		case map[telemetrypkg.ToolKey]int64:
			for _, v := range m {
				n += v
			}
		case map[telemetrypkg.HookKey]int64:
			for _, v := range m {
				n += v
			}
		case map[string]int64:
			for _, v := range m {
				n += v
			}
		}
		return n
	}
	t.Logf("skills=%d tools=%d hooks=%d agents=%d turns=%d models=%d skipped=%d duration_s=%d api_ms=%d",
		sum(s.Skills), sum(s.Tools), sum(s.Hooks), sum(s.Agents), s.Turns, len(s.Models), s.Skipped, s.DurationSeconds, s.APIDurationMS)
}

func TestParseCopilotSessionEventsAfterShutdownNotInSnapshot(t *testing.T) {
	const tail = `{"type":"hook.end","timestamp":"2026-10-12T10:00:00Z","data":{"hookType":"preToolUse","success":true}}` + "\n"
	s, _, ok := parseCopilotSession(strings.NewReader(sessionFixture+tail), func(m string) string { return m }, nil, nil, sessionTotals{})
	if !ok {
		t.Fatal("no shutdown found")
	}
	if n := s.Hooks[telemetrypkg.HookKey{HookType: "preToolUse", Outcome: "success"}]; n != 1 {
		t.Errorf("event after the last shutdown leaked into the snapshot: %d, want 1", n)
	}
}

func TestLockSession(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "l")
	unlock, ok := lockSession(lock)
	if !ok {
		t.Fatal("first lock refused")
	}
	if _, ok := lockSession(lock); ok {
		t.Fatal("second lock granted")
	}
	unlock()
	if _, ok := lockSession(lock); !ok {
		t.Fatal("lock not released")
	}
}
