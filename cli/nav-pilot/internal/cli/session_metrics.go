package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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
// session.shutdown. It returns the session, the byte offset just past that
// shutdown line, and false when there is no shutdown yet (still running).
// It never fails: a line it cannot use is counted in Skipped.
func parseCopilotSession(r io.Reader, model func(string) string, skills, agents map[string]bool) (telemetrypkg.CopilotSession, int64, bool) {
	s := telemetrypkg.CopilotSession{
		Client: "copilot", Skills: map[telemetrypkg.SkillKey]int64{}, Tools: map[telemetrypkg.ToolKey]int64{},
		Hooks: map[telemetrypkg.HookKey]int64{}, Agents: map[string]int64{}, Models: map[string]telemetrypkg.SessionModel{},
		DurationSeconds: -1,
	}
	var done telemetrypkg.CopilotSession
	var doneAt, offset int64
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
		case "session.shutdown":
			var d sessionShutdown
			if !decode(&d) {
				break
			}
			for id, m := range d.ModelMetrics {
				k := model(id)
				sm := s.Models[k]
				sm.Input += m.TokenDetails["input"].TokenCount
				sm.Output += m.TokenDetails["output"].TokenCount
				sm.CacheRead += m.TokenDetails["cache_read"].TokenCount
				sm.CacheWrite += m.TokenDetails["cache_write"].TokenCount
				sm.PremiumRequests += m.Requests.Cost
				sm.NanoAIU += m.TotalNanoAiu
				s.Models[k] = sm
			}
			s.ShutdownType = label(d.ShutdownType, shortLabel.MatchString(d.ShutdownType))
			s.APIDurationMS = d.TotalAPIDurationMs
			s.APIModel = model(d.CurrentModel)
			if end, err := time.Parse(time.RFC3339Nano, ev.Timestamp); err == nil && d.SessionStartTime > 0 {
				if sec := end.Sub(time.UnixMilli(d.SessionStartTime)) / time.Second; sec >= 0 {
					s.DurationSeconds = int64(sec)
				}
			}
			done, doneAt, found = s, offset, true
		}
		if bad {
			s.Skipped++
		}
	}
	return done, doneAt, found
}

// recordCopilotSessions records each Copilot session that ended since `since`
// and was not recorded before. A marker in the session directory holds the
// offset read so far, so a resumed session counts only what came after.
// Opted out, nothing is read.
//
// ponytail: two nav-pilot processes exiting at the same moment can both
// record one session; add a lock file if the dashboards show doubles.
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
		var from int64
		if b, err := os.ReadFile(marker); err == nil {
			from, _ = strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		}
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		if _, err := f.Seek(from, io.SeekStart); err != nil {
			f.Close()
			continue
		}
		if skills == nil {
			skills, agents = navItemNames()
		}
		s, n, ok := parseCopilotSession(f, configModelLabel, skills, agents)
		f.Close()
		if !ok {
			continue
		}
		telemetry.RecordCopilotSession(s)
		_ = os.WriteFile(marker, []byte(strconv.FormatInt(from+n, 10)), 0o600)
	}
}
