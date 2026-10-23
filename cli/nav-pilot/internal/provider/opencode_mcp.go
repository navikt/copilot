package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// The MCP registry policy, applied to OpenCode (#1027).
//
// Copilot CLI asks GitHub which MCP registry Nav's org policy names
// (GET /copilot/mcp_registry) and, when the policy says registry_only, runs
// only the servers that registry lists. OpenCode asks no one. So a nav-pilot
// launch of OpenCode asks the same question, through `gh api`, fetches the
// same registry, and turns off every configured MCP server it does not list,
// with {"enabled": false} in OPENCODE_CONFIG_CONTENT. The user's opencode.json
// is never edited, and a plain `opencode` is not affected.
//
// A remote server matches a registry remote by URL. A local server matches a
// registry package when its command names the package (npx @playwright/mcp).
//
// A question nobody answered changes nothing: no gh, no policy, or a registry
// that did not answer leaves every server as it is, with a warning. Only an
// answer the registry gave turns a server off.

// MCPRegistryHelpURL is where the message sends someone whose server was
// turned off: the approved list, and how to get one added.
const MCPRegistryHelpURL = "https://ki-utvikling.nav.no/verktoy (approved servers); to add one: https://github.com/navikt/copilot/blob/main/apps/mcp-registry/README.md#adding-servers"

// MCPPolicyTimeout bounds each gh and registry call. A var so the e2e build
// can give its fake gh more time on a loaded machine.
var MCPPolicyTimeout = 5 * time.Second

var envPlaceholder = regexp.MustCompile(`\{env:([^}]+)\}`)

// MCPBlockedEnv carries the servers the policy turned off, and the others,
// as {"blocked": [...], "listed": [...]}, to the hooks bridge, which refuses
// the blocked servers' tools: enabled=false is only how the session
// starts, and OpenCode's /mcp dialog can connect a server anyway.
const MCPBlockedEnv = "NAV_PILOT_MCP_BLOCKED"

// mcpServer is one server in the user's OpenCode config.
type mcpServer struct {
	Type        string            `json:"type"`
	URL         string            `json:"url"`
	Command     []string          `json:"command"`
	Enabled     *bool             `json:"enabled"`
	Environment map[string]string `json:"environment"`
	Env         map[string]string `json:"env"`
	hadEnv      bool
	hadEnvName  bool
}

// mcpRegistry is what the registry lists: remote URLs and package ids.
type mcpRegistry struct {
	URL      string
	Remotes  map[string]bool
	Packages map[string]bool
	// Servers is each listed server's remote URLs, by the registry's name
	// for it (io.github.navikt/github-mcp). The only source of the hosts an
	// MCP server may reach under cplt (mcp_hosts.go).
	Servers map[string][]string
	// Entries is every listed server in full, in the registry's order, for
	// nav-pilot mcp (mcp_servers.go).
	Entries []MCPServerEntry
}

// fetchMCPPolicy and fetchMCPRegistry are asked once per process: an OpenCode
// launch asks both for its policy check and for the MCP hosts (mcp_hosts.go).
// Vars so tests answer them.
var (
	fetchMCPPolicy   = sync.OnceValues(askMCPPolicy)
	fetchMCPRegistry = memoMCPRegistry(askMCPRegistry)
	// mcpHTTPClient is the registry's client, a var so a test can fail on use.
	mcpHTTPClient = &http.Client{}
)

func memoMCPRegistry(ask func(string) (mcpRegistry, error)) func(string) (mcpRegistry, error) {
	var mu sync.Mutex
	type answer struct {
		reg mcpRegistry
		err error
	}
	seen := map[string]answer{}
	return func(base string) (mcpRegistry, error) {
		mu.Lock()
		defer mu.Unlock()
		if a, ok := seen[base]; ok {
			return a.reg, a.err
		}
		reg, err := ask(base)
		seen[base] = answer{reg, err}
		return reg, err
	}
}

// askMCPPolicy asks GitHub, as the user, which registry the org policy
// names. registry is "" when no policy restricts MCP servers.
func askMCPPolicy() (registry string, err error) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), MCPPolicyTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, gh, "api", "/copilot/mcp_registry").Output()
	if err != nil {
		return "", fmt.Errorf("gh api /copilot/mcp_registry: %w", err)
	}
	var resp struct {
		Registries []struct {
			URL    string                 `json:"url"`
			Access string                 `json:"registry_access"`
			Owner  struct{ Priority int } `json:"owner"`
		} `json:"mcp_registries"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("reading the MCP policy: %w", err)
	}
	sort.SliceStable(resp.Registries, func(i, j int) bool {
		return resp.Registries[i].Owner.Priority < resp.Registries[j].Owner.Priority
	})
	for _, r := range resp.Registries {
		if r.URL != "" {
			switch r.Access {
			case "registry_only":
				return r.URL, nil
			case "", "allow_all":
				return "", nil
			default:
				return "", fmt.Errorf("the MCP policy has registry_access %q, which nav-pilot does not know", r.Access)
			}
		}
	}
	return "", nil
}

// askMCPRegistry lists a registry's servers (MCP Registry v0.1).
func askMCPRegistry(base string) (mcpRegistry, error) {
	reg := mcpRegistry{URL: base, Remotes: map[string]bool{}, Packages: map[string]bool{}, Servers: map[string][]string{}}
	ctx, cancel := context.WithTimeout(context.Background(), MCPPolicyTimeout)
	defer cancel()
	client := mcpHTTPClient
	cursor := ""
	for range 20 {
		u := strings.TrimSuffix(base, "/") + "/v0.1/servers?limit=100"
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return reg, err
		}
		res, err := client.Do(req)
		if err != nil {
			return reg, err
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		res.Body.Close()
		if err != nil {
			return reg, err
		}
		if res.StatusCode != http.StatusOK {
			return reg, fmt.Errorf("%s answered %s", u, res.Status)
		}
		var page struct {
			Servers []struct {
				Server MCPServerEntry `json:"server"`
				Meta   struct {
					Official struct {
						Status string `json:"status"`
					} `json:"io.modelcontextprotocol.registry/official"`
					Nav struct {
						Setup        []MCPSetupStep    `json:"setupInstructions"`
						SandboxHosts []string          `json:"sandboxHosts"`
						Tools        []string          `json:"tools"`
						ToolRisk     map[string]string `json:"toolRisk"`
					} `json:"io.github.navikt/registry"`
				} `json:"_meta"`
			} `json:"servers"`
			Metadata struct {
				NextCursor string `json:"nextCursor"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return reg, fmt.Errorf("reading %s: %w", u, err)
		}
		for _, s := range page.Servers {
			e := s.Server
			if st := s.Meta.Official.Status; st != "" {
				e.Status = st
			}
			e.Setup = s.Meta.Nav.Setup
			e.SandboxHosts = s.Meta.Nav.SandboxHosts
			e.Tools = s.Meta.Nav.Tools
			e.ToolRisk = s.Meta.Nav.ToolRisk
			reg.Entries = append(reg.Entries, e)
			for _, r := range s.Server.Remotes {
				reg.Remotes[normalizeMCPURL(r.URL)] = true
				if s.Server.Name != "" {
					reg.Servers[s.Server.Name] = append(reg.Servers[s.Server.Name], r.URL)
				}
			}
			for _, p := range s.Server.Packages {
				reg.Packages[strings.ToLower(p.Identifier)] = true
			}
		}
		if cursor = page.Metadata.NextCursor; cursor == "" {
			if len(reg.Remotes)+len(reg.Packages) == 0 {
				// An empty list is not an answer: a registry that lists nothing
				// would turn every server off.
				return reg, fmt.Errorf("%s lists no servers", u)
			}
			return reg, nil
		}
	}
	return reg, nil
}

func normalizeMCPURL(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return strings.TrimSuffix(strings.ToLower(s), "/")
	}
	u.Scheme, u.Host = strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u.String()
}

// listed reports whether the registry lists the server.
func (r mcpRegistry) listed(s mcpServer) bool {
	if s.hadEnvName {
		return false
	}
	switch s.Type {
	case "remote":
		return r.Remotes[normalizeMCPURL(s.URL)]
	case "local":
		if len(s.Environment) > 0 || len(s.Env) > 0 {
			return false
		}
	default:
		// OpenCode knows these two. Anything else is not a server the
		// registry can be said to list.
		return false
	}
	pkg := localPackage(s.Command)
	if pkg == "" || !validMCPPackageSpec(pkg) {
		return false
	}
	if r.Packages[pkg] {
		return true
	}
	// A pinned version: @playwright/mcp@latest, mcp-server@1.2.
	// Reject npm alias/protocol specifiers (@npm:..., @file:..., @git+..., URLs/paths).
	if i := strings.LastIndex(pkg, "@"); i > 0 {
		ver := pkg[i+1:]
		if ver != "" && !strings.ContainsAny(ver, `:/\+@`) && r.Packages[pkg[:i]] {
			return true
		}
	}
	return false
}

func validMCPPackageSpec(pkg string) bool {
	if pkg == "" || strings.ContainsAny(pkg, `:\`) || strings.Contains(pkg, "..") {
		return false
	}
	name := pkg
	if i := strings.LastIndex(pkg, "@"); i > 0 {
		ver := pkg[i+1:]
		if ver == "" || strings.ContainsAny(ver, `:/\+@`) {
			return false
		}
		name = pkg[:i]
	}
	if strings.HasPrefix(name, "@") {
		return strings.Count(name, "/") == 1 && !strings.HasSuffix(name, "/")
	}
	return !strings.Contains(name, "/")
}

// localPackage is the package a local server's command runs: the first
// non-flag argument of a package runner (npx, bunx, uvx, pnpm dlx, yarn dlx,
// pipx run), or the command itself. Only that position counts, so a package
// name passed to some other program as data does not make it that package.
func localPackage(command []string) string {
	if len(command) == 0 || strings.HasPrefix(command[0], ".") {
		return ""
	}
	exe := strings.ToLower(filepath.Base(command[0]))
	rest := command[1:]
	switch exe {
	case "npx", "bunx":
	case "uvx":
		return localPythonRunnerPackage(rest)
	case "pnpm", "yarn", "pipx":
		want := "dlx"
		if exe == "pipx" {
			want = "run"
		}
		if len(rest) == 0 || rest[0] != want {
			return ""
		}
		rest = rest[1:]
		if exe == "pipx" {
			return localPythonRunnerPackage(rest)
		}
	default:
		if strings.ContainsAny(command[0], `/\`) {
			return ""
		}
		return exe
	}
	var pkg string
	for k := 0; k < len(rest); k++ {
		a := rest[k]
		if !strings.HasPrefix(a, "-") {
			if pkg == "" {
				pkg = strings.ToLower(a)
			}
			for _, rem := range rest[k+1:] {
				if rem == "-p" || rem == "--package" || rem == "-c" || rem == "--call" ||
					strings.HasPrefix(rem, "--package=") || strings.HasPrefix(rem, "--call=") {
					return ""
				}
			}
			return pkg
		}
		if a == "-y" || a == "--yes" {
			continue
		}
		// npx --package <name> / -p <name> names the package outright (at most once).
		if (a == "-p" || a == "--package") && pkg == "" && k+1 < len(rest) && !strings.HasPrefix(rest[k+1], "-") {
			pkg = strings.ToLower(rest[k+1])
			k++
			continue
		}
		if v, ok := strings.CutPrefix(a, "--package="); ok && pkg == "" && v != "" && !strings.HasPrefix(v, "-") {
			pkg = strings.ToLower(v)
			continue
		}
		return ""
	}
	return pkg
}

func localPythonRunnerPackage(rest []string) string {
	if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
		return ""
	}
	for _, a := range rest[1:] {
		if a == "--with" || a == "--from" || strings.HasPrefix(a, "--with=") || strings.HasPrefix(a, "--from=") {
			return ""
		}
	}
	return strings.ToLower(rest[0])
}

func openCodeEnvGetter(env []string) func(string) string {
	return func(k string) string {
		for _, e := range env {
			if v, ok := strings.CutPrefix(e, k+"="); ok {
				return v
			}
		}
		return ""
	}
}

// openCodeMCPServers reads the MCP servers the user's OpenCode config
// defines for this launch, in OpenCode's merge order: global config, the file
// OPENCODE_CONFIG names, the project's files from the git root down to the
// project directory, OPENCODE_CONFIG_DIR, and OPENCODE_CONFIG_CONTENT. A
// later definition of a name is merged over an earlier one. A file that does
// not parse is skipped; OpenCode would refuse it anyway.
func openCodeMCPServers(projectDir string, env []string) map[string]mcpServer {
	return openCodeMCPServersIn(openCodeConfigDocs(projectDir, env, false), openCodeEnvGetter(env))
}

// openCodeUserMCPServers is the servers in the user's own OpenCode config:
// the global files and ~/.opencode, never a project's or an environment
// variable's, which a repository can set.
func openCodeUserMCPServers() map[string]mcpServer {
	env := os.Environ()
	return openCodeMCPServersIn(openCodeConfigDocs("", env, true), openCodeEnvGetter(env))
}

func openCodeMCPServersIn(docs [][]byte, getenv func(string) string) map[string]mcpServer {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	v2 := openCodeMajor() >= 2
	servers := map[string]mcpServer{}
	unmarshalServer := func(name string, raw json.RawMessage, v2Native bool) {
		hadEnvName := envPlaceholder.MatchString(name)
		hadEnv := hadEnvName || envPlaceholder.Match(raw)
		expanded := expandOpenCodeServerJSON(raw, getenv)
		s := servers[name]
		s.hadEnv = s.hadEnv || hadEnv
		s.hadEnvName = s.hadEnvName || hadEnvName
		if v2Native {
			var d struct {
				Disabled *bool `json:"disabled"`
			}
			if json.Unmarshal(expanded, &s) == nil && json.Unmarshal(expanded, &d) == nil {
				if d.Disabled != nil {
					on := !*d.Disabled
					s.Enabled = &on
				}
				servers[name] = s
			}
		} else if json.Unmarshal(expanded, &s) == nil {
			servers[name] = s
		}
		// If a server key uses {env:VAR}, OpenCode inside the cplt sandbox
		// expands stripped variables to "", so disable and block that key too
		// without ever expanding the host variable into the server name.
		if hadEnvName {
			if emptyName := envPlaceholder.ReplaceAllString(name, ""); emptyName != "" && emptyName != name {
				es := servers[emptyName]
				es.Type = s.Type
				es.Enabled = s.Enabled
				es.hadEnv = true
				es.hadEnvName = true
				servers[emptyName] = es
			}
		}
	}
	for _, doc := range docs {
		var cfg struct {
			MCP map[string]json.RawMessage `json:"mcp"`
		}
		if json.Unmarshal(stripJSONC(doc), &cfg) != nil {
			if bytes.Contains(doc, []byte(`"mcp"`)) {
				fmt.Fprintf(os.Stderr, "%s nav-pilot could not read an OpenCode config with MCP servers, so those servers were not checked against Nav's MCP registry; they run as configured.\n", domain.Yellow("⚠"))
			}
			continue
		}
		for name, raw := range cfg.MCP {
			// opencode 2's own shape: mcp.servers.<name>, mcp.timeout. A
			// key of either name is a server only with a type, as opencode
			// 2 reads it (core/src/config/normalize.ts at v2.0.24). Only on
			// opencode 2: opencode 1 reads "servers" as a server's name.
			if v2 && (name == "servers" || name == "timeout") {
				var probe struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(expandOpenCodeServerJSON(raw, getenv), &probe) == nil && probe.Type != "local" && probe.Type != "remote" {
					if name == "servers" {
						var native map[string]json.RawMessage
						_ = json.Unmarshal(raw, &native)
						for n, r := range native {
							unmarshalServer(n, r, true)
						}
					}
					continue
				}
			}
			unmarshalServer(name, raw, false)
		}
	}
	return servers
}

// isOpenCodeWalkStopDir reports whether the ancestor config walk must stop
// before reading d: shared top-level directories (/tmp, /var/tmp, os.TempDir(),
// /Users, /home, /) are never part of a user's project tree.
func isOpenCodeWalkStopDir(d, home string) bool {
	d = filepath.Clean(d)
	switch d {
	case "/", "/tmp", "/private/tmp", "/var/tmp", "/private/var/tmp", "/Users", "/home":
		return true
	}
	if tmp := filepath.Clean(os.TempDir()); tmp != "" && d == tmp {
		return true
	}
	if evalTmp, err := filepath.EvalSymlinks(os.TempDir()); err == nil && evalTmp != "" && d == filepath.Clean(evalTmp) {
		return true
	}
	if home != "" {
		cleanHome := filepath.Clean(home)
		if d == filepath.Dir(cleanHome) {
			return true
		}
	}
	return false
}

// openCodeConfigDocs is the user's OpenCode config documents for this launch,
// in OpenCode's merge order (see openCodeMCPServers). Unquoted {env:VAR}
// placeholders are replaced with JSON primitives so the document parses; quoted
// {env:VAR} placeholders are expanded only per MCP server value in memory so
// host secrets in server keys or permissions never leak into OPENCODE_CONFIG_CONTENT.
func openCodeConfigDocs(projectDir string, env []string, userOnly bool) [][]byte {
	var docs [][]byte
	for _, doc := range openCodeConfigDocuments(projectDir, env, userOnly) {
		docs = append(docs, doc.content)
	}
	return docs
}

type openCodeConfigDocument struct {
	path    string
	content []byte
}

func openCodeConfigDocuments(projectDir string, env []string, userOnly bool) []openCodeConfigDocument {
	getenv := openCodeEnvGetter(env)
	home, _ := os.UserHomeDir()
	var docs []openCodeConfigDocument
	read := func(dir string, names ...string) {
		for _, n := range names {
			path := filepath.Join(dir, n)
			if b, err := os.ReadFile(path); err == nil {
				docs = append(docs, openCodeConfigDocument{path, expandUnquotedEnv(b, getenv)})
			}
		}
	}
	read(openCodeConfigDir(), "config.json", "opencode.json", "opencode.jsonc")
	if userOnly {
		if home != "" {
			read(filepath.Join(home, ".opencode"), "opencode.json", "opencode.jsonc")
		}
		return docs
	}
	// opencode 2 reads OPENCODE_CONFIG_DIR as its global config, below
	// everything else; opencode 1 ranks it above the project.
	v2 := openCodeMajor() >= 2
	if d := getenv("OPENCODE_CONFIG_DIR"); d != "" && v2 {
		read(d, "opencode.json", "opencode.jsonc")
	}
	if f := getenv("OPENCODE_CONFIG"); f != "" {
		read(filepath.Dir(f), filepath.Base(f))
	}
	if v := strings.ToLower(getenv("OPENCODE_DISABLE_PROJECT_CONFIG")); v != "true" && v != "1" {
		if projectDir == "" {
			projectDir, _ = os.Getwd()
		}
		projectDir, _ = filepath.Abs(projectDir)
		root := source.FindGitRoot(projectDir)
		if root == "" || v2 {
			if !v2 && home != "" && domain.PathWithinRoot(home, projectDir) {
				root = filepath.Clean(home)
			} else {
				root = "/"
			}
		}
		var dirs []string
		for d := projectDir; ; d = filepath.Dir(d) {
			if isOpenCodeWalkStopDir(d, home) {
				break
			}
			dirs = append(dirs, d)
			if d == root || d == filepath.Dir(d) {
				break
			}
		}
		slices.Reverse(dirs)
		for _, d := range dirs {
			read(d, "opencode.json", "opencode.jsonc")
			read(filepath.Join(d, ".opencode"), "opencode.json", "opencode.jsonc")
		}
	}
	// OpenCode reads ~/.opencode as a config directory as well.
	if home != "" {
		read(filepath.Join(home, ".opencode"), "opencode.json", "opencode.jsonc")
	}
	if d := getenv("OPENCODE_CONFIG_DIR"); d != "" && !v2 {
		read(d, "opencode.json", "opencode.jsonc")
	}
	if c := getenv(openCodeConfigContentEnv); c != "" {
		docs = append(docs, openCodeConfigDocument{openCodeConfigContentEnv, expandUnquotedEnv([]byte(c), getenv)})
	}
	return docs
}

// expandOpenCodeServerJSON expands {env:VAR} inside a single server's JSON
// value so URL/command matching against the registry sees the resolved string,
// escaping characters that would break JSON string syntax.
func expandOpenCodeServerJSON(raw json.RawMessage, getenv func(string) string) json.RawMessage {
	return envPlaceholder.ReplaceAllFunc(raw, func(m []byte) []byte {
		val := getenv(string(envPlaceholder.FindSubmatch(m)[1]))
		encoded, err := json.Marshal(val)
		if err != nil || len(encoded) < 2 {
			return nil
		}
		return encoded[1 : len(encoded)-1]
	})
}

// Unquoted placeholders must parse without expanding secrets in JSON strings.
func expandUnquotedEnv(b []byte, getenv func(string) string) []byte {
	var out bytes.Buffer
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out.WriteByte(c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			out.WriteByte(c)
			continue
		}
		if c == '{' && bytes.HasPrefix(b[i:], []byte("{env:")) {
			if end := bytes.IndexByte(b[i:], '}'); end > 5 {
				name := string(b[i+5 : i+end])
				v := strings.TrimSpace(getenv(name))
				if v == "true" || v == "false" || v == "null" || (len(v) > 0 && (v[0] == '-' || (v[0] >= '0' && v[0] <= '9')) && json.Valid([]byte(v))) {
					out.WriteString(v)
				} else {
					out.WriteString("null")
				}
				i += end
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.Bytes()
}

// applyOpenCodeMCPPolicy turns off, for this launch, every enabled MCP server
// the org's registry does not list, and says which on stderr. With no server
// configured it asks nothing, so most launches never touch the network.
func applyOpenCodeMCPPolicy(env []string, projectDir string) []string {
	servers := openCodeMCPServers(projectDir, env)
	var names []string
	for name, s := range servers {
		if s.Enabled == nil || *s.Enabled {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		SeenChanged("mcp-off", "")
		return env
	}
	sort.Strings(names)
	off, err := unlistedMCPServers(servers, names)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s MCP servers were not checked against Nav's MCP registry (%v); they run as configured.\n", domain.Yellow("⚠"), err)
		return env
	}
	offChanged := SeenChanged("mcp-off", strings.Join(off, ","))
	if len(off) == 0 {
		return env
	}
	mcp := map[string]any{}
	v2 := openCodeMajor() >= 2
	for _, name := range off {
		if !v2 {
			mcp[name] = map[string]any{"enabled": false}
			continue
		}
		// opencode 2 drops an entry that only says enabled, so the server
		// would run: it needs the type and the command or URL as well.
		// Never write host-expanded {env:VAR} values into OPENCODE_CONFIG_CONTENT.
		s := servers[name]
		cmd := s.Command
		urlStr := s.URL
		if s.hadEnv {
			cmd = []string{"false"}
			urlStr = "https://disabled.invalid"
		}
		e := map[string]any{"type": "local", "command": cmd, "enabled": false}
		if s.Type == "remote" || s.Type == "" && len(s.Command) == 0 {
			e = map[string]any{"type": "remote", "url": urlStr, "enabled": false}
		}
		mcp[name] = e
	}
	if offChanged || Verbose {
		fmt.Fprintf(os.Stderr, "%s MCP servers turned off for this session (not in Nav's MCP registry): %s. See %s\n",
			domain.Yellow("⚠"), strings.Join(off, ", "), MCPRegistryHelpURL)
	}
	var listed []string
	for name := range servers {
		if !slices.Contains(off, name) {
			listed = append(listed, name)
		}
	}
	blocked, _ := json.Marshal(map[string][]string{"blocked": off, "listed": listed})
	env, _ = telemetry.SetEnvValue(env, MCPBlockedEnv, string(blocked))
	return withOpenCodeConfigContent(env, map[string]any{"mcp": mcp})
}

// unlistedMCPServers is the verdict: which of names the org's registry does
// not list. Nil with no error when no policy restricts MCP servers.
func unlistedMCPServers(servers map[string]mcpServer, names []string) ([]string, error) {
	registry, err := fetchMCPPolicy()
	if err != nil || registry == "" {
		return nil, err
	}
	return unlistedIn(registry, servers, names)
}

// unlistedIn is unlistedMCPServers against a registry already looked up.
func unlistedIn(registry string, servers map[string]mcpServer, names []string) ([]string, error) {
	reg, err := fetchMCPRegistry(registry)
	if err != nil {
		return nil, fmt.Errorf("%s did not answer: %w", registry, err)
	}
	var off []string
	for _, name := range names {
		if !reg.listed(servers[name]) {
			off = append(off, name)
		}
	}
	return off, nil
}

// OpenCodeMCPReport is doctor's view: each enabled server and whether the
// org's registry lists it. err says why nothing could be checked.
//
// With no registry_only policy, nothing is reported: no registry was asked.
func OpenCodeMCPReport(projectDir string) (listed, unlisted []string, err error) {
	servers := openCodeMCPServers(projectDir, os.Environ())
	var names []string
	for name, s := range servers {
		if s.Enabled == nil || *s.Enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, nil, nil
	}
	registry, err := fetchMCPPolicy()
	if err != nil || registry == "" {
		return nil, nil, err
	}
	// The policy once: doctor used to ask GitHub for it twice (#1072).
	off, err := unlistedIn(registry, servers, names)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range names {
		if !slices.Contains(off, n) {
			listed = append(listed, n)
		}
	}
	return listed, off, nil
}

// stripJSONC removes // and /* */ comments and trailing commas outside
// strings, which is what OpenCode's JSONC adds to JSON. Comments go first,
// so a comma before a comment and a closing brace is still trailing.
func stripJSONC(b []byte) []byte {
	return stripJSONCPass(stripJSONCPass(b, true), false)
}

func stripJSONCPass(b []byte, comments bool) []byte {
	var out bytes.Buffer
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case inStr:
			out.WriteByte(c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
			out.WriteByte(c)
		case comments && c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
		case comments && c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
		case !comments && c == ',':
			j := i + 1
			for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
				j++
			}
			if j < len(b) && (b[j] == '}' || b[j] == ']') {
				continue
			}
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.Bytes()
}
