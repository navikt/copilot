package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// applyOpenCodeOwnDirs lets the session read the instructions, agents and
// skills nav-pilot installs in opencode's config directory, and in a staged
// launch's OPENCODE_CONFIG_DIR, without an external_directory request. Those
// directories are outside the project, a model that opens a file there gets
// asked, and `opencode run` rejects the request and ends the session (#1120).
// Everything else outside the project still asks.
//
// OPENCODE_CONFIG_CONTENT is merged over the user's config, and an object
// replaces a plain string there, so the user's own external_directory string
// goes first as "*". When the user already allows or denies every directory,
// or sets the whole permission block as one string, this adds nothing: an
// object would change what they chose.
func applyOpenCodeOwnDirs(env []string, projectDir string) []string {
	var whole, ext string
	for _, doc := range openCodeConfigDocs(projectDir, env) {
		var cfg struct {
			Permission json.RawMessage `json:"permission"`
		}
		if json.Unmarshal(stripJSONC(doc), &cfg) != nil || cfg.Permission == nil {
			continue
		}
		if json.Unmarshal(cfg.Permission, &whole) == nil {
			ext = ""
			continue
		}
		whole = ""
		var p struct {
			ExternalDirectory json.RawMessage `json:"external_directory"`
		}
		if json.Unmarshal(cfg.Permission, &p) == nil && p.ExternalDirectory != nil {
			ext = ""
			_ = json.Unmarshal(p.ExternalDirectory, &ext)
		}
	}
	if whole != "" || ext == "allow" || ext == "deny" {
		return env
	}
	rules := map[string]any{}
	if ext != "" {
		rules["*"] = ext // "*" sorts before any absolute path, so it comes first
	}
	dirs := []string{openCodeConfigDir()}
	for _, e := range env {
		if d, ok := strings.CutPrefix(e, "OPENCODE_CONFIG_DIR="); ok && d != "" {
			dirs = append(dirs, d)
		}
	}
	for _, d := range dirs {
		for _, sub := range []string{"instructions", "agents", "skills"} {
			rules[filepath.ToSlash(filepath.Join(d, sub))+"/*"] = "allow"
		}
	}
	return withOpenCodeConfigContent(env, map[string]any{"permission": map[string]any{"external_directory": rules}})
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
	fmt.Fprintf(os.Stderr, "%s opencode %s is outside the tested range (%s). Hooks, the dispatch gate and the session policy may not apply as described. See %s.\n",
		domain.Yellow("⚠"), v, OpenCodeTestedRange, domain.Bold("nav-pilot doctor"))
}
