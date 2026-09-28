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
	docs := openCodeConfigDocs(projectDir, env)
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
func userPermission(docs [][]byte, key string) (str string, deny bool) {
	isDeny := func(raw json.RawMessage) bool {
		var s string
		return json.Unmarshal(raw, &s) == nil && s == "deny"
	}
	for _, doc := range docs {
		var cfg struct {
			Permission json.RawMessage `json:"permission"`
		}
		if json.Unmarshal(stripJSONC(doc), &cfg) != nil || cfg.Permission == nil {
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
	return str, deny
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
