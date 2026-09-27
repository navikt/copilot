package provider

import (
	"fmt"
	"os"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Policy nav-pilot enforces on every OpenCode session it launches (#1027,
// decision 4 in #1022). It travels in OPENCODE_CONFIG_CONTENT, which OpenCode
// merges after the user's and the project's config, so it holds whatever
// either says and nothing is written to the user's file.
//
//   - share "disabled": /share would upload the session to opencode.ai.
//   - autoupdate "notify": an update past the tested range reaches the
//     developer as a notice, not as a new binary mid-week.
var openCodePolicy = map[string]any{"share": "disabled", "autoupdate": "notify"}

// OpenCodeTestedRange is the OpenCode versions nav-pilot's OpenCode path is
// tested against: the plugin hooks the bridge and the dispatch gate rely on,
// the config merge order, the permission schema. Raise it after running the
// e2e journeys and the real-opencode checks in docs/opencode-hooks.md against
// the new release.
const OpenCodeTestedRange = ">=1.18.20,<1.19"

func applyOpenCodePolicy(env []string) []string {
	return withOpenCodeConfigContent(env, openCodePolicy)
}

// OpenCodeVersionStatus reports the installed opencode's version and whether
// it is inside [OpenCodeTestedRange]. err is set when the version could not
// be read.
func OpenCodeVersionStatus() (version string, tested bool, err error) {
	out, err := runStagedProbe(5*time.Second, "opencode", "--version")
	if err != nil {
		return "", false, err
	}
	v, err := parseClientVersion("opencode", out)
	if err != nil {
		return "", false, err
	}
	rng, err := agentpakke.ParseVersionRange(OpenCodeTestedRange)
	if err != nil {
		return "", false, err
	}
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]), rng.Contains(v), nil
}

// warnUntestedOpenCode says, and only says, when the opencode about to launch
// is outside the tested range. A version check must not be what stops a
// session: the developer may have good reason to run a newer opencode, and
// the parts that could break fail in the safe direction (redaction withholds).
func warnUntestedOpenCode() {
	v, tested, err := OpenCodeVersionStatus()
	if err != nil || tested {
		return
	}
	fmt.Fprintf(os.Stderr, "%s opencode %s is outside the versions nav-pilot is tested with (%s). Hooks, the dispatch gate or the policy may not apply as described; %s says more.\n",
		domain.Yellow("⚠"), v, OpenCodeTestedRange, domain.Bold("nav-pilot doctor"))
}
