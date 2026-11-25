package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// nav-pilot mcp list|enable|disable|prune: the MCP servers in Nav's registry,
// what the user's clients have configured, and what stands between a
// configured server and a working one under cplt. list only reads. enable,
// disable and prune are the only paths that write a client config: enable and
// disable only the one asked for, prune only registry-named entries the
// registry no longer lists.

// Test seams.
var (
	mcpRegistryServers = providerpkg.MCPRegistryServers
	mcpRegistryRefresh = providerpkg.RefreshMCPRegistry
	mcpConfigured      = providerpkg.ConfiguredMCPServers
	mcpCpltPath        = func() string { p, _ := findCplt(); return p }
	mcpCpltConfigGet   = cpltConfigGet
	mcpLookPath        = exec.LookPath
	narrowMCPApproval  = providerpkg.NarrowMCPApproval
)

// mcpLaunchClient is the client a plain nav-pilot launches: whose servers
// the host state is about, as at launch.
func mcpLaunchClient() string {
	cfg, _ := readConfig()
	return resolve(cfg, CLIOverrides{}).Client
}

// mcpNoteStale says the registry list is an old answer, and why.
func mcpNoteStale(stale error) {
	if stale != nil {
		fmt.Fprintf(os.Stderr, "%s Nav's MCP registry could not be read again (%v)\n", yellow("⚠"), stale)
	}
}

func cmdMCP(args []string) error {
	mcpLoginStates = map[string]string{} // one probe per run
	var client string
	var jsonOut, refresh, dryRun, yes bool
	var pos []string
	var topts mcpToolOpts
	toolFlag := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--json":
			jsonOut = true
		case "--refresh":
			refresh = true
		case "-n", "--dry-run":
			dryRun = true
		case "-y", "--yes":
			yes = true
		case "--tools":
			if i+1 >= len(args) {
				return fmt.Errorf("--tools requires a comma-separated list of tool names")
			}
			i++
			topts.tools = []string{}
			for _, t := range strings.Split(args[i], ",") {
				if t = strings.TrimSpace(t); t != "" && !slices.Contains(topts.tools, t) {
					topts.tools = append(topts.tools, t)
				}
			}
			if len(topts.tools) == 0 {
				return fmt.Errorf("--tools needs at least one tool name; to remove a server: nav-pilot mcp disable <name>")
			}
			toolFlag = true
		case "--all-tools":
			topts.all, toolFlag = true, true
		case "--allow-host-exec":
			topts.allowHostExec, toolFlag = true, true
		case "--client":
			if i+1 >= len(args) {
				return fmt.Errorf("--client requires a value (copilot or opencode)")
			}
			i++
			client = args[i]
		case "-h", "--help":
			printHelp(os.Stdout, "mcp")
			return nil
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("unknown flag %s\n\n%s", a, mcpUsage)
			}
			pos = append(pos, a)
		}
	}
	if client != "" && client != providerpkg.MCPClientCopilot && client != providerpkg.MCPClientOpenCode {
		return fmt.Errorf("--client %q: nav-pilot mcp knows copilot and opencode", client)
	}
	if len(pos) == 0 {
		pos = []string{"list"}
	}
	if toolFlag && pos[0] != "enable" {
		return errMCPToolsFlag
	}
	if refresh && pos[0] != "list" && pos[0] != "ls" && pos[0] != "enable" {
		return fmt.Errorf("--refresh is for mcp list and mcp enable")
	}
	if (dryRun || yes) && pos[0] != "prune" {
		return fmt.Errorf("--dry-run and --yes are only for mcp prune")
	}
	if topts.all && topts.tools != nil {
		return fmt.Errorf("--tools and --all-tools: pick one")
	}
	switch pos[0] {
	case "list", "ls":
		if len(pos) > 1 {
			return fmt.Errorf("mcp list takes no arguments\n\n%s", mcpUsage)
		}
		return cmdMCPList(jsonOut, refresh)
	case "prune":
		if len(pos) > 1 {
			return fmt.Errorf("mcp prune takes no arguments\n\n%s", mcpUsage)
		}
		if jsonOut {
			return fmt.Errorf("--json is for mcp list and mcp enable")
		}
		return cmdMCPPrune(client, dryRun, yes)
	case "enable", "disable":
		// Only the servers named: never all of them by omission.
		if len(pos) < 2 {
			return fmt.Errorf("mcp %s needs one or more server names; see them with: nav-pilot mcp list\n\n%s", pos[0], mcpUsage)
		}
		if jsonOut && pos[0] != "enable" {
			return fmt.Errorf("--json is for mcp list and mcp enable")
		}
		// One JSON document per run: two servers would print two objects.
		if jsonOut && len(pos) > 2 {
			return fmt.Errorf("mcp enable --json takes one server name. Fix: run nav-pilot mcp enable <name> --json once per server")
		}
		// A brokered server needs no client config, so no client either:
		// the client list is asked for only when one is written.
		clients := func() ([]string, error) { return mcpClients(client) }
		if pos[0] == "enable" {
			return cmdMCPEnable(pos[1:], clients, topts, jsonOut, refresh)
		}
		return cmdMCPDisable(pos[1:], clients)
	}
	return fmt.Errorf("unknown mcp command %q\n\n%s", pos[0], mcpUsage)
}

const mcpUsage = `Usage: nav-pilot mcp [list] [--json] [--refresh]
       nav-pilot mcp enable <name>... [--client copilot|opencode] [--tools a,b | --all-tools] [--allow-host-exec] [--json] [--refresh]
       nav-pilot mcp disable <name>... [--client copilot|opencode]
       nav-pilot mcp prune [--dry-run] [--yes] [--client copilot|opencode]

A server cplt runs on the host gets its read tools by default.
--all-tools, or a write tool named in --tools, also allows its writes.`

// mcpProblem is one thing that keeps a configured server from working, and
// the exact command that fixes it.
type mcpProblem struct {
	Server  string `json:"server"`
	Client  string `json:"client,omitempty"`
	Problem string `json:"problem"`
	Fix     string `json:"fix"`
}

type mcpHostRow struct {
	Host    string `json:"host"`
	Cplt    string `json:"cplt,omitempty"`    // cplt's probe verdict: ALLOWED, BLOCKED-ALLOWLIST, ...
	Consent string `json:"consent,omitempty"` // allowed, pending, declined (nav-pilot's host consent)
	// DNS is how the allowed host was classified when it was answered:
	// private, or unknown when the lookup failed without saying.
	DNS string `json:"dns,omitempty"`
}

type mcpServerRow struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Copilot     bool   `json:"copilot"`
	OpenCode    bool   `json:"opencode"`
	// Tools is which tools are on: all, read-only (GitHub), "n of m".
	Tools string `json:"tools,omitempty"`
	// HostExec is the tools on that run outside the sandbox.
	HostExec []string     `json:"hostExec,omitempty"`
	Hosts    []mcpHostRow `json:"hosts,omitempty"`
	// Login and Path are for a server cplt brokers on the host: the host
	// login ("nais: ok") and how it runs ("cplt broker", "direct (...)").
	Login string `json:"login,omitempty"`
	Path  string `json:"path,omitempty"`
}

type mcpReport struct {
	Registry string `json:"registry"`
	Stale    string `json:"stale,omitempty"`
	// OpenCodeProject is servers OpenCode loads from a project's config or an
	// OPENCODE_CONFIG* variable, which nav-pilot does not count.
	OpenCodeProject []string       `json:"opencode_project,omitempty"`
	Problems        []mcpProblem   `json:"problems"`
	Servers         []mcpServerRow `json:"servers"`
}

func cmdMCPList(jsonOut, refresh bool) error {
	if refresh {
		if err := mcpRegistryRefresh(); err != nil {
			return fmt.Errorf("could not read Nav's MCP registry again: %w", err)
		}
	}
	registry, entries, stale, err := mcpRegistryServers()
	if err != nil {
		if unreachable(err) {
			debugLog("%v", err)
			return errors.New("could not reach Nav's MCP registry. Check the network or naisdevice, and try again")
		}
		return fmt.Errorf("could not list MCP servers: %w", err)
	}
	providerpkg.MCPClient = mcpLaunchClient()
	conf := mcpConfigured(entries)
	rep := diagnoseMCP(registry, entries, conf, "")
	rep.OpenCodeProject = conf.OpenCodeProject
	if stale != nil {
		rep.Stale = stale.Error()
	}
	if jsonOut {
		return outputJSON(rep)
	}
	printMCPReport(rep)
	return nil
}

func printMCPReport(rep mcpReport) {
	if len(rep.Problems) == 0 {
		fmt.Printf("%s No configuration problems found with your MCP servers.\n", green("✓"))
	} else {
		fmt.Printf("%s\n", bold(fmt.Sprintf("Problems (%d)", len(rep.Problems))))
		printMCPProblems(rep.Problems)
	}
	fmt.Println("Checks configured servers and sandbox policy, not live connections, authentication, built-in servers, or the active agent's tools.")
	if rep.Stale != "" {
		fmt.Printf("%s Nav's MCP registry could not be read again (%s)\n", yellow("⚠"), rep.Stale)
	}
	if len(rep.OpenCodeProject) > 0 {
		names := make([]string, len(rep.OpenCodeProject))
		for i, n := range rep.OpenCodeProject {
			names[i] = safe(n, 64)
		}
		fmt.Printf("%s OpenCode also loads %s here, from the project's config or OPENCODE_CONFIG*. nav-pilot counts only your own config: it neither checks nor allows hosts for these.\n",
			dim("ℹ"), strings.Join(names, ", "))
	}
	fmt.Printf("\n%s %s\n", bold("Servers in Nav's MCP registry"), dim(rep.Registry))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  NAME\tCOPILOT\tOPENCODE\tTOOLS\tLOGIN\tPATH\tSTATUS\tDESCRIPTION")
	on := func(b bool) string {
		if b {
			return "on"
		}
		return "-"
	}
	dash := func(s string) string {
		if s == "" {
			return "-"
		}
		return s
	}
	// cplt adds a brokered server's client entries at launch.
	client := func(s mcpServerRow, b bool) string {
		if s.Path == "cplt broker" {
			return "cplt"
		}
		return on(b)
	}
	for _, s := range rep.Servers {
		status := s.Status
		if status == "" {
			status = "active"
		}
		tools := s.Tools
		if tools == "" {
			tools = "-"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", safe(s.Name, 64), client(s, s.Copilot), client(s, s.OpenCode), tools, dash(s.Login), dash(s.Path), safe(status, 16), safe(s.Description, 70))
	}
	_ = tw.Flush()
	fmt.Printf("\nEnable: %s\n", bold("nav-pilot mcp enable <name>... [--client copilot|opencode]"))
}

func safeFix(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			b.WriteRune('\uFFFD')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func printMCPProblems(problems []mcpProblem) {
	for _, p := range problems {
		who := safe(p.Server, 64)
		if p.Client != "" {
			who += " (" + p.Client + ")"
		}
		fmt.Printf("  %s %s: %s\n", yellow("⚠"), who, safe(p.Problem, 0))
		if p.Fix != "" {
			fmt.Printf("      Fix: %s\n", bold(safeFix(p.Fix)))
		}
	}
}

// diagnoseMCP is the list report, or with only set, the problems of that one
// server. Problems come first in the report: a user runs this because
// something does not work.
func diagnoseMCP(registry string, entries []providerpkg.MCPServerEntry, conf providerpkg.MCPConfigured, only string) mcpReport {
	rep := mcpReport{Registry: registry, Problems: []mcpProblem{}}
	cpltPath := mcpCpltPath()
	mode := mcpHostsMode()
	var st providerpkg.MCPHostState
	if mode != "off" && cpltPath != "" {
		st, _ = readMCPHostState()
	}
	probe := newMCPProbe(cpltPath)
	defer probe.close()
	targets := map[string][]string{}
	for _, e := range entries {
		if (only == "" || e.Name == only) && (conf.Copilot[e.Name] || conf.OpenCode[e.Name]) {
			for _, u := range mcpEntryURLs(e) {
				if t, extra, ok := mcpProbeTarget(u, st); ok {
					targets[t] = extra
				}
			}
		}
	}
	probe.prefetch(targets)
	var brokerSt *mcpCpltStatusDoc
	brokerAsked := false
	add := func(p mcpProblem) {
		// One line per fix: two checks can find the same missing key.
		if !slices.ContainsFunc(rep.Problems, func(q mcpProblem) bool { return q.Server == p.Server && q.Client == p.Client && q.Fix == p.Fix }) {
			rep.Problems = append(rep.Problems, p)
		}
	}
	for _, e := range entries {
		if only != "" && e.Name != only {
			continue
		}
		row := mcpServerRow{Name: e.Name, Description: e.Description, Status: e.Status, Copilot: conf.Copilot[e.Name], OpenCode: conf.OpenCode[e.Name]}
		if e.Meta.Broker != nil && (mcpBrokerEnabled(e.Name) || row.Copilot || row.OpenCode) {
			if !brokerAsked {
				brokerSt, brokerAsked = mcpCpltStatus(cpltPath), true
			}
			for _, p := range mcpBrokerRow(&row, e, cpltPath, brokerSt) {
				add(p)
			}
			rep.Servers = append(rep.Servers, row)
			continue
		}
		if !row.Copilot && !row.OpenCode {
			rep.Servers = append(rep.Servers, row)
			continue
		}
		// Which tools are on, per client; the row shows the widest.
		var toolProblems []mcpProblem
		widest := -1
		var narrowClients []string
		fullGH := false
		for _, client := range []string{providerpkg.MCPClientCopilot, providerpkg.MCPClientOpenCode} {
			if (client == providerpkg.MCPClientCopilot && !row.Copilot) || (client == providerpkg.MCPClientOpenCode && !row.OpenCode) {
				continue
			}
			i, ok := mcpToolsOf(e, client)
			if !ok {
				continue
			}
			if n := len(i.state.On); i.state.All || n > widest {
				widest, row.Tools = n, i.cell(e)
				if i.state.All {
					widest = len(e.Tools) + 1
				}
			}
			fullGH = fullGH || i.fullGH
			if len(i.hostExec) > 0 {
				narrowClients = append(narrowClients, client)
			}
			for _, t := range i.hostExec {
				if !slices.Contains(row.HostExec, t) {
					row.HostExec = append(row.HostExec, t)
				}
			}
			toolProblems = append(toolProblems, mcpToolProblems(e, client, i)...)
		}
		// The flags are either client's, whichever is widest.
		if row.Tools != "" {
			if row.Tools == "read-only" && fullGH {
				row.Tools = "all"
			}
			if len(row.HostExec) > 0 {
				row.Tools += " ⚠ host-exec"
			}
			if fullGH {
				row.Tools += " ⚠ full endpoint"
			}
		}
		if !e.Usable() {
			add(mcpProblem{Server: e.Name, Problem: "marked " + safe(e.Status, 32) + " in the registry; it still runs, but cannot be enabled for new installs", Fix: "nav-pilot mcp disable " + shellQuote(e.Name)})
		}
		// Remotes and sandboxHosts both: a package server can name hosts too.
		for _, u := range mcpEntryURLs(e) {
			h, p := mcpRemoteHost(e.Name, u, cpltPath, mode, st, probe)
			row.Hosts = append(row.Hosts, h)
			if port, ok := strings.CutPrefix(fixOf(p), "cplt config set allow.localhost "); ok && len(row.HostExec) > 0 {
				// The port is a way out of the sandbox while those tools
				// are on: narrow first, then open it.
				// Every client that has them, before the port opens.
				p.Problem += "; ⚠ " + mcpLoopbackWarning(e, port, row.HostExec, mcpLoopbackFix(e, narrowClients))
				p.Fix = mcpNarrowFixes(e, narrowClients) + " && " + p.Fix
			}
			if p != nil {
				add(*p)
			}
		}
		if len(e.Remotes) == 0 && len(e.Packages) > 0 {
			// ponytail: judged by the recipe enable writes (the first
			// package); a hand-written launch line of the user's is not.
			for _, p := range mcpPackageProblems(e, cpltPath) {
				add(p)
			}
		}
		for _, p := range mcpSetupProblems(e, cpltPath) {
			add(p)
		}
		for _, p := range toolProblems {
			// A localhost fix above already narrows first.
			if !slices.ContainsFunc(rep.Problems, func(q mcpProblem) bool { return q.Server == p.Server && strings.Contains(q.Fix, p.Fix) }) {
				add(p)
			}
		}
		rep.Servers = append(rep.Servers, row)
	}
	if only == "" {
		for _, k := range conf.CopilotOther {
			add(mcpUnlisted(entries, k, providerpkg.MCPClientCopilot))
		}
		for _, k := range conf.OpenCodeOther {
			add(mcpUnlisted(entries, k, providerpkg.MCPClientOpenCode))
		}
		for _, r := range conf.OpenCodeRefused {
			add(mcpProblem{Server: r[0], Client: providerpkg.MCPClientOpenCode, Problem: providerpkg.MCPRegistryMismatch + ", so nav-pilot turns it off when it starts OpenCode", Fix: fmt.Sprintf("nav-pilot mcp disable %s --client opencode && nav-pilot mcp enable %s --client opencode", shellQuote(r[0]), shellQuote(r[1]))})
		}
		if cpltPath == "" {
			add(mcpProblem{Server: "cplt", Problem: "cplt is not installed, so no host was checked against the sandbox", Fix: domain.CpltInstallHint(domain.PkgForInstall())})
		}
	}
	return rep
}

// mcpEntryURLs is a remote server's URLs and its registry sandboxHosts (an
// OAuth host, say), so every host it needs is probed and gets a fix line.
func mcpEntryURLs(e providerpkg.MCPServerEntry) []string {
	var out []string
	for _, r := range e.Remotes {
		out = append(out, r.URL)
	}
	for _, h := range e.SandboxHosts {
		out = append(out, "https://"+h)
	}
	return out
}

// mcpRemoteHost probes one remote under cplt and says what to do about a block.
func mcpRemoteHost(server, raw, cpltPath, mode string, st providerpkg.MCPHostState, probe *mcpProbe) (mcpHostRow, *mcpProblem) {
	target, extra, ok := mcpProbeTarget(raw, st)
	if !ok {
		return mcpHostRow{Host: raw}, nil
	}
	host, port, _ := net.SplitHostPort(target)
	row := mcpHostRow{Host: target}
	ip := net.ParseIP(host)
	loopback := host == "localhost" || (ip != nil && ip.IsLoopback())
	if !loopback {
		row.Host = host
		row.Consent = mcpConsentOf(host, st)
		for _, g := range st.Grant {
			switch {
			case g.Host != host:
			case g.Private:
				row.DNS = "private"
			case g.Unknown:
				row.DNS = "unknown"
			}
		}
	}
	if cpltPath == "" {
		return row, nil
	}
	// extra is what the launch adds for a granted private host, so the probe
	// answers for the session the user will actually get.
	verdict, cpltFix := probe.net(target, extra)
	row.Cplt = verdict
	if !strings.HasPrefix(verdict, "BLOCKED") {
		return row, nil
	}
	p := &mcpProblem{Server: server, Problem: "cplt blocks " + row.Host}
	switch {
	case verdict == "BLOCKED-PORT" || loopback:
		p.Problem += " (localhost port " + port + " is closed in the sandbox)"
		p.Fix = "cplt config set allow.localhost " + shellQuote(port)
		return row, p
	case row.Consent == "allowed" && row.DNS == "unknown" && verdict == "BLOCKED-PRIVATE-RESOLVED":
		p.Problem += " (it resolves to a private address, but its DNS lookup failed when you allowed it, so nav-pilot does not waive it yet)"
		p.Fix = "start nav-pilot where the host resolves (naisdevice on); it looks the host up again, and the launch after applies it"
		return row, p
	case row.Consent == "allowed":
		p.Problem += " until nav-pilot's allowlist is updated"
		p.Fix = "start nav-pilot once; the launch applies the hosts you allowed"
		return row, p
	case mode == "off":
		p.Problem += " and nav-pilot does not ask about MCP hosts (mcp_hosts = off)"
		p.Fix = "nav-pilot config set mcp_hosts ask"
		return row, p
	case row.Consent == "pending":
		p.Problem += " (not answered yet)"
		p.Fix = "start nav-pilot in a terminal; it asks whether to allow it"
		return row, p
	}
	switch verdict {
	case "BLOCKED-PRIVATE-RESOLVED":
		p.Problem += " (it resolves to a private address)"
		p.Fix = "cplt config set proxy.allow_private_domains " + shellQuote(host)
	case "BLOCKED-ALLOWLIST":
		p.Problem += " (not in the allowlist)"
		p.Fix = "cplt config set allow.domains " + shellQuote(host)
	default:
		p.Problem += " (" + strings.ToLower(strings.TrimPrefix(verdict, "BLOCKED-")) + ")"
		p.Fix = cpltFix
	}
	if row.Consent == "declined" {
		p.Problem += "; you declined it in nav-pilot"
	}
	return row, p
}

func mcpConsentOf(host string, st providerpkg.MCPHostState) string {
	has := func(hs []providerpkg.MCPHost) bool {
		return slices.ContainsFunc(hs, func(h providerpkg.MCPHost) bool { return h.Host == host })
	}
	switch {
	case has(st.Grant):
		return "allowed"
	case has(st.Pending):
		return "pending"
	case st.Record != nil && !st.Record.Approved && has(st.Previous):
		return "declined"
	}
	return ""
}

// mcpPackageProblems is what a package server needs to start inside cplt.
func mcpPackageProblems(e providerpkg.MCPServerEntry, cpltPath string) []mcpProblem {
	runtime, _ := e.Packages[0].Launch()
	if runtime == "" {
		return []mcpProblem{{Server: e.Name, Problem: "a " + e.Packages[0].RegistryType + " package, which nav-pilot has no launch recipe for", Fix: "configure it by hand; see " + safe(e.Name, 64) + " in the registry"}}
	}
	var out []mcpProblem
	if _, err := mcpLookPath(runtime); err != nil {
		fix := map[string]string{"pnpm": "npm install -g pnpm", "uvx": "brew install uv"}[runtime]
		out = append(out, mcpProblem{Server: e.Name, Problem: runtime + " is not on your PATH; the client starts the server with it", Fix: fix})
	}
	// pnpm dlx runs the package from its cache, where cplt allows no exec.
	if runtime == "pnpm" && cpltPath != "" && !mcpCpltHas(cpltPath, "sandbox.allow_cache_exec", "pnpm/dlx") {
		out = append(out, mcpProblem{Server: e.Name, Problem: "cplt stops pnpm dlx from running the package from its cache (EPERM)", Fix: "cplt config set sandbox.allow_cache_exec pnpm/dlx"})
	}
	return out
}

var cpltConfigKeyRe = regexp.MustCompile(`^[a-z0-9_.]+$`)

// splitPOSIXWords splits a setup command into words, honouring single quotes,
// double quotes with \" \\ \$ \` escapes, and backslash escapes outside quotes.
// Commands with unterminated quotes or unquoted shell metacharacters are rejected.
func splitPOSIXWords(s string) ([]string, bool) {
	var words []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case ' ', '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case '\'':
			inWord = true
			i++
			start := i
			for i < len(s) && s[i] != '\'' {
				i++
			}
			if i >= len(s) {
				return nil, false
			}
			cur.WriteString(s[start:i])
		case '"':
			inWord = true
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' {
					if i+1 >= len(s) {
						return nil, false
					}
					switch next := s[i+1]; next {
					case '"', '\\', '$', '`':
						cur.WriteByte(next)
						i += 2
						continue
					}
				}
				if s[i] == '$' || s[i] == '`' {
					return nil, false // the shell would expand it; the Fix must not differ
				}
				cur.WriteByte(s[i])
				i++
			}
			if i >= len(s) {
				return nil, false
			}
		case '\\':
			if i+1 >= len(s) || s[i+1] == '\n' || s[i+1] == '\r' {
				return nil, false
			}
			i++
			inWord = true
			cur.WriteByte(s[i])
		case ';', '&', '|', '<', '>', '(', ')', '$', '`', '\n', '\r':
			return nil, false
		default:
			inWord = true
			cur.WriteByte(c)
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, true
}

// mcpSetupProblems is the registry's own `cplt config set <key> <value>`
// setup steps that the user's cplt config does not have yet.
func mcpSetupProblems(e providerpkg.MCPServerEntry, cpltPath string) []mcpProblem {
	if cpltPath == "" {
		return nil
	}
	var out []mcpProblem
	for _, s := range e.Setup {
		for _, c := range s.Commands {
			f, ok := splitPOSIXWords(c)
			if !ok || len(f) != 5 || f[0] != "cplt" || f[1] != "config" || f[2] != "set" {
				continue
			}
			if !cpltConfigKeyRe.MatchString(f[3]) {
				continue
			}
			if !mcpCpltHas(cpltPath, f[3], f[4]) {
				out = append(out, mcpProblem{
					Server:  e.Name,
					Problem: "the registry's setup needs " + safe(f[3], 64) + " to include " + safe(f[4], 128),
					Fix:     "cplt config set " + shellQuote(f[3]) + " " + shellQuote(f[4]),
				})
			}
		}
	}
	return out
}

// mcpCpltHas reports whether a cplt config key holds value, as a scalar or
// one item of an array.
func mcpCpltHas(cpltPath, key, value string) bool {
	got := mcpCpltConfigGet(cpltPath, key)
	return slices.Contains(strings.FieldsFunc(got, func(r rune) bool {
		return r == '[' || r == ']' || r == ',' || r == ' ' || r == '"' || r == '\n'
	}), value)
}

// mcpUnlisted is a configured server the registry does not list by that name.
func mcpUnlisted(entries []providerpkg.MCPServerEntry, key, client string) mcpProblem {
	p := mcpProblem{Server: key, Client: client}
	if client == providerpkg.MCPClientCopilot {
		p.Problem = "not in Nav's MCP registry under this name, so Copilot's org policy blocks it"
	} else {
		p.Problem = providerpkg.MCPRegistryMismatch + ", so nav-pilot turns it off when it starts OpenCode"
	}
	for _, e := range entries {
		if short := e.Name[strings.LastIndex(e.Name, "/")+1:]; strings.EqualFold(short, key) {
			p.Problem += "; the registry's name for it is " + safe(e.Name, 64)
			p.Fix = fmt.Sprintf("nav-pilot mcp disable %s --client %s && nav-pilot mcp enable %s --client %s", shellQuote(key), shellQuote(client), shellQuote(e.Name), shellQuote(client))
			return p
		}
	}
	p.Fix = fmt.Sprintf("nav-pilot mcp disable %s --client %s, or ask for it in the registry: %s", shellQuote(key), shellQuote(client), providerpkg.MCPRegistryHelpURL)
	return p
}

// mcpProbe asks cplt, statically, whether the proxy lets a target through.
// Probes run in parallel (prefetch) and each is bounded by a short timeout.
type mcpProbe struct {
	cplt string
	dir  string
	tmp  bool
	mu   sync.Mutex
	seen map[string][2]string
}

func newMCPProbe(cpltPath string) *mcpProbe {
	p := &mcpProbe{cplt: cpltPath, seen: map[string][2]string{}}
	// The policy of the repo the user is in, .cplt.toml included. cplt
	// refuses to sandbox $HOME or /tmp, so outside a repo it gets an empty
	// directory of its own.
	if p.dir = source.FindGitRoot("."); p.dir == "" && cpltPath != "" {
		if d, err := os.MkdirTemp("", "nav-pilot-mcp-"); err == nil {
			p.dir, p.tmp = d, true
		}
	}
	return p
}

func (p *mcpProbe) close() {
	if p.tmp {
		_ = os.RemoveAll(p.dir)
	}
}

// net is the probe's verdict name (ALLOWED, BLOCKED-ALLOWLIST, ...) and
// cplt's own fix text; "" when cplt did not answer.
func (p *mcpProbe) net(target string, extra []string) (string, string) {
	key := strings.Join(append(slices.Clone(extra), target), " ")
	p.mu.Lock()
	r, ok := p.seen[key]
	p.mu.Unlock()
	if !ok {
		args := append(slices.Clone(extra), "-d", p.dir, "check", "--json", "net", "--no-connect", target)
		r[0], r[1] = mcpProbeRun(p.cplt, args)
		p.mu.Lock()
		p.seen[key] = r
		p.mu.Unlock()
	}
	return r[0], r[1]
}

// prefetch runs the probes for these targets at once, so a list costs one
// probe's time rather than one per host.
func (p *mcpProbe) prefetch(targets map[string][]string) {
	if p.cplt == "" {
		return
	}
	var wg sync.WaitGroup
	for target, extra := range targets {
		wg.Go(func() { p.net(target, extra) })
	}
	wg.Wait()
}

// mcpProbeTarget is the cplt target for a remote URL: host:port, and the
// flag the launch adds for a granted private host.
func mcpProbeTarget(raw string, st providerpkg.MCPHostState) (string, []string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", nil, false
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	var extra []string
	for _, g := range st.Grant {
		if g.Host == host && g.Private {
			extra = []string{"--allow-private-domain", host}
		}
	}
	return net.JoinHostPort(host, port), extra, true
}

// mcpProbeRun runs cplt; a var so tests answer without cplt.
var mcpProbeRun = func(cplt string, args []string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), cpltCommandTimeout+time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, cplt, args...).Output() // blocked exits non-zero
	var res struct {
		Items []struct {
			Name string `json:"name"`
			Fix  string `json:"fix"`
		} `json:"items"`
	}
	if json.Unmarshal(out, &res) != nil || len(res.Items) == 0 {
		return "", ""
	}
	return res.Items[0].Name, res.Items[0].Fix
}

// mcpResolve finds a registry server by its full name or the part after the
// last slash, when that is unique.
func mcpResolve(entries []providerpkg.MCPServerEntry, name string) (providerpkg.MCPServerEntry, error) {
	var hits []providerpkg.MCPServerEntry
	for _, e := range entries {
		if e.Name == name {
			return e, nil
		}
		if strings.EqualFold(e.Name[strings.LastIndex(e.Name, "/")+1:], name) {
			hits = append(hits, e)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return providerpkg.MCPServerEntry{}, fmt.Errorf("%q is not in Nav's MCP registry. See what is: nav-pilot mcp list", safe(name, 64))
	}
	var names []string
	for _, h := range hits {
		names = append(names, h.Name)
	}
	return providerpkg.MCPServerEntry{}, fmt.Errorf("%q matches %s; use the full name", safe(name, 64), strings.Join(names, ", "))
}

// mcpClientInstalled is a var so tests decide which clients exist.
var mcpClientInstalled = func(client string) bool {
	p, err := providerFor(client)
	return err == nil && p.Available()
}

// mcpClients is the clients enable and disable change: --client, else every
// client nav-pilot writes MCP config for that is installed.
func mcpClients(flag string) ([]string, error) {
	if flag != "" {
		return []string{flag}, nil
	}
	var out []string
	for _, c := range []string{providerpkg.MCPClientCopilot, providerpkg.MCPClientOpenCode} {
		if mcpClientInstalled(c) {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("neither copilot nor opencode is installed; name the client with --client copilot or --client opencode")
	}
	return out, nil
}

// errMCPFailed is the exit for a run where some names failed; each was
// reported on its own line already.
func errMCPFailed(verb string, failed, total int) error {
	return fmt.Errorf("%d of %d server(s) could not be %s", failed, total, verb)
}

func mcpFail(name string, err error) {
	fmt.Fprintf(os.Stderr, "%s %s: %v\n", red("✗"), safe(name, 64), err)
}

func cmdMCPEnable(names []string, clientsOf func() ([]string, error), topts mcpToolOpts, jsonOut, refresh bool) error {
	// A name the cache does not know may be newer than the cache: read the
	// registry once more before failing on it.
	if !refresh {
		if _, entries, _, err := mcpRegistryServers(); err == nil {
			for _, name := range names {
				if _, err := mcpResolve(entries, name); err != nil && strings.Contains(err.Error(), "is not in Nav's MCP registry") {
					refresh = true
					break
				}
			}
		}
	}
	if refresh {
		if err := mcpRegistryRefresh(); err != nil {
			fmt.Fprintf(os.Stderr, "nav-pilot: could not read Nav's MCP registry again: %v\n", err)
		}
	}
	registry, entries, stale, err := mcpRegistryServers()
	if err != nil {
		return fmt.Errorf("could not reach Nav's MCP registry, so nothing was changed: %w", err)
	}
	mcpNoteStale(stale)
	failed := 0
	// Partition first: cplt runs a brokered server, the client config holds
	// every other. A name that resolves to neither fails here.
	var brokered, managed []providerpkg.MCPServerEntry
	for _, name := range names {
		e, err := mcpResolve(entries, name)
		switch {
		case err != nil:
		case !e.Usable():
			err = fmt.Errorf("%s is %s in the registry (%s), so nav-pilot will not enable it", e.Name, safe(e.Status, 32), registry)
		case e.Meta.Broker != nil:
			brokered = append(brokered, e)
			continue
		case len(e.Remotes) == 0 && len(e.Packages) == 0:
			// Only cplt can run such a server (a host CLI), and this registry
			// answer has no broker extension for it yet.
			err = fmt.Errorf("the registry does not offer %s through cplt yet, so nothing was written. Fix: nav-pilot update, or wait until the registry is deployed and run nav-pilot mcp enable %s again", e.Name, shellQuote(e.Name))
		case jsonOut:
			err = fmt.Errorf("--json is only for servers cplt runs on the host. Fix: nav-pilot mcp enable %s", shellQuote(e.Name))
		default:
			managed = append(managed, e)
			continue
		}
		mcpFail(name, err)
		failed++
	}
	var clients []string
	if len(managed) > 0 {
		if clients, err = clientsOf(); err != nil {
			return err
		}
	}
	// Every pick comes before any write, brokered servers included, so Esc
	// in a later picker leaves the whole command undone, not half of it.
	type pick struct {
		e      providerpkg.MCPServerEntry
		choice providerpkg.MCPToolChoice
	}
	var picks []pick
	for _, e := range managed {
		// No picker when every client already has the server: without a
		// flag its entry is kept, so a pick would be thrown away.
		o := topts
		o.noPick = !topts.explicit() && !slices.ContainsFunc(clients, func(c string) bool { return providerpkg.MCPConfigKeyFor(c, e) == "" })
		choice, err := mcpChooseTools(e, o)
		if errors.As(err, new(cancelledError)) {
			return err
		}
		if err != nil {
			mcpFail(e.Name, err)
			failed++
			continue
		}
		picks = append(picks, pick{e, choice})
	}
	failed += enableBrokeredServers(brokered, topts, jsonOut)
	var enabled []providerpkg.MCPServerEntry
	for _, p := range picks {
		e, choice := p.e, p.choice
		ok := true
		var wrote []string
		for _, client := range clients {
			w, err := mcpEnableIn(client, e, choice, topts.explicit())
			if err != nil {
				mcpFail(e.Name+" ("+client+")", err)
				ok = false
			}
			if w {
				wrote = append(wrote, client)
			}
		}
		if !ok {
			failed++
			continue
		}
		// Only about entries written now: an existing one kept its tools.
		if len(wrote) > 0 && !topts.explicit() && !choice.All && len(choice.Tools) < len(e.Tools) {
			// One per client: without --client, enable adds the server to
			// every client.
			var hints []string
			for _, c := range wrote {
				h := "nav-pilot mcp enable " + shellQuote(e.Name) + " --client " + shellQuote(c) + " --tools <a,b>"
				if len(e.HostExecTools()) > 0 {
					h += " --allow-host-exec"
				}
				hints = append(hints, h)
			}
			hint := strings.Join(hints, " && ")
			fmt.Printf("%s In the new %s entry, tools that act in other systems or run outside the sandbox are off. To pick them: %s\n", dim("ℹ"), strings.Join(wrote, " and "), bold(hint))
		}
		if !slices.ContainsFunc(enabled, func(x providerpkg.MCPServerEntry) bool { return x.Name == e.Name }) {
			enabled = append(enabled, e)
		}
	}
	if len(enabled) > 0 {
		// The sandbox half: the launch's host consent, one screen for every
		// server (default No, one line without a terminal, nothing with
		// mcp_hosts = off).
		client := mcpLaunchClient()
		if !slices.Contains(clients, client) {
			client = clients[0]
		}
		noteMCPHostConsent(client, true)
		conf := mcpConfigured(entries)
		for _, e := range enabled {
			mcpEnableFollowUp(registry, entries, conf, e, clients)
		}
		fmt.Printf("%s Start a new %s session to load it.\n", dim("ℹ"), strings.Join(clients, " or "))
	}
	if failed > 0 {
		return errMCPFailed("enabled", failed, len(names))
	}
	return nil
}

// mcpBackfillOpenCodeProtocol adds a missing protocol to OpenCode entries of
// these registry servers, one line per entry it changes.
func mcpBackfillOpenCodeProtocol(entries []providerpkg.MCPServerEntry) {
	if len(entries) == 0 {
		return
	}
	byName := make(map[string]providerpkg.MCPServerEntry, len(entries))
	for _, e := range entries {
		byName[e.Name] = e
	}
	for _, key := range providerpkg.MCPConfigKeys(providerpkg.MCPClientOpenCode) {
		name := providerpkg.OpenCodeRegistryNameFor(key, entries)
		if name == "" {
			continue
		}
		e, ok := byName[name]
		if !ok {
			continue
		}
		p, ch, err := providerpkg.BackfillOpenCodeProtocol(key, e)
		if err != nil {
			if errors.Is(err, providerpkg.ErrMCPConfigHasComments) {
				continue
			}
			fmt.Printf("%s Could not set protocol on %s: %v\n", yellow("⚠"), safe(key, 64), err)
			continue
		}
		if p == "" {
			continue
		}
		fmt.Printf("%s Set protocol %q on %s in %s, from the registry\n", green("✓"), p, safe(key, 64), ch.Path)
	}
}

// mcpEnableIn writes the server into the client's config with the chosen
// tools. An entry already there is kept, unless the user chose tools
// (explicit): then only its tools change. The bool is whether it wrote a
// new entry.
func mcpEnableIn(client string, e providerpkg.MCPServerEntry, c providerpkg.MCPToolChoice, explicit bool) (bool, error) {
	key := providerpkg.MCPConfigKeyFor(client, e)
	if client == providerpkg.MCPClientOpenCode && key != "" {
		mcpBackfillOpenCodeProtocol([]providerpkg.MCPServerEntry{e})
	}
	if key != "" && explicit {
		ch, err := providerpkg.SetMCPServerTools(client, key, e, c)
		if errors.Is(err, providerpkg.ErrMCPConfigHasComments) {
			return false, fmt.Errorf("%s has comments or trailing commas, which a rewrite would lose, so nav-pilot did not change it", ch.Path)
		}
		if err != nil {
			return false, err
		}
		backup := ""
		if ch.Backup != "" {
			backup = " " + dim("(previous file: "+ch.Backup+")")
		}
		if !ch.Changed {
			fmt.Printf("%s %s already has these tools for %s in %s: %s\n", dim("•"), e.Name, client, ch.Path, mcpDescribeChoice(e, c))
			return false, nil
		}
		fmt.Printf("%s Set the tools of %s for %s in %s: %s%s\n", green("✓"), bold(e.Name), client, ch.Path, mcpDescribeChoice(e, c), backup)
		return false, nil
	}
	if key != "" && key != e.Name {
		fmt.Printf("%s %s is already enabled for %s, as %s\n", dim("•"), e.Name, client, safe(key, 64))
		return false, nil
	}
	entry, err := providerpkg.MCPClientEntry(client, e, c)
	if err != nil {
		return false, err
	}
	ch, err := providerpkg.SetMCPServer(client, e.Name, entry, e, c)
	if errors.Is(err, providerpkg.ErrMCPConfigHasComments) {
		key := "mcpServers"
		if client == providerpkg.MCPClientOpenCode {
			key = "mcp"
		}
		snippet, _ := json.MarshalIndent(map[string]json.RawMessage{e.Name: entry}, "", "  ")
		msg := fmt.Sprintf("%s has comments or trailing commas, which a rewrite would lose, so nav-pilot did not change it. Add this under %q yourself:\n%s", ch.Path, key, snippet)
		if rules := providerpkg.OpenCodeRules(e.Name, c); client == providerpkg.MCPClientOpenCode && len(rules) > 0 {
			msg += "\nand these, in this order, at the end of \"permission\":"
			for _, r := range rules {
				msg += fmt.Sprintf("\n  %q: %q", r[0], r[1])
			}
		}
		return false, errors.New(msg)
	}
	if err != nil {
		return false, err
	}
	switch {
	case ch.Changed && ch.Backup != "":
		fmt.Printf("%s Enabled %s for %s in %s %s\n", green("✓"), bold(e.Name), client, ch.Path, dim("(previous file: "+ch.Backup+")"))
		fmt.Printf("  Tools on: %s\n", mcpDescribeChoice(e, c))
	case ch.Changed:
		fmt.Printf("%s Enabled %s for %s in %s\n", green("✓"), bold(e.Name), client, ch.Path)
		fmt.Printf("  Tools on: %s\n", mcpDescribeChoice(e, c))
	case ch.Existing != nil:
		fmt.Printf("%s %s already has an entry for %s that differs from the registry's; kept yours.\n  To change only its tools: %s\n  To replace it: %s\n",
			dim("•"), ch.Path, e.Name, bold(fmt.Sprintf("nav-pilot mcp enable %s --client %s --tools <a,b> (or --all-tools)", shellQuote(e.Name), shellQuote(client))),
			bold(fmt.Sprintf("nav-pilot mcp disable %s --client %s && nav-pilot mcp enable %s --client %s", shellQuote(e.Name), shellQuote(client), shellQuote(e.Name), shellQuote(client))))
	default:
		fmt.Printf("%s %s is already enabled for %s in %s\n", dim("•"), e.Name, client, ch.Path)
	}
	short := e.Name[strings.LastIndex(e.Name, "/")+1:]
	if client == providerpkg.MCPClientCopilot && short != e.Name && slices.Contains(providerpkg.MCPConfigKeys(client), short) {
		fmt.Printf("%s Your config also has %s, which Copilot's org policy blocks. Remove it: %s\n", yellow("⚠"), short, bold("nav-pilot mcp disable "+shellQuote(short)+" --client copilot"))
	}
	return ch.Changed, nil
}

// mcpEnableFollowUp prints what the server still needs after the write.
func mcpEnableFollowUp(registry string, entries []providerpkg.MCPServerEntry, conf providerpkg.MCPConfigured, e providerpkg.MCPServerEntry, clients []string) {
	rep := diagnoseMCP(registry, entries, conf, e.Name)
	if len(rep.Problems) > 0 {
		fmt.Printf("\n%s\n", bold("Still needed for "+e.Name))
		printMCPProblems(rep.Problems)
	}
	var other []providerpkg.MCPSetupStep
	for _, s := range e.Setup {
		if !slices.ContainsFunc(s.Commands, func(c string) bool { return strings.HasPrefix(c, "cplt config set ") }) {
			other = append(other, s)
		}
	}
	if len(other) > 0 {
		fmt.Printf("\n%s\n", bold("Setup steps from the registry for "+e.Name))
		for _, s := range other {
			fmt.Printf("  %s\n", safe(s.Title, 120))
			for _, c := range s.Commands {
				fmt.Printf("      %s\n", bold(safe(c, 200)))
			}
		}
	}
	if len(e.Remotes) > 0 && strings.HasPrefix(e.Remotes[0].URL, "https://") {
		fmt.Printf("%s %s: if it needs a sign-in, %s asks the first time it connects.\n", dim("ℹ"), e.Name, strings.Join(clients, " or "))
	}
}

func cmdMCPDisable(names []string, clientsOf func() ([]string, error)) error {
	var entries []providerpkg.MCPServerEntry
	var regErr, stale error
	fetched := false
	failed, removed := 0, 0
	for _, name := range names {
		// A brokered server is its cplt spec, plus any client entry the user
		// wrote by hand: cplt overrides that only while a spec is there.
		id := name
		if !strings.Contains(id, "/") && !mcpBrokerEnabled(id) {
			if !fetched {
				_, entries, stale, regErr = mcpRegistryServers()
				mcpNoteStale(stale)
				fetched = true
			}
			if e, err := mcpResolve(entries, name); err == nil {
				id = e.Name
			}
		}
		gone, err := mcpDisableBroker(id)
		if err != nil {
			mcpFail(name, err)
			failed++
			continue
		}
		clients, err := clientsOf()
		if err != nil && gone {
			continue
		}
		if err != nil {
			mcpFail(name, err)
			failed++
			continue
		}
		found := gone
		for _, client := range clients {
			keys := providerpkg.MCPConfigKeys(client)
			key := name
			if !slices.Contains(keys, key) {
				// Not a name in the file: the registry's full name for it.
				if !fetched {
					_, entries, stale, regErr = mcpRegistryServers()
					mcpNoteStale(stale)
					fetched = true
				}
				e, err := mcpResolve(entries, name)
				if err != nil {
					continue
				}
				if key = providerpkg.MCPConfigKeyFor(client, e); key == "" {
					continue
				}
			}
			found = true
			ch, err := providerpkg.RemoveMCPServer(client, key, mcpKnownTools(client, key, entries))
			if errors.Is(err, providerpkg.ErrMCPConfigHasComments) {
				err = fmt.Errorf("%s has comments or trailing commas, so nav-pilot did not change it; remove %q from it yourself", ch.Path, key)
			}
			if err != nil {
				mcpFail(name+" ("+client+")", err)
				failed++
				continue
			}
			if ch.Changed {
				removed++
				fmt.Printf("%s Disabled %s for %s in %s %s\n", green("✓"), bold(key), client, ch.Path, dim("(previous file: "+ch.Backup+")"))
			}
		}
		if !found {
			msg := fmt.Sprintf("not in your %s MCP config", strings.Join(clients, " or "))
			if regErr != nil {
				msg += fmt.Sprintf(" (Nav's MCP registry did not answer, so a short name could not be looked up: %v)", regErr)
			}
			mcpFail(name, errors.New(msg+". See: nav-pilot mcp list"))
			failed++
		}
	}
	if removed > 0 {
		dropMCPHosts()
	}
	if failed > 0 {
		return errMCPFailed("disabled", failed, len(names))
	}
	return nil
}

// dropMCPHosts narrows the host approval to the servers still configured
// in either client, so a dropped host is asked about again if its server
// comes back, and brings the allowlist file in step.
func dropMCPHosts() {
	mcpConfigScope()
	if cpltInstalled() {
		gone, err := narrowMCPApproval()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not update the MCP host approval: %v\n", yellow("⚠"), err)
			return
		}
		if len(gone) > 0 {
			sort.Strings(gone)
			fmt.Printf("%s No longer allowed in the sandbox, as no other MCP server needs them: %s\n", green("✓"), strings.Join(gone, ", "))
		}
	}
	syncMCPAllowlist()
}

func fixOf(p *mcpProblem) string {
	if p == nil {
		return ""
	}
	return p.Fix
}

// mcpRegistryNameRe matches the registry's reverse-DNS <namespace>/<name>
// format, the key nav-pilot mcp enable writes. A dot in the namespace is
// required, so short names and plain org/server keys are never matched.
var mcpRegistryNameRe = regexp.MustCompile(`^[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)+/[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// mcpPrunable is whether name looks like an entry nav-pilot wrote: the
// registry's format, under a namespace the registry still has servers in.
// A server the user added under any other name is never touched.
func mcpPrunable(namespaces map[string]bool, name string) bool {
	ns, _, ok := strings.Cut(name, "/")
	return ok && namespaces[ns] && mcpRegistryNameRe.MatchString(name)
}

func mcpEntryWord(n int) string {
	if n == 1 {
		return "entry"
	}
	return "entries"
}

type mcpPrunePlan struct {
	client      string
	path        string
	keys        []string
	hasComments bool
}

func cmdMCPPrune(clientFlag string, dryRun, yes bool) error {
	_, entries, stale, err := mcpRegistryServers()
	if err != nil {
		return fmt.Errorf("could not reach Nav's MCP registry, so nothing was changed: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("the MCP registry returned no servers, so nothing was changed")
	}
	if stale != nil {
		// A cached answer can miss servers added since; pruning by it would delete valid entries.
		return fmt.Errorf("could not read Nav's MCP registry again (%v), so nothing was changed", stale)
	}

	clients := []string{providerpkg.MCPClientCopilot, providerpkg.MCPClientOpenCode}
	if clientFlag != "" {
		clients = []string{clientFlag}
	}

	listed := make(map[string]bool, len(entries))
	namespaces := map[string]bool{}
	for _, e := range entries {
		listed[e.Name] = true
		if ns, _, ok := strings.Cut(e.Name, "/"); ok {
			namespaces[ns] = true
		}
	}

	var plans []mcpPrunePlan
	removable, skipped := 0, 0
	for _, client := range clients {
		path, fileKeys, hasComments := providerpkg.MCPConfigKeysAndComments(client)
		if len(fileKeys) == 0 {
			continue
		}
		// Read the keys straight from the file (both OpenCode layouts, disabled
		// entries included): the registry's URL/package matching misses
		// renamed servers that kept their URL.
		var candidates []string
		for _, k := range fileKeys {
			if !listed[k] && mcpPrunable(namespaces, k) {
				candidates = append(candidates, k)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		plans = append(plans, mcpPrunePlan{client: client, path: path, keys: candidates, hasComments: hasComments})
		if hasComments {
			skipped += len(candidates)
		} else {
			removable += len(candidates)
		}
	}

	if removable == 0 && skipped == 0 {
		fmt.Printf("%s Nothing to prune.\n", dim("•"))
		return nil
	}

	printCommentSkip := func(p mcpPrunePlan, dry bool) {
		quoted := make([]string, len(p.keys))
		for i, k := range p.keys {
			quoted[i] = fmt.Sprintf("%q", safe(k, 64))
		}
		verb := "did"
		if dry {
			verb = "would"
		}
		fmt.Printf("%s %s has comments or trailing commas, so nav-pilot %s not change it; remove %s from it yourself\n",
			yellow("⚠"), p.path, verb, strings.Join(quoted, ", "))
	}

	if dryRun {
		for _, p := range plans {
			if p.hasComments {
				continue
			}
			for _, k := range p.keys {
				fmt.Printf("%s Would remove %s for %s in %s\n", dim("•"), bold(safe(k, 64)), p.client, p.path)
			}
		}
		for _, p := range plans {
			if p.hasComments {
				printCommentSkip(p, true)
			}
		}
		switch {
		case removable > 0 && skipped > 0:
			fmt.Printf("Dry run: would remove %d %s, skip %d in commented file(s).\n", removable, mcpEntryWord(removable), skipped)
		case removable > 0:
			fmt.Printf("Dry run: would remove %d %s.\n", removable, mcpEntryWord(removable))
		default:
			fmt.Printf("Dry run: nothing would be changed; %d %s skipped in commented file(s).\n", skipped, mcpEntryWord(skipped))
		}
		return nil
	}

	if removable == 0 {
		for _, p := range plans {
			printCommentSkip(p, false)
		}
		fmt.Printf("Removed 0 entries; skipped %d %s in commented file(s).\n", skipped, mcpEntryWord(skipped))
		return nil
	}

	commentedPrinted := false
	if !yes {
		for _, p := range plans {
			if p.hasComments {
				continue
			}
			for _, k := range p.keys {
				fmt.Printf("  %s %s (%s, %s)\n", dim("•"), safe(k, 64), p.client, p.path)
			}
		}
		for _, p := range plans {
			if p.hasComments {
				printCommentSkip(p, false)
				commentedPrinted = true
			}
		}
		if !isInteractive() {
			return fmt.Errorf("without a terminal to ask, pass --yes to remove %d %s (or --dry-run to preview)", removable, mcpEntryWord(removable))
		}
		if !mcpAsk(fmt.Sprintf("Remove %d MCP %s no longer in Nav's MCP registry?", removable, mcpEntryWord(removable))) {
			fmt.Println(dim("Cancelled. Nothing was changed."))
			return nil
		}
	}

	removedCount, failed := 0, 0
	var backups []string
	for _, p := range plans {
		if p.hasComments {
			if !commentedPrinted {
				printCommentSkip(p, false)
			}
			continue
		}
		removedKeys, ch, err := providerpkg.RemoveMCPServers(p.client, p.keys)
		if errors.Is(err, providerpkg.ErrMCPConfigHasComments) {
			skipped += len(p.keys)
			printCommentSkip(p, false)
			continue
		}
		if err != nil {
			mcpFail(p.client, err)
			failed++
			continue
		}
		for _, k := range removedKeys {
			fmt.Printf("%s Removed %s for %s in %s\n", green("✓"), bold(safe(k, 64)), p.client, ch.Path)
			removedCount++
		}
		if ch.Backup != "" {
			backups = append(backups, ch.Backup)
		}
	}

	if removedCount > 0 {
		dropMCPHosts()
	}

	summary := fmt.Sprintf("Removed %d %s", removedCount, mcpEntryWord(removedCount))
	if skipped > 0 {
		summary += fmt.Sprintf("; skipped %d in commented file(s)", skipped)
	}
	if len(backups) > 0 {
		summary += fmt.Sprintf(" (previous file(s): %s)", strings.Join(backups, ", "))
	}
	fmt.Println(summary + ".")

	if failed > 0 {
		return fmt.Errorf("%d client config(s) could not be changed", failed)
	}
	return nil
}
