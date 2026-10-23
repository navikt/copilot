package cli

import (
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// What cplt says about the broker: `cplt mcp status --json`, and what that
// means for one server.

const (
	mcpCpltUpgrade     = "brew upgrade cplt"
	mcpCpltInstall     = "brew install navikt/tap/cplt"
	mcpStatusFailedFix = "cd <your repo> && nav-pilot mcp list"
)

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
	start := time.Now()
	out, err := runBoundedTimeout(mcpCpltStatusTimeout, false, cpltPath, "mcp", "status", "--json")
	var msg string
	if ee := (*exec.ExitError)(nil); errors.As(err, &ee) {
		msg = strings.TrimSpace(string(ee.Stderr))
	}
	if strings.Contains(msg, "unrecognized subcommand") {
		return nil // clap's answer from a cplt without `mcp status`
	}
	var doc mcpCpltStatusDoc
	switch {
	case time.Since(start) >= mcpCpltStatusTimeout:
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
		return "cplt (macOS only)", nil, []mcpProblem{{Server: id, Problem: "cplt's MCP broker runs on macOS only for now, so here the server is left as configured", Fix: "use it on macOS; Linux support is tracked in navikt/cplt#834"}}
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
