package cli

import (
	"encoding/json"
	"errors"
	"os"
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
	mcpCpltDoctorFix   = "cplt doctor --fix"
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
	Failed string `json:"-"`
}

// mcpCpltStatusTimeout is a var so tests time out fast.
var mcpCpltStatusTimeout = 15 * time.Second

// mcpCpltStatus is cplt's status, nil for no cplt or an old cplt (only
// when cplt says it has no such subcommand). A cplt before navikt/cplt#846
// refuses to run outside a repo, so a failure is retried once from the git
// root above the working directory; a newer cplt answers anywhere.
func mcpCpltStatus(cpltPath string) *mcpCpltStatusDoc {
	if cpltPath == "" {
		return nil
	}
	st, retry := mcpCpltStatusIn("", cpltPath)
	if !retry {
		return st
	}
	if wd, err := os.Getwd(); err == nil {
		if root := findGitRoot(wd); root != "" && root != wd {
			if again, _ := mcpCpltStatusIn(root, cpltPath); again == nil || again.Failed == "" {
				return again
			}
		}
	}
	return st
}

// mcpCpltStatusIn runs the status in dir ("" for the working directory);
// retry is set for a failure that was neither a timeout nor an old cplt.
func mcpCpltStatusIn(dir, cpltPath string) (st *mcpCpltStatusDoc, retry bool) {
	start := time.Now()
	out, err := runBoundedIn(dir, mcpCpltStatusTimeout, false, cpltPath, "mcp", "status", "--json")
	var msg string
	if ee := (*exec.ExitError)(nil); errors.As(err, &ee) {
		msg = strings.TrimSpace(string(ee.Stderr))
	}
	if strings.Contains(msg, "unrecognized subcommand") {
		return nil, false // clap's answer from a cplt without `mcp status`
	}
	var doc mcpCpltStatusDoc
	retry = true
	switch {
	case time.Since(start) >= mcpCpltStatusTimeout:
		msg, retry = "timed out after "+mcpCpltStatusTimeout.String(), false
	case err == nil && json.Unmarshal(out, &doc) == nil && doc.Schema != nil:
		return &doc, false
	case err == nil:
		msg = "its output was not the status JSON"
	case msg == "":
		msg = err.Error()
	}
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	return &mcpCpltStatusDoc{Failed: safe(msg, 160)}, retry
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

// mcpOffProblem is cplt's off_reason as the problem: cplt doctor fixes a
// sandbox it can set up (bwrap, AppArmor), else the reason's own "Fix: ".
func mcpOffProblem(id, reason string) mcpProblem {
	reason = safe(reason, 200)
	if reason == "" {
		return mcpProblem{Server: id, Problem: "cplt's MCP broker does not run here, so the server is left as configured", Fix: "cplt doctor"}
	}
	problem, fix, _ := strings.Cut(reason, "Fix: ")
	fix = strings.TrimSpace(fix)
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "bwrap") || strings.Contains(lower, "bubblewrap") || strings.Contains(lower, "apparmor"):
		fix = mcpCpltDoctorFix
	case fix == "":
		fix = "cplt doctor"
	}
	return mcpProblem{Server: id, Problem: "cplt's MCP broker is off here: " + strings.TrimRight(strings.TrimSpace(problem), ". "), Fix: fix}
}
