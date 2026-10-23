package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// The nav-pilot half of cplt's MCP host broker (navikt/cplt
// docs/design/mcp-host-broker.md). For a registry server with the broker
// extension, enable writes one spec file to the user's cplt config and logs
// in on the host; cplt runs the server on the host, enforces the spec and
// adds the client entries at launch. nav-pilot writes no client config here.

const (
	mcpBrokerKey     = "io.github.navikt/broker"
	mcpBrokerVersion = 1
	mcpCpltUpgrade   = "brew upgrade cplt"
	mcpNaisInstall   = "https://doc.nais.io/operate/cli/how-to/install/"
)

// mcpBrokerDir is $XDG_CONFIG_HOME/cplt/mcp, where cplt reads specs.
func mcpBrokerDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "cplt", "mcp"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cplt", "mcp"), nil
}

// mcpBrokerSpecPath is the spec file for a server id, as cplt names it.
func mcpBrokerSpecPath(id string) (string, error) {
	dir, err := mcpBrokerDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, strings.ReplaceAll(id, "/", "__")+".json"), nil
}

// mcpBrokerEnabled is whether a spec file for e is there.
func mcpBrokerEnabled(id string) bool {
	p, err := mcpBrokerSpecPath(id)
	if err != nil {
		return false
	}
	_, err = os.Lstat(p)
	return err == nil
}

// mcpBrokerTools is what the spec allows, and whether that needs write
// consent. Reads by default; a tool the registry does not class is never on.
func mcpBrokerTools(e providerpkg.MCPServerEntry, o mcpToolOpts) (allow []string, write bool, err error) {
	classes := e.Meta.Broker.Tools.Classes
	switch {
	case o.tools != nil:
		for _, t := range o.tools {
			c, ok := classes[t]
			if !ok {
				return nil, false, fmt.Errorf("%s has no brokered tool %q. Fix: pick from %s", e.Name, safe(t, 64), strings.Join(slices.Sorted(maps.Keys(classes)), ","))
			}
			if c == providerpkg.MCPRiskHostExec {
				return nil, false, fmt.Errorf("%s is host-exec, which cplt never brokers. Fix: nav-pilot mcp enable %s", t, e.Name)
			}
		}
		allow = slices.Clone(o.tools)
	default:
		for t, c := range classes {
			if c == "" || c == providerpkg.MCPRiskRead || (o.all && c != providerpkg.MCPRiskHostExec) {
				allow = append(allow, t)
			}
		}
	}
	for _, t := range allow {
		if c := classes[t]; c != "" && c != providerpkg.MCPRiskRead {
			write = true
		}
	}
	sort.Strings(allow)
	if len(allow) == 0 {
		return nil, false, fmt.Errorf("no read tools to turn on for %s. Fix: nav-pilot mcp enable %s --tools <a,b>", e.Name, e.Name)
	}
	return allow, write, nil
}

// mcpBrokerSpec is the server.json cplt reads: the registry's extension with
// the user's consent filled in.
func mcpBrokerSpec(e providerpkg.MCPServerEntry, allow []string, write bool) ([]byte, error) {
	b := e.Meta.Broker
	ext := map[string]any{
		"v":          mcpBrokerVersion,
		"credential": b.Credential,
		"command":    b.Command,
		"tools":      map[string]any{"allow": allow, "classes": b.Tools.Classes},
		"login":      map[string]string{"run": b.Login.Run, "status": b.Login.Status},
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

// mcpCpltStatus is `cplt mcp status --json`, or nil from a cplt without the
// broker (or no cplt).
type mcpCpltStatusDoc struct {
	Schema  []int `json:"schema"`
	Servers []struct {
		ID           string `json:"id"`
		State        string `json:"state"`
		CredentialOK *bool  `json:"credential_ok"`
	} `json:"servers"`
	Rejected []struct {
		File   string `json:"file"`
		Reason string `json:"reason"`
		Fix    string `json:"fix"`
	} `json:"rejected"`
	Failed string `json:"-"`
}

const mcpStatusFailedFix = "cd <your repo> && nav-pilot mcp list"

// A cplt that has the command but could not answer (it refuses to run in
// ~, as a launch does) gives a doc with only Failed set.
func mcpCpltStatus(cpltPath string) *mcpCpltStatusDoc {
	if cpltPath == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cpltPath, "mcp", "status", "--json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var doc mcpCpltStatusDoc
	if err == nil && json.Unmarshal(out, &doc) == nil && doc.Schema != nil {
		return &doc
	}
	msg := strings.TrimSpace(stderr.String())
	// clap's answer for a subcommand it does not have: an old cplt.
	if err == nil || msg == "" || strings.Contains(msg, "unrecognized subcommand") {
		return nil
	}
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	return &mcpCpltStatusDoc{Failed: safe(msg, 160)}
}

// mcpOldCpltNotice says once per cplt version that it cannot broker yet.
func mcpOldCpltNotice(cpltPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, cpltPath, "--version").Output()
	v := strings.TrimSpace(string(out))
	key := "cplt-mcp-broker-" + strings.Map(func(r rune) rune {
		if r == '.' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') {
			return r
		}
		return '_'
	}, v)
	if providerpkg.FirstTime(key) {
		fmt.Fprintf(os.Stderr, "%s %s cannot run MCP servers on the host yet, so it leaves them as configured. Fix: %s\n", yellow("⚠"), safe(v, 64), bold(mcpCpltUpgrade))
	}
}

// mcpRunHost runs a registry command (login or status) on the host. With
// quiet, its output is discarded and stdin is closed.
func mcpRunHost(command string, quiet bool) error {
	f := strings.Fields(command)
	if len(f) == 0 {
		return errors.New("no command")
	}
	if _, err := exec.LookPath(f[0]); err != nil {
		return err
	}
	ctx, cancel := context.Background(), func() {}
	if quiet {
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, f[0], f[1:]...)
	if !quiet {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	}
	return cmd.Run()
}

// mcpLoginState is "ok", "logged out" or "not installed", from the
// registry's status command.
func mcpLoginState(b *providerpkg.MCPBroker) string {
	err := mcpRunHost(b.Login.Status, true)
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, exec.ErrNotFound):
		return "not installed"
	}
	return "logged out"
}

func mcpLoginTool(b *providerpkg.MCPBroker) string {
	if f := strings.Fields(b.Login.Status); len(f) > 0 {
		return f[0]
	}
	return "login"
}

type mcpBrokerResult struct {
	Server string   `json:"server"`
	Spec   string   `json:"spec"`
	Tools  []string `json:"tools"`
	Login  string   `json:"login"`
	Path   string   `json:"path"`
	Fix    []string `json:"fix,omitempty"`
}

// mcpEnableBroker writes e's spec and checks the host login.
func mcpEnableBroker(e providerpkg.MCPServerEntry, o mcpToolOpts, jsonOut bool) error {
	b := e.Meta.Broker
	if b.V != mcpBrokerVersion {
		return fmt.Errorf("the registry's broker spec for %s is v%d; this nav-pilot writes v%d. Fix: nav-pilot update", e.Name, b.V, mcpBrokerVersion)
	}
	allow, write, err := mcpBrokerTools(e, o)
	if err != nil {
		return err
	}
	cpltPath := mcpCpltPath()
	st := mcpCpltStatus(cpltPath)
	if st != nil && st.Failed == "" && !slices.Contains(st.Schema, mcpBrokerVersion) {
		return fmt.Errorf("cplt reads broker spec v%v, not v%d, so nothing was written. Fix: %s", st.Schema, mcpBrokerVersion, mcpCpltUpgrade)
	}
	path, err := mcpBrokerSpecPath(e.Name)
	if err != nil {
		return err
	}
	data, err := mcpBrokerSpec(e, allow, write)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("could not create %s: %w", filepath.Dir(path), err)
	}
	// Through a temp file and a rename, so cplt never reads half a spec.
	if err := writeFileAtomic(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	res := mcpBrokerResult{Server: e.Name, Spec: path, Tools: allow, Path: mcpBrokerPath(cpltPath, st)}
	if !jsonOut {
		kind := "read-only"
		if write {
			kind = "with writes you picked"
		}
		fmt.Printf("%s Enabled %s: %d tools, %s. Under cplt it runs on the host. %s\n", green("✓"), bold(e.Name), len(allow), kind, dim("("+path+")"))
	}
	switch {
	case cpltPath == "":
		msg := "without cplt nothing enforces this spec: no read-only guarantee"
		if len(b.ArgChecks) > 0 {
			msg += "; nothing checks that " + strings.Join(slices.Sorted(maps.Keys(b.ArgChecks)), ", ") + " only reads"
		}
		res.Fix = append(res.Fix, "brew install navikt/tap/cplt")
		if !jsonOut {
			fmt.Fprintf(os.Stderr, "%s %s. Fix: %s\n", yellow("⚠"), msg, bold("brew install navikt/tap/cplt"))
		}
	case st == nil:
		res.Fix = append(res.Fix, mcpCpltUpgrade)
		if !jsonOut {
			mcpOldCpltNotice(cpltPath)
		}
	case st.Failed != "":
		res.Fix = append(res.Fix, mcpStatusFailedFix)
		if !jsonOut {
			fmt.Fprintf(os.Stderr, "%s cplt mcp status did not answer here (%s), so the spec was written unchecked. Fix: %s\n", yellow("⚠"), st.Failed, bold(mcpStatusFailedFix))
		}
	}

	res.Login = mcpLoginState(b)
	tool := mcpLoginTool(b)
	switch res.Login {
	case "not installed":
		res.Fix = append(res.Fix, mcpNaisInstall)
		if !jsonOut {
			fmt.Fprintf(os.Stderr, "%s %s is not installed. Fix: install it (%s), then run %s\n", yellow("⚠"), tool, mcpNaisInstall, bold(b.Login.Run))
		}
	case "logged out":
		if jsonOut || !isInteractive() {
			res.Fix = append(res.Fix, b.Login.Run)
			if !jsonOut {
				fmt.Printf("%s %s: not logged in on this machine. Fix: %s\n", yellow("⚠"), tool, bold(b.Login.Run))
			}
		} else if mcpAsk(fmt.Sprintf("%s: not logged in. Log in now? Runs `%s` on this machine.", tool, b.Login.Run)) {
			if err := mcpRunHost(b.Login.Run, false); err != nil {
				return fmt.Errorf("%s failed (%v). Fix: run it yourself: %s", b.Login.Run, err, b.Login.Run)
			}
			res.Login = mcpLoginState(b)
		} else {
			fmt.Printf("%s Log in later: %s\n", dim("•"), bold(b.Login.Run))
		}
	}
	if jsonOut {
		return outputJSON(res)
	}
	return nil
}

// mcpBrokerPath is the PATH column for a brokered server.
func mcpBrokerPath(cpltPath string, st *mcpCpltStatusDoc) string {
	if cpltPath == "" {
		return "direct (no read-only guarantee)"
	}
	if st == nil || st.Failed != "" {
		return "-"
	}
	return "cplt broker"
}

// mcpDisableBroker removes e's spec; false when there was none.
func mcpDisableBroker(id string) (bool, error) {
	path, err := mcpBrokerSpecPath(id)
	if err != nil {
		return false, err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	fmt.Printf("%s Disabled %s: removed %s\n", green("✓"), bold(id), path)
	return true, nil
}

// mcpBrokerRow fills LOGIN and PATH for an enabled brokered server, and its
// problems.
func mcpBrokerRow(row *mcpServerRow, e providerpkg.MCPServerEntry, cpltPath string, st *mcpCpltStatusDoc) []mcpProblem {
	b := e.Meta.Broker
	tool := mcpLoginTool(b)
	login := mcpLoginState(b)
	row.Login = tool + ": " + login
	row.Path = mcpBrokerPath(cpltPath, st)
	var out []mcpProblem
	switch login {
	case "logged out":
		out = append(out, mcpProblem{Server: e.Name, Problem: "not logged in on this machine", Fix: b.Login.Run})
	case "not installed":
		out = append(out, mcpProblem{Server: e.Name, Problem: tool + " is not installed", Fix: "install it (" + mcpNaisInstall + "), then " + b.Login.Run})
	}
	switch {
	case cpltPath == "":
		out = append(out, mcpProblem{Server: e.Name, Problem: "without cplt nothing enforces its read-only spec", Fix: "brew install navikt/tap/cplt"})
	case st == nil:
		out = append(out, mcpProblem{Server: e.Name, Problem: "this cplt cannot run MCP servers on the host, so it leaves them as configured", Fix: mcpCpltUpgrade})
	case st.Failed != "":
		out = append(out, mcpProblem{Server: e.Name, Problem: "cplt mcp status did not answer here (" + st.Failed + ")", Fix: mcpStatusFailedFix})
	default:
		file := strings.ReplaceAll(e.Name, "/", "__") + ".json"
		for _, r := range st.Rejected {
			if filepath.Base(r.File) == file {
				row.Path = "rejected"
				fix := r.Fix
				if fix == "" {
					fix = "nav-pilot mcp enable " + e.Name
				}
				out = append(out, mcpProblem{Server: e.Name, Problem: "cplt rejected its spec: " + safe(r.Reason, 200), Fix: safe(fix, 200)})
			}
		}
		for _, s := range st.Servers {
			if s.ID == e.Name && s.State != "brokered" {
				row.Path = "cplt " + safe(s.State, 16)
			}
		}
	}
	var spec struct {
		Meta map[string]struct {
			Tools struct {
				Allow []string `json:"allow"`
			} `json:"tools"`
		} `json:"_meta"`
	}
	if p, err := mcpBrokerSpecPath(e.Name); err == nil {
		if data, err := os.ReadFile(p); err == nil && json.Unmarshal(data, &spec) == nil {
			row.Tools = fmt.Sprintf("%d of %d", len(spec.Meta[mcpBrokerKey].Tools.Allow), len(b.Tools.Classes))
		}
	}
	return out
}
