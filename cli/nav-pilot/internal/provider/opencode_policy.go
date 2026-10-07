package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
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

// OpenCodeInstallVersion is the release nav-pilot's installer asks for: inside
// OpenCodeTestedRange, and the one the hooks were verified against. Passing a
// version makes opencode's install script skip its api.github.com lookup,
// which fails with "Failed to fetch version information" behind a rate limit
// or a proxy (#1345). Raise it with the range.
const OpenCodeInstallVersion = "1.18.32"

func applyOpenCodePolicy(env []string) []string {
	return withOpenCodeConfigContent(env, openCodePolicy)
}

// applyOpenCodeOwnDirs lets the session read the instructions, agents and
// skills nav-pilot installs in opencode's config directory, and in a staged
// launch's OPENCODE_CONFIG_DIR, without an external_directory request. Those
// directories are outside the project, so a model that opens a file there
// gets asked, and `opencode run` rejects the request and ends the session
// (#1120). Everything else outside the project still asks.
//
// external_directory "allow" would also let the edit tools write there, and
// a file written to agents/ or instructions/ reaches every later session. So
// edits to the same directories are denied: nav-pilot's sync owns them.
// OpenCode matches an edit by the path relative to the worktree (the git
// root, or "/" outside a repo), so the deny rule is written that way.
//
// OPENCODE_CONFIG_CONTENT is merged over the user's config, and an object
// replaces a plain string there, so the user's own string for either
// permission goes first as "*". A user who denies external_directory
// wholesale gets nothing added.
func applyOpenCodeOwnDirs(env []string, projectDir string) []string {
	docs := openCodeConfigDocs(projectDir, env, false)
	ext, denied := userPermission(docs, "external_directory")
	if denied {
		return env
	}
	edit, _ := userPermission(docs, "edit")
	extRules, editRules := map[string]any{}, map[string]any{}
	// "*" sorts before any path, so it comes first, and the last match wins.
	if ext != "" {
		extRules["*"] = ext
	}
	if edit != "" {
		editRules["*"] = edit
	}
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	worktree := source.FindGitRoot(projectDir)
	if worktree == "" {
		worktree = "/"
	}
	dirs := []string{openCodeConfigDir()}
	for _, e := range env {
		if d, ok := strings.CutPrefix(e, "OPENCODE_CONFIG_DIR="); ok && d != "" {
			dirs = append(dirs, d)
		}
	}
	for _, d := range dirs {
		for _, sub := range []string{"instructions", "agents", "skills"} {
			p := filepath.ToSlash(filepath.Join(d, sub))
			extRules[p+"/*"] = "allow"
			if rel, err := filepath.Rel(worktree, filepath.Join(d, sub)); err == nil {
				editRules[filepath.ToSlash(rel)+"/*"] = "deny"
			}
		}
	}
	return withOpenCodeConfigContent(env, map[string]any{"permission": map[string]any{
		"external_directory": extRules,
		"edit":               editRules,
	}})
}

// userPermission reads permission.<key> across the user's OpenCode configs in
// merge order. str is the last plain-string value for it, which our object
// would otherwise replace; a later object form clears it. deny is whether any
// file denies it wholesale: "deny" as its string, "*": "deny" in its object,
// or the whole permission block denying.
//
// On opencode 2 it also reads the flat `permissions` list, which opencode 2
// joins across files in the same order, the last match winning
// (core/src/permission.ts at v2.0.24): deny is then whether the last rule for
// key on every resource ("*") denies. A list rule is added to, never
// replaced, so it sets no str.
func userPermission(docs [][]byte, key string) (str string, deny bool) {
	isDeny := func(raw json.RawMessage) bool {
		var s string
		return json.Unmarshal(raw, &s) == nil && s == "deny"
	}
	v2, listDeny := openCodeMajor() >= 2, false
	for _, doc := range docs {
		var cfg struct {
			Permission  json.RawMessage `json:"permission"`
			Permissions []struct {
				Action, Resource, Effect string
			} `json:"permissions"`
		}
		if json.Unmarshal(stripJSONC(doc), &cfg) != nil {
			continue
		}
		if v2 {
			for _, r := range cfg.Permissions {
				// ponytail: path.Match for opencode's Wildcard; the same for
				// action names, which have no slash.
				if ok, _ := path.Match(r.Action, key); ok && r.Resource == "*" {
					listDeny = r.Effect == "deny"
				}
			}
		}
		if cfg.Permission == nil {
			continue
		}
		// A string block is {"*": value} to OpenCode, per file.
		if isDeny(cfg.Permission) {
			deny = true
		}
		var block map[string]json.RawMessage
		if json.Unmarshal(cfg.Permission, &block) != nil {
			continue
		}
		if isDeny(block["*"]) {
			deny = true
		}
		v, ok := block[key]
		if !ok {
			continue
		}
		var s string
		var obj map[string]json.RawMessage
		switch {
		case json.Unmarshal(v, &s) == nil:
			str, deny = s, deny || s == "deny"
		case json.Unmarshal(v, &obj) == nil:
			str, deny = "", deny || isDeny(obj["*"])
		}
	}
	return str, deny || listDeny
}

// OpenCodeVersionStatus reports the installed opencode's version and whether
// it is inside [OpenCodeTestedRange]. err is set when the version could not
// be read.
func OpenCodeVersionStatus() (version string, tested bool, err error) {
	out, err := cachedVersion("opencode", 5*time.Second)
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
	if err != nil || tested || !SeenChanged("opencode-untested", v+" "+OpenCodeTestedRange) {
		return
	}
	fmt.Fprintf(os.Stderr, "%s opencode %s is outside the tested range (%s). Hooks, the dispatch gate and the session policy may not apply as described. See %s.\n",
		domain.Yellow("⚠"), v, OpenCodeTestedRange, domain.Bold("nav-pilot doctor"))
}

// CheckOpenCodeMajor stops a launch on opencode 2, which the warning above is
// not enough for: opencode 2 rejects --agent and --model on the TUI, does not
// load nav-pilot's hooks plugins (its plugin API is new), runs sessions in a
// shared background service that ignores the launch's environment, and reads
// OPENCODE_CONFIG_DIR in place of the user's config rather than alongside it.
// A session that started anyway would run without nav-pilot's hooks and MCP
// policy.
// A prerelease such as "opencode v2.1.0-beta.1" does not parse as a version,
// so the major is read from the raw line; any other unreadable version
// launches, as warnUntestedOpenCode does.
func CheckOpenCodeMajor() error {
	v, _, err := OpenCodeVersionStatus()
	if err != nil {
		out, _ := cachedVersion("opencode", 5*time.Second)
		m := openCodeMajorPattern.FindStringSubmatch(strings.TrimSpace(out))
		if m == nil {
			return nil
		}
		v = strings.TrimSpace(out)
		if n, _ := strconv.Atoi(m[1]); n < 2 {
			return nil
		}
	} else if strings.HasPrefix(v, "0.") || strings.HasPrefix(v, "1.") {
		return nil
	}
	return fmt.Errorf("opencode %s is opencode 2 or newer, which nav-pilot does not launch yet: its flags, plugins and config loading changed.\n\n  Install opencode 1: %s",
		strings.TrimPrefix(v, "opencode "), domain.Bold(OpenCode1InstallHint()))
}

// openCodeMajor is the installed opencode's major version, read from the raw
// version line so a prerelease counts. Unreadable is 0, which every caller
// treats as opencode 1: that path is the one nav-pilot has always taken.
func openCodeMajor() int {
	out, _ := cachedVersion("opencode", 5*time.Second)
	m := openCodeMajorPattern.FindStringSubmatch(strings.TrimSpace(out))
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

var openCodeMajorPattern = regexp.MustCompile(`(?i)^(?:opencode )?v?(\d+)\.`)

// minOpenCode2CpltStamp is the first cplt release with navikt/cplt#716, where
// the opencode 2 client spawns its session's service inside the sandbox. An
// older cplt lets the client attach to the host's background service, which
// runs with none of the launch's environment: no hooks, no gate, no policy.
// errCpltTooOld marks checkOpenCode2Launch's refusal of a cplt older than
// minOpenCode2CpltStamp, so telemetry can tell it from argument refusals.
var errCpltTooOld = errors.New("cplt too old")

const minOpenCode2CpltStamp = "2026.10.07-103638"

// checkOpenCode2Launch refuses an opencode 2 launch that would run outside the
// sandboxed per-session service: a cplt without #716 (or one whose version
// cannot be read, fail-closed as checkCpltFloor), or arguments that connect
// the client to another server. Nil on opencode 1.
func checkOpenCode2Launch(args []string) error {
	if openCodeMajor() < 2 {
		return nil
	}
	if len(args) > 0 && args[0] == "attach" {
		return fmt.Errorf("attach is not allowed on opencode 2: it connects to a server outside the sandboxed session nav-pilot starts")
	}
	for _, a := range args {
		if a == "--server" || a == "--attach" || strings.HasPrefix(a, "--server=") || strings.HasPrefix(a, "--attach=") {
			return fmt.Errorf("%s is not allowed on opencode 2: it connects to a server outside the sandboxed session nav-pilot starts", a)
		}
	}
	out, err := probeCpltVersion()
	if errors.Is(err, errCpltNotFound) {
		return nil // the launch's own cplt-missing path (ErrCpltMissing) says how to install it
	}
	found := strings.TrimSpace(out)
	if err != nil {
		found = err.Error()
	}
	if stamp := cpltStamp(out); err != nil || stamp == "" || stamp < minOpenCode2CpltStamp {
		return fmt.Errorf("%w: opencode 2 needs cplt %s or newer (navikt/cplt#716), found %q: an older cplt runs the session in the host's background service, without nav-pilot's hooks.\n\n  Upgrade it: %s",
			errCpltTooOld, minOpenCode2CpltStamp, found, domain.Bold(cpltUpgradeHint()))
	}
	return nil
}

// OpenCodeScriptInstall is opencode's own installer, pinned to the tested release.
const OpenCodeScriptInstall = "curl -fsSL https://opencode.ai/install | bash -s -- --version " + OpenCodeInstallVersion

// brewKegPattern pulls the formula name out of a Homebrew Cellar path.
var brewKegPattern = regexp.MustCompile(`/Cellar/([^/]+)/`)

// OpenCode1InstallHint is the command that puts opencode 1 back on this
// machine, chosen by how the opencode on PATH was installed. npm's opencode 2
// package (@opencode/cli) owns the same bin, so installing opencode-ai over it
// fails with EEXIST. On Homebrew, opencode 2 comes from homebrew-core's
// opencode or anomalyco/tap/opencode-v2, while anomalyco/tap/opencode is still
// opencode 1, so the hint removes the keg actually on PATH and installs that.
func OpenCode1InstallHint() string {
	path, _ := exec.LookPath("opencode")
	resolved, _ := filepath.EvalSymlinks(path)
	if strings.Contains(resolved, "node_modules") {
		return "npm uninstall -g @opencode/cli && npm i -g opencode-ai@" + OpenCodeInstallVersion
	}
	if domain.PkgOwner(path) == domain.PkgBrew {
		keg := "opencode"
		if m := brewKegPattern.FindStringSubmatch(resolved); m != nil {
			keg = m[1]
		}
		return "brew uninstall " + keg + " && brew install anomalyco/tap/opencode"
	}
	return OpenCodeScriptInstall
}
