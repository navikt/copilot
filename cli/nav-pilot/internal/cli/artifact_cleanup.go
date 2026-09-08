package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// afterArtifactRemoved cleans up what deleting an artifact's own file leaves
// behind. Two kinds are more than the file the tracker names.
//
// A hook is a script plus an entry in a config the CLI reads. Removing only the
// script left the entry pointing at a file that is gone, and Copilot CLI then
// ran a missing path on every matching tool call. A skill is a directory whose
// SKILL.md is what makes it a skill; removing only the marker left a directory
// that is no longer a skill and that nothing will ever clean up.
//
// Called from both removal paths, because there are two and they had drifted:
// sync's deletion branch and the retired-artifact sweep.
func afterArtifactRemoved(scope *InstallScope, absLocal string) {
	if scope == nil {
		return
	}

	hooksDir := scope.DstPath(KindHook.Dir)
	if filepath.Dir(absLocal) == hooksDir && strings.HasSuffix(absLocal, KindHook.Suffix) {
		name := strings.TrimSuffix(filepath.Base(absLocal), KindHook.Suffix)
		if scope.IsUser() {
			// User scope gives each hook its own config file, so the file loop
			// removes it only if it happens to be tracked. Take it here too.
			_ = os.Remove(filepath.Join(hooksDir, source.UserHookConfigName(name)))
			return
		}
		removed, err := source.RemoveRepoHooks(hooksDir, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not update %s: %v\n", yellow("⚠"), source.RepoHooksConfig, err)
			return
		}
		if removed > 0 {
			fmt.Printf("  %s %s (hook entry in %s)\n", red("×"), name, source.RepoHooksConfig)
		}
		return
	}

	skillsDir := scope.DstPath(KindSkill.Dir)
	if filepath.Base(absLocal) == KindSkill.Marker {
		dir := filepath.Dir(absLocal)
		if filepath.Dir(dir) == skillsDir {
			_ = os.RemoveAll(dir)
		}
	}
}
