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

	"github.com/google/uuid"
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

// recordCopilotSessions records the sessions in ids, the ones this launch
// started or resumed, that ended and were not recorded before. Any other
// session, such as a bare copilot running alongside, is never read. A marker
// in the session directory holds the offset read so far and the totals at
// that point, so a resumed session counts only what came after. Opted out,
// nothing is read.
//
// A lock file per session keeps two overlapping nav-pilot exits from both
// recording it: the one that loses the race skips the session.
func recordCopilotSessions(root string, ids []string) {
	if !telemetryEnabled() {
		return
	}
	var skills, agents map[string]bool
	for _, id := range ids {
		if !copilotSessionID.MatchString(id) {
			continue
		}
		p := filepath.Join(root, id, "events.jsonl")
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

// baselineCopilotSession runs at launch. An existing session with no marker,
// one run without nav-pilot before, gets one at the end of what is there, so
// recordCopilotSessions counts only what this launch adds. A new session has
// no events file yet and gets nothing. Opted out, nothing is read.
//
// The file is streamed, never held in memory: a long session runs past 100 MB,
// and that one-time read then costs the launch about two seconds. Copilot is
// not writing this session at launch, so the end of the file is the offset.
//
// It reports whether the session may be recorded at exit: false when an
// existing session could not get its marker, since recordCopilotSessions
// would then read it from the start and count the history before this launch.
func baselineCopilotSession(root, id string) bool {
	if !telemetryEnabled() || !copilotSessionID.MatchString(id) {
		return true
	}
	dir := filepath.Join(root, id)
	marker := filepath.Join(dir, sessionMetricsMarker)
	unlock, ok := lockSession(marker + ".lock")
	if !ok {
		return false
	}
	defer unlock()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		return err == nil
	}
	f, err := os.Open(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		return os.IsNotExist(err) // a new session
	}
	defer f.Close()
	_, mark, _ := parseCopilotSession(f, configModelLabel, nil, nil, sessionTotals{})
	if mark.Offset, err = f.Seek(0, io.SeekEnd); err != nil {
		return false
	}
	b, err := json.Marshal(mark)
	if err != nil {
		return false
	}
	return os.WriteFile(marker, b, 0o600) == nil
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

// copilotSessionID is a session id as Copilot names its session directory.
var copilotSessionID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// copilotValueFlags are Copilot CLI's global options that take a required
// value, from `copilot --help` of GitHub Copilot CLI 1.0.95 (2026-10-12).
// Options with an optional value ([<value>], [<tools>...]) are left out on
// purpose: a word after them may be their value or a subcommand, and
// copilotSessionArgs then adds nothing. -r, --resume and --session-id are read
// separately below. A flag missing here errs the same safe way.
var copilotValueFlags = setOf("-i", "--interactive", "-p", "--prompt", "--model",
	"--reasoning-effort", "--context", "--auto-tier", "--agent", "-n", "--name",
	"--session-id", "-C", "--log-dir", "--extension-sdk-path", "--log-level",
	"--stream", "--output-format", "--add-dir", "--attachment", "--disable-mcp-server",
	"--add-github-mcp-toolset", "--add-github-mcp-tool", "--plugin-dir",
	"--additional-mcp-config", "--mcp-github-auth", "--max-autopilot-continues",
	"--mode", "--dynamic-retrieval", "--enable-mcp-server", "--max-ai-credits",
	"--usage-output-file")

// copilotSessionArgs picks the session a Copilot launch will run, from the
// arguments after "--". With none that choose a session, it is a new one
// with an id of nav-pilot's own, passed as --session-id; the caller drops it
// for a Copilot CLI before 1.0.51 ([provider.CopilotTakesSessionID]). A full
// id after --resume, -r or --session-id is that session. Anything else
// (--continue, the picker, a name or id prefix, a subcommand) leaves the id
// unknown: nothing is added and no session is recorded.
//
// A subcommand may follow global flags (--model x sessions). Any word that is
// not the value of a flag in copilotValueFlags is taken as one, and so is "--":
// when unsure, nothing is added.
func copilotSessionArgs(extra []string) (id string, add []string) {
	for i := 0; i < len(extra); i++ {
		a := extra[i]
		if a == "--" || !strings.HasPrefix(a, "-") {
			return "", nil
		}
		if copilotValueFlags[a] || (a == "-r" || a == "--resume") && i+1 < len(extra) && copilotSessionID.MatchString(extra[i+1]) {
			i++ // its value, whatever it looks like
		}
	}
	for i, a := range extra {
		name, val, eq := strings.Cut(a, "=")
		switch name {
		case "--session-id", "--resume", "-r":
			if !eq && i+1 < len(extra) {
				val = extra[i+1]
			}
			if copilotSessionID.MatchString(val) {
				return val, nil
			}
			return "", nil
		case "--continue", "--connect":
			return "", nil
		}
		if len(a) > 2 && a[:2] == "-r" && a[2] != '-' {
			// glued short form: -r<id>
			if copilotSessionID.MatchString(a[2:]) {
				return a[2:], nil
			}
			return "", nil
		}
	}
	id = uuid.NewString()
	return id, []string{"--session-id=" + id}
}
