package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

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
	OpenCode2TestedRange = ">=2.0.24,<2.2"
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

// ownLocalProviderBlock reports whether v is the local provider block this
// process put in openCodeLaunchConfig, compared as JSON.
func ownLocalProviderBlock(v any) bool {
	providers, _ := openCodeLaunchConfig["provider"].(map[string]any)
	own, ok := providers[LocalProviderID]
	if !ok {
		return false
	}
	a, _ := json.Marshal(own)
	b, _ := json.Marshal(v)
	return bytes.Equal(a, b)
}

func applyOpenCodePolicy(env []string) []string {
	return withOpenCodeConfigContent(env, openCodePolicy)
}

// OpenCode 2 gives global provider policies precedence over inline config.
// Refuse conflicting rules rather than silently widening this session.
// pakke names the agentpakke of a staged launch, "" otherwise.
func applyOpenCodeProviderPolicy(env []string, projectDir string, allowLocal bool, args []string, pakke string) ([]string, error) {
	allowed := []string{"github-copilot"}
	if allowLocal {
		allowed = append(allowed, LocalProviderID)
	}
	want := strings.Join(allowed, ", ")
	getenv := openCodeEnvGetter(env)
	// Not installed: cplt's own not-found message says what to do.
	if _, err := lookPath("opencode"); err != nil {
		return withOpenCodeConfigContent(env, map[string]any{"enabled_providers": allowed}), nil
	}
	major := openCodeMajor()
	if major == 0 {
		return nil, fmt.Errorf("cannot verify OpenCode provider policy: version unavailable; check opencode --version before launching nav-pilot")
	}
	model := openCodeModelArg(args)
	stage := getenv("OPENCODE_CONFIG_DIR")
	for _, doc := range openCodeConfigDocuments(projectDir, env, false) {
		where := doc.path
		if pakke != "" && stage != "" && strings.HasPrefix(doc.path, stage+string(filepath.Separator)) {
			where = "agentpakke " + pakke
		}
		conflict := func(what string) error {
			if where != doc.path {
				return fmt.Errorf("%s ships a config that %s; launch another pakke or ask its owner. nav-pilot sessions allow only %s", where, what, want)
			}
			return fmt.Errorf("%s in %s conflicts with nav-pilot's provider restriction; nav-pilot sessions allow only %s", what, doc.path, want)
		}
		// Exact keys: OpenCode reads them case-sensitively, encoding/json
		// would not.
		var top map[string]json.RawMessage
		if err := json.Unmarshal(stripJSONC(doc.content), &top); err != nil {
			return nil, fmt.Errorf("cannot verify OpenCode provider policy in %s: invalid configuration; correct the config before launching nav-pilot", doc.path)
		}
		if doc.path == openCodeConfigContentEnv && model == "" {
			_ = json.Unmarshal(top["model"], &model)
		}
		// A redefined allowed provider sends its traffic elsewhere (measured on
		// opencode 2.1.2: options.baseURL, settings.baseURL, npm). Any routing
		// key anywhere under the entry counts, models and variants included,
		// in the legacy provider map and opencode 2's native providers map.
		// The mlx block is nav-pilot's own, with the loop guard's address: in
		// the global config, or in OPENCODE_CONFIG_CONTENT when that config
		// has comments. No other config may reroute it.
		for _, mapKey := range []string{"provider", "providers"} {
			var providers map[string]json.RawMessage
			_ = json.Unmarshal(top[mapKey], &providers)
			for _, id := range allowed {
				raw, ok := providers[id]
				if !ok || (id == LocalProviderID && doc.path == openCodeConfigPath()) {
					continue
				}
				var v any
				_ = json.Unmarshal(raw, &v)
				// A commented config is not rewritten, so the block rides in
				// OPENCODE_CONFIG_CONTENT via openCodeLaunchConfig: exactly
				// nav-pilot's own block passes, anything changed in it does not.
				if id == LocalProviderID && mapKey == "provider" && doc.path == openCodeConfigContentEnv && ownLocalProviderBlock(v) {
					continue
				}
				if k := openCodeRoutingKey(v); k != "" {
					return nil, conflict("redefines provider " + id + " (" + mapKey + "." + id + ": " + k + ")")
				}
			}
		}
		if major < 2 {
			continue
		}
		// OpenCode substitutes these before parsing, so an unresolved value
		// here could become an allow rule; fail closed.
		if hasOpenCodePlaceholder(top["enabled_providers"]) || hasOpenCodePlaceholder(top["experimental"]) {
			return nil, conflict("uses a {file:} or {env:} placeholder in enabled_providers or experimental")
		}
		var enabled []string
		if raw, ok := top["enabled_providers"]; ok && json.Unmarshal(raw, &enabled) != nil {
			return nil, fmt.Errorf("cannot verify OpenCode provider policy in %s: enabled_providers is not a list of strings", doc.path)
		}
		for _, id := range enabled {
			if doc.path != openCodeConfigContentEnv && !slices.Contains(allowed, id) {
				return nil, conflict("enables provider " + id + " (enabled_providers)")
			}
		}
		var exp map[string]json.RawMessage
		_ = json.Unmarshal(top["experimental"], &exp)
		var policies []map[string]json.RawMessage
		if raw, ok := exp["policies"]; ok && json.Unmarshal(raw, &policies) != nil {
			return nil, fmt.Errorf("cannot verify OpenCode provider policy in %s: experimental.policies is not a list", doc.path)
		}
		for _, p := range policies {
			var action, effect, resource string
			_ = json.Unmarshal(p["action"], &action)
			_ = json.Unmarshal(p["effect"], &effect)
			_ = json.Unmarshal(p["resource"], &resource)
			// Any non-exact allow may re-enable providers after our deny.
			if action == "provider.use" && effect == "allow" && !slices.Contains(allowed, resource) {
				return nil, conflict("allows provider " + resource + " (experimental.policies)")
			}
		}
	}
	if provider, _, ok := strings.Cut(model, "/"); ok && !slices.Contains(allowed, provider) {
		if pakke != "" {
			return nil, fmt.Errorf("agentpakke %s launches model %s, but nav-pilot sessions allow only %s; launch another pakke or ask its owner", pakke, model, want)
		}
		return nil, fmt.Errorf("model %s is from provider %s, but nav-pilot sessions allow only %s; pick a model from those providers with --model", model, provider, want)
	}
	return withOpenCodeConfigContent(env, map[string]any{"enabled_providers": allowed}), nil
}

// openCodeRoutingKey returns the path of the first key under v that can
// point a provider or model at another backend, "" when there is none. A
// routing key refuses whatever its value, placeholder or not.
func openCodeRoutingKey(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(t)) {
			switch k {
			case "npm", "package", "canonical", "api", "baseURL", "baseUrl", "endpoint", "url":
				return k
			}
			// A substituted key could become one of the above. Placeholders
			// in other values (apiKey) are fine.
			if hasOpenCodePlaceholder([]byte(k)) {
				return k
			}
			if sub := openCodeRoutingKey(t[k]); sub != "" {
				return k + "." + sub
			}
		}
	case []any:
		for _, e := range t {
			if sub := openCodeRoutingKey(e); sub != "" {
				return sub
			}
		}
	}
	return ""
}

func hasOpenCodePlaceholder(raw []byte) bool {
	return bytes.Contains(raw, []byte("{file:")) || bytes.Contains(raw, []byte("{env:"))
}

// openCodeModelArg is the session model named on the opencode command line.
func openCodeModelArg(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		if (a == "--model" || a == "-m") && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--model="); ok {
			return v
		}
	}
	return ""
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
	out, err := cachedVersion("opencode", clientProbeTimeout)
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

// OpenCode1EndOfSupport is the last day nav-pilot supports opencode 1 (#1907).
const OpenCode1EndOfSupport = "2026-11-19"

// OpenCode1Notice is the upgrade nudge for an opencode 1 user, "" on any
// other major or an unreadable version.
func OpenCode1Notice() string {
	if openCodeMajor() != 1 {
		return ""
	}
	n := fmt.Sprintf("opencode 1 support ends after %s. Upgrade to opencode 2: %s\n"+
		"  The upgrade is one-way: opencode 2 converts the opencode 1 database. You will need to log in again (auth.json is not carried over).",
		OpenCode1EndOfSupport, domain.Bold(OpenCode2InstallHint()))
	if hostOS == "linux" {
		n += " Linux needs bubblewrap."
	}
	return n
}

// warnOpenCode1 prints OpenCode1Notice at launch, once per opencode version,
// and the CpltMCPNotice once per cplt version.
func warnOpenCode1() {
	out, _ := cachedVersion("opencode", clientProbeTimeout)
	if n := OpenCode1Notice(); n != "" && SeenChanged("opencode1-notice", strings.TrimSpace(out)) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), n)
	}
	if n, stamp := cpltMCPNotice(); n != "" && SeenChanged("cplt-mcp-notice", stamp) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), n)
	}
}

// CpltMCPNotice nudges an opencode 2 user whose cplt is at or above
// openCode2CpltFloor but below openCode2CpltRecommended; "" otherwise.
func CpltMCPNotice() string { n, _ := cpltMCPNotice(); return n }

func cpltMCPNotice() (notice, stamp string) {
	if openCodeMajor() != 2 {
		return "", ""
	}
	out, err := probeCpltVersion()
	stamp = cpltStamp(out)
	if err != nil || stamp == "" || stamp < openCode2CpltFloor || stamp >= openCode2CpltRecommended {
		return "", ""
	}
	return fmt.Sprintf("cplt %s or newer is recommended for opencode 2: it brings GitHub MCP in opencode (io.github.navikt/github-mcp, navikt/cplt#807) and lets nav-pilot block the API keys opencode's voice feature reads even when they are passed through (navikt/cplt#817). Update cplt: %s",
		openCode2CpltRecommended, domain.Bold(cpltUpgradeHint())), stamp
}

// openCodeVoiceEnv are the env vars opencode 2.1.3's voice feature reads
// (packages/core/src/voice.ts:179-184). Voice is not allowed under nav-pilot.
var openCodeVoiceEnv = []string{"OPENAI_API_KEY", "XAI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY", "GEMINI_API_KEY"}

// withDenyVoiceEnv adds --deny-env for openCodeVoiceEnv on opencode 2 when
// cplt is at least openCode2CpltRecommended (navikt/cplt#817). An older cplt
// rejects the unknown flag, so it gets none; cpltMCPNotice nudges instead.
func withDenyVoiceEnv(args []string) []string {
	if openCodeMajor() != 2 {
		return args
	}
	out, err := probeCpltVersion()
	if err != nil || cpltStamp(out) < openCode2CpltRecommended {
		return args
	}
	for _, v := range openCodeVoiceEnv {
		args = append(args, "--deny-env", v)
	}
	return args
}

// CheckOpenCodeMajor stops a launch on opencode 3 or newer, which the warning
// above is not enough for: opencode 2 changed the flags, the plugin API and
// how config loads, and nav-pilot's bridge only exists for 1 and 2. A session
// on an unknown major could start without nav-pilot's hooks and MCP policy.
// A prerelease such as "opencode v3.0.0-beta.1" does not parse as a version,
// so the major is read from the raw line. Any other unreadable version passes
// here; applyOpenCodeProviderPolicy then refuses the launch, because it cannot
// tell which config precedence applies.
func CheckOpenCodeMajor() error {
	v, _, err := OpenCodeVersionStatus()
	if err != nil {
		out, _ := cachedVersion("opencode", clientProbeTimeout)
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
// version line so a prerelease counts. Unreadable is 0; provider-policy
// enforcement refuses that case rather than guessing config precedence.
func openCodeMajor() int {
	out, _ := cachedVersion("opencode", clientProbeTimeout)
	m := openCodeMajorPattern.FindStringSubmatch(strings.TrimSpace(out))
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

var openCodeMajorPattern = regexp.MustCompile(`(?i)^(?:opencode )?v?(\d+)\.`)

// openCode2CpltFloor is the oldest cplt nav-pilot runs opencode 2 under, on
// macOS and Linux alike: release 2026.10.22-172931-b37a8ab. Earlier floors
// brought navikt/cplt#716 (the session's service spawns inside the sandbox,
// not the host's background service with no hooks, gate or policy), #720,
// #722, #735 (fail closed on an unreadable opencode version), #736 (no
// credential leak through planted .agents, .claude or .opencode symlinks) and
// #740 (opencode 2 under bubblewrap on Linux). This floor adds #795 (v2
// favourites and history), #799 (git plumbing for opencode's snapshots) and
// #798 (v2 plugin and MCP grants). Raised 2026-10-22 in #1907.
const openCode2CpltFloor = "2026.10.22-172931"

// openCode2CpltRecommended is the cplt nav-pilot nudges opencode 2 users
// towards without requiring it: navikt/cplt#807, the host-side auth proxy
// GitHub MCP needs in opencode, and navikt/cplt#817, --deny-env for the voice
// keys (withDenyVoiceEnv). Owner decision 2026-10-23: nudge, don't require.
const openCode2CpltRecommended = "2026.10.23-070444"

// errCpltTooOld marks checkOpenCode2Launch's refusal of a cplt older than
// minOpenCode2CpltStamp, so telemetry can tell it from argument refusals.
var errCpltTooOld = errors.New("cplt too old")

// hostOS is runtime.GOOS; tests set it.
var hostOS = runtime.GOOS

// minOpenCode2CpltStamp is the cplt floor for opencode 2 on hostOS, "" where
// cplt does not run it. On Linux cplt runs opencode 2 under bubblewrap and
// refuses a host without it, with install instructions.
func minOpenCode2CpltStamp() string {
	if hostOS == "darwin" || hostOS == "linux" {
		return openCode2CpltFloor
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
	out, _ := cachedVersion("opencode", clientProbeTimeout)
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
// platform without a floor (an unreadable opencode version there), the
// floor stands in so cplt's fail-closed refusal (#735) is still present.
func checkOpenCode2Cplt() error {
	floor := minOpenCode2CpltStamp()
	if floor == "" {
		floor = openCode2CpltFloor
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
		return fmt.Errorf("%w: opencode 2 needs cplt %s or newer (favourites and history, git snapshots, plugin and MCP grants), found %q.\n\n  Update cplt: %s",
			errCpltTooOld, floor, found, domain.Bold(cpltUpgradeHint()))
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

// OpenCode2InstallVersion is the opencode 2 release nav-pilot is tested with.
const OpenCode2InstallVersion = "2.1.3"

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
	npm := "npm i -g @opencode/cli@" + OpenCode2InstallVersion
	switch {
	case strings.Contains(resolved, "node_modules"):
		// opencode-ai owns the bin link; installing over it fails with EEXIST.
		return "npm uninstall -g opencode-ai && " + npm
	case strings.Contains(resolved, "/.opencode/bin/"):
		// The install script's binary would shadow npm's on PATH.
		return "rm ~/.opencode/bin/opencode && " + npm
	}
	return npm
}
