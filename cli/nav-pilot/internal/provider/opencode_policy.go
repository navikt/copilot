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
	"runtime"
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

// OpenCode1TestedRange and OpenCode2TestedRange are the OpenCode versions nav-pilot's OpenCode path is
// tested against: the plugin hooks the bridge and the dispatch gate rely on,
// the config merge order, the permission schema. Raise it after running the
// e2e journeys and the real-opencode checks in docs/opencode-hooks.md against
// the new release. opencode 1 and 2 are tested separately, so each major has
// its own range; OpenCodeTestedRangeFor picks one.
const (
	OpenCode1TestedRange = ">=1.18.20,<1.19"
	OpenCode2TestedRange = ">=2.0.24,<2.1"
)

// OpenCodeTestedRangeFor is the tested range for version's major.
func OpenCodeTestedRangeFor(version string) string {
	if n, _ := strconv.Atoi(strings.SplitN(version, ".", 2)[0]); n >= 2 {
		return OpenCode2TestedRange
	}
	return OpenCode1TestedRange
}

// OpenCodeInstallVersion is the release nav-pilot's installer asks for: inside
// OpenCode1TestedRange, and the one the hooks were verified against. Passing a
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
// it is inside [OpenCodeTestedRangeFor]. err is set when the version could not
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
	version = fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2])
	rng, err := agentpakke.ParseVersionRange(OpenCodeTestedRangeFor(version))
	if err != nil {
		return "", false, err
	}
	return version, rng.Contains(v), nil
}

// warnUntestedOpenCode says, and only says, when the opencode about to launch
// is outside the tested range. A version check must not be what stops a
// session: the developer may have good reason to run a newer opencode, and
// the parts that could break fail in the safe direction (redaction withholds).
func warnUntestedOpenCode() {
	v, tested, err := OpenCodeVersionStatus()
	if err != nil || tested || !SeenChanged("opencode-untested", v+" "+OpenCodeTestedRangeFor(v)) {
		return
	}
	fmt.Fprintf(os.Stderr, "%s opencode %s is outside the tested range (%s). Hooks, the dispatch gate and the session policy may not apply as described. See %s.\n",
		domain.Yellow("⚠"), v, OpenCodeTestedRangeFor(v), domain.Bold("nav-pilot doctor"))
}

// CheckOpenCodeMajor stops a launch on opencode 3 or newer, which the warning
// above is not enough for: opencode 2 changed the flags, the plugin API and
// how config loads, and nav-pilot's bridge only exists for 1 and 2. A session
// on an unknown major could start without nav-pilot's hooks and MCP policy.
// A prerelease such as "opencode v3.0.0-beta.1" does not parse as a version,
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
		if n, _ := strconv.Atoi(m[1]); n < 3 {
			return nil
		}
	} else if n, _ := strconv.Atoi(strings.SplitN(v, ".", 2)[0]); n < 3 {
		return nil
	}
	// opencode 2 runs under cplt on macOS and Linux (checkOpenCode2Launch).
	major, hint := "1", OpenCode1InstallHint()
	if minOpenCode2CpltStamp() != "" {
		major, hint = "2", OpenCode2InstallHint()
	}
	return fmt.Errorf("opencode %s is newer than nav-pilot supports (opencode 1 and 2): its flags, plugins and config loading may have changed.\n\n  Install opencode %s: %s",
		strings.TrimPrefix(v, "opencode "), major, domain.Bold(hint))
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
// The floor is 2026.10.08-081501: it carries #716, #720 and #722, plus
// navikt/cplt#736 (AGENTS.md grants from ancestor directories, stow symlinks),
// the fix for a credential leak through planted <project>/.agents, .claude or
// .opencode symlinks in opencode 2 sessions, and #735, which fails closed on
// an opencode whose version it cannot read.
// errCpltTooOld marks checkOpenCode2Launch's refusal of a cplt older than
// minOpenCode2CpltStamp, so telemetry can tell it from argument refusals.
var errCpltTooOld = errors.New("cplt too old")

// hostOS is runtime.GOOS; tests set it.
var hostOS = runtime.GOOS

// minOpenCode2CpltStamp is the cplt floor for opencode 2 on hostOS, "" where
// cplt does not run it. Linux needs navikt/cplt#740 (2026.10.08-092800),
// which runs opencode 2 under bubblewrap; cplt itself refuses a host without
// it, with install instructions.
func minOpenCode2CpltStamp() string {
	switch hostOS {
	case "darwin":
		return "2026.10.08-081501"
	case "linux":
		return "2026.10.08-092800"
	}
	return ""
}

// checkOpenCode2Launch refuses an opencode 2 launch that would run outside the
// sandboxed per-session service: a cplt without #716 (or one whose version
// cannot be read, fail-closed as checkCpltFloor), arguments that connect
// the client to another server, or a platform other than macOS and Linux, where cplt
// does not run opencode 2. Nil on opencode 1. An opencode whose version
// cannot be read is launched as opencode 1 but still needs the cplt floor, so
// cplt's own fail-closed refusal (#735) is there if it really is opencode 2.
var lookPath = exec.LookPath

func checkOpenCode2Launch(args []string) error {
	out, _ := cachedVersion("opencode", 5*time.Second)
	if !openCodeMajorPattern.MatchString(strings.TrimSpace(out)) {
		if _, err := lookPath("opencode"); err != nil {
			return nil // not installed: nothing to launch, doctor reports it elsewhere
		}
		// cplt may still see opencode 2, so keep the argument refusals too.
		if err := checkOpenCode2Cplt(); err != nil {
			return err
		}
	} else if openCodeMajor() < 2 {
		return nil
	} else if minOpenCode2CpltStamp() == "" {
		return fmt.Errorf("opencode 2 runs under cplt on macOS and Linux only, and nav-pilot never launches a client outside cplt.\n\n  Install opencode 1: %s",
			domain.Bold(OpenCode1InstallHint()))
	}
	if len(args) > 0 && args[0] == "attach" {
		return fmt.Errorf("attach is not allowed on opencode 2: it connects to a server outside the sandboxed session nav-pilot starts")
	}
	for _, a := range args {
		if a == "--" {
			break // what follows is the message, not options
		}
		if a == "--standalone" {
			return fmt.Errorf("--standalone is not supported on opencode 2 under cplt: the client cannot reach a standalone service in the sandbox")
		}
		if a == "--server" || a == "--attach" || strings.HasPrefix(a, "--server=") || strings.HasPrefix(a, "--attach=") {
			return fmt.Errorf("%s is not allowed on opencode 2: it connects to a server outside the sandboxed session nav-pilot starts", a)
		}
	}
	return checkOpenCode2Cplt()
}

// checkOpenCode2Cplt refuses a cplt older than minOpenCode2CpltStamp. On a
// platform without a floor (an unreadable opencode version there), the macOS
// floor stands in so cplt's fail-closed refusal (#735) is still present.
func checkOpenCode2Cplt() error {
	floor := minOpenCode2CpltStamp()
	if floor == "" {
		floor = "2026.10.08-081501"
	}
	out, err := probeCpltVersion()
	if errors.Is(err, errCpltNotFound) {
		return nil // the launch's own cplt-missing path (ErrCpltMissing) says how to install it
	}
	found := strings.TrimSpace(out)
	if err != nil {
		found = err.Error()
	}
	if stamp := cpltStamp(out); err != nil || stamp == "" || stamp < floor {
		return fmt.Errorf("%w: opencode 2 needs cplt %s or newer (navikt/cplt#716, #722, #735, #736, #740 on Linux), found %q: %s.\n\n  Upgrade it: %s",
			errCpltTooOld, floor, found, oldCpltConsequence(), domain.Bold(cpltUpgradeHint()))
	}
	return nil
}

// OpenCodeLaunchCheck is checkOpenCode2Launch for doctor: what a plain
// `nav-pilot opencode` would be refused for, nil on opencode 1.
func OpenCodeLaunchCheck() error { return checkOpenCode2Launch(nil) }

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

// openCode2InstallVersion is the opencode 2 release nav-pilot is tested with.
const openCode2InstallVersion = "2.0.24"

// OpenCode2InstallHint replaces the opencode on PATH with opencode 2, through
// Homebrew when that is how it came, otherwise npm.
func OpenCode2InstallHint() string {
	path, _ := exec.LookPath("opencode")
	resolved, _ := filepath.EvalSymlinks(path)
	if domain.PkgOwner(path) == domain.PkgBrew {
		keg := "opencode"
		if m := brewKegPattern.FindStringSubmatch(resolved); m != nil {
			keg = m[1]
		}
		return "brew uninstall " + keg + " && brew install anomalyco/tap/opencode-v2"
	}
	return "npm i -g @opencode/cli@" + openCode2InstallVersion
}

// oldCpltConsequence is what a cplt below the floor does with opencode 2:
// before #740 it refused it on Linux; on macOS it ran the host's service.
func oldCpltConsequence() string {
	if hostOS == "linux" {
		return "an older cplt refuses to run opencode 2 on Linux"
	}
	return "an older cplt runs the session in the host's background service, without nav-pilot's hooks"
}
