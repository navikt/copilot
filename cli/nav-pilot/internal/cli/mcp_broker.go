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
	"regexp"
	"slices"
	"sort"
	"strconv"
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
	mcpBrokerKey       = "io.github.navikt/broker"
	mcpBrokerVersion   = 1
	mcpCpltUpgrade     = "brew upgrade cplt"
	mcpCpltInstall     = "brew install navikt/tap/cplt"
	mcpNaisInstall     = "https://doc.nais.io/operate/cli/how-to/install/"
	mcpStatusFailedFix = "cd <your repo> && nav-pilot mcp list"
)

// mcpMechanism is what nav-pilot runs on the host for a credential
// mechanism. Hand-kept, like cplt's own table: nav-pilot never runs a
// command the registry supplies. The registry's login commands must match
// these, and are otherwise only text.
type mcpMechanism struct {
	command       []string // the server, as cplt pins it
	status, login []string
	// minVersion is the first release whose server checks what cplt's broker
	// relies on; older gets a nudge, never a refusal.
	version    []string
	minVersion string
	upgrade    string
}

var mcpMechanisms = map[string]mcpMechanism{
	"nais-keychain": {
		command: []string{"nais", "alpha", "mcp", "serve"},
		status:  []string{"nais", "auth", "print-access-token"},
		login:   []string{"nais", "login", "-n"},
		// 7.0.2 checks every GraphQL operation and fragment (nais/cli 6ddc986).
		version:    []string{"nais", "--version"},
		minVersion: "7.0.2",
		upgrade:    "brew upgrade nais",
	},
	"none": {},
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
	if len(m.command) > 0 && !slices.Equal(b.Command, m.command) {
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

// mcpBrokerSafePath is the spec path for a write or a remove: refused when
// it, or any directory between it and the config base, is a symlink, so
// neither lands in a directory the link points at.
func mcpBrokerSafePath(id string) (string, error) {
	path, err := mcpBrokerSpecPath(id)
	if err != nil {
		return "", err
	}
	base := filepath.Dir(filepath.Dir(filepath.Dir(path))) // the config base
	if err := checkSymlink(path, base); err != nil {
		return "", fmt.Errorf("%w, so nav-pilot did not touch it. Fix: replace the link under %s with a directory", err, base)
	}
	return path, nil
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
	}
	if len(m.login) > 0 {
		ext["login"] = map[string]string{"run": strings.Join(m.login, " "), "status": strings.Join(m.status, " ")}
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

// mcpCpltStatusDoc is `cplt mcp status --json`. Failed is set, and the rest
// empty, when a cplt that has the command did not answer: it refuses to run
// in ~ as a launch does, timed out, or printed something else.
type mcpCpltStatusDoc struct {
	Schema            []int `json:"schema"`
	PlatformSupported *bool `json:"platform_supported"`
	Servers           []struct {
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

// mcpCpltStatusTimeout is a var so tests time out fast.
var mcpCpltStatusTimeout = 15 * time.Second

// mcpCpltStatus is cplt's status, nil for no cplt or an old cplt (only
// when cplt says it has no such subcommand).
func mcpCpltStatus(cpltPath string) *mcpCpltStatusDoc {
	if cpltPath == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), mcpCpltStatusTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cpltPath, "mcp", "status", "--json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	msg := strings.TrimSpace(stderr.String())
	if strings.Contains(msg, "unrecognized subcommand") {
		return nil // clap's answer from a cplt without `mcp status`
	}
	var doc mcpCpltStatusDoc
	switch {
	case ctx.Err() != nil:
		msg = "timed out after " + mcpCpltStatusTimeout.String()
	case err == nil && json.Unmarshal(out, &doc) == nil && doc.Schema != nil:
		return &doc
	case err == nil:
		msg = "its output was not the status JSON"
	case msg == "":
		msg = err.Error()
	}
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	return &mcpCpltStatusDoc{Failed: safe(msg, 160)}
}

// mcpRunHost runs one of nav-pilot's own commands on the host. With quiet,
// its output is discarded, stdin is closed, and it gets 10 seconds.
func mcpRunHost(argv []string, quiet bool) error {
	if len(argv) == 0 {
		return errors.New("no command")
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return err
	}
	ctx, cancel := context.Background(), func() {}
	if quiet {
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if !quiet {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	}
	return cmd.Run()
}

// mcpLoginState is "ok", "logged out" or "not installed", from the
// mechanism's status command.
func mcpLoginState(m mcpMechanism) string {
	err := mcpRunHost(m.status, true)
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, exec.ErrNotFound):
		return "not installed"
	}
	return "logged out"
}

var mcpSemver = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// mcpOldServer is the nudge when the host CLI is older than the
// mechanism's minVersion, or "". An unreadable version says nothing.
func mcpOldServer(m mcpMechanism) string {
	if m.minVersion == "" || len(m.version) == 0 {
		return ""
	}
	if _, err := exec.LookPath(m.version[0]); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, m.version[0], m.version[1:]...).Output()
	got, want := mcpSemver.FindStringSubmatch(string(out)), mcpSemver.FindStringSubmatch(m.minVersion)
	if got == nil {
		return ""
	}
	for i := 1; i <= 3; i++ {
		g, _ := strconv.Atoi(got[i])
		w, _ := strconv.Atoi(want[i])
		if g != w {
			if g > w {
				return ""
			}
			return fmt.Sprintf("%s %s is older than %s, which checks every GraphQL operation the agent sends", m.version[0], got[0], m.minVersion)
		}
	}
	return ""
}

type mcpBrokerResult struct {
	Server string   `json:"server"`
	Spec   string   `json:"spec"`
	Tools  []string `json:"tools"`
	Login  string   `json:"login,omitempty"`
	Path   string   `json:"path"`
	Fix    []string `json:"fix,omitempty"`
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
	if st.Failed == "" && !slices.Contains(st.Schema, mcpBrokerVersion) {
		return fmt.Errorf("cplt reads broker spec v%v, not v%d, so nothing was written. Fix: %s && nav-pilot mcp enable %s", st.Schema, mcpBrokerVersion, mcpCpltUpgrade, e.Name)
	}
	data, err := mcpBrokerSpec(e, m, allow, write)
	if err != nil {
		return err
	}
	path, err := mcpBrokerSafePath(e.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("could not create %s: %w", filepath.Dir(path), err)
	}
	if path, err = mcpBrokerSafePath(e.Name); err != nil { // MkdirAll follows links
		return err
	}
	// Through a temp file and a rename, so cplt never reads half a spec.
	if err := writeFileAtomic(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	// What cplt makes of the spec now it is there: brokered, off, rejected,
	// or a platform it does not broker on.
	if after := mcpCpltStatus(cpltPath); after != nil {
		st = after
	}
	where, _, probs := mcpBrokerState(cpltPath, st, e.Name)
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
	for _, p := range probs {
		res.Fix = append(res.Fix, p.Fix)
		if !jsonOut {
			fmt.Fprintf(os.Stderr, "%s %s. Fix: %s\n", yellow("⚠"), p.Problem, bold(p.Fix))
		}
	}
	if old := mcpOldServer(m); old != "" {
		res.Fix = append(res.Fix, m.upgrade)
		if !jsonOut {
			fmt.Fprintf(os.Stderr, "%s %s. Fix: %s\n", yellow("⚠"), old, bold(m.upgrade))
		}
	}
	if len(m.status) == 0 {
		return mcpBrokerDone(res, jsonOut)
	}
	tool, login := m.status[0], strings.Join(m.login, " ")
	res.Login = mcpLoginState(m)
	switch res.Login {
	case "not installed":
		res.Fix = append(res.Fix, mcpNaisInstall)
		if !jsonOut {
			fmt.Fprintf(os.Stderr, "%s %s is not installed. Fix: install it (%s), then run %s\n", yellow("⚠"), tool, mcpNaisInstall, bold(login))
		}
	case "logged out":
		if jsonOut || !isInteractive() {
			res.Fix = append(res.Fix, login)
			if !jsonOut {
				fmt.Printf("%s %s: not logged in on this machine. Fix: %s\n", yellow("⚠"), tool, bold(login))
			}
		} else if mcpAskYes(fmt.Sprintf("%s: not logged in. Log in now? Runs `%s` on this machine.", tool, login)) {
			if err := mcpRunHost(m.login, false); err != nil {
				return fmt.Errorf("%s failed (%v). Fix: run it yourself: %s", login, err, login)
			}
			res.Login = mcpLoginState(m)
		} else {
			fmt.Printf("%s Log in later: %s\n", dim("•"), bold(login))
		}
	}
	return mcpBrokerDone(res, jsonOut)
}

func mcpBrokerDone(res mcpBrokerResult, jsonOut bool) error {
	if jsonOut {
		return outputJSON(res)
	}
	return nil
}

// mcpBrokerState is what cplt says about server id: the PATH column, its
// credential probe, and the problems with their fixes.
func mcpBrokerState(cpltPath string, st *mcpCpltStatusDoc, id string) (path string, credOK *bool, out []mcpProblem) {
	switch {
	case cpltPath == "":
		return "not configured (no cplt)", nil, []mcpProblem{{Server: id, Problem: "it runs only under cplt, which is not installed", Fix: mcpCpltInstall}}
	case st == nil:
		return "-", nil, []mcpProblem{{Server: id, Problem: "this cplt cannot run MCP servers on the host, so it leaves them as configured", Fix: mcpCpltUpgrade}}
	case st.Failed != "":
		return "-", nil, []mcpProblem{{Server: id, Problem: "cplt mcp status did not answer here (" + st.Failed + ")", Fix: mcpStatusFailedFix}}
	case st.PlatformSupported != nil && !*st.PlatformSupported:
		return "cplt (macOS only)", nil, []mcpProblem{{Server: id, Problem: "cplt's MCP broker runs on macOS only for now, so here the server is left as configured", Fix: "use it on macOS; Linux support is tracked in navikt/cplt#825"}}
	}
	path = "cplt broker"
	file := strings.ReplaceAll(id, "/", "__") + ".json"
	for _, r := range st.Rejected {
		if filepath.Base(r.File) == file {
			path = "rejected"
			fix := r.Fix
			if fix == "" {
				fix = "nav-pilot mcp enable " + id
			}
			out = append(out, mcpProblem{Server: id, Problem: "cplt rejected its spec: " + safe(r.Reason, 200), Fix: safe(fix, 200)})
		}
	}
	found := path == "rejected"
	for _, s := range st.Servers {
		if s.ID != id {
			continue
		}
		found, credOK = true, s.CredentialOK
		if s.State == "off" {
			path = "cplt off"
			out = append(out, mcpProblem{Server: id, Problem: "cplt's MCP broker is off", Fix: "cplt config set sandbox.mcp_broker true"})
		} else if s.State != "brokered" {
			path = "cplt " + safe(s.State, 16)
		}
	}
	if !found {
		path = "-"
	}
	return path, credOK, out
}

// mcpDisableBroker removes e's spec; false when there was none.
func mcpDisableBroker(id string) (bool, error) {
	if !mcpBrokerEnabled(id) {
		return false, nil
	}
	path, err := mcpBrokerSafePath(id)
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
// problems. The login comes from cplt's status when it has one, else from
// the mechanism's status command.
func mcpBrokerRow(row *mcpServerRow, e providerpkg.MCPServerEntry, cpltPath string, st *mcpCpltStatusDoc) []mcpProblem {
	m, err := mcpBrokerCheck(e)
	if err != nil {
		row.Path = "-"
		return []mcpProblem{{Server: e.Name, Problem: strings.SplitN(err.Error(), ". Fix: ", 2)[0], Fix: "nav-pilot update"}}
	}
	path, credOK, out := mcpBrokerState(cpltPath, st, e.Name)
	row.Path = path
	if !mcpBrokerEnabled(e.Name) {
		// A client entry the user wrote by hand, with no spec: it runs as
		// written, without cplt's tool and GraphQL checks.
		row.Path, credOK = "direct (outside the broker)", nil
		out = []mcpProblem{{Server: e.Name, Problem: "configured in your client but not through cplt's broker, so nothing keeps it read-only", Fix: "nav-pilot mcp enable " + e.Name}}
	}
	if old := mcpOldServer(m); old != "" {
		out = append(out, mcpProblem{Server: e.Name, Problem: old, Fix: m.upgrade})
	}
	if len(m.status) > 0 {
		tool, login := m.status[0], strings.Join(m.login, " ")
		state := "logged out"
		if credOK != nil {
			if *credOK {
				state = "ok"
			}
		} else {
			state = mcpLoginState(m)
		}
		row.Login = tool + ": " + state
		switch state {
		case "logged out":
			out = append(out, mcpProblem{Server: e.Name, Problem: "not logged in on this machine", Fix: login})
		case "not installed":
			out = append(out, mcpProblem{Server: e.Name, Problem: tool + " is not installed", Fix: "install it (" + mcpNaisInstall + "), then " + login})
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
			row.Tools = fmt.Sprintf("%d of %d", len(spec.Meta[mcpBrokerKey].Tools.Allow), len(e.Meta.Broker.Tools.Classes))
		}
	}
	return out
}
