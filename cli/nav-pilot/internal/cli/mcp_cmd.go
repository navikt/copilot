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
	"slices"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// nav-pilot mcp list|enable|disable: the MCP servers in Nav's registry, what
// the user's clients have configured, and what stands between a configured
// server and a working one under cplt. list only reads. enable and disable
// are the only paths that write a client config, and only the one asked for.

// Test seams.
var (
	mcpRegistryServers = providerpkg.MCPRegistryServers
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
	var client string
	var jsonOut bool
	var pos []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--json":
			jsonOut = true
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
	switch pos[0] {
	case "list", "ls":
		if len(pos) > 1 {
			return fmt.Errorf("mcp list takes no arguments\n\n%s", mcpUsage)
		}
		return cmdMCPList(jsonOut)
	case "enable", "disable":
		// Only the servers named: never all of them by omission.
		if len(pos) < 2 {
			return fmt.Errorf("mcp %s needs one or more server names; see them with: nav-pilot mcp list\n\n%s", pos[0], mcpUsage)
		}
		if jsonOut {
			return fmt.Errorf("--json is for mcp list")
		}
		clients, err := mcpClients(client)
		if err != nil {
			return err
		}
		if pos[0] == "enable" {
			return cmdMCPEnable(pos[1:], clients)
		}
		return cmdMCPDisable(pos[1:], clients)
	}
	return fmt.Errorf("unknown mcp command %q\n\n%s", pos[0], mcpUsage)
}

const mcpUsage = `Usage: nav-pilot mcp [list] [--json]
       nav-pilot mcp enable <name>... [--client copilot|opencode]
       nav-pilot mcp disable <name>... [--client copilot|opencode]`

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
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	Copilot     bool         `json:"copilot"`
	OpenCode    bool         `json:"opencode"`
	Hosts       []mcpHostRow `json:"hosts,omitempty"`
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

func cmdMCPList(jsonOut bool) error {
	registry, entries, stale, err := mcpRegistryServers()
	if err != nil {
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
		fmt.Printf("%s No problems found with your MCP servers.\n", green("✓"))
	} else {
		fmt.Printf("%s\n", bold(fmt.Sprintf("Problems (%d)", len(rep.Problems))))
		printMCPProblems(rep.Problems)
	}
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
	fmt.Fprintln(tw, "  NAME\tCOPILOT\tOPENCODE\tSTATUS\tDESCRIPTION")
	on := func(b bool) string {
		if b {
			return "on"
		}
		return "-"
	}
	for _, s := range rep.Servers {
		status := s.Status
		if status == "" {
			status = "active"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", safe(s.Name, 64), on(s.Copilot), on(s.OpenCode), safe(status, 16), safe(s.Description, 70))
	}
	_ = tw.Flush()
	fmt.Printf("\nEnable: %s\n", bold("nav-pilot mcp enable <name>... [--client copilot|opencode]"))
}

func printMCPProblems(problems []mcpProblem) {
	for _, p := range problems {
		who := safe(p.Server, 64)
		if p.Client != "" {
			who += " (" + p.Client + ")"
		}
		fmt.Printf("  %s %s: %s\n", yellow("⚠"), who, p.Problem)
		fmt.Printf("      Fix: %s\n", bold(p.Fix))
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
			for _, r := range e.Remotes {
				if t, extra, ok := mcpProbeTarget(r.URL, st); ok {
					targets[t] = extra
				}
			}
		}
	}
	probe.prefetch(targets)
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
		if !row.Copilot && !row.OpenCode {
			rep.Servers = append(rep.Servers, row)
			continue
		}
		if !e.Usable() {
			add(mcpProblem{Server: e.Name, Problem: "retired in the registry (status " + e.Status + "); the org policy will stop running it", Fix: "nav-pilot mcp disable " + e.Name})
		}
		if len(e.Remotes) > 0 {
			for _, r := range e.Remotes {
				h, p := mcpRemoteHost(e.Name, r.URL, cpltPath, mode, st, probe)
				row.Hosts = append(row.Hosts, h)
				if p != nil {
					add(*p)
				}
			}
		} else if len(e.Packages) > 0 {
			// ponytail: judged by the recipe enable writes (the first
			// package); a hand-written launch line of the user's is not.
			for _, p := range mcpPackageProblems(e, cpltPath) {
				add(p)
			}
		}
		for _, p := range mcpSetupProblems(e, cpltPath) {
			add(p)
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
		if cpltPath == "" {
			add(mcpProblem{Server: "cplt", Problem: "cplt is not installed, so no host was checked against the sandbox", Fix: "brew install navikt/tap/cplt"})
		}
	}
	return rep
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
		p.Fix = "cplt config set allow.localhost " + port
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
		p.Fix = "cplt config set proxy.allow_private_domains " + host
	case "BLOCKED-ALLOWLIST":
		p.Problem += " (not in the allowlist)"
		p.Fix = "cplt config set allow.domains " + host
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
		return []mcpProblem{{Server: e.Name, Problem: "a " + e.Packages[0].RegistryType + " package, which nav-pilot has no launch recipe for", Fix: "configure it by hand; see " + e.Name + " in the registry"}}
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

// mcpSetupProblems is the registry's own `cplt config set <key> <value>`
// setup steps that the user's cplt config does not have yet.
func mcpSetupProblems(e providerpkg.MCPServerEntry, cpltPath string) []mcpProblem {
	if cpltPath == "" {
		return nil
	}
	var out []mcpProblem
	for _, s := range e.Setup {
		for _, c := range s.Commands {
			f := strings.Fields(c)
			if len(f) != 5 || f[0] != "cplt" || f[1] != "config" || f[2] != "set" {
				continue
			}
			if !mcpCpltHas(cpltPath, f[3], f[4]) {
				out = append(out, mcpProblem{Server: e.Name, Problem: "the registry's setup needs " + f[3] + " to include " + f[4], Fix: safe(c, 200)})
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
		p.Problem = "not in Nav's MCP registry, so nav-pilot turns it off when it starts OpenCode"
	}
	for _, e := range entries {
		if short := e.Name[strings.LastIndex(e.Name, "/")+1:]; strings.EqualFold(short, key) {
			p.Problem += "; the registry's name for it is " + e.Name
			p.Fix = fmt.Sprintf("nav-pilot mcp disable %s --client %s && nav-pilot mcp enable %s --client %s", key, client, e.Name, client)
			return p
		}
	}
	p.Fix = fmt.Sprintf("nav-pilot mcp disable %s --client %s, or ask for it in the registry: %s", key, client, providerpkg.MCPRegistryHelpURL)
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

func cmdMCPEnable(names []string, clients []string) error {
	registry, entries, stale, err := mcpRegistryServers()
	if err != nil {
		return fmt.Errorf("could not reach Nav's MCP registry, so nothing was changed: %w", err)
	}
	mcpNoteStale(stale)
	failed := 0
	var enabled []providerpkg.MCPServerEntry
	for _, name := range names {
		e, err := mcpResolve(entries, name)
		if err == nil && !e.Usable() {
			err = fmt.Errorf("%s is %s in the registry (%s), so nav-pilot will not enable it", e.Name, safe(e.Status, 32), registry)
		}
		if err != nil {
			mcpFail(name, err)
			failed++
			continue
		}
		ok := true
		for _, client := range clients {
			if err := mcpEnableIn(client, e); err != nil {
				mcpFail(e.Name+" ("+client+")", err)
				ok = false
			}
		}
		if !ok {
			failed++
			continue
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
		noteMCPHostConsent(client)
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

func mcpEnableIn(client string, e providerpkg.MCPServerEntry) error {
	if key := providerpkg.MCPConfigKeyFor(client, e); key != "" && key != e.Name {
		fmt.Printf("%s %s is already enabled for %s, as %s\n", dim("•"), e.Name, client, safe(key, 64))
		return nil
	}
	entry, err := providerpkg.MCPClientEntry(client, e)
	if err != nil {
		return err
	}
	ch, err := providerpkg.SetMCPServer(client, e.Name, entry)
	if errors.Is(err, providerpkg.ErrMCPConfigHasComments) {
		key := "mcpServers"
		if client == providerpkg.MCPClientOpenCode {
			key = "mcp"
		}
		snippet, _ := json.MarshalIndent(map[string]json.RawMessage{e.Name: entry}, "", "  ")
		return fmt.Errorf("%s has comments or trailing commas, which a rewrite would lose, so nav-pilot did not change it. Add this under %q yourself:\n%s", ch.Path, key, snippet)
	}
	if err != nil {
		return err
	}
	switch {
	case ch.Changed && ch.Backup != "":
		fmt.Printf("%s Enabled %s for %s in %s %s\n", green("✓"), bold(e.Name), client, ch.Path, dim("(previous file: "+ch.Backup+")"))
	case ch.Changed:
		fmt.Printf("%s Enabled %s for %s in %s\n", green("✓"), bold(e.Name), client, ch.Path)
	case ch.Existing != nil:
		fmt.Printf("%s %s already has an entry for %s that differs from the registry's; kept yours.\n  To replace it: %s\n",
			dim("•"), ch.Path, e.Name, bold(fmt.Sprintf("nav-pilot mcp disable %s --client %s && nav-pilot mcp enable %s --client %s", e.Name, client, e.Name, client)))
	default:
		fmt.Printf("%s %s is already enabled for %s in %s\n", dim("•"), e.Name, client, ch.Path)
	}
	short := e.Name[strings.LastIndex(e.Name, "/")+1:]
	if client == providerpkg.MCPClientCopilot && short != e.Name && slices.Contains(providerpkg.MCPConfigKeys(client), short) {
		fmt.Printf("%s Your config also has %s, which Copilot's org policy blocks. Remove it: %s\n", yellow("⚠"), short, bold("nav-pilot mcp disable "+short+" --client copilot"))
	}
	return nil
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

func cmdMCPDisable(names []string, clients []string) error {
	var entries []providerpkg.MCPServerEntry
	var regErr, stale error
	fetched := false
	failed, removed := 0, 0
	for _, name := range names {
		found := false
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
			ch, err := providerpkg.RemoveMCPServer(client, key)
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
