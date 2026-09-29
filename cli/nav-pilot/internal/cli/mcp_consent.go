package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The launch screen for the hosts the user's MCP servers need under cplt
// (provider/mcp_hosts.go). The verdict (what is pending, what is granted)
// lives in the provider; this file only words the question, through the same
// askProposalConsent and consent store the agentpakke proposal uses.
//
// One screen for every pending MCP source, grouped per server. The agentpakke
// question is not on it: a pakke's consent is keyed on the install scope,
// which a launch does not know, so it stays at install and sync.

// readMCPHostState and refreshMCPRegistry are vars so tests answer them
// without a registry.
var (
	readMCPHostState   = providerpkg.ReadMCPHostState
	refreshMCPRegistry = providerpkg.RefreshMCPRegistryIfDue
)

// recordMCPHosts and classifyMCPHosts are vars for the same reason.
var (
	recordMCPHosts     = providerpkg.RecordMCPHosts
	classifyMCPHosts   = providerpkg.ClassifyMCPHosts
	reclassifyMCPHosts = providerpkg.ReclassifyMCPHostsInBackground
	// cpltProtectsNavPilotState probes cplt, only when there is a question.
	cpltProtectsNavPilotState = providerpkg.CpltProtectsNavPilotState
)

// mcpHostEffect is the line that says what allowing a host does to the
// sandbox. Placeholder wording until cplt prints its own effect text per key
// (navikt/cplt#635); replace this function's body with cplt's text then.
var mcpHostEffect = func(h providerpkg.MCPHost) string {
	if h.Private {
		return "private address: lifts cplt's DNS-rebinding guard for this name"
	}
	return ""
}

// notedMCPHosts is the non-interactive notes already printed this run.
var notedMCPHosts bool

// mcpHostsMode is the user's mcp_hosts: ask unless the config says off.
func mcpHostsMode() string {
	cfg, err := readConfig()
	if err != nil || cfg == nil || cfg.MCPHosts == nil || !containsStr(validMCPHosts, *cfg.MCPHosts) {
		return "ask"
	}
	return *cfg.MCPHosts
}

// mcpConfigScope sets whose MCP servers count, and whether mcp_hosts is off,
// as a plain launch would. For writers of the allowlist file outside a launch:
// unset, an OpenCode user's section would be computed for Copilot.
func mcpConfigScope() {
	cfg, _ := readConfig()
	providerpkg.MCPClient = resolve(cfg, CLIOverrides{}).Client
	providerpkg.MCPHostsOff = mcpHostsMode() == "off"
}

// noteMCPHostConsent asks about MCP registry hosts outside the recorded
// approval, once per host set, and keeps nav-pilot's allowlist file in step.
// Best effort: every failure path grants nothing new.
func noteMCPHostConsent(client string) {
	providerpkg.MCPClient = client
	providerpkg.MCPHostsOff = mcpHostsMode() == "off"
	// Without cplt there is no sandbox to open, and no reason to ask the
	// registry anything.
	if !cpltInstalled() {
		return
	}
	if !providerpkg.MCPHostsOff {
		st, err := readMCPHostState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s MCP servers: could not read the consent record, so nothing was asked: %v\n", yellow("⚠"), err)
			return
		}
		if st.Pending != nil {
			askMCPHosts(st)
		}
		if len(st.Current.Hosts) > 0 {
			reclassifyMCPHosts()
		}
	}
	syncMCPAllowlist()
}

// mcpServerHosts groups hosts per server, the unit a person decides on.
func mcpServerHosts(hosts []providerpkg.MCPHost) (servers []string, by map[string][]providerpkg.MCPHost) {
	by = map[string][]providerpkg.MCPHost{}
	for _, h := range hosts {
		for _, s := range h.Servers {
			if _, ok := by[s]; !ok {
				servers = append(servers, s)
			}
			by[s] = append(by[s], h)
		}
	}
	sort.Strings(servers)
	return servers, by
}

func askMCPHosts(st providerpkg.MCPHostState) {
	servers, by := mcpServerHosts(st.Pending)
	if !isInteractive() {
		// One line per server, once per run, nothing recorded.
		if !notedMCPHosts {
			notedMCPHosts = true
			for _, s := range servers {
				fmt.Fprintf(os.Stderr, "%s MCP server %s needs %s in the sandbox; only an interactive launch can allow it.\n",
					dim("ℹ"), safe(s, 64), strings.Join(mcpHostNames(by[s]), ", "))
			}
		}
		return
	}
	if !cpltProtectsNavPilotState() {
		// An answer recorded now would never be applied, and asking again
		// at every launch is worse than saying why once per run.
		if !notedMCPHosts {
			notedMCPHosts = true
			for _, s := range servers {
				fmt.Fprintf(os.Stderr, "%s MCP server %s needs hosts in the sandbox; nav-pilot asks once cplt is new enough to deny writes to ~/.nav-pilot/. Upgrade cplt.\n",
					dim("ℹ"), safe(s, 64))
			}
		}
		return
	}
	hosts := classifyMCPHosts(st.Pending)
	servers, by = mcpServerHosts(hosts)
	approve := false // Enter declines.
	if err := askProposalConsent(mcpHostsTitle(servers), mcpHostsDescription(servers, by, st), &approve); err != nil {
		return // Ctrl-C is not an answer.
	}
	if err := recordMCPHosts(hosts, approve); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not record the answer about MCP hosts: %v. You will be asked again.\n", yellow("⚠"), err)
		return
	}
	for _, s := range servers {
		names := mcpHostNames(by[s])
		if approve {
			fmt.Printf("%s %s can reach %s from the sandbox.\n", green("✓"), bold(safe(s, 64)), strings.Join(names, ", "))
			continue
		}
		fmt.Fprintf(os.Stderr, "%s Declined. %s will not connect from the sandbox. To allow it later:\n%s\n",
			yellow("⚠"), bold(safe(s, 64)), indented(mcpManualCommands(by[s])))
	}
}

// mcpManualCommands is what a user runs to allow the hosts in cplt by hand.
func mcpManualCommands(hosts []providerpkg.MCPHost) []string {
	var out []string
	for _, h := range hosts {
		out = append(out, "cplt config set allow.domains "+h.Host)
		if h.Private {
			out = append(out, "cplt config set proxy.allow_private_domains "+h.Host)
		}
	}
	return out
}

func mcpHostNames(hosts []providerpkg.MCPHost) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, h.Host)
	}
	return out
}

// mcpHostsTitle does not say the sandbox blocks the hosts: under the standard
// preset a public host is reachable already, and only strict or a private
// address blocks it.
func mcpHostsTitle(servers []string) string {
	if len(servers) == 1 {
		return fmt.Sprintf("MCP server %s connects to these hosts. Allow them in the sandbox?", safe(servers[0], 64))
	}
	return fmt.Sprintf("%d MCP servers connect to these hosts. Allow them all in the sandbox?", len(servers))
}

// mcpHostsDescription lists each server with its hosts, why each is there and
// what allowing it does, and, for a set answered before, what changed.
// Server names come from the user's config and hosts from the registry, so
// both are printed through safe().
func mcpHostsDescription(servers []string, by map[string][]providerpkg.MCPHost, st providerpkg.MCPHostState) string {
	var b strings.Builder
	for _, s := range servers {
		fmt.Fprintf(&b, "%s (Nav's MCP registry)\n", safe(s, 64))
		for _, h := range by[s] {
			fmt.Fprintf(&b, "  %s  used by MCP server %s\n", safe(h.Host, 253), safe(s, 64))
			if effect := mcpHostEffect(h); effect != "" {
				fmt.Fprintf(&b, "    %s\n", effect)
			}
		}
	}
	if st.Record != nil {
		was, now := mcpHostNames(st.Previous), mcpHostNames(st.Pending)
		var changes []string
		for _, h := range now {
			if !slices.Contains(was, h) {
				changes = append(changes, "+ "+safe(h, 253))
			}
		}
		for _, h := range was {
			if !slices.Contains(now, h) {
				changes = append(changes, "- "+safe(h, 253))
			}
		}
		if len(changes) > 0 {
			fmt.Fprintf(&b, "\nChanged since you last answered: %s\n", strings.Join(changes, ", "))
		}
	}
	b.WriteString("\nThe hosts come from the registry's entry for each server, never from your MCP config. " +
		"No port is opened, no file, nothing runs. The allowlist and blocklist still apply.")
	if root := source.FindGitRoot("."); root != "" {
		if _, err := os.Stat(filepath.Join(root, ".cplt.toml")); err == nil {
			b.WriteString("\nThis repository's .cplt.toml is approved separately, with " + bold("cplt trust") + ".")
		}
	}
	return b.String()
}

// reportMCPHosts is doctor's view of the MCP registry hosts: what is allowed,
// what waits for an answer, and the loopback servers the user opens by hand.
func reportMCPHosts(w io.Writer, cpltPath string) {
	if mcpHostsMode() == "off" {
		fmt.Fprintf(w, "      %s\n", dim("ℹ MCP hosts are off (mcp_hosts = off)"))
		return
	}
	cfg, _ := readConfig()
	providerpkg.MCPClient = resolve(cfg, CLIOverrides{}).Client
	// Doctor reads the registry when the launch's cache of it is due, so the
	// launch never has to.
	refreshErr := refreshMCPRegistry()
	st, err := readMCPHostState()
	if err != nil {
		fmt.Fprintf(w, "      %s Could not read whether MCP hosts are allowed: %v\n", yellow("⚠"), err)
		return
	}
	if refreshErr != nil || st.FetchErr != nil {
		why := refreshErr
		if why == nil {
			why = st.FetchErr
		}
		fmt.Fprintf(w, "      %s Nav's MCP registry did not answer (%v); %d allowed MCP host(s) stay in force\n",
			yellow("⚠"), why, len(st.Grant))
	}
	switch {
	case st.Pending != nil:
		servers, by := mcpServerHosts(st.Pending)
		for _, s := range servers {
			fmt.Fprintf(w, "      %s %s (MCP): %d host(s) not answered. The next nav-pilot launch asks.\n",
				yellow("⚠"), safe(s, 64), len(by[s]))
		}
	case st.Record != nil && !st.Record.Approved && len(st.Current.Hosts) > 0:
		fmt.Fprintf(w, "      %s\n", dim("ℹ MCP hosts declined: "+strings.Join(mcpHostNames(st.Current.Hosts), ", ")))
	}
	if len(st.Grant) > 0 {
		if reason := providerpkg.WaiverBlockedReason(st.Record); reason != "" {
			fmt.Fprintf(w, "      %s MCP hosts are allowed but not applied at launch: %s\n", yellow("⚠"), safe(reason, proposalReasonWidth))
		} else {
			fmt.Fprintf(w, "      %s MCP hosts allowed: %s\n", green("✓"), strings.Join(mcpHostNames(st.Grant), ", "))
		}
	}
	if len(st.Current.Loopback) == 0 {
		return
	}
	allowed := strings.FieldsFunc(cpltConfigGet(cpltPath, "allow.localhost"), func(r rune) bool {
		return r == '[' || r == ']' || r == ',' || r == ' '
	})
	for _, l := range st.Current.Loopback {
		if slices.Contains(allowed, l.Port) {
			continue
		}
		fmt.Fprintf(w, "      %s %s (MCP) listens on localhost:%s, which the sandbox blocks. Open: %s\n",
			dim("ℹ"), safe(l.Server, 64), l.Port, bold("cplt config set allow.localhost "+l.Port))
	}
}
