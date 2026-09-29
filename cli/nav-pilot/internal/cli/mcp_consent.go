package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The launch pre-flight for the hosts the user's MCP servers need under cplt
// (provider/mcp_hosts.go). Deliberately thin: it asks through the same
// askProposalConsent the pakke proposal uses and records in the same store.
// The wording and shape of the question are expected to change; the verdict
// (what is pending, what is granted) lives in the provider and does not.

// readMCPHostState is a var so tests answer it without a registry.
var readMCPHostState = providerpkg.ReadMCPHostState

// recordMCPHosts and classifyMCPHosts are vars for the same reason.
var (
	recordMCPHosts   = providerpkg.RecordMCPHosts
	classifyMCPHosts = providerpkg.ClassifyMCPHosts
)

// notedMCPHosts is the non-interactive note already printed this run.
var notedMCPHosts bool

// noteMCPHostConsent asks about MCP registry hosts outside the recorded
// approval, once per host set, and keeps nav-pilot's allowlist file in step.
// Best effort: every failure path grants nothing new.
func noteMCPHostConsent() {
	// Without cplt there is no sandbox to open, and no reason to ask the
	// registry anything.
	if !cpltInstalled() {
		return
	}
	st, err := readMCPHostState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s MCP servers: could not read the consent record, so nothing was asked: %v\n", yellow("⚠"), err)
		return
	}
	if st.Pending != nil {
		askMCPHosts(st)
	}
	syncMCPAllowlist()
}

func askMCPHosts(st providerpkg.MCPHostState) {
	if !isInteractive() {
		if !notedMCPHosts {
			notedMCPHosts = true
			fmt.Fprintf(os.Stderr, "%s Your MCP servers need %s under cplt; only an interactive nav-pilot launch can approve them.\n",
				dim("ℹ"), strings.Join(mcpHostNames(st.Pending), ", "))
		}
		return
	}
	hosts := classifyMCPHosts(st.Pending)
	approve := false
	if err := askProposalConsent(mcpHostsTitle(st), mcpHostsDescription(hosts, st), &approve); err != nil {
		return // Ctrl-C is not an answer.
	}
	if err := recordMCPHosts(hosts, approve); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not record the answer about MCP hosts: %v — you will be asked again.\n", yellow("⚠"), err)
		return
	}
	if !approve {
		fmt.Fprintf(os.Stderr, "%s Declined. MCP servers that need those hosts will fail under cplt until you approve them.\n", yellow("⚠"))
	}
}

func mcpHostNames(hosts []providerpkg.MCPHost) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, h.Host)
	}
	return out
}

func mcpHostsTitle(st providerpkg.MCPHostState) string {
	if st.Record != nil {
		return "Your MCP servers need different hosts from Nav's MCP registry. Allow them in the sandbox?"
	}
	return "Your MCP servers need hosts from Nav's MCP registry. Allow them in the sandbox?"
}

// mcpHostsDescription lists every host, the servers that need it, and what
// the grant is. Server names come from the user's own config and hosts from
// the registry, so both are printed through safe().
func mcpHostsDescription(hosts []providerpkg.MCPHost, st providerpkg.MCPHostState) string {
	var approved []string
	for _, h := range st.Grant {
		approved = append(approved, h.Host)
	}
	var b strings.Builder
	for _, h := range hosts {
		mark := "+"
		if slices.Contains(approved, h.Host) {
			mark = " "
		}
		fmt.Fprintf(&b, "%s %s  (MCP server %s)", mark, safe(h.Host, 253), safeList(h.Servers))
		if h.Private {
			b.WriteString("  private address: cplt's DNS-rebinding guard is lifted for this name")
		}
		b.WriteString("\n")
	}
	b.WriteString("\nThe hosts come from the registry's entry for each server, never from your MCP config. " +
		"They are added to nav-pilot's cplt allowlist file, where one is in use. Nothing else in the sandbox changes.")
	if root := source.FindGitRoot("."); root != "" {
		if _, err := os.Stat(filepath.Join(root, ".cplt.toml")); err == nil {
			b.WriteString("\nThis repository's .cplt.toml is approved separately, with " + bold("cplt trust") + ".")
		}
	}
	return b.String()
}

// reportMCPHosts is doctor's view of the MCP registry hosts: what is granted,
// what waits for an answer, and the loopback servers the user opens by hand.
func reportMCPHosts(w io.Writer, cpltPath string) {
	st, err := readMCPHostState()
	if err != nil {
		fmt.Fprintf(w, "      %s Could not read whether MCP registry hosts are approved: %v\n", yellow("⚠"), err)
		return
	}
	if st.FetchErr != nil {
		fmt.Fprintf(w, "      %s Nav's MCP registry did not answer (%v); %d approved MCP host(s) stay in force\n",
			yellow("⚠"), st.FetchErr, len(st.Grant))
	}
	switch {
	case st.Pending != nil:
		fmt.Fprintf(w, "      %s MCP servers need %d host(s) you have not approved: %s\n",
			yellow("⚠"), len(st.Pending), strings.Join(mcpHostNames(st.Pending), ", "))
		fmt.Fprintf(w, "          %s Launch nav-pilot from a terminal and answer the question.\n", yellow("Solution:"))
	case st.Record != nil && !st.Record.Approved && len(st.Current.Hosts) > 0:
		fmt.Fprintf(w, "      %s MCP registry hosts declined: %s\n", dim("ℹ"), strings.Join(mcpHostNames(st.Current.Hosts), ", "))
	}
	if len(st.Grant) > 0 {
		if reason := providerpkg.WaiverBlockedReason(st.Record); reason != "" {
			fmt.Fprintf(w, "      %s MCP registry hosts are approved but not applied at launch: %s\n", yellow("⚠"), safe(reason, proposalReasonWidth))
		} else {
			fmt.Fprintf(w, "      %s MCP registry hosts approved: %s\n", green("✓"), strings.Join(mcpHostNames(st.Grant), ", "))
		}
	}
	if len(st.Current.Loopback) == 0 {
		return
	}
	allowed := cpltConfigGet(cpltPath, "allow.localhost")
	for _, l := range st.Current.Loopback {
		if slices.Contains(strings.FieldsFunc(allowed, func(r rune) bool { return r == '[' || r == ']' || r == ',' || r == ' ' }), l.Port) {
			continue
		}
		fmt.Fprintf(w, "      %s MCP server %s runs on this machine (port %s), and cplt blocks localhost. To allow it:\n",
			dim("ℹ"), safe(l.Server, 64), l.Port)
		fmt.Fprintf(w, "          %s\n", bold("cplt config set allow.localhost "+l.Port))
	}
}
