package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// The nav-pilot half of cplt's MCP host broker (navikt/cplt
// docs/design/mcp-host-broker.md). For a registry server with the broker
// extension, enable writes one spec file to the user's cplt config and logs
// in on the host; cplt runs the server on the host, enforces the spec and
// adds the client entries at launch. nav-pilot writes no client config here.
//
// This file: the mechanism table, the registry check and the spec content.
// mcp_broker_file.go: the spec file. mcp_broker_cplt.go: cplt's status.
// mcp_broker_host.go: commands on the host. mcp_broker_cmd.go: enable and
// the list row.

const (
	mcpBrokerKey     = "io.github.navikt/broker"
	mcpBrokerVersion = 1
)

// mcpMechanism is everything nav-pilot knows about a credential mechanism.
// Hand-kept, like cplt's own table: nav-pilot never runs a command the
// registry supplies; the registry's login commands must match these and are
// otherwise only text. apps/mcp-registry/testdata/broker-mechanisms.json
// keeps this table and the registry's in step.
type mcpMechanism struct {
	command []string // the server, as cplt pins it
	status  []string // the registry's login.status, matched as written
	// probe exits 0 when the server's login is usable. It differs from
	// status for nais: without -n nais prints the gcloud token, which the
	// MCP server does not use (navikt/cplt#843).
	probe   []string
	login   []string
	install string // where to get the CLI
	// version, minVersion, upgrade and oldNotice: an older CLI gets a
	// nudge, never a refusal.
	version    []string
	minVersion string
	upgrade    string
	oldNotice  string
}

// tool is the CLI the mechanism's commands run.
func (m mcpMechanism) tool() string { return m.status[0] }

var mcpMechanisms = map[string]mcpMechanism{
	"nais-keychain": {
		command: []string{"nais", "alpha", "mcp", "serve"},
		status:  []string{"nais", "auth", "print-access-token"},
		probe:   []string{"nais", "auth", "print-access-token", "-n"},
		login:   []string{"nais", "login", "-n"},
		install: "https://doc.nais.io/operate/cli/how-to/install/",
		// 7.0.2 checks every GraphQL operation and fragment (nais/cli 6ddc986).
		version:    []string{"nais", "--version"},
		minVersion: "7.0.2",
		upgrade:    "brew upgrade nais",
		oldNotice:  "which checks every GraphQL operation the agent sends",
	},
}

var mcpBrokerClasses = []string{providerpkg.MCPRiskRead, providerpkg.MCPRiskWrite, providerpkg.MCPRiskExternal}

// mcpBrokerCheck is the registry extension against nav-pilot's table: the
// version, the mechanism and its pinned commands, and every tool's class.
func mcpBrokerCheck(e providerpkg.MCPServerEntry) (mcpMechanism, error) {
	b := e.Meta.Broker
	fix := "Fix: nav-pilot update, or report it: " + providerpkg.MCPRegistryHelpURL
	if b.V != mcpBrokerVersion {
		return mcpMechanism{}, fmt.Errorf("the registry's broker spec for %s is v%d; this nav-pilot writes v%d. %s", e.Name, b.V, mcpBrokerVersion, fix)
	}
	m, ok := mcpMechanisms[b.Credential]
	if !ok {
		return m, fmt.Errorf("the registry names credential %q for %s, which nav-pilot does not know. %s", safe(b.Credential, 32), e.Name, fix)
	}
	if !slices.Equal(b.Command, m.command) {
		return m, fmt.Errorf("the registry's command for %s is not %q. %s", e.Name, strings.Join(m.command, " "), fix)
	}
	if b.Login.Run != strings.Join(m.login, " ") || b.Login.Status != strings.Join(m.status, " ") {
		return m, fmt.Errorf("the registry's login commands for %s are not the ones nav-pilot runs for %s. %s", e.Name, b.Credential, fix)
	}
	if len(b.Tools.Classes) == 0 {
		return m, fmt.Errorf("the registry classes no tools for %s. %s", e.Name, fix)
	}
	for t, c := range b.Tools.Classes {
		if !slices.Contains(mcpBrokerClasses, c) {
			return m, fmt.Errorf("the registry gives %s of %s class %q (read, write or external). %s", safe(t, 64), e.Name, safe(c, 32), fix)
		}
	}
	return m, nil
}

// mcpBrokerTools is what the spec allows, and whether that needs write
// consent. Reads by default; --all-tools and --tools may add writes. A tool
// the registry does not class is never on. Classes are checked already.
func mcpBrokerTools(e providerpkg.MCPServerEntry, o mcpToolOpts) (allow []string, write bool, err error) {
	classes := e.Meta.Broker.Tools.Classes
	switch {
	case o.tools != nil:
		for _, t := range o.tools {
			if _, ok := classes[t]; !ok {
				return nil, false, fmt.Errorf("%s has no brokered tool %q. Fix: nav-pilot mcp enable %s --tools <one or more of: %s>", e.Name, safe(t, 64), e.Name, strings.Join(slices.Sorted(maps.Keys(classes)), ","))
			}
		}
		allow = slices.Clone(o.tools)
	default:
		for t, c := range classes {
			if c == providerpkg.MCPRiskRead || o.all {
				allow = append(allow, t)
			}
		}
	}
	for _, t := range allow {
		write = write || classes[t] != providerpkg.MCPRiskRead
	}
	sort.Strings(allow)
	if len(allow) == 0 {
		return nil, false, fmt.Errorf("no read tools to turn on for %s. Fix: nav-pilot mcp enable %s --tools <a,b>", e.Name, e.Name)
	}
	return allow, write, nil
}

// mcpBrokerSpec is the server.json cplt reads: the registry's extension with
// the user's consent filled in, and nav-pilot's own login commands.
func mcpBrokerSpec(e providerpkg.MCPServerEntry, m mcpMechanism, allow []string, write bool) ([]byte, error) {
	b := e.Meta.Broker
	ext := map[string]any{
		"v":          mcpBrokerVersion,
		"credential": b.Credential,
		"command":    b.Command,
		"tools":      map[string]any{"allow": allow, "classes": b.Tools.Classes},
		"login":      map[string]string{"run": strings.Join(m.login, " "), "status": strings.Join(m.status, " ")},
	}
	if len(b.EgressHosts) > 0 {
		ext["egressHosts"] = b.EgressHosts
	}
	if len(b.ArgChecks) > 0 {
		ext["argChecks"] = b.ArgChecks
	}
	if write {
		ext["writeConsent"] = true
	}
	return json.MarshalIndent(map[string]any{
		"$schema": "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json",
		"name":    e.Name,
		"version": e.Version,
		"_meta":   map[string]any{mcpBrokerKey: ext},
	}, "", "  ")
}
