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
		Command: source.HookCommand(filepath.ToSlash(scriptPath), meta.TimeoutSec),
		Timeout: meta.TimeoutSec,
	}

	hooksDir := scope.DstPath(KindHook.Dir)
	if !scope.IsUser() {
		return explainSandboxedWrite(source.MergeRepoHooks(hooksDir, []source.HookEntry{entry}), KindHook, hooksDir)
	}

	if err := explainSandboxedWrite(source.WriteUserHook(hooksDir, entry), KindHook, hooksDir); err != nil {
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

// hookOfPath names the hook a tracked path belongs to: its script, in either
// scope, or its generated ~/.copilot/hooks/<name>.json entry in user scope
// (config true). Empty when the path is no hook's.
//
// Sync asks the source about a hook by this name and never by the path. A
// path lookup falls back to .github/hooks/, where the source repo keeps hooks
// of its own, so a hook the agentpakke had dropped still resolved and kept
// running in every repo that installed it (#982).
func hookOfPath(scope *InstallScope, localPath string) (name string, config bool) {
	dir, file := filepath.Split(filepath.ToSlash(localPath))
	if dir != filepath.ToSlash(scope.RelPath(KindHook.Dir))+"/" {
		return "", false
	}
	if name, ok := strings.CutSuffix(file, KindHook.Suffix); ok {
		return name, false
	}
	if name, ok := strings.CutSuffix(file, ".json"); ok && scope.IsUser() {
		return name, true
	}
	return "", false
}

// refreshHookRegistration rewrites what makes a hook the source still ships
// run: its entry in the repo's copilot-hooks.json, or its own
// ~/.copilot/hooks/<name>.json, whose new hash is recorded so uninstall still
// knows it as nav-pilot's. Install goes through [activateHook] as well, so a
// new matcher or timeout reaches both scopes the same way.
func refreshHookRegistration(scope *InstallScope, art Resolved) error {
	var res installResult
	if err := activateHook(scope, art, &res); err != nil {
		return err
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil || len(res.Files) == 0 {
		return err
	}
	for i := range state.Files {
		if state.Files[i].Path == res.Files[0].Path && state.Files[i].Hash != res.Files[0].Hash {
			state.Files[i].Hash = res.Files[0].Hash
			return writeScopedState(scope, state)
		}
	}
	return nil
}

// deactivateRepoHooks strips nav-pilot's entries out of the shared repo hooks
// config on uninstall. The config is not a tracked file — it is shared, and
// deleting it would take the user's own hooks with it — so the ordinary file
// loop cannot do this, and something has to.
func deactivateRepoHooks(scope *InstallScope, dryRun, quiet bool) int {
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
			if !quiet {
				fmt.Printf("  %s %s (hook entry in %s)\n", dim("×"), name, source.RepoHooksConfig)
			}
		}
		return len(names)
	}
	removed, err := source.RemoveRepoHooks(hooksDir)
	if err != nil {
		fmt.Printf("  %s Could not update %s: %v\n", yellow("⚠"), path, err)
		return 0
	}
	for _, name := range names {
		if !quiet {
			fmt.Printf("  %s %s (hook entry in %s)\n", red("×"), name, source.RepoHooksConfig)
		}
	}
	return removed
}

// promptModeRepoHooksEnv is the Copilot CLI's opt-in for loading repo hooks in
// prompt mode without trusting the folder. It is in the CLI's changelog and not
// in its public documentation, so nav-pilot names it and never sets it.
const promptModeRepoHooksEnv = "GITHUB_COPILOT_PROMPT_MODE_REPO_HOOKS"

// repoHooksCanFire reports whether repo-scope hooks in repoDir load at all, and
// says which of the three conditions is carrying them.
//
// The conditions are the CLI's, measured in #888 and written down in
// [source.FolderTrusted]: a trusted folder, or either env var. The env vars are
// read here rather than there because they are this process's environment, not
// a fact about the folder — and because a user who exports one in their profile
// has it in nav-pilot's environment too, which is what makes reading them worth
// anything.
func repoHooksCanFire(repoDir string) (reason string, ok bool) {
	if os.Getenv("COPILOT_ALLOW_ALL") == "true" {
		return "COPILOT_ALLOW_ALL=true", true
	}
	if os.Getenv(promptModeRepoHooksEnv) == "true" {
		return promptModeRepoHooksEnv + "=true", true
	}
	userScope, err := ScopeUser()
	if err != nil {
		return "", false
	}
	if source.FolderTrusted(userScope.RootDir, repoDir) {
		return "the folder is trusted", true
	}
	return "", false
}

// warnRepoHooksNeedTrust says, at the moment of installing, that the hook
// entries just written to .github/hooks/ will not load here.
//
// It belongs at install time and not only in `doctor` because the belief a hook
// forms is formed here: the file is written, committed and visible, and #888 is
// about it being inert all the same. Saying it afterwards means saying it to
// someone who has already told their team the gate is on.
//
// Nothing is printed when the hooks do load, and nothing when the scope has no
// repo hook entries at all.
func warnRepoHooksNeedTrust(scope *InstallScope) {
	if scope == nil || scope.IsUser() {
		return
	}
	path := filepath.Join(scope.DstPath(KindHook.Dir), source.RepoHooksConfig)
	if len(source.HookNamesIn(path)) == 0 {
		return
	}
	if _, ok := repoHooksCanFire(scope.RootDir); ok {
		return
	}
	fmt.Printf("%s The hook entries in %s are installed and inert here.\n",
		yellow("⚠"), source.RepoHooksConfig)
	fmt.Printf("  %s loads repo hooks only in a folder you have trusted, and %s never asks.\n",
		bold("copilot"), bold("copilot -p"))
	fmt.Printf("  Run %s in %s and answer the folder-trust prompt with\n", bold("copilot"), scope.RootDir)
	fmt.Printf("  %s. Everyone else in the repo\n", bold(`"Yes, and remember this folder for future sessions"`))
	fmt.Printf("  has to do the same, each on their own machine. Run %s to check.\n", bold("nav-pilot doctor"))
	fmt.Println()
}

// reportHooks is the `doctor` section for the fifth artifact kind. A hook is
// executable code the CLI runs on every matching tool call, so what it says is
// which scripts are installed and where — and, once any are, whether they can
// fire at all.
func reportHooks(repoDir string, userScope *InstallScope) {
	found, repoFound := 0, 0
	if repoDir != "" {
		path := filepath.Join(ScopeRepo(repoDir).DstPath(KindHook.Dir), source.RepoHooksConfig)
		names := source.HookNamesIn(path)
		found += len(names)
		repoFound = len(names)
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
	if repoFound == 0 {
		return
	}

	// The one thing this section is for. Repo hooks load only in a folder the
	// Copilot CLI trusts, and in prompt mode nothing asks — so an installed,
	// committed, perfectly correct .github/hooks/ entry can be doing nothing at
	// all, and every other line above still says ✓ (#888).
	if reason, ok := repoHooksCanFire(repoDir); ok {
		fmt.Printf("      %s Repo hooks load here (%s).\n", green("✓"), reason)
		return
	}
	fmt.Printf("      %s Inert in %s: %s is not a trusted folder.\n", yellow("⚠"), bold("copilot -p"), repoDir)
	fmt.Printf("          %s Run %s here and answer the folder-trust prompt with\n", yellow("Solution:"), bold("copilot"))
	fmt.Printf("          %s. Plain %s trusts this\n", bold(`"Yes, and remember this folder for future sessions"`), bold(`"Yes"`))
	fmt.Printf("          session only, and leaves the next %s run without the hooks.\n", bold("-p"))
	fmt.Printf("          %s loads them for one run without trusting anything.\n", bold(promptModeRepoHooksEnv+"=true"))
	fmt.Printf("          It is in the CLI's changelog and not in its public docs, so check it still works.\n")
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

// explainSandboxedWrite turns the bare permission error an artifact write gets
// inside a cplt sandbox into one that names the tool doing the denying.
//
// cplt protects exactly the paths nav-pilot installs into, and for a reason
// that is not a bug: what lands there is instruction and code the agent reads
// or runs later, unsandboxed, on the host. `~/.copilot/hooks` and
// `~/.copilot/settings.json` are in cplt's host-persistence deny list for the
// Copilot agent (cplt src/agent.rs, #331), `~/.copilot/skills/` and
// `~/.pi/agent/skills/` joined it in cplt#508, and `.github/hooks` is re-bound
// read-only in the project directory (cplt src/sandbox_policy.rs
// `PROTECTED_IN_ROOT`, #347). None of that is going to change, and nav-pilot
// should not try to route around a deliberate guard rail.
//
// It takes the kind rather than assuming one. Wired to hooks alone, the
// explanation moved the moment cplt denied something else: after cplt#508 an
// install inside a session failed one artifact earlier, on a skill, with a bare
// "permission denied" (#862). The deny list has grown once and will grow again.
//
// What it can do is stop the failure reading as a broken install. Left alone
// the user sees an EPERM naming a path and neither tool, on a command that
// works perfectly well one shell out.
//
// The net is permission-denied and read-only-filesystem: macOS Seatbelt returns
// EPERM for the deny, Linux returns EROFS for the read-only bind. A disk-full
// or missing-directory error is a real error and is passed through untouched,
// because claiming cplt denied something it did not is its own bug.
func explainSandboxedWrite(err error, kind *ArtifactKind, path string) error {
	if err == nil || !insideCpltSandbox() {
		return err
	}
	if !errors.Is(err, fs.ErrPermission) && !errors.Is(err, syscall.EROFS) {
		return err
	}
	return fmt.Errorf("%w\n\n"+
		"    This looks like cplt: nav-pilot is running inside a cplt sandbox (%s is set),\n"+
		"    and cplt denies writes to where nav-pilot puts a %s — %s here. What lands\n"+
		"    there is read or run later on the host, outside the sandbox, so cplt refuses\n"+
		"    to let a sandboxed process plant it. That is deliberate, and nav-pilot will\n"+
		"    not work around it.\n\n"+
		"    Run the install from a shell outside cplt instead",
		err, cpltSandboxEnvVar, kind.Name, path)
}
