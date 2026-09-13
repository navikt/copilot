package agentpakke

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// mcpRegistryAPI is the registry endpoint listing the servers it publishes.
// A var so tests can point it at a stub: nothing in the suite may depend on the
// real service being reachable.
var mcpRegistryAPI = MCPRegistryURL + "/v0.1/servers"

// MCPServerFindings checks the manifest's declared MCP servers against the live
// registry. It reports one finding per name the registry does not publish, and
// a warning when the registry could not answer at all.
//
// The registry is a service, not a list this binary carries: servers are added
// and retired without a nav-pilot release. A copy compiled into the binary
// would reject a server added after this version shipped, with a message
// claiming the registry does not publish it — which would be false, and false
// in the direction that blocks work.
//
// Unreachable is not a verdict. No network in CI, a sandbox, or an outage says
// nothing about whether a name is right, and a gate that fails on an unrelated
// outage is a gate people learn to skip. So membership fails open with a
// warning saying it was not checked, and fails closed only on an answer the
// registry actually gave. A question that could not be answered is never
// rendered as a bad answer.
//
// A manifest that declares no servers never touches the network.
func (m *Manifest) MCPServerFindings() (findings []error, warning string) {
	if len(m.MCPServers) == 0 {
		return nil, ""
	}
	return m.mcpFindings(registryServerNames())
}

// mcpFindings is the verdict half, split out so the policy is testable without
// a server. published is nil for "the registry did not answer".
func (m *Manifest) mcpFindings(published map[string]bool) (findings []error, warning string) {
	if published == nil {
		return nil, fmt.Sprintf(
			"could not reach Nav's MCP registry (%s), so the %d server(s) in mcpServers were not checked against it. "+
				"That is a warning, not a violation: being offline is not evidence that a name is wrong",
			mcpRegistryAPI, len(m.MCPServers))
	}
	for _, name := range m.MCPServers {
		if published[name] {
			continue
		}
		findings = append(findings, fmt.Errorf(
			"mcpServers: %q is not a server Nav's MCP registry publishes. "+
				"A server is defined in the registry and nowhere else, so name one it lists, "+
				"or have the server added there first (%s)", name, MCPRegistryURL))
	}
	return findings, ""
}

// registryServerNames returns the names the registry publishes, or nil when the
// question could not be answered. Every failure mode collapses to nil on
// purpose: the caller treats nil as "unknown", never as "publishes nothing".
func registryServerNames() map[string]bool {
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(mcpRegistryAPI)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var doc struct {
		Servers []struct {
			Server struct {
				Name string `json:"name"`
			} `json:"server"`
		} `json:"servers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil
	}
	names := make(map[string]bool, len(doc.Servers))
	for _, s := range doc.Servers {
		if s.Server.Name != "" {
			names[s.Server.Name] = true
		}
	}
	// A registry that lists nothing is a deploy that went wrong, not a registry
	// that publishes nothing. Rejecting every declared name over it is the one
	// outcome worth avoiding, so it counts as no answer.
	if len(names) == 0 {
		return nil
	}
	return names
}
