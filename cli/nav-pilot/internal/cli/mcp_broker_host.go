package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// Commands nav-pilot runs on the host for a mechanism: only the ones in its
// own table (mcp_broker.go), never a registry string.

const mcpHostTimeout = 10 * time.Second

// mcpLoginState is "ok", "logged out" or "not installed", from the
// mechanism's probe. Its output is discarded.
func mcpLoginState(m mcpMechanism) string {
	_, err := runBoundedTimeout(mcpHostTimeout, false, m.probe[0], m.probe[1:]...)
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, exec.ErrNotFound):
		return "not installed"
	}
	return "logged out"
}

// mcpRunLogin runs the mechanism's login in the terminal, with no deadline:
// it waits on the user and the browser.
func mcpRunLogin(m mcpMechanism) error {
	cmd := exec.Command(m.login[0], m.login[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
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
