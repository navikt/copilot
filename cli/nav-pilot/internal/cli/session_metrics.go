package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// Session metrics come from Copilot CLI's own record of a session,
// ~/.copilot/session-state/<session>/events.jsonl. Only a fixed set of fields
// is decoded, into the structs below: content, result, arguments and messages
// are never part of them, so they are never read into memory as values.

const sessionMetricsMarker = "nav-pilot-session-metrics"

// copilotBuiltinTools are the tool names Copilot CLI ships. Any other tool
// (MCP servers, extensions) is "other": its name can say what a team builds.
var copilotBuiltinTools = setOf("bash", "read_bash", "write_bash", "stop_bash", "list_bash",
	"powershell", "read_powershell", "write_powershell", "stop_powershell", "list_powershell",
	"view", "create", "edit", "str_replace_editor", "apply_patch", "glob", "grep", "rg",
	"web_fetch", "web_search", "skill", "task", "read_agent", "list_agents", "write_agent",
	"report_intent", "update_todo", "think", "ask_user", "store_memory", "sql",
	"fetch_copilot_cli_documentation", "exit_plan_mode")

var copilotHookTypes = setOf("sessionStart", "sessionEnd", "userPromptSubmitted",
	"preToolUse", "postToolUse", "errorOccurred", "agentStop", "subagentStop")

// shortLabel bounds a free-text label Copilot sets itself (skill source and
// trigger, shutdown type) without listing every value it may grow.
var shortLabel = regexp.MustCompile(`^[a-z][a-z_-]{0,31}$`)

func setOf(s ...string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}

func label(v string, ok bool) string {
	if v == "" {
		return "unset"
	}
	if !ok {
		return "other"
	}
	return v
}

// navItemNames is the allowlist of skill and agent names: what nav-pilot
// installed from the agentpakke manifest, per the user and launch-directory
// state files. A skill or agent someone wrote themselves is not in them.
func navItemNames() (skills, agents map[string]bool) {
	skills, agents = map[string]bool{}, map[string]bool{}
	var scopes []*InstallScope
	if s, err := ScopeUser(); err == nil {
		scopes = append(scopes, s)
	}
	if wd, err := os.Getwd(); err == nil {
		if root := source.FindGitRoot(wd); root != "" {
			wd = root
		}
		scopes = append(scopes, ScopeRepo(wd))
	}
	for _, sc := range scopes {
		st, err := readScopedState(sc)
		if err != nil || st == nil {
			continue
		}
		for _, f := range st.Files {
			p := filepath.ToSlash(f.Path)
			// skills/<name>/SKILL.md, agents/<name>.agent.md, with or
			// without the .github/ prefix a repo scope uses.
			p = strings.TrimPrefix(p, ".github/")
			switch {
			case strings.HasPrefix(p, "skills/"):
				if n := strings.Split(p, "/"); len(n) > 2 {
					skills[n[1]] = true
				}
			case strings.HasPrefix(p, "agents/") && strings.HasSuffix(p, ".agent.md"):
				agents[strings.TrimSuffix(path.Base(p), ".agent.md")] = true
			}
		}
	}
	return skills, agents
}

type sessionEvent struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// The data of each event type nav-pilot reads. Only these fields are decoded.
type (
	skillInvoked struct{ Name, Source, Trigger string }
	toolStart    struct {
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
	}
	toolComplete struct {
		ToolCallID string `json:"toolCallId"`
		Success    *bool  `json:"success"`
	}
	hookEnd struct {
		HookType string `json:"hookType"`
		Success  *bool  `json:"success"`
	}
	subagentSelected struct {
		AgentName string `json:"agentName"`
	}
	sessionShutdown struct {
		ShutdownType       string `json:"shutdownType"`
		SessionStartTime   int64  `json:"sessionStartTime"`
		TotalAPIDurationMs int64  `json:"totalApiDurationMs"`
		CurrentModel       string `json:"currentModel"`
		ModelMetrics       map[string]struct {
			Requests struct {
				Cost float64 `json:"cost"`
			} `json:"requests"`
			TotalNanoAiu int64                                 `json:"totalNanoAiu"`
			TokenDetails map[string]struct{ TokenCount int64 } `json:"tokenDetails"`
		} `json:"modelMetrics"`
	}
)

// sessionTotals is what session.shutdown reports as running totals for the
// whole session, resumes included. The marker keeps the last ones, so a
// resumed session records only the difference.
type sessionTotals struct {
	Models        map[string]telemetrypkg.SessionModel `json:"models,omitempty"`
	APIDurationMS int64                                `json:"api_ms,omitempty"`
}

// sessionMarker is the marker file: the offset read so far and the totals at
// the last shutdown.
type sessionMarker struct {
	Offset int64 `json:"offset"`
	sessionTotals
}

func (t sessionTotals) since(prev sessionTotals) sessionTotals {
	d := sessionTotals{Models: map[string]telemetrypkg.SessionModel{}, APIDurationMS: t.APIDurationMS - prev.APIDurationMS}
	for k, m := range t.Models {
		p := prev.Models[k]
		d.Models[k] = telemetrypkg.SessionModel{
			Input: m.Input - p.Input, Output: m.Output - p.Output, CacheRead: m.CacheRead - p.CacheRead, CacheWrite: m.CacheWrite - p.CacheWrite,
			PremiumRequests: m.PremiumRequests - p.PremiumRequests, NanoAIU: m.NanoAIU - p.NanoAIU,
		}
	}
	return d
}

func outcome(ok *bool) string {
	if ok == nil {
		return "unset"
	}
	if *ok {
		return "success"
	}
	return "error"
}

// parseCopilotSession reads events up to and including the last
// session.shutdown. It returns the session, a marker with the byte offset just
// past that shutdown line and its totals, and false when there is no shutdown
// yet (still running). Tokens, cost and API time are what grew since prev; the
// duration runs from the last session.start or session.resume read. It never
// fails: a line it cannot use is counted in Skipped.
func parseCopilotSession(r io.Reader, model func(string) string, skills, agents map[string]bool, prev sessionTotals) (telemetrypkg.CopilotSession, sessionMarker, bool) {
	s := telemetrypkg.CopilotSession{
		Client: "copilot", Skills: map[telemetrypkg.SkillKey]int64{}, Tools: map[telemetrypkg.ToolKey]int64{},
		Hooks: map[telemetrypkg.HookKey]int64{}, Agents: map[string]int64{}, Models: map[string]telemetrypkg.SessionModel{},
		DurationSeconds: -1,
	}
	var done telemetrypkg.CopilotSession
	var mark sessionMarker
	var offset int64
	var segmentStart time.Time
	found := false
	toolNames := map[string]string{}
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			break
		}
		offset += int64(len(line))
		if err != nil {
			break // a partial last line: Copilot is still writing it
		}
		var ev sessionEvent
		if json.Unmarshal(line, &ev) != nil || ev.Type == "" {
			s.Skipped++
			continue
		}
		bad := false
		decode := func(v any) bool {
			if json.Unmarshal(ev.Data, v) != nil {
				bad = true
				return false
			}
			return true
		}
		switch ev.Type {
		case "skill.invoked":
			var d skillInvoked
			if decode(&d) {
				s.Skills[telemetrypkg.SkillKey{
					Skill:   label(d.Name, skills[d.Name]),
					Source:  label(d.Source, shortLabel.MatchString(d.Source)),
					Trigger: label(d.Trigger, shortLabel.MatchString(d.Trigger)),
				}]++
			}
		case "tool.execution_start":
			var d toolStart
			if decode(&d) && d.ToolCallID != "" {
				toolNames[d.ToolCallID] = d.ToolName
			}
		case "tool.execution_complete":
			var d toolComplete
			if decode(&d) {
				name := toolNames[d.ToolCallID]
				s.Tools[telemetrypkg.ToolKey{Tool: label(name, copilotBuiltinTools[name]), Outcome: outcome(d.Success)}]++
			}
		case "hook.end":
			var d hookEnd
			if decode(&d) {
				s.Hooks[telemetrypkg.HookKey{HookType: label(d.HookType, copilotHookTypes[d.HookType]), Outcome: outcome(d.Success)}]++
			}
		case "subagent.selected":
			var d subagentSelected
			if decode(&d) {
				s.Agents[label(d.AgentName, agents[d.AgentName])]++
			}
		case "assistant.turn_start":
			s.Turns++
		case "session.start", "session.resume":
			if ts, err := time.Parse(time.RFC3339Nano, ev.Timestamp); err == nil {
				segmentStart = ts
			}
		case "session.shutdown":
			var d sessionShutdown
			if !decode(&d) {
				break
			}
			cum := sessionTotals{Models: map[string]telemetrypkg.SessionModel{}, APIDurationMS: d.TotalAPIDurationMs}
			for id, m := range d.ModelMetrics {
				k := model(id)
				sm := cum.Models[k]
				sm.Input += m.TokenDetails["input"].TokenCount
				sm.Output += m.TokenDetails["output"].TokenCount
				sm.CacheRead += m.TokenDetails["cache_read"].TokenCount
				sm.CacheWrite += m.TokenDetails["cache_write"].TokenCount
				sm.PremiumRequests += m.Requests.Cost
				sm.NanoAIU += m.TotalNanoAiu
				cum.Models[k] = sm
			}
			delta := cum.since(prev)
			prev = cum
			for k, m := range delta.Models {
				sm := s.Models[k]
				sm.Input += m.Input
				sm.Output += m.Output
				sm.CacheRead += m.CacheRead
				sm.CacheWrite += m.CacheWrite
				sm.PremiumRequests += m.PremiumRequests
				sm.NanoAIU += m.NanoAIU
				s.Models[k] = sm
			}
			s.APIDurationMS += delta.APIDurationMS
			s.ShutdownType = label(d.ShutdownType, shortLabel.MatchString(d.ShutdownType))
			s.APIModel = model(d.CurrentModel)
			start := segmentStart
			if start.IsZero() && d.SessionStartTime > 0 {
				start = time.UnixMilli(d.SessionStartTime)
			}
			if end, err := time.Parse(time.RFC3339Nano, ev.Timestamp); err == nil && !start.IsZero() {
				if sec := end.Sub(start) / time.Second; sec >= 0 {
					s.DurationSeconds = int64(sec)
				}
			}
			done = s
			done.Skills, done.Tools, done.Hooks = maps.Clone(s.Skills), maps.Clone(s.Tools), maps.Clone(s.Hooks)
			done.Agents, done.Models = maps.Clone(s.Agents), maps.Clone(s.Models)
			mark, found = sessionMarker{Offset: offset, sessionTotals: cum}, true
		}
		if bad {
			s.Skipped++
		}
	}
	return done, mark, found
}

// recordCopilotSessions records each Copilot session that ended since `since`
// and was not recorded before. A marker in the session directory holds the
// offset read so far and the totals at that point, so a resumed session counts
// only what came after. Opted out, nothing is read.
//
// A lock file per session keeps two overlapping nav-pilot exits from both
// recording it: the one that loses the race skips the session.
func recordCopilotSessions(root string, since time.Time) {
	if !telemetryEnabled() || since.IsZero() {
		return
	}
	paths, _ := filepath.Glob(filepath.Join(root, "*", "events.jsonl"))
	var skills, agents map[string]bool
	for _, p := range paths {
		if fi, err := os.Stat(p); err != nil || fi.ModTime().Before(since) {
			continue
		}
		marker := filepath.Join(filepath.Dir(p), sessionMetricsMarker)
		unlock, ok := lockSession(marker + ".lock")
		if !ok {
			continue
		}
		var prev sessionMarker
		if b, err := os.ReadFile(marker); err == nil {
			_ = json.Unmarshal(b, &prev)
		}
		f, err := os.Open(p)
		if err != nil {
			unlock()
			continue
		}
		if _, err := f.Seek(prev.Offset, io.SeekStart); err != nil {
			f.Close()
			unlock()
			continue
		}
		if skills == nil {
			skills, agents = navItemNames()
		}
		s, mark, ok := parseCopilotSession(f, configModelLabel, skills, agents, prev.sessionTotals)
		f.Close()
		if ok {
			telemetry.RecordCopilotSession(s)
			mark.Offset += prev.Offset
			if b, err := json.Marshal(mark); err == nil {
				_ = os.WriteFile(marker, b, 0o600)
			}
		}
		unlock()
	}
}

// lockSession takes an exclusive lock file. One older than a minute is a
// crashed holder's and is taken over.
func lockSession(lock string) (unlock func(), ok bool) {
	for range 2 {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if f.Close() != nil {
				_ = os.Remove(lock)
				return nil, false
			}
			return func() { _ = os.Remove(lock) }, true
		}
		if fi, serr := os.Stat(lock); serr != nil || time.Since(fi.ModTime()) < time.Minute {
			if serr == nil {
				return nil, false
			}
			continue
		}
		_ = os.Remove(lock)
	}
	return nil, false
}
