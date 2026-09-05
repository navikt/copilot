package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// activateHook writes the config entry that makes an installed hook script
// actually run. The script alone is inert: the CLI runs it only when a
// preToolUse entry points at it.
//
// The two scopes need different things, and both shapes are read off working
// examples rather than chosen — see the note at the top of
// internal/source/hooks.go.
//
//   - repo: one entry merged into the shared .github/hooks/copilot-hooks.json,
//     which the user may already have their own entries in. Nothing nav-pilot
//     did not mark is added to, updated, or removed.
//   - user: one file of its own at ~/.copilot/hooks/<name>.json. Nothing to
//     merge, and the file is tracked in state so uninstall takes it away with
//     the script.
//
// The command a repo entry runs is repo-relative (the CLI runs it from the
// workspace root, which is the shape the entry measured in #557 already has). A
// user entry has no such root, so its command carries the absolute path to the
// installed script.
func activateHook(scope *InstallScope, art Resolved, result *installResult) error {
	meta := source.LoadHookMeta(art.AbsPath)
	fileName := art.Name + KindHook.Suffix

	scriptPath := scope.RelPath(KindHook.Dir, fileName)
	if scope.IsUser() {
		scriptPath = scope.DstPath(KindHook.Dir, fileName)
	}

	entry := source.HookEntry{
		Name:    art.Name,
		Matcher: meta.Matcher,
		Command: source.HookCommand(filepath.ToSlash(scriptPath)),
		Timeout: meta.TimeoutSec,
	}

	hooksDir := scope.DstPath(KindHook.Dir)
	if !scope.IsUser() {
		return explainHookWrite(source.MergeRepoHooks(hooksDir, []source.HookEntry{entry}), hooksDir)
	}

	if err := explainHookWrite(source.WriteUserHook(hooksDir, entry), hooksDir); err != nil {
		return err
	}
	configRel := scope.RelPath(KindHook.Dir, source.UserHookConfigName(art.Name))
	hash, err := rawArtifactHash(filepath.Join(scope.RootDir, configRel), false)
	if err != nil {
		return err
	}
	result.Files = append(result.Files, InstalledFile{Path: configRel, Hash: hash})
	return nil
}

// deactivateRepoHooks strips nav-pilot's entries out of the shared repo hooks
// config on uninstall. The config is not a tracked file — it is shared, and
// deleting it would take the user's own hooks with it — so the ordinary file
// loop cannot do this, and something has to.
func deactivateRepoHooks(scope *InstallScope, dryRun bool) int {
	if scope.IsUser() {
		return 0 // user-scope hook configs are tracked files; the file loop has them
	}
	hooksDir := scope.DstPath(KindHook.Dir)
	path := filepath.Join(hooksDir, source.RepoHooksConfig)
	names := source.HookNamesIn(path)
	if len(names) == 0 {
		return 0
	}
	if dryRun {
		for _, name := range names {
			fmt.Printf("  %s %s (hook entry in %s)\n", dim("×"), name, source.RepoHooksConfig)
		}
		return len(names)
	}
	removed, err := source.RemoveRepoHooks(hooksDir)
	if err != nil {
		fmt.Printf("  %s Could not update %s: %v\n", yellow("⚠"), path, err)
		return 0
	}
	for _, name := range names {
		fmt.Printf("  %s %s (hook entry in %s)\n", red("×"), name, source.RepoHooksConfig)
	}
	return removed
}

// reportHooks is the `doctor` section for the fifth artifact kind. A hook is
// executable code the CLI runs on every matching tool call, so what it says is
// which scripts are installed and where — and, once any are, the one condition
// under which they silently do not load.
func reportHooks(repoDir string, userScope *InstallScope) {
	found := 0
	if repoDir != "" {
		path := filepath.Join(ScopeRepo(repoDir).DstPath(KindHook.Dir), source.RepoHooksConfig)
		names := source.HookNamesIn(path)
		found += len(names)
		if len(names) > 0 {
			fmt.Printf("    • Repo scope (%s): %s %s\n", path, green("✓"), strings.Join(names, ", "))
		} else {
			fmt.Printf("    • Repo scope (.github/hooks): none installed\n")
		}
	}
	if userScope != nil {
		var names []string
		dir := userScope.DstPath(KindHook.Dir)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			names = append(names, source.HookNamesIn(filepath.Join(dir, e.Name()))...)
		}
		sort.Strings(names)
		found += len(names)
		if len(names) > 0 {
			fmt.Printf("    • User scope (%s): %s %s\n", dir, green("✓"), strings.Join(names, ", "))
		} else {
			fmt.Printf("    • User scope (~/.copilot/hooks): none installed\n")
		}
	}
	if found == 0 {
		return
	}
	fmt.Printf("      %s Hooks run code on every matching tool call. Read them before you trust them.\n", dim("Note:"))
	fmt.Printf("      %s In prompt mode (%s) an untrusted folder does not load repo hooks.\n", yellow("Note:"), bold("copilot -p"))
	fmt.Printf("          Trust the folder, or set %s for that run.\n", bold("GITHUB_COPILOT_PROMPT_MODE_REPO_HOOKS=true"))
	fmt.Printf("          The variable is in the CLI's changelog but not its public docs, so verify it still works before relying on it.\n")
}

// ─── hook writes inside a cplt sandbox ───────────────────────────────────────

// cpltSandboxEnvVar is the marker cplt exports into every process it sandboxes.
// It is cplt's own recursion guard — set once in `sandbox_exec.rs` on both the
// macOS Seatbelt and the Linux Landlock path, so it is present whatever backend
// is in use — and cplt documents it as the way a process tells it is inside:
// its README says cplt "refuses to nest… via the `__CPLT_WRAPPED` environment
// variable", its agent-facing brief tells the agent "you are running under cplt
// if `$__CPLT_WRAPPED` is set", and its own Gradle init script keys on it.
//
// There is no better signal. cplt sets no positive "I am cplt" variable of its
// own, and the alternatives are worse: the proxy variables it injects are also
// set by any corporate proxy, and probing the filesystem for a denial is a
// side effect, not a check.
const cpltSandboxEnvVar = "__CPLT_WRAPPED"

// insideCpltSandbox reports whether this process is running inside a cplt
// sandbox.
func insideCpltSandbox() bool {
	_, ok := os.LookupEnv(cpltSandboxEnvVar)
	return ok
}

// explainHookWrite turns the bare permission error a hook write gets inside a
// cplt sandbox into one that names the tool doing the denying.
//
// cplt protects exactly the paths nav-pilot's hooks land in, and for a reason
// that is not a bug: a hook file is code that runs later, unsandboxed, on the
// host. `~/.copilot/hooks` and `~/.copilot/settings.json` are in cplt's
// host-persistence deny list for the Copilot agent (cplt src/agent.rs, #331),
// and `.github/hooks` is re-bound read-only in the project directory (cplt
// src/sandbox_policy.rs `PROTECTED_IN_ROOT`, #347). Neither is going to change,
// and nav-pilot should not try to route around a deliberate guard rail.
//
// What it can do is stop the failure reading as a broken install. Left alone
// the user sees an EPERM naming a path and neither tool, on a command that
// works perfectly well one shell out.
//
// The net is permission-denied and read-only-filesystem: macOS Seatbelt returns
// EPERM for the deny, Linux returns EROFS for the read-only bind. A disk-full
// or missing-directory error is a real error and is passed through untouched,
// because claiming cplt denied something it did not is its own bug.
func explainHookWrite(err error, path string) error {
	if err == nil || !insideCpltSandbox() {
		return err
	}
	if !errors.Is(err, fs.ErrPermission) && !errors.Is(err, syscall.EROFS) {
		return err
	}
	return fmt.Errorf("%w\n\n"+
		"    This looks like cplt: nav-pilot is running inside a cplt sandbox (%s is set),\n"+
		"    and cplt denies writes to Copilot hook paths — %s here. A hook file is code\n"+
		"    that runs later on the host, outside the sandbox, so cplt refuses to let a\n"+
		"    sandboxed process plant one. That is deliberate, and nav-pilot will not work\n"+
		"    around it.\n\n"+
		"    Run the install from a shell outside cplt instead",
		err, cpltSandboxEnvVar, path)
}
