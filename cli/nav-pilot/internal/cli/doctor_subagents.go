package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Copilot CLI lets ~/.copilot/settings.json override the model a custom agent
// runs on when it is started as a subagent:
//
//	{"subagents": {"agents": {"security-champion": {"model": "inherit"}}}}
//
// The key is the agent's frontmatter `name:`, not its filename (measured on
// CLI 1.0.94-3: a filename key had no effect). An override beats the agent's
// own `model:` pin, so a pinned reviewer silently runs on the parent model. A
// model the client does not know is worse: the CLI falls back to the parent
// model and still labels the run with the unknown name
// (docs/golden-baselines/2026-10-08-subagent-arv, runs D1/D2).
//
// doctor only reads the file and warns. It is the user's file, and inside cplt
// it is read-only (or unreadable) anyway.

const subagentSettingsLabel = "~/.copilot/settings.json"

// readSubagentModels returns subagents.agents.<key>.model from a Copilot
// settings file. A missing file is no overrides, not an error.
func readSubagentModels(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s struct {
		Subagents struct {
			Agents map[string]struct {
				Model string `json:"model"`
			} `json:"agents"`
		} `json:"subagents"`
	}
	if err := json.Unmarshal(stripJSONC(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))), &s); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range s.Subagents.Agents {
		if m := strings.TrimSpace(v.Model); m != "" {
			out[k] = m
		}
	}
	return out, nil
}

// stripJSONC removes // and /* */ comments and trailing commas outside
// strings, so a hand-edited settings file still parses.
func stripJSONC(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inStr := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch {
		case inStr:
			out = append(out, c)
			if c == '\\' && i+1 < len(in) {
				i++
				out = append(out, in[i])
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			i--
		case c == '/' && i+1 < len(in) && in[i+1] == '*':
			end := strings.Index(string(in[i+2:]), "*/")
			if end < 0 {
				return append(out, in[i:]...) // leave it for the decoder to reject
			}
			i += end + 3
		case c == '}' || c == ']':
			// Drop a trailing comma before the closer.
			j := len(out) - 1
			for j >= 0 && strings.IndexByte(" \t\r\n", out[j]) >= 0 {
				j--
			}
			if j >= 0 && out[j] == ',' {
				out = append(out[:j], out[j+1:]...)
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// subagentOverrideWarnings compares each pinned agent with its override.
// catalogue is the client's model list; nil means it could not be read, and
// then an override that is not inherit and not a pinned agent's other model is
// reported as unchecked, never as unknown: nav-pilot's own list comes from
// models.dev and is not this account's catalogue.
//
// Assumption, not measured: Copilot resolves a display label ("Claude Opus
// 5.5") in settings.json the way it does in frontmatter, so labels and ids are
// compared alike.
func subagentOverrideWarnings(pins []pinnedModel, overrides map[string]string, catalogue []string) []subagentWarning {
	known := func(model string) bool {
		if strings.EqualFold(model, "auto") || strings.EqualFold(domain.CopilotModelIDForLabel(model), "auto") {
			return true // resolved client-side, never in the catalogue
		}
		id := domain.CopilotModelIDForLabel(model)
		if id == "" {
			id = model
		}
		for _, c := range catalogue {
			if strings.EqualFold(c, id) {
				return true
			}
		}
		return false
	}
	pinned := map[string]pinnedModel{}
	for _, p := range pins {
		pinned[p.Name] = p
	}
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var warns []subagentWarning
	for _, k := range keys {
		o := overrides[k]
		p, isPinned := pinned[k]
		agent := "@" + k
		if isPinned {
			agent = "@" + p.Agent // what the user types; the key is the frontmatter name
		}
		var msg string
		unchecked := false
		switch {
		case strings.EqualFold(o, "inherit"):
			if isPinned {
				msg = fmt.Sprintf("%s is pinned to %s, but %s overrides it to inherit for subagents; delegated runs use the parent model.",
					agent, p.Label, subagentSettingsLabel)
			}
		case catalogue == nil && !isPinned:
			msg = fmt.Sprintf("%s sets the subagent model for %s to %q; the client's model list could not be read, so it was not checked.",
				subagentSettingsLabel, agent, o)
			unchecked = true
		case catalogue != nil && !known(o):
			msg = fmt.Sprintf("%s sets the subagent model for %s to %q, which Copilot does not know; the CLI silently runs it on the parent model and still labels it %q.",
				subagentSettingsLabel, agent, o, o)
		case isPinned && !overrideMatchesPin(o, p):
			msg = fmt.Sprintf("%s is pinned to %s, but %s overrides it to %s for subagents; delegated runs use %s.",
				agent, p.Label, subagentSettingsLabel, o, o)
		}
		if msg != "" {
			warns = append(warns, subagentWarning{Key: k, Msg: msg, Unchecked: unchecked})
		}
	}
	return warns
}

type subagentWarning struct {
	Key, Msg  string
	Unchecked bool // could not be checked; not a warning
}

func overrideMatchesPin(override string, p pinnedModel) bool {
	if strings.EqualFold(override, p.Label) {
		return true
	}
	id := domain.CopilotModelIDForLabel(override)
	return id != "" && strings.EqualFold(id, p.ID)
}

// reportSubagentOverrides prints the subagent-override section of doctor.
// Warn-only: it never fails the health check and never writes.
func reportSubagentOverrides(w io.Writer, path string, pins []pinnedModel, catalogue []string) {
	overrides, err := readSubagentModels(path)
	if err != nil {
		fmt.Fprintf(w, "    %s Could not read %s, so subagent overrides were not checked: %v\n", dim("-"), subagentSettingsLabel, err)
		return
	}
	warns := subagentOverrideWarnings(pins, overrides, catalogue)
	if len(warns) == 0 {
		fmt.Fprintf(w, "    %s No subagent override changes an agent's model pin\n", green("✓"))
		return
	}
	for _, wn := range warns {
		if wn.Unchecked {
			fmt.Fprintf(w, "    %s %s\n", dim("-"), wn.Msg)
			continue
		}
		fmt.Fprintf(w, "    %s %s\n", yellow("⚠"), wn.Msg)
		fmt.Fprintf(w, "      %s Remove subagents.agents.%s.model from %s, or set it to the agent's own model.\n",
			yellow("Solution:"), wn.Key, subagentSettingsLabel)
	}
}
