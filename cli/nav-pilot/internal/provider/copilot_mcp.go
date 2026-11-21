package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// CopilotMCPSourcesEnv carries {configKey: registryName} for every MCP server
// configured for a Copilot CLI launch, with "" for servers whose URL does not
// uniquely match an entry in Nav's MCP registry. Including unverified keys lets
// the redaction hook resolve longest-prefix matches and fail closed on prefix
// collisions. Absent when no server matched or the registry could not be read.
const CopilotMCPSourcesEnv = "NAV_PILOT_COPILOT_MCP_SOURCES"

type copilotMCPServer struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	Command string `json:"command"`
}

func (s copilotMCPServer) isRemote() bool {
	if s.URL == "" || strings.Contains(s.URL, "${") {
		return false
	}
	return s.Type == "http" || s.Type == "sse" || (s.Type == "" && s.Command == "")
}

// withCopilotMCPSources strips any inherited CopilotMCPSourcesEnv, resolves
// Copilot CLI's configured MCP servers against Nav's MCP registry by remote
// URL, and passes the verified map through cplt when at least one server
// matched. Any --additional-mcp-config flag fails closed because it can
// override a server key at runtime.
func withCopilotMCPSources(env []string, projectDir string, extraArgs []string) ([]string, []string) {
	env = slices.DeleteFunc(env, func(e string) bool {
		return strings.HasPrefix(e, CopilotMCPSourcesEnv+"=")
	})
	for _, a := range extraArgs {
		if a == "--additional-mcp-config" || strings.HasPrefix(a, "--additional-mcp-config=") {
			return env, nil
		}
	}
	servers := copilotMCPServers(projectDir, env)
	if len(servers) == 0 {
		return env, nil
	}
	reg, ok := copilotMCPRegistry()
	if !ok {
		return env, nil
	}
	src := make(map[string]string, len(servers))
	anyVerified := false
	for key, s := range servers {
		name := ""
		if s.isRemote() {
			name = reg.nameOf(mcpServer{Type: "remote", URL: s.URL})
		}
		src[key] = name
		if name != "" {
			anyVerified = true
		}
	}
	if !anyVerified {
		return env, nil
	}
	b, err := json.Marshal(src)
	if err != nil {
		return env, nil
	}
	env, _ = telemetry.SetEnvValue(env, CopilotMCPSourcesEnv, string(b))
	return env, []string{"--pass-env", CopilotMCPSourcesEnv}
}

func copilotMCPRegistry() (mcpRegistry, bool) {
	if c, ok := readMCPRegistryCache(); ok && time.Since(c.At) <= mcpRegistryTTL {
		return c.Registry, true
	}
	registry, err := fetchMCPPolicy()
	if err != nil || registry == "" {
		return mcpRegistry{}, false
	}
	reg, err := fetchMCPRegistry(registry)
	if err != nil {
		return mcpRegistry{}, false
	}
	return reg, true
}

// copilotMCPServers reads Copilot CLI's MCP server definitions in Copilot's
// precedence order: user config (~/.copilot/mcp-config.json or $COPILOT_HOME),
// then workspace .mcp.json (or .github/mcp.json when .mcp.json is absent) from
// the git root down to projectDir. Later definitions overwrite earlier ones.
func copilotMCPServers(projectDir string, env []string) map[string]copilotMCPServer {
	servers := map[string]copilotMCPServer{}
	read := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cfg struct {
			MCPServers map[string]copilotMCPServer `json:"mcpServers"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			return
		}
		for k, v := range cfg.MCPServers {
			servers[k] = v
		}
	}
	home, _ := os.UserHomeDir()
	if ch := openCodeEnvGetter(env)("COPILOT_HOME"); ch != "" {
		read(filepath.Join(ch, "mcp-config.json"))
	} else if home != "" {
		read(filepath.Join(home, ".copilot", "mcp-config.json"))
	}
	dir := projectDir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		root := source.FindGitRoot(dir)
		if root == "" {
			root = dir
		}
		var dirs []string
		for d := dir; ; d = filepath.Dir(d) {
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
			p := filepath.Join(d, ".mcp.json")
			if _, err := os.Stat(p); err == nil {
				read(p)
				continue
			}
			read(filepath.Join(d, ".github", "mcp.json"))
		}
	}
	return servers
}
