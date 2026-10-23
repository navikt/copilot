package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// Which tools nav-pilot mcp enable turns on, and what mcp list and doctor
// say about the ones that are on. Reads and writes are on by default;
// external and host-exec tools (the registry's toolRisk) only when the user
// picks them, and host-exec only after a yes or --allow-host-exec.

// mcpToolOpts is enable's tool flags.
type mcpToolOpts struct {
	tools         []string // --tools a,b
	all           bool     // --all-tools
	allowHostExec bool     // --allow-host-exec
	noPick        bool     // the defaults, never the terminal's picker
}

// explicit is a choice the user made, which may change an existing entry.
func (o mcpToolOpts) explicit() bool { return o.tools != nil || o.all }

var mcpRiskLabel = map[string]string{
	providerpkg.MCPRiskRead:     "read",
	providerpkg.MCPRiskWrite:    "write",
	providerpkg.MCPRiskExternal: "external: acts in another system",
	providerpkg.MCPRiskHostExec: "host-exec: runs outside the cplt sandbox",
}

// mcpPickTools is the terminal's multi-select, the defaults ticked. A var so
// tests answer it.
var mcpPickTools = func(e providerpkg.MCPServerEntry) ([]string, error) {
	var opts []huh.Option[string]
	for _, t := range e.Tools {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%-36s %s", t, mcpRiskLabel[e.RiskOf(t)]), t).
			Selected(slices.Contains(e.DefaultTools(), t)))
	}
	pick := []string{}
	// Esc cancels, like ctrl+c; the description names it, so no footer.
	err := huh.NewForm(huh.NewGroup(escHelpField{huh.NewMultiSelect[string]().
		Title("Which tools of " + e.Name + " should the agent get?").
		Description("Space picks, Enter confirms, Esc cancels. Reads and writes are picked; the rest are off by default.").
		Options(opts...).Value(&pick)})).WithShowHelp(false).WithTheme(navTheme()).Run()
	return pick, err
}

// mcpAsk is a yes/no in the terminal, default no. A var so tests answer it.
// Esc, like ctrl+c, is no.
var mcpAsk = func(title string) bool {
	ok := false
	return isInteractive() && runField(huh.NewConfirm().Title(title).Value(&ok)) == nil && ok
}

// mcpChooseTools is the tools enable turns on for e.
func mcpChooseTools(e providerpkg.MCPServerEntry, o mcpToolOpts) (providerpkg.MCPToolChoice, error) {
	if o.noPick {
		// Every client keeps its entry: the choice is only compared with
		// it, so nothing here may refuse.
		c := providerpkg.MCPToolChoice{Tools: e.DefaultTools()}
		if ro := e.GitHubReadonlyURL(); ro != "" || len(e.Tools) == 0 {
			c = providerpkg.MCPToolChoice{All: true, URL: ro}
		}
		return c, nil
	}
	if len(e.Tools) == 0 {
		// A registry without tool lists (an older answer, say) cannot tell
		// a safe tool from a risky one.
		if e.Loopback() && !o.all {
			return providerpkg.MCPToolChoice{}, fmt.Errorf("the registry lists no tools for %s, which runs on your machine outside the sandbox; to turn on every tool anyway: nav-pilot mcp enable %s --all-tools", e.Name, e.Name)
		}
		if o.tools != nil {
			return providerpkg.MCPToolChoice{}, fmt.Errorf("the registry lists no tools for %s to pick from; use --all-tools", e.Name)
		}
		c := providerpkg.MCPToolChoice{All: true}
		// GitHub's full endpoint writes around cplt's guard on gh: only
		// on --all-tools.
		if ro := e.GitHubReadonlyURL(); ro != "" {
			if o.all {
				c.URL = e.Remotes[0].URL
			} else {
				c.URL = ro
			}
		}
		fmt.Printf("%s The registry lists no tools for %s to pick from.\n", dim("ℹ"), e.Name)
		return c, nil
	}
	c := providerpkg.MCPToolChoice{}
	switch {
	case o.all:
		c.All, c.Tools = true, slices.Clone(e.Tools)
	case o.tools != nil:
		for _, t := range o.tools {
			if !slices.Contains(e.Tools, t) {
				return c, fmt.Errorf("%s has no tool %q; its tools: %s", e.Name, safe(t, 64), strings.Join(e.Tools, ", "))
			}
		}
		c.Tools = o.tools
	case isInteractive() && !o.noPick:
		pick, err := mcpPickTools(e)
		if errors.Is(err, huh.ErrUserAborted) {
			return c, cancelledError{nothingWritten: true}
		}
		if err != nil {
			return c, err
		}
		c.Tools = pick
	default:
		c.Tools = e.DefaultTools()
	}

	// A host-exec tool runs with the user's rights, outside cplt: only on
	// a yes, or --allow-host-exec where nobody can answer.
	if hx := mcpIntersect(c.Tools, e.HostExecTools()); len(hx) > 0 && !o.allowHostExec {
		if !isInteractive() {
			return c, fmt.Errorf("%s %s on your machine, outside the cplt sandbox. To turn %s on anyway, add --allow-host-exec", strings.Join(hx, ", "), mcpRuns(hx), mcpIt(hx))
		}
		if !mcpAsk(fmt.Sprintf("%s %s on your machine, outside the cplt sandbox. The agent can do anything you can. Turn %s on?", strings.Join(hx, ", "), mcpRuns(hx), mcpIt(hx))) {
			c.All = false
			c.Tools = slices.DeleteFunc(slices.Clone(c.Tools), func(t string) bool { return slices.Contains(hx, t) })
			fmt.Printf("%s Left off: %s\n", dim("•"), strings.Join(hx, ", "))
		}
	}
	if !c.All && len(c.Tools) == 0 {
		return c, fmt.Errorf("no tools left to turn on for %s, so nothing was changed. To remove it: nav-pilot mcp disable %s", e.Name, e.Name)
	}

	if ro := e.GitHubReadonlyURL(); ro != "" {
		if c.All || slices.ContainsFunc(c.Tools, func(t string) bool { return e.RiskOf(t) == providerpkg.MCPRiskExternal }) {
			c.URL = e.Remotes[0].URL
			fmt.Printf("%s %s: GitHub's full endpoint, so the agent can write to GitHub through MCP. That goes around cplt's guard on gh.\n", yellow("⚠"), e.Name)
		} else {
			c.URL = ro
			// The default is every tool the read-only endpoint has, which
			// are all reads, so a registry name gone stale upstream costs
			// nothing.
			def := e.DefaultTools()
			c.All = len(c.Tools) == len(def) && len(mcpIntersect(c.Tools, def)) == len(def)
		}
	}
	if c.All {
		c.Tools = nil
	}
	return c, nil
}

func mcpIntersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// mcpDescribeChoice is one line on what the choice turns on.
func mcpDescribeChoice(e providerpkg.MCPServerEntry, c providerpkg.MCPToolChoice) string {
	where, of := "", e.Tools
	switch {
	case c.URL != "" && providerpkg.IsGitHubReadonlyURL(c.URL):
		// The read-only endpoint has only the reads.
		where, of = " on GitHub's read-only endpoint", e.DefaultTools()
	case e.GitHubReadonlyURL() != "":
		where = " on GitHub's full endpoint"
	}
	if c.All {
		return "every tool" + where
	}
	var off []string
	for _, t := range of {
		if !slices.Contains(c.Tools, t) {
			off = append(off, t)
		}
	}
	s := fmt.Sprintf("%d of %d tools%s", len(c.Tools), len(of), where)
	if len(off) > 0 {
		s += "; off: " + strings.Join(off, ", ")
	}
	return s
}

// mcpToolsInfo is what a configured server's entry turns on in one client.
type mcpToolsInfo struct {
	state    providerpkg.MCPToolState
	hostExec []string // host-exec tools that are on
	fullGH   bool     // GitHub's full endpoint
}

func mcpToolsOf(e providerpkg.MCPServerEntry, client string) (mcpToolsInfo, bool) {
	key := e.Name
	if client == providerpkg.MCPClientOpenCode {
		if key = providerpkg.MCPConfigKeyFor(client, e); key == "" {
			return mcpToolsInfo{}, false
		}
	}
	st, ok := providerpkg.MCPToolsOn(client, key, e)
	if !ok {
		return mcpToolsInfo{}, false
	}
	info := mcpToolsInfo{state: st, hostExec: mcpIntersect(st.On, e.HostExecTools())}
	if st.All && len(e.Tools) == 0 && e.Loopback() {
		// Every tool of a server on the host, with none of them known.
		info.hostExec = []string{mcpAllItsTools}
	}
	info.fullGH = e.GitHubReadonlyURL() != "" && !providerpkg.IsGitHubReadonlyURL(st.URL)
	return info, true
}

// cell is the TOOLS column, before its flags.
func (i mcpToolsInfo) cell(e providerpkg.MCPServerEntry) string {
	s := fmt.Sprintf("%d of %d", len(i.state.On), len(e.Tools))
	switch {
	case i.state.All && e.GitHubReadonlyURL() != "" && !i.fullGH:
		s = "read-only"
	case i.state.All:
		s = "all"
	}
	return s
}

// mcpNarrowFix is the command that turns the server down to its default
// tools in the client.
func mcpNarrowFix(e providerpkg.MCPServerEntry, client string) string {
	cmd := fmt.Sprintf("nav-pilot mcp enable %s --client %s --tools %s", e.Name, client, strings.Join(e.DefaultTools(), ","))
	if len(e.Tools) == 0 {
		// Nothing to pick from: a fresh entry is GitHub's read-only
		// endpoint, or for another server the way to turn it off.
		cmd = fmt.Sprintf("nav-pilot mcp disable %s --client %s", e.Name, client)
		if e.GitHubReadonlyURL() != "" {
			cmd += fmt.Sprintf(" && nav-pilot mcp enable %s --client %s", e.Name, client)
		}
	}
	return cmd
}

// mcpToolProblems is the nudges for a configured server in one client: a
// host-exec tool on, or GitHub's full endpoint. Not failures: the user may
// have chosen them.
func mcpToolProblems(e providerpkg.MCPServerEntry, client string, i mcpToolsInfo) []mcpProblem {
	var out []mcpProblem
	if len(i.hostExec) > 0 {
		what := "has " + strings.Join(i.hostExec, ", ") + " on"
		if i.state.All {
			what = "has every tool on, including " + strings.Join(i.hostExec, ", ")
		}
		problem := what + ", which " + mcpRuns(i.hostExec) + " on your machine outside the cplt sandbox"
		if slices.Equal(i.hostExec, []string{mcpAllItsTools}) {
			problem = "has every tool on, and the registry lists none of them, so any of them may run on your machine outside the cplt sandbox"
		}
		out = append(out, mcpProblem{Server: e.Name, Client: client, Problem: problem, Fix: mcpNarrowFix(e, client)})
	}
	if i.fullGH {
		out = append(out, mcpProblem{Server: e.Name, Client: client,
			Problem: "uses GitHub's full MCP endpoint, so the agent can write to GitHub around cplt's guard on gh",
			Fix:     mcpNarrowFix(e, client)})
	}
	return out
}

// mcpHostExecOn is the host-exec tools the server has on in either client.
func mcpHostExecOn(e providerpkg.MCPServerEntry, conf providerpkg.MCPConfigured) []string {
	var out []string
	for client, on := range map[string]bool{providerpkg.MCPClientCopilot: conf.Copilot[e.Name], providerpkg.MCPClientOpenCode: conf.OpenCode[e.Name]} {
		if !on {
			continue
		}
		if i, ok := mcpToolsOf(e, client); ok {
			for _, t := range i.hostExec {
				if !slices.Contains(out, t) {
					out = append(out, t)
				}
			}
		}
	}
	slices.Sort(out)
	return out
}

// mcpLoopbackWarning is what opening a localhost port means when the server
// behind it has host-exec tools on: the port is a way out of the sandbox.
// fix is the command that turns them off.
func mcpLoopbackWarning(e providerpkg.MCPServerEntry, port string, hx []string, fix string) string {
	return fmt.Sprintf("opening localhost:%s lets the agent reach %s, which %s on your machine outside the sandbox. Turn %s off first (%s), then open the port",
		port, strings.Join(hx, ", "), mcpRuns(hx), mcpIt(hx), fix)
}

// mcpLoopbackFix is the short form of the narrowing command for the
// warning, which doctor cuts at 600: --tools with a placeholder, and for a
// server without a tool list, which --tools cannot pick from, the real
// commands (they are short).
func mcpLoopbackFix(e providerpkg.MCPServerEntry, clients []string) string {
	if len(e.Tools) == 0 {
		return mcpNarrowFixes(e, clients)
	}
	// One per client: without --client, enable adds the server to every
	// client.
	var fixes []string
	for _, c := range clients {
		fixes = append(fixes, "nav-pilot mcp enable "+e.Name+" --client "+c+" --tools <a,b>")
	}
	return strings.Join(fixes, " && ")
}

// mcpKnownTools is the registry's tool list for the server under key in the
// client, from the entries disable already has or the cache: nil when
// unknown, so disable never waits for the registry for it.
func mcpKnownTools(client, key string, entries []providerpkg.MCPServerEntry) []string {
	if entries == nil {
		entries = mcpCachedEntries()
	}
	name := key
	if client == providerpkg.MCPClientOpenCode {
		name = providerpkg.OpenCodeRegistryNameFor(key, entries)
	}
	for _, e := range entries {
		if e.Name == name && len(e.Tools) > 0 {
			return e.Tools
		}
	}
	return nil
}

// mcpNarrowFixes is mcpNarrowFix for each client, in one command line.
func mcpNarrowFixes(e providerpkg.MCPServerEntry, clients []string) string {
	var fixes []string
	for _, c := range clients {
		fixes = append(fixes, mcpNarrowFix(e, c))
	}
	return strings.Join(fixes, " && ")
}

// mcpAllItsTools stands for the tools of a server on the host that lists
// none: every one of them is on.
const mcpAllItsTools = "all of its tools"

func mcpRuns(tools []string) string {
	if len(tools) == 1 && tools[0] != mcpAllItsTools {
		return "runs"
	}
	return "run"
}

func mcpIt(tools []string) string {
	if len(tools) == 1 && tools[0] != mcpAllItsTools {
		return "it"
	}
	return "them"
}

var errMCPToolsFlag = errors.New("--tools, --all-tools and --allow-host-exec are for mcp enable")

// Doctor's half: local files and the cached registry only, never a wait.

// mcpCachedEntries is a var so tests answer it.
var mcpCachedEntries = providerpkg.CachedMCPRegistryEntries

// mcpLoopbackNote is the warning for opening the port of a server with
// host-exec tools on, "" when it has none on.
func mcpLoopbackNote(server, port string) string {
	entries := mcpCachedEntries()
	i := slices.IndexFunc(entries, func(e providerpkg.MCPServerEntry) bool { return e.Name == server })
	if i < 0 {
		return ""
	}
	e := entries[i]
	conf := providerpkg.ConfiguredMCPServers(entries)
	hx := mcpHostExecOn(e, conf)
	if len(hx) == 0 {
		return ""
	}
	on := map[string]bool{providerpkg.MCPClientCopilot: conf.Copilot[e.Name], providerpkg.MCPClientOpenCode: conf.OpenCode[e.Name]}
	var clients []string
	for _, c := range []string{providerpkg.MCPClientCopilot, providerpkg.MCPClientOpenCode} {
		if info, ok := mcpToolsOf(e, c); on[c] && ok && len(info.hostExec) > 0 {
			clients = append(clients, c)
		}
	}
	return mcpLoopbackWarning(e, port, hx, mcpLoopbackFix(e, clients))
}

// reportMCPTools is doctor's nudge for a configured server with a tool on
// that runs outside the sandbox, or GitHub's full endpoint. Local files
// and the cached registry only.
func reportMCPTools(w io.Writer) {
	entries := mcpCachedEntries()
	conf := providerpkg.ConfiguredMCPServers(entries)
	for _, e := range entries {
		for _, client := range []string{providerpkg.MCPClientCopilot, providerpkg.MCPClientOpenCode} {
			if !conf.Copilot[e.Name] && client == providerpkg.MCPClientCopilot || !conf.OpenCode[e.Name] && client == providerpkg.MCPClientOpenCode {
				continue
			}
			i, ok := mcpToolsOf(e, client)
			if !ok {
				continue
			}
			for _, p := range mcpToolProblems(e, client, i) {
				fmt.Fprintf(w, "      %s %s (MCP, %s) %s. Narrow it: %s\n", yellow("⚠"), safe(e.Name, 64), client, p.Problem, bold(p.Fix))
			}
		}
	}
}
