package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// Commands nav-pilot runs on the host for a mechanism: only the ones in its
// own table (mcp_broker.go), never a registry string.

const mcpHostTimeout = 10 * time.Second

// mcpProbeTimeout is a var so tests time out fast.
var mcpProbeTimeout = mcpHostTimeout

// mcpLoginStates caches each probe's answer for one nav-pilot run, so a
// list or enable probes once; cmdMCP clears it, and a login forgets it.
var mcpLoginStates = map[string]string{}

// mcpLoginState is "ok", "logged out", "not installed" or "unknown (timed
// out)", from the mechanism's probe. Its output is discarded.
func mcpLoginState(m mcpMechanism) string {
	key := strings.Join(m.probe, " ")
	if s, ok := mcpLoginStates[key]; ok {
		return s
	}
	start := time.Now()
	_, err := runBoundedTimeout(mcpProbeTimeout, false, m.probe[0], m.probe[1:]...)
	s := "logged out"
	switch {
	case err == nil:
		s = "ok"
	case errors.Is(err, exec.ErrNotFound):
		s = "not installed"
	case time.Since(start) >= mcpProbeTimeout:
		s = "unknown (timed out)"
	}
	mcpLoginStates[key] = s
	return s
}

// mcpRunLogin runs the mechanism's login in the terminal, with no deadline:
// it waits on the user and the browser.
func mcpRunLogin(m mcpMechanism) error {
	cmd := exec.Command(m.login[0], m.login[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	delete(mcpLoginStates, strings.Join(m.probe, " "))
	return cmd.Run()
}

// mcpConfirmLogin asks whether to log in now; Enter is yes. A var so tests
// answer it.
var mcpConfirmLogin = func(title string) bool { return confirm(title, false, true) }

var mcpSemver = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

func mcpParseSemver(s string) (agentpakke.Semver3, bool) {
	m := mcpSemver.FindStringSubmatch(s)
	if m == nil {
		return agentpakke.Semver3{}, false
	}
	var v agentpakke.Semver3
	for i := range v {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	return v, true
}

// mcpOldServer is the nudge when the host CLI is older than the
// mechanism's minVersion, or "". An unreadable version says nothing.
func mcpOldServer(m mcpMechanism) string {
	out, err := runBoundedTimeout(mcpHostTimeout, true, m.version[0], m.version[1:]...)
	if err != nil {
		return ""
	}
	got, ok := mcpParseSemver(string(out))
	want, _ := mcpParseSemver(m.minVersion)
	if !ok || got.Compare(want) >= 0 {
		return ""
	}
	return fmt.Sprintf("%s %s is older than %s, %s", m.tool(), got, m.minVersion, m.oldNotice)
}
