package cli

import (
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// What cplt says about the broker: `cplt mcp status --json`, and what that
// means for one server.

const (
	mcpStatusFailedFix = "cd <your repo> && nav-pilot mcp list"
	mcpCpltDoctorFix   = "cplt doctor --fix"
)

var (
	mcpCpltUpgrade = domain.CpltUpgradeHint(domain.PkgBrew)
	mcpCpltInstall = domain.CpltInstallHint(domain.PkgBrew)
)

// mcpCpltStatusDoc is `cplt mcp status --json`. Failed is set, and the rest
// empty, when a cplt that has the command did not answer: it refuses to run
// in ~ as a launch does, timed out, or printed something else. OffReason is
// why the broker does not run here when PlatformSupported is false.
type mcpCpltStatusDoc struct {
	Schema            []int  `json:"schema"`
	PlatformSupported *bool  `json:"platform_supported"`
	OffReason         string `json:"off_reason"`
	Servers           []struct {
		ID           string `json:"id"`
		State        string `json:"state"`
		CredentialOK *bool  `json:"credential_ok"`
		// CredentialNote says why CredentialOK is null, e.g. "timed out".
		CredentialNote string `json:"credential_note"`
	} `json:"servers"`
	Rejected []struct {
		File   string `json:"file"`
		Reason string `json:"reason"`
		Fix    string `json:"fix"`
	} `json:"rejected"`
	Failed   string `json:"-"`
	TimedOut bool   `json:"-"`
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
		return &mcpCpltStatusDoc{Failed: "timed out after " + mcpCpltStatusTimeout.String(), TimedOut: true}
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
// credential probe with the note when that is null, and the problems with
// their fixes.
func mcpBrokerState(cpltPath string, st *mcpCpltStatusDoc, id string) (path string, credOK *bool, credNote string, out []mcpProblem) {
	switch {
	case cpltPath == "":
		return "not configured (no cplt)", nil, "", []mcpProblem{{Server: id, Problem: "it runs only under cplt, which is not installed", Fix: mcpCpltInstall}}
	case st == nil:
		return "-", nil, "", []mcpProblem{{Server: id, Problem: "this cplt cannot run MCP servers on the host, so it leaves them as configured", Fix: mcpCpltUpgrade}}
	case st.Failed != "":
		return "-", nil, "", []mcpProblem{{Server: id, Problem: "cplt mcp status did not answer here (" + st.Failed + ")", Fix: mcpStatusFailedFix}}
	case st.PlatformSupported != nil && !*st.PlatformSupported:
		return "cplt (off here)", nil, "", []mcpProblem{mcpOffProblem(id, st.OffReason)}
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
		found, credOK, credNote = true, s.CredentialOK, safe(s.CredentialNote, 40)
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
	return path, credOK, credNote, out
}

// mcpOffProblem is the first sentence of cplt's off_reason (navikt/cplt
// src/sandbox.rs mcp_refusal_text) as the problem. Only an unusable
// Bubblewrap is something `cplt doctor --fix` repairs; for the rest (D-Bus
// endpoints it cannot mask) cplt doctor says what to do. The reason's own
// fix text, which can be a multi-line script, stays out of the line.
func mcpOffProblem(id, reason string) mcpProblem {
	fix := "cplt doctor"
	if strings.Contains(reason, "is not usable") {
		fix = mcpCpltDoctorFix
	}
	problem, _, _ := strings.Cut(reason, "\n")
	if i := strings.Index(problem, ": without"); i > 0 {
		problem = problem[:i]
	} else if i := strings.Index(problem, ". "); i > 0 {
		problem = problem[:i]
	}
	problem = strings.TrimRight(strings.TrimSpace(problem), ".")
	if problem == "" {
		return mcpProblem{Server: id, Problem: "cplt's MCP broker does not run here, so the server is left as configured", Fix: fix}
	}
	return mcpProblem{Server: id, Problem: "cplt's MCP broker is off here: " + safe(problem, 200), Fix: fix}
}
