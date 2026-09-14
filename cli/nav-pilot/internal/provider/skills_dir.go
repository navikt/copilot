package provider

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// NAV_PILOT_SKILLS_DIR (#858, step 1).
//
// Skills materialize per client, in a different place each time: a Tier 1
// copilot install writes ~/.copilot/skills/<name>/, a Tier 1 opencode install
// writes <opencode config dir>/skills/<name>/, pi is handed
// ~/.nav-pilot/pi/skills by flag, and a staged Tier 2 launch reads
// <payload>/skills/<name>/. The whole skill directory is copied, scripts
// included, so a skill that ships one is only a path away from working — but
// the path differs per client, and a skill that writes one client's path in its
// text is wrong on the others. That is what nais/pilot's observability skill
// does today, and its author had no way to write it once.
//
// So nav-pilot names the directory at launch instead, and the skill writes
//
//	bash "$NAV_PILOT_SKILLS_DIR/nais-observability/mimir-query.sh" <tenant> <promql>
//
// which is right on whichever client nav-pilot actually materialized it for.

// SkillsDirEnv is the variable every launch path exports when it materialized
// skills for the client it is launching.
const SkillsDirEnv = "NAV_PILOT_SKILLS_DIR"

// materializedSkillsDir returns root/skills, or "" when nothing was
// materialized there.
//
// "" is the honest answer and the reason this is one function rather than a
// filepath.Join at six call sites: a variable a skill can test for is something
// a skill can branch on, while a path to a directory that does not exist reads
// as a working install right up to the point the script fails to open.
func materializedSkillsDir(root string) string {
	if root == "" {
		return ""
	}
	dir := filepath.Join(root, "skills")
	if !dirHasEntries(dir) {
		return ""
	}
	return dir
}

// copilotSkillsRoot is where a Tier 1 copilot install puts its content:
// user scope is ~/.copilot, and skills land directly under it
// ([domain.ScopeUser]). Repo-scope installs write .github/skills/ in the
// project instead, which copilot finds on its own and which is not one fixed
// path a launch can name.
func copilotSkillsRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".copilot")
}

// withSkillsDirEnv exports the skills root on a launch environment. An empty
// dir leaves the environment alone, so nothing is exported where nothing was
// materialized.
//
// A nil env means "inherit the parent's", which has to be materialized before
// a value can be added to it — otherwise setting the variable would silently
// drop every other variable the session needs.
func withSkillsDirEnv(env []string, dir string) []string {
	if dir == "" {
		return env
	}
	if env == nil {
		env = os.Environ()
	}
	env, _ = telemetry.SetEnvValue(env, SkillsDirEnv, dir)
	return env
}

// insertCpltPassEnv splices --pass-env into a cplt vector whose "--" separator
// is already in place.
//
// The legacy copilot launch builds its own argument vector (BuildCopilotArgs)
// rather than going through cpltArgv, so it is a second seam and needs the flag
// inserted rather than appended: after "--" it would reach the copilot binary,
// which has no such flag. A vector with no separator is left alone — that is
// the plain-copilot spelling, where there is no cplt to pass anything through.
func insertCpltPassEnv(args []string, key string) []string {
	i := slices.Index(args, "--")
	if i < 0 {
		return args
	}
	return slices.Insert(slices.Clone(args), i, "--pass-env", key)
}
