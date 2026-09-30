package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
)

// Which of a server's tools the client turns on. The registry classes each
// tool (toolRisk in _meta); nav-pilot mcp enable turns on reads and writes
// and leaves the rest off until the user picks them. Copilot keeps the
// choice as the server's tools list, OpenCode as permission rules. Nothing
// here runs at launch.

// Tool risk classes, as the registry serves them. A tool with no class is a
// read.
const (
	MCPRiskRead     = "read"
	MCPRiskWrite    = "write"
	MCPRiskExternal = "external"  // a visible effect in another system
	MCPRiskHostExec = "host-exec" // runs on the user's machine, outside cplt
)

// GitHub's MCP server has a read-only endpoint. nav-pilot uses it unless the
// user picks a tool that writes to GitHub: a write through MCP goes around
// cplt's guard on gh.
const (
	githubMCPURL         = "https://api.githubcopilot.com/mcp/"
	githubMCPReadonlyURL = "https://api.githubcopilot.com/mcp/readonly"
)

// RiskOf is the tool's class; read when the registry gives none.
func (e MCPServerEntry) RiskOf(tool string) string {
	if r := e.ToolRisk[tool]; r != "" {
		return r
	}
	return MCPRiskRead
}

// DefaultTools is the tools enable turns on: reads and writes, in the
// registry's order.
func (e MCPServerEntry) DefaultTools() []string {
	var out []string
	for _, t := range e.Tools {
		if r := e.RiskOf(t); r == MCPRiskRead || r == MCPRiskWrite {
			out = append(out, t)
		}
	}
	return out
}

// HostExecTools is the server's tools that run outside the sandbox.
func (e MCPServerEntry) HostExecTools() []string {
	var out []string
	for _, t := range e.Tools {
		if e.RiskOf(t) == MCPRiskHostExec {
			out = append(out, t)
		}
	}
	return out
}

// Loopback is a server an app on the user's machine serves on localhost.
func (e MCPServerEntry) Loopback() bool {
	for _, r := range e.Remotes {
		if u, err := url.Parse(r.URL); err == nil {
			ip := net.ParseIP(u.Hostname())
			if strings.EqualFold(u.Hostname(), "localhost") || (ip != nil && ip.IsLoopback()) {
				return true
			}
		}
	}
	return false
}

// GitHubReadonlyURL is the read-only endpoint for GitHub's MCP server, ""
// for any other server.
func (e MCPServerEntry) GitHubReadonlyURL() string {
	if len(e.Remotes) > 0 && normalizeMCPURL(e.Remotes[0].URL) == normalizeMCPURL(githubMCPURL) {
		return githubMCPReadonlyURL
	}
	return ""
}

// IsGitHubReadonlyURL is whether u is GitHub's read-only MCP endpoint.
func IsGitHubReadonlyURL(u string) bool {
	return normalizeMCPURL(u) == normalizeMCPURL(githubMCPReadonlyURL)
}

// MCPToolChoice is the tools a client entry turns on.
type MCPToolChoice struct {
	All   bool     // every tool, including any the server adds later
	Tools []string // when not All
	// URL replaces the registry's remote URL (GitHub's read-only endpoint);
	// "" keeps it.
	URL string
}

// openCodeToolID is how OpenCode names a server's tools:
// S(key) + "_" + S(tool), where S replaces anything outside [a-zA-Z0-9_-].
func openCodeToolID(key, tool string) string {
	return openCodeToolPrefix(key) + openCodeIDChars.ReplaceAllString(tool, "_")
}

func openCodeToolPrefix(key string) string {
	return openCodeIDChars.ReplaceAllString(key, "_") + "_"
}

var openCodeIDChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// OpenCodeRules is the permission rules that turn on only the chosen tools:
// every tool of the server denied, then each chosen one allowed. OpenCode
// takes the last rule that matches, so the order is the meaning. All is no
// rules.
func OpenCodeRules(key string, c MCPToolChoice) [][2]string {
	if c.All {
		return nil
	}
	rules := [][2]string{{openCodeToolPrefix(key) + "*", "deny"}}
	for _, t := range c.Tools {
		rules = append(rules, [2]string{openCodeToolID(key, t), "allow"})
	}
	return rules
}

// setOpenCodeRules replaces the rules nav-pilot writes for the server (its
// deny-all and one per registry tool) with the choice's, at the end of the
// permission map. Rules of the user's own are kept where they are.
func setOpenCodeRules(top *jsonObject, key string, e MCPServerEntry, c MCPToolChoice) error {
	perm := jsonObject{vals: map[string]json.RawMessage{}}
	if raw, ok := top.vals["permission"]; ok {
		var err error
		if perm, err = parseJSONObject(raw); err != nil {
			return errors.New(`"permission" is not an object, so nav-pilot cannot add per-tool rules to it`)
		}
	}
	// An "ask" the user set, for every tool, the server or one tool, stays
	// an ask: a chosen tool is asked for, not allowed without a question.
	isAsk := func(k string) bool {
		var v string
		return json.Unmarshal(perm.vals[k], &v) == nil && v == "ask"
	}
	all := openCodeToolPrefix(key) + "*"
	serverAsk := isAsk("*") || isAsk(all)
	asked := map[string]bool{all: isAsk(all)}
	for _, t := range e.Tools {
		asked[openCodeToolID(key, t)] = isAsk(openCodeToolID(key, t))
	}
	perm.del(all)
	for _, t := range e.Tools {
		perm.del(openCodeToolID(key, t))
	}
	rules := OpenCodeRules(key, c)
	if c.All {
		// No deny to write, but the asks stay.
		if asked[all] {
			rules = append(rules, [2]string{all, "ask"})
		}
		for _, t := range e.Tools {
			if id := openCodeToolID(key, t); asked[id] {
				rules = append(rules, [2]string{id, "ask"})
			}
		}
	}
	for _, r := range rules {
		if r[1] == "allow" && (serverAsk || asked[r[0]]) {
			r[1] = "ask"
		}
		v, _ := json.Marshal(r[1])
		perm.set(r[0], v)
	}
	if len(perm.keys) == 0 {
		top.del("permission")
		return nil
	}
	top.set("permission", perm.marshal())
	return nil
}

// SetMCPServerTools changes only which tools an existing entry turns on:
// Copilot's tools list (and GitHub's URL), OpenCode's permission rules (and
// GitHub's URL). The rest of the entry is kept. A change leaves a backup.
func SetMCPServerTools(client, key string, e MCPServerEntry, c MCPToolChoice) (MCPConfigChange, error) {
	return editMCPConfig(client, func(top, servers *jsonObject) (json.RawMessage, bool, error) {
		raw, ok := servers.vals[key]
		if !ok {
			return nil, false, errors.New(key + " is not in the file")
		}
		entry, err := parseJSONObject(raw)
		if err != nil {
			return nil, false, errors.New(key + " is not an object, so nav-pilot left it alone")
		}
		before := compactJSON(top.marshal())
		if c.URL != "" {
			v, _ := json.Marshal(c.URL)
			entry.set("url", v)
		}
		if client == MCPClientOpenCode {
			if err := setOpenCodeRules(top, key, e, c); err != nil {
				return nil, false, err
			}
		} else {
			v, _ := json.Marshal(copilotTools(c))
			entry.set("tools", v)
		}
		servers.set(key, entry.marshal())
		// Unchanged is no write, so the backup of the last real change
		// stays. Order counts: OpenCode's rules are read in order.
		top.set(mcpServersKey(client), servers.marshal())
		return nil, !bytes.Equal(before, compactJSON(top.marshal())), nil
	})
}

func compactJSON(b []byte) []byte {
	var out bytes.Buffer
	if json.Compact(&out, b) != nil {
		return b
	}
	return out.Bytes()
}

func copilotTools(c MCPToolChoice) []string {
	if c.All {
		return []string{"*"}
	}
	if c.Tools == nil {
		return []string{}
	}
	return c.Tools
}

// MCPToolState is which tools a configured entry turns on.
type MCPToolState struct {
	All bool     // every tool, also those the registry does not list
	On  []string // the registry's tools that are on
	URL string   // the entry's URL, for a remote
}

// MCPToolsOn reads which of the server's tools the client's entry under key
// turns on. ok is false when the entry cannot be read.
// ponytail: OpenCode is read from the user file nav-pilot writes; rules in
// a project's config or OpenCode's legacy top-level "tools" map are not.
func MCPToolsOn(client, key string, e MCPServerEntry) (st MCPToolState, ok bool) {
	path, err := MCPConfigPath(client)
	if err != nil {
		return st, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st, false
	}
	top, err := parseJSONObject(stripJSONC(data))
	if err != nil {
		return st, false
	}
	servers, err := parseJSONObject(top.vals[mcpServersKey(client)])
	if err != nil {
		return st, false
	}
	var entry struct {
		URL   string          `json:"url"`
		Tools json.RawMessage `json:"tools"`
	}
	if json.Unmarshal(servers.vals[key], &entry) != nil {
		return st, false
	}
	st.URL = entry.URL
	if client == MCPClientOpenCode {
		var rules [][2]string
		if perm, err := parseJSONObject(top.vals["permission"]); err == nil {
			for _, k := range perm.keys {
				var v string
				if json.Unmarshal(perm.vals[k], &v) == nil {
					rules = append(rules, [2]string{k, v})
				}
			}
		}
		on := func(tool string) bool { return openCodeAllows(rules, openCodeToolID(key, tool)) }
		// A tool the registry has never heard of stands for the ones the
		// server adds later.
		st.All = on("\x00unlisted")
		for _, t := range e.Tools {
			if st.All || on(t) {
				st.On = append(st.On, t)
			}
		}
		return st, true
	}
	var tools []string
	if len(entry.Tools) > 0 && json.Unmarshal(entry.Tools, &tools) != nil {
		return st, false
	}
	// No tools field is every tool.
	st.All = len(entry.Tools) == 0 || slices.Contains(tools, "*")
	for _, t := range e.Tools {
		if st.All || slices.Contains(tools, t) {
			st.On = append(st.On, t)
		}
	}
	return st, true
}

// openCodeAllows is OpenCode's answer for a tool: the last rule whose
// pattern matches decides, and a tool no rule matches is allowed. ask still
// shows the tool to the model, so it counts as on.
func openCodeAllows(rules [][2]string, id string) bool {
	allow := true
	for _, r := range rules {
		if openCodeMatch(r[0], id) {
			allow = r[1] != "deny"
		}
	}
	return allow
}

// openCodeMatch is OpenCode's wildcard: * is any run, ? any one character.
func openCodeMatch(pattern, s string) bool {
	q := regexp.QuoteMeta(pattern)
	q = strings.ReplaceAll(q, `\*`, ".*")
	q = strings.ReplaceAll(q, `\?`, ".")
	ok, _ := regexp.MatchString("^"+q+"$", s)
	return ok
}

// CachedMCPRegistryEntries is the registry's servers from the cache only,
// nil without one: for doctor, which never waits for the registry here.
func CachedMCPRegistryEntries() []MCPServerEntry {
	c, _ := readMCPRegistryCache()
	return c.Registry.Entries
}
