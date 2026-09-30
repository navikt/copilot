package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

// The client configs `nav-pilot mcp enable|disable` writes, and the registry
// entries it writes from. Nothing here runs at launch: a client config is
// only ever rewritten because the user ran enable or disable.

// MCPServerEntry is one server as Nav's MCP registry lists it.
type MCPServerEntry struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Status      string       `json:"status"` // from _meta when the registry serves it there
	Remotes     []MCPRemote  `json:"remotes"`
	Packages    []MCPPackage `json:"packages"`
	// Setup is the registry's setupInstructions (_meta), steps the user
	// takes once, such as a cplt config key or a browser download.
	Setup []MCPSetupStep `json:"setup,omitempty"`
	// SandboxHosts is the registry's sandboxHosts (_meta): hosts the server
	// needs besides its remotes, such as Figma's OAuth at api.figma.com.
	SandboxHosts []string `json:"sandboxHosts,omitempty"`
	// Tools is the registry's list of the server's tools (_meta), and
	// ToolRisk the class of each that is more than a read (mcp_tools.go).
	Tools    []string          `json:"tools,omitempty"`
	ToolRisk map[string]string `json:"toolRisk,omitempty"`
}

// MCPRemote is a server reached over HTTP.
type MCPRemote struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// MCPPackage is a server the client starts as a local process.
type MCPPackage struct {
	RegistryType string `json:"registryType"`
	Identifier   string `json:"identifier"`
	Version      string `json:"version"`
	Arguments    []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"packageArguments"`
	Env []struct {
		Name       string `json:"name"`
		IsRequired bool   `json:"isRequired"`
	} `json:"environmentVariables"`
}

// MCPSetupStep is one of the registry's setup instructions.
type MCPSetupStep struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
}

// Usable is false for a server the registry has retired.
func (e MCPServerEntry) Usable() bool {
	return e.Status == "" || e.Status == "active"
}

// Launch is how the client starts a package server: the runtime and its
// arguments, the same recipe Min Copilot shows (install-commands.ts). Empty
// runtime for a registry type nav-pilot has no recipe for.
func (p MCPPackage) Launch() (runtime string, args []string) {
	switch p.RegistryType {
	case "npm":
		id := p.Identifier
		if p.Version != "" {
			id += "@" + p.Version
		}
		runtime, args = "pnpm", []string{"dlx", id}
	case "pypi":
		runtime, args = "uvx", []string{p.Identifier}
	default:
		return "", nil
	}
	for _, a := range p.Arguments {
		if a.Name != "" {
			args = append(args, a.Name)
		}
		if a.Value != "" {
			args = append(args, a.Value)
		}
	}
	return runtime, args
}

// MCPRegistryServers is the servers of the registry the org policy names, or
// Nav's own when there is no policy to ask: the same choice as the launch,
// and the same cache (mcp_hosts.go). A cache past its day, or one without
// entries, is read again first: nav-pilot mcp is asked for, so it may wait
// for gh and the registry, each bounded by [mcpPolicyTimeout]. When that
// read fails, a cache there is still answers, and stale says why it is old.
func MCPRegistryServers() (registry string, servers []MCPServerEntry, stale, err error) {
	c, ok := readMCPRegistryCache()
	if !ok || c.Registry.Entries == nil || c.Schema < mcpRegistryCacheSchema || time.Since(c.At) > mcpRegistryTTL {
		if rerr := refreshMCPRegistry(); rerr != nil {
			if !ok || c.Registry.Entries == nil {
				return "", nil, nil, rerr
			}
			stale = fmt.Errorf("%w; showing its answer from %s", rerr, c.At.Local().Format("2006-01-02 15:04"))
		} else if c, ok = readMCPRegistryCache(); !ok {
			return "", nil, nil, errMCPRegistryNotRead
		}
	}
	return c.Registry.URL, c.Registry.Entries, stale, nil
}

// NarrowMCPApproval cuts the recorded MCP host approval down to the hosts the
// servers in either client's user config still need, by the cached registry
// answer, and returns the hosts it dropped. Nothing changes without an
// approval or a cache.
func NarrowMCPApproval() (gone []string, err error) {
	rec, err := readMCPRecord()
	if err != nil || rec == nil || !rec.Approved {
		return nil, err
	}
	c, ok := readMCPRegistryCache()
	if !ok {
		return nil, nil
	}
	cur := matchMCPHosts(c.Registry, copilotMCPServerNames(), openCodeUserMCPServers())
	var keep []MCPHost
	for _, h := range recordedMCPHosts(rec) {
		if slices.ContainsFunc(cur.Hosts, func(o MCPHost) bool { return o.Host == h.Host }) {
			keep = append(keep, h)
		} else {
			gone = append(gone, h.Host)
		}
	}
	if len(gone) == 0 {
		return nil, nil
	}
	return gone, RecordMCPHosts(keep, true)
}

// MCP clients nav-pilot can write a server into.
const (
	MCPClientCopilot  = "copilot"
	MCPClientOpenCode = "opencode"
)

// MCPConfigPath is the user-level config file the client reads its MCP
// servers from. For OpenCode an existing opencode.jsonc is used when there
// is no opencode.json.
func MCPConfigPath(client string) (string, error) {
	switch client {
	case MCPClientCopilot:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".copilot", "mcp-config.json"), nil
	case MCPClientOpenCode:
		dir := artifacts.OpenCodeConfigDir()
		p := filepath.Join(dir, "opencode.json")
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat(filepath.Join(dir, "opencode.jsonc")); err == nil {
				return filepath.Join(dir, "opencode.jsonc"), nil
			}
		}
		return p, nil
	}
	return "", fmt.Errorf("unknown client %q: use copilot or opencode", client)
}

// mcpServersKey is the top-level key the client keeps its servers under.
func mcpServersKey(client string) string {
	if client == MCPClientOpenCode {
		return "mcp"
	}
	return "mcpServers"
}

// MCPClientEntry is the config entry for the server in the client's format,
// with Copilot's tools list from the choice (OpenCode keeps it as permission
// rules, which SetMCPServer writes). A remote is preferred over a package: it
// needs no runtime, no package cache and nothing run from it inside cplt.
func MCPClientEntry(client string, e MCPServerEntry, c MCPToolChoice) (json.RawMessage, error) {
	var v any
	tools := copilotTools(c)
	switch {
	case len(e.Remotes) > 0:
		r := e.Remotes[0]
		if c.URL != "" {
			r.URL = c.URL
		} else if ro := e.GitHubReadonlyURL(); ro != "" {
			// GitHub's full endpoint only when the choice names it.
			r.URL = ro
		}
		if client == MCPClientOpenCode {
			v = map[string]any{"type": "remote", "url": r.URL, "enabled": true}
		} else {
			t := "http"
			if r.Type == "sse" {
				t = "sse"
			}
			v = map[string]any{"type": t, "url": r.URL, "tools": tools}
		}
	case len(e.Packages) > 0:
		runtime, args := e.Packages[0].Launch()
		if runtime == "" {
			return nil, fmt.Errorf("%s is a %s package, which nav-pilot has no launch recipe for", e.Name, e.Packages[0].RegistryType)
		}
		if client == MCPClientOpenCode {
			v = map[string]any{"type": "local", "command": append([]string{runtime}, args...), "enabled": true}
		} else {
			v = map[string]any{"type": "local", "command": runtime, "args": args, "tools": tools}
		}
	default:
		return nil, fmt.Errorf("the registry lists no remote or package for %s", e.Name)
	}
	return json.Marshal(v)
}

// ErrMCPConfigHasComments is a config with comments or trailing commas, which
// a rewrite would lose; the caller shows the entry to paste instead.
var ErrMCPConfigHasComments = errors.New("the file has comments or trailing commas, which nav-pilot would not keep")

// MCPConfigChange is what a write did.
type MCPConfigChange struct {
	Path    string
	Backup  string // "" when there was no file to back up, or nothing changed
	Changed bool
	// Existing is the entry already under the name when it differs from the
	// one asked for; it is kept, not replaced.
	Existing json.RawMessage
}

// SetMCPServer adds the entry under name in the client's config, and for
// OpenCode the permission rules for the choice of tools. Every other key and
// server is kept as it was, in its order. An equal entry changes nothing; a
// different one under the same name is kept and returned.
func SetMCPServer(client, name string, entry json.RawMessage, e MCPServerEntry, c MCPToolChoice) (MCPConfigChange, error) {
	return editMCPConfig(client, func(top, servers *jsonObject) (json.RawMessage, bool, error) {
		if old, ok := servers.vals[name]; ok {
			if jsonEqual(old, entry) {
				return nil, false, nil
			}
			return old, false, nil
		}
		if client == MCPClientOpenCode {
			if err := setOpenCodeRules(top, name, e, c); err != nil {
				return nil, false, err
			}
		}
		servers.set(name, entry)
		return nil, true, nil
	})
}

// RemoveMCPServer drops name from the client's config. Absent is no change.
// For OpenCode its permission rules go too (dropOpenCodeRules); tools is the
// registry's tool list for it, nil when unknown.
func RemoveMCPServer(client, name string, tools []string) (MCPConfigChange, error) {
	return editMCPConfig(client, func(top, servers *jsonObject) (json.RawMessage, bool, error) {
		if !servers.del(name) {
			return nil, false, nil
		}
		if client == MCPClientOpenCode {
			dropOpenCodeRules(top, name, tools, servers.keys)
		}
		return nil, true, nil
	})
}

// openCodeBuiltinPermissions is OpenCode's own permission keys with an
// underscore, the ones a server's tool prefix can look like ("external_"
// and external_directory).
var openCodeBuiltinPermissions = []string{"external_directory", "doom_loop"}

// dropOpenCodeRules removes the permission rules of a removed server. With
// its tool list known, that is its deny-all and one key per tool. Without,
// every key under its tool prefix, except OpenCode's own keys and those
// under the longer prefix of a server still there ("a_b_x" is a_b's when
// both a and a_b exist).
func dropOpenCodeRules(top *jsonObject, name string, tools, others []string) {
	perm, err := parseJSONObject(top.vals["permission"])
	if err != nil {
		return
	}
	prefix := openCodeToolPrefix(name)
	for _, k := range slices.Clone(perm.keys) {
		if !strings.HasPrefix(k, prefix) || slices.Contains(openCodeBuiltinPermissions, k) {
			continue
		}
		if tools != nil && k != prefix+"*" && !slices.ContainsFunc(tools, func(t string) bool { return openCodeToolID(name, t) == k }) {
			continue
		}
		if slices.ContainsFunc(others, func(o string) bool {
			p := openCodeToolPrefix(o)
			return len(p) > len(prefix) && strings.HasPrefix(k, p)
		}) {
			continue
		}
		perm.del(k)
	}
	if len(perm.keys) == 0 {
		top.del("permission")
		return
	}
	top.set("permission", perm.marshal())
}

// MCPConfigKeys is the server names in the client's user config, in order.
func MCPConfigKeys(client string) []string {
	path, err := MCPConfigPath(client)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if client == MCPClientOpenCode {
		data = stripJSONC(data)
	}
	top, err := parseJSONObject(data)
	if err != nil {
		return nil
	}
	servers, err := parseJSONObject(top.vals[mcpServersKey(client)])
	if err != nil {
		return nil
	}
	return servers.keys
}

// MCPConfigKeyFor is the name the server already has in the client's user
// config, or "". Copilot's policy knows a server only by the registry's
// name; OpenCode's also by its URL or package, so a server the user added
// by hand under a name of their own is found too.
func MCPConfigKeyFor(client string, e MCPServerEntry) string {
	keys := MCPConfigKeys(client)
	if slices.Contains(keys, e.Name) {
		return e.Name
	}
	if client != MCPClientOpenCode {
		return ""
	}
	path, _ := MCPConfigPath(client)
	data, _ := os.ReadFile(path)
	var cfg struct {
		MCP map[string]mcpServer `json:"mcp"`
	}
	if json.Unmarshal(stripJSONC(data), &cfg) != nil {
		return ""
	}
	for _, k := range keys {
		if openCodeRegistryName([]MCPServerEntry{e}, k, cfg.MCP[k]) != "" {
			return k
		}
	}
	return ""
}

func editMCPConfig(client string, edit func(top, servers *jsonObject) (json.RawMessage, bool, error)) (MCPConfigChange, error) {
	path, err := MCPConfigPath(client)
	if err != nil {
		return MCPConfigChange{}, err
	}
	ch := MCPConfigChange{Path: path}
	// Write through a symlink (a dotfiles repo), never over it.
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	mode := os.FileMode(0o600) // a client config can hold tokens in headers
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		data = nil
	case err != nil:
		return ch, err
	default:
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
	}
	if len(bytes.TrimSpace(data)) > 0 && !bytes.Equal(stripJSONC(data), data) {
		return ch, fmt.Errorf("%s: %w", ch.Path, ErrMCPConfigHasComments)
	}
	top := jsonObject{vals: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(data)) > 0 {
		if top, err = parseJSONObject(data); err != nil {
			return ch, fmt.Errorf("%s is not a JSON object, so nav-pilot left it alone: %w", ch.Path, err)
		}
	}
	key := mcpServersKey(client)
	servers := jsonObject{vals: map[string]json.RawMessage{}}
	if raw, ok := top.vals[key]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if servers, err = parseJSONObject(raw); err != nil {
			return ch, fmt.Errorf("%q in %s is not an object, so nav-pilot left it alone: %w", key, ch.Path, err)
		}
	}
	// A new file gets the schema first and the servers before any
	// permission rules the edit adds.
	if client == MCPClientOpenCode && len(top.keys) == 0 {
		top.set("$schema", json.RawMessage(`"https://opencode.ai/config.json"`))
	}
	if _, ok := top.vals[key]; !ok {
		top.set(key, json.RawMessage(`{}`))
	}
	existing, changed, err := edit(&top, &servers)
	if err != nil {
		return ch, fmt.Errorf("%s: %w", ch.Path, err)
	}
	ch.Existing = existing
	if !changed {
		return ch, nil
	}
	top.set(key, servers.marshal())
	var out bytes.Buffer
	if err := json.Indent(&out, top.marshal(), "", "  "); err != nil {
		return ch, err
	}
	out.WriteByte('\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ch, err
	}
	if data != nil {
		ch.Backup = ch.Path + ".bak"
		if err := writeMCPConfigAtomic(path+".bak", data, mode); err != nil {
			return ch, fmt.Errorf("backing up %s: %w", ch.Path, err)
		}
	}
	if err := writeMCPConfigAtomic(path, out.Bytes(), mode); err != nil {
		return ch, err
	}
	ch.Changed = true
	return ch, nil
}

func writeMCPConfigAtomic(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(mode)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// jsonObject is a JSON object with its keys in document order and its values
// untouched, so a rewrite keeps what nav-pilot did not mean to change.
type jsonObject struct {
	keys []string
	vals map[string]json.RawMessage
}

func parseJSONObject(data []byte) (jsonObject, error) {
	o := jsonObject{vals: map[string]json.RawMessage{}}
	dec := json.NewDecoder(bytes.NewReader(data))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return o, errors.New("not a JSON object")
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return o, err
		}
		k, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return o, err
		}
		o.set(k, v)
	}
	if _, err := dec.Token(); err != nil {
		return o, err
	}
	return o, nil
}

func (o *jsonObject) set(k string, v json.RawMessage) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *jsonObject) del(k string) bool {
	if _, ok := o.vals[k]; !ok {
		return false
	}
	delete(o.vals, k)
	o.keys = slices.DeleteFunc(o.keys, func(s string) bool { return s == k })
	return true
}

func (o jsonObject) marshal() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		b.Write(o.vals[k])
	}
	b.WriteByte('}')
	return b.Bytes()
}

func jsonEqual(a, b json.RawMessage) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && fmt.Sprint(x) == fmt.Sprint(y)
}

// MCPConfigured is the registry servers the user's clients have configured,
// by registry name, and the configured names the registry does not list.
type MCPConfigured struct {
	Copilot       map[string]bool
	OpenCode      map[string]bool
	CopilotOther  []string
	OpenCodeOther []string
	// OpenCodeProject is the servers OpenCode loads here from a project's
	// config or an OPENCODE_CONFIG* variable. Shown, never counted: the
	// repository chose them, not the user.
	OpenCodeProject []string
}

// ConfiguredMCPServers matches the user's client configs against the
// registry. Copilot's policy matches on the server's name only, so a Copilot
// server under any other name is unlisted even when its URL is the
// registry's (#601). OpenCode matches by name, remote URL or package, as
// the launch's policy check does; a server turned off does not count.
func ConfiguredMCPServers(entries []MCPServerEntry) MCPConfigured {
	c := MCPConfigured{Copilot: map[string]bool{}, OpenCode: map[string]bool{}}
	byName := map[string]bool{}
	for _, e := range entries {
		byName[e.Name] = true
	}
	for _, k := range copilotMCPServerNames() {
		if byName[k] {
			c.Copilot[k] = true
		} else {
			c.CopilotOther = append(c.CopilotOther, k)
		}
	}
	oc := openCodeUserMCPServers()
	for k := range openCodeMCPServers("", os.Environ()) {
		if _, ok := oc[k]; !ok {
			c.OpenCodeProject = append(c.OpenCodeProject, k)
		}
	}
	slices.Sort(c.OpenCodeProject)
	keys := make([]string, 0, len(oc))
	for k := range oc {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		s := oc[k]
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		if name := openCodeRegistryName(entries, k, s); name != "" {
			c.OpenCode[name] = true
		} else {
			c.OpenCodeOther = append(c.OpenCodeOther, k)
		}
	}
	return c
}

func openCodeRegistryName(entries []MCPServerEntry, key string, s mcpServer) string {
	pkg := localPackage(s.Command)
	if i := strings.LastIndex(pkg, "@"); i > 0 {
		pkg = pkg[:i]
	}
	for _, e := range entries {
		if e.Name == key {
			return e.Name
		}
		for _, r := range e.Remotes {
			if s.Type == "remote" && normalizeMCPURL(r.URL) == normalizeMCPURL(s.URL) {
				return e.Name
			}
		}
		// enable writes GitHub's read-only endpoint; it is the same server.
		if ro := e.GitHubReadonlyURL(); s.Type == "remote" && ro != "" && IsGitHubReadonlyURL(s.URL) {
			return e.Name
		}
		for _, p := range e.Packages {
			if s.Type == "local" && pkg != "" && strings.EqualFold(p.Identifier, pkg) {
				return e.Name
			}
		}
	}
	return ""
}
