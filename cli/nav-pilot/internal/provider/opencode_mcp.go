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

const mcpPolicyTimeout = 5 * time.Second

var envPlaceholder = regexp.MustCompile(`\{env:([^}]+)\}`)

// MCPBlockedEnv carries the servers the policy turned off, and the others,
// as {"blocked": [...], "listed": [...]}, to the hooks bridge, which refuses
// the blocked servers' tools: enabled=false is only how the session
// starts, and OpenCode's /mcp dialog can connect a server anyway.
const MCPBlockedEnv = "NAV_PILOT_MCP_BLOCKED"

// mcpServer is one server in the user's OpenCode config.
type mcpServer struct {
	Type    string   `json:"type"`
	URL     string   `json:"url"`
	Command []string `json:"command"`
	Enabled *bool    `json:"enabled"`
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
	ctx, cancel := context.WithTimeout(context.Background(), mcpPolicyTimeout)
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
	ctx, cancel := context.WithTimeout(context.Background(), mcpPolicyTimeout)
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
				Server struct {
					Name     string                        `json:"name"`
					Remotes  []struct{ URL string }        `json:"remotes"`
					Packages []struct{ Identifier string } `json:"packages"`
				} `json:"server"`
			} `json:"servers"`
			Metadata struct {
				NextCursor string `json:"nextCursor"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return reg, fmt.Errorf("reading %s: %w", u, err)
		}
		for _, s := range page.Servers {
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
	switch s.Type {
	case "remote":
		return r.Remotes[normalizeMCPURL(s.URL)]
	case "local":
	default:
		// OpenCode knows these two. Anything else is not a server the
		// registry can be said to list.
		return false
	}
	pkg := localPackage(s.Command)
	if pkg == "" {
		return false
	}
	if r.Packages[pkg] {
		return true
	}
	// A pinned version: @playwright/mcp@latest, mcp-server@1.2.
	if i := strings.LastIndex(pkg, "@"); i > 0 && r.Packages[pkg[:i]] {
		return true
	}
	return false
}

// localPackage is the package a local server's command runs: the first
// non-flag argument of a package runner (npx, bunx, uvx, pnpm dlx, yarn dlx,
// pipx run), or the command itself. Only that position counts, so a package
// name passed to some other program as data does not make it that package.
func localPackage(command []string) string {
	if len(command) == 0 {
		return ""
	}
	exe := strings.ToLower(filepath.Base(command[0]))
	rest := command[1:]
	switch exe {
	case "npx", "bunx", "uvx":
	case "pnpm", "yarn", "pipx":
		want := "dlx"
		if exe == "pipx" {
			want = "run"
		}
		if len(rest) == 0 || rest[0] != want {
			return ""
		}
		rest = rest[1:]
	default:
		return exe
	}
	for k := 0; k < len(rest); k++ {
		a := rest[k]
		if !strings.HasPrefix(a, "-") {
			return strings.ToLower(a)
		}
		// npx --package <name> / -p <name> names the package outright.
		if (a == "-p" || a == "--package") && k+1 < len(rest) {
			return strings.ToLower(rest[k+1])
		}
		if v, ok := strings.CutPrefix(a, "--package="); ok {
			return strings.ToLower(v)
		}
	}
	return ""
}

// openCodeMCPServers reads the MCP servers the user's OpenCode config
// defines for this launch, in OpenCode's merge order: global config, the file
// OPENCODE_CONFIG names, the project's files from the git root down to the
// project directory, OPENCODE_CONFIG_DIR, and OPENCODE_CONFIG_CONTENT. A
// later definition of a name is merged over an earlier one. A file that does
// not parse is skipped; OpenCode would refuse it anyway.
func openCodeMCPServers(projectDir string, env []string) map[string]mcpServer {
	servers := map[string]mcpServer{}
	for _, doc := range openCodeConfigDocs(projectDir, env) {
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
			s := servers[name]
			if json.Unmarshal(raw, &s) == nil {
				servers[name] = s
			}
		}
	}
	return servers
}

// openCodeConfigDocs is the user's OpenCode config documents for this launch,
// in OpenCode's merge order (see openCodeMCPServers), with {env:VAR} already
// replaced. Comments are left in; parse with stripJSONC.
func openCodeConfigDocs(projectDir string, env []string) [][]byte {
	getenv := func(k string) string {
		for _, e := range env {
			if v, ok := strings.CutPrefix(e, k+"="); ok {
				return v
			}
		}
		return ""
	}
	var docs [][]byte
	read := func(dir string, names ...string) {
		for _, n := range names {
			if b, err := os.ReadFile(filepath.Join(dir, n)); err == nil {
				docs = append(docs, b)
			}
		}
	}
	read(openCodeConfigDir(), "config.json", "opencode.json", "opencode.jsonc")
	if f := getenv("OPENCODE_CONFIG"); f != "" {
		read(filepath.Dir(f), filepath.Base(f))
	}
	if v := strings.ToLower(getenv("OPENCODE_DISABLE_PROJECT_CONFIG")); v != "true" && v != "1" {
		if projectDir == "" {
			projectDir, _ = os.Getwd()
		}
		projectDir, _ = filepath.Abs(projectDir)
		// Outside a git repo OpenCode's worktree is "/", and it walks all
		// the way up.
		root := source.FindGitRoot(projectDir)
		if root == "" {
			root = "/"
		}
		var dirs []string
		for d := projectDir; ; d = filepath.Dir(d) {
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
	if home, err := os.UserHomeDir(); err == nil {
		read(filepath.Join(home, ".opencode"), "opencode.json", "opencode.jsonc")
	}
	if d := getenv("OPENCODE_CONFIG_DIR"); d != "" {
		read(d, "opencode.json", "opencode.jsonc")
	}
	if c := getenv(openCodeConfigContentEnv); c != "" {
		docs = append(docs, []byte(c))
	}
	// {env:VAR} is replaced as raw text before OpenCode parses, so it is
	// here too: a URL behind a variable must be matched by its value, and
	// an unquoted placeholder must not make the document unreadable.
	for i, doc := range docs {
		docs[i] = envPlaceholder.ReplaceAllFunc(doc, func(m []byte) []byte {
			return []byte(getenv(string(envPlaceholder.FindSubmatch(m)[1])))
		})
	}
	return docs
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
		return env
	}
	sort.Strings(names)
	off, err := unlistedMCPServers(servers, names)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s MCP servers were not checked against Nav's MCP registry (%v); they run as configured.\n", domain.Yellow("⚠"), err)
		return env
	}
	if len(off) == 0 {
		return env
	}
	mcp := map[string]any{}
	for _, name := range off {
		mcp[name] = map[string]any{"enabled": false}
	}
	fmt.Fprintf(os.Stderr, "%s MCP servers turned off for this session (not in Nav's MCP registry): %s. See %s\n",
		domain.Yellow("⚠"), strings.Join(off, ", "), MCPRegistryHelpURL)
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
