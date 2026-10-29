package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// nav-pilot mcp enable and list for a brokered server.

type mcpBrokerResult struct {
	Server string   `json:"server"`
	Spec   string   `json:"spec"`
	Tools  []string `json:"tools"`
	Login  string   `json:"login,omitempty"`
	Path   string   `json:"path"`
	Fix    []string `json:"fix,omitempty"`
}

// enableBrokeredServers writes each server's spec and checks its login, after
// every picker of the command; it returns how many failed.
func enableBrokeredServers(servers []providerpkg.MCPServerEntry, o mcpToolOpts, jsonOut bool) (failed int) {
	for _, e := range servers {
		if err := mcpEnableBroker(e, o, jsonOut); err != nil {
			mcpFail(e.Name, err)
			failed++
		}
	}
	return failed
}

// mcpEnableBroker writes e's spec and checks the host login. Without a cplt
// that brokers, it writes nothing: no spec would be enforced.
func mcpEnableBroker(e providerpkg.MCPServerEntry, o mcpToolOpts, jsonOut bool) error {
	m, err := mcpBrokerCheck(e)
	if err != nil {
		return err
	}
	allow, write, err := mcpBrokerTools(e, o)
	if err != nil {
		return err
	}
	cpltPath := mcpCpltPath()
	if cpltPath == "" {
		return fmt.Errorf("%s runs only under cplt, which is not installed, so nothing was written. Fix: %s && nav-pilot mcp enable %s", e.Name, mcpCpltInstall, e.Name)
	}
	st := mcpCpltStatus(cpltPath)
	if st == nil {
		return fmt.Errorf("this cplt cannot run MCP servers on the host, so nothing was written. Fix: %s && nav-pilot mcp enable %s", mcpCpltUpgrade, e.Name)
	}
	if st.Failed != "" {
		// No schema to check the spec against: say so rather than guess.
		fix := "cd <your repo> && nav-pilot mcp enable " + e.Name
		if st.TimedOut {
			fix = "run it again: nav-pilot mcp enable " + e.Name
		}
		return fmt.Errorf("cplt mcp status did not answer here (%s), so nothing was written. Fix: %s", st.Failed, fix)
	}
	if !slices.Contains(st.Schema, mcpBrokerVersion) {
		return fmt.Errorf("cplt reads broker spec v%v, not v%d, so nothing was written. Fix: %s && nav-pilot mcp enable %s", st.Schema, mcpBrokerVersion, mcpCpltUpgrade, e.Name)
	}
	data, err := mcpBrokerSpec(e, m, allow, write)
	if err != nil {
		return err
	}
	path, err := mcpWriteBrokerSpec(e.Name, data)
	if err != nil {
		return err
	}
	// What cplt makes of the spec now it is there: brokered, off, rejected,
	// or a platform it does not broker on.
	if after := mcpCpltStatus(cpltPath); after != nil {
		st = after
	}
	where, credOK, credNote, probs := mcpBrokerState(cpltPath, st, e.Name)
	res := mcpBrokerResult{Server: e.Name, Spec: path, Tools: allow, Path: where}
	if !jsonOut {
		kind := "read-only"
		if write {
			kind = "including writes you picked"
		}
		runs := "cplt runs it on the host."
		if where != "cplt broker" {
			runs = "cplt does not run it yet:"
		}
		fmt.Printf("%s Enabled %s: %d tools, %s. %s %s\n", green("✓"), bold(e.Name), len(allow), kind, runs, dim("("+path+")"))
	}
	warn := func(problem, fix string) {
		res.Fix = append(res.Fix, fix)
		if !jsonOut {
			fmt.Fprintf(os.Stderr, "%s %s. Fix: %s\n", yellow("⚠"), problem, bold(fix))
		}
	}
	for _, p := range probs {
		warn(p.Problem, p.Fix)
	}
	if old := mcpOldServer(m); old != "" {
		warn(old, m.upgrade)
	}
	tool, login := m.tool(), strings.Join(m.login, " ")
	res.Login = mcpCredState(m, credOK, credNote)
	switch {
	case res.Login == "not installed":
		warn(tool+" is not installed", "install it ("+m.install+"), then run "+login)
	case strings.HasPrefix(res.Login, "unknown ("):
		warn("could not check the "+tool+" login "+strings.TrimPrefix(res.Login, "unknown "), "run nav-pilot mcp enable "+e.Name+" again, or check it with "+login)
	case res.Login == "logged out":
		switch {
		case jsonOut || !isInteractive():
			res.Fix = append(res.Fix, login)
			if !jsonOut {
				fmt.Printf("%s %s: not logged in on this machine. Fix: %s\n", yellow("⚠"), tool, bold(login))
			}
		case mcpConfirmLogin(fmt.Sprintf("%s: not logged in. Log in now? Runs `%s` on this machine.", tool, login)):
			if err := mcpRunLogin(m); err != nil {
				return fmt.Errorf("%s failed (%v). Fix: run it yourself: %s", login, err, login)
			}
			// Ask cplt again: the host probe runs only when cplt cannot say.
			if after := mcpCpltStatus(cpltPath); after != nil {
				st = after
			}
			_, credOK, credNote, _ = mcpBrokerState(cpltPath, st, e.Name)
			res.Login = mcpCredState(m, credOK, credNote)
		default:
			fmt.Printf("%s Log in later: %s\n", dim("•"), bold(login))
		}
	}
	if jsonOut {
		return outputJSON(res)
	}
	return nil
}

// mcpCredState is the login state from cplt's credential_ok and
// credential_note. The host probe runs only when cplt gave no answer: on the
// host it reads config the brokered server can write (#2058).
func mcpCredState(m mcpMechanism, credOK *bool, credNote string) string {
	switch {
	case credOK == nil && credNote != "":
		return "unknown (" + credNote + ")" // cplt could not tell; not logged out
	case credOK == nil:
		return mcpLoginState(m)
	case *credOK:
		return "ok"
	}
	return "logged out"
}

// mcpBrokerRow fills LOGIN, PATH and TOOLS for a brokered server, enabled or
// configured in a client by hand, and its problems. The login comes from
// cplt's status when it has one, else from the mechanism's status command.
func mcpBrokerRow(row *mcpServerRow, e providerpkg.MCPServerEntry, cpltPath string, st *mcpCpltStatusDoc) []mcpProblem {
	m, err := mcpBrokerCheck(e)
	if err != nil {
		row.Path = "-"
		return []mcpProblem{{Server: e.Name, Problem: strings.SplitN(err.Error(), ". Fix: ", 2)[0], Fix: "nav-pilot update"}}
	}
	path, credOK, credNote, out := mcpBrokerState(cpltPath, st, e.Name)
	row.Path = path
	if !mcpBrokerEnabled(e.Name) {
		// A client entry the user wrote by hand, with no spec: it runs as
		// written, without cplt's tool and argument checks.
		row.Path, credOK, credNote = "direct (outside the broker)", nil, ""
		out = []mcpProblem{{Server: e.Name, Problem: "configured in your client but not through cplt's broker, so nothing keeps it read-only", Fix: "nav-pilot mcp enable " + e.Name}}
	}
	if old := mcpOldServer(m); old != "" {
		out = append(out, mcpProblem{Server: e.Name, Problem: old, Fix: m.upgrade})
	}
	tool, login := m.tool(), strings.Join(m.login, " ")
	state := mcpCredState(m, credOK, credNote)
	row.Login = tool + ": " + state
	switch state {
	case "logged out":
		out = append(out, mcpProblem{Server: e.Name, Problem: "not logged in on this machine", Fix: login})
	case "not installed":
		out = append(out, mcpProblem{Server: e.Name, Problem: tool + " is not installed", Fix: "install it (" + m.install + "), then " + login})
	}
	if n := mcpBrokerAllowed(e.Name); n >= 0 {
		row.Tools = fmt.Sprintf("%d of %d", n, len(e.Meta.Broker.Tools.Classes))
	}
	return out
}
