package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// `nav-pilot pakke` and `nav-pilot pakke access`: one place to see the
// agentpakke a project uses and to change an answer given at first run
// (#1962). Nothing here keeps state of its own. It reads the install's state
// file, the source that state names and the consent record, and an answer goes
// through the same path first run takes.

const (
	accessGranted  = "granted"
	accessDeclined = "declined"
	accessNotAsked = "not asked"
	// accessByCplt is hosts the user's own cplt config already allows, which
	// is the access whatever the record says (#1319).
	accessByCplt = "granted (cplt config)"
)

func accessHeld(state string) bool { return state == accessGranted || state == accessByCplt }

// pakkeScope is the scope the commands speak for: the one the flags named, or
// else the repository, and the user scope when the repository has no install.
func pakkeScope(scope *InstallScope, scopeProvided bool) (*InstallScope, *StateFile, error) {
	state, err := readScopedState(scope)
	if err != nil || state != nil || scopeProvided || scope.IsUser() {
		return scope, state, err
	}
	user, err := ScopeUser()
	if err != nil {
		return scope, nil, nil //nolint:nilerr // no user home: the repo answer stands
	}
	userState, err := readScopedState(user)
	if err != nil || userState == nil {
		return scope, nil, err
	}
	return user, userState, nil
}

// installedSource resolves the source an installed scope came from, at the
// revision it is pinned to: what the state recorded, which is what is
// installed, and the repo's lock file only when the state names no source. The
// lock can say something else: a self-install keeps the author's reused base
// there, and a pulled lock is not installed until the next install or sync.
func installedSource(scope *InstallScope, state *StateFile) (*Source, error) {
	repo, ref := state.SourceRepo, state.SourceSHA
	if repo == "" {
		declRepo, declRef, err := declaredPin(scope, "", "")
		if err != nil {
			return nil, err
		}
		repo, ref = declRepo, declRef
	}
	if repo == "" {
		repo = scopeSourceRepo(state)
	}
	// A full sha names one revision, read from the pinned cache: offline and
	// fast after the first fetch. Anything else resolves as list does.
	if len(ref) == 40 && !filepath.IsAbs(repo) {
		return resolvePinned(ref, repo)
	}
	fmt.Fprintln(os.Stderr, dim("Resolving source..."))
	return resolveSource(ref, repo)
}

// proposalState is what the source's agentpakke asks for, in plain words, and
// this scope's answer. access is "" when it asks for nothing.
func proposalState(scope *InstallScope, src *Source) (access, state string, err error) {
	if src == nil || src.Pakke == nil {
		return "", "", nil
	}
	proposal := src.Pakke.CpltProposal()
	if proposal == nil {
		return "", "", nil
	}
	hosts, reads := proposal.AllowPrivateDomains(), proposal.AllowRead()
	if len(hosts) == 0 && len(reads) == 0 {
		return "", "", nil
	}
	access = proposalAccess(hosts, reads)
	covered := len(reads) == 0 && privateDomainsCover(cpltPrivateDomains(), hosts)
	prior, err := artifacts.ReadProposalConsent(scope, src.Pakke.Name)
	switch {
	case err != nil:
		return access, "", err
	case (prior == nil || prior.Hash != proposal.Hash()) && covered:
		return access, accessByCplt, nil
	case prior == nil || prior.Hash != proposal.Hash():
		// A changed block voids the answer: it is asked again at the next
		// install or sync, so for now it has none.
		return access, accessNotAsked, nil
	case prior.Approved:
		// Approved is not applied: the launch drops a record it cannot trust,
		// and so does doctor. Not held, so Next points somewhere.
		if reason := providerpkg.WaiverBlockedReason(prior); reason != "" {
			return access, accessGranted + ", not applied: " + reason, nil
		}
		return access, accessGranted, nil
	case covered:
		return access, accessByCplt, nil
	default:
		return access, accessDeclined, nil
	}
}

// installedContents narrows what the source ships to what this scope has on
// disk: an item counts when a tracked, not-ignored file sits at its place in
// the layout and is still there. A pin whose revision is on disk tracks no
// files, and has everything its revision ships.
func installedContents(scope *InstallScope, m *Manifest, state *StateFile) *Manifest {
	if len(state.Files) == 0 && pinnedRevisionOnDisk(state) {
		return m
	}
	var paths []string
	for _, f := range state.Files {
		if f.Status != fileStatusIgnored && onDisk(scope, f) {
			paths = append(paths, "/"+filepath.ToSlash(f.Path))
		}
	}
	keep := func(kind *ArtifactKind, names []string) []string {
		var out []string
		for _, name := range names {
			at := "/" + kind.Dir + "/" + name
			for _, p := range paths {
				// The file form ends in the kind's own suffix, so "review" is
				// not "review.extra"; the directory form is the name and a slash.
				file := kind.Suffix != "" && strings.HasSuffix(p, at+kind.Suffix)
				dir := (kind.IsDir || kind.CanBeDir) && strings.Contains(p, at+"/")
				if file || dir {
					out = append(out, name)
					break
				}
			}
		}
		return out
	}
	return &Manifest{
		Name:         m.Name,
		Agents:       keep(KindAgent, m.Agents),
		Skills:       keep(KindSkill, m.Skills),
		Instructions: keep(KindInstruction, m.Instructions),
		Prompts:      keep(KindPrompt, m.Prompts),
		Hooks:        keep(KindHook, m.Hooks),
		Extensions:   keep(source.KindExtension, m.Extensions),
	}
}

func cmdPakkeStatus(scope *InstallScope, scopeProvided bool) error {
	scope, state, err := pakkeScope(scope, scopeProvided)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		fmt.Println("No agentpakke is installed here.")
		fmt.Printf("Next: %s\n", bold(installCommandFor(nil, nil)))
		return nil
	}
	src, err := installedSource(scope, state)
	if err != nil {
		return err
	}
	defer src.Cleanup()
	resolver, bases, err := composedResolverFor(src, "")
	if err != nil {
		return err
	}
	defer bases.cleanup()

	fmt.Printf("%s (%s)\n", bold("agentpakke "+state.Collection), scope.Name)
	pin := pinLabel(state)
	if state.PakkeVersion != "" {
		pin += " (" + state.PakkeVersion + ")"
	}
	fmt.Printf("  %-16s %s\n", "Source:", pin)

	hooksMissing := false
	if src.Pakke != nil {
		shipped, err := pakkeContents(resolver, src)
		if err != nil {
			return err
		}
		installed := installedContents(scope, shipped, state)
		hooks := installed.Hooks
		installed.Hooks = nil
		printManifestContents(installed)
		switch {
		case len(shipped.Hooks) == 0:
		case len(hooks) == len(shipped.Hooks):
			fmt.Printf("  %-16s installed (%d)\n", "Hooks:", len(hooks))
		default:
			fmt.Printf("  %-16s %d of %d not installed\n", "Hooks:", len(shipped.Hooks)-len(hooks), len(shipped.Hooks))
			hooksMissing = true
		}
	}

	access, answer, err := proposalState(scope, src)
	if err != nil {
		fmt.Printf("  %-16s %s %s: could not read the answer: %v\n", "Access:", yellow("⚠"), access, err)
	} else if access != "" {
		fmt.Printf("  %-16s %s — %s\n", "Access:", access, answer)
	}
	open := err == nil && access != "" && !accessHeld(answer)
	next := ""
	switch {
	case open && !cpltInstalled():
		// The question is about cplt's sandbox; without cplt it is not asked.
		next = cpltInstallHint
	case open && hooksMissing:
		next = "nav-pilot pakke access, then nav-pilot install"
	case open:
		next = "nav-pilot pakke access"
	case hooksMissing:
		next = "nav-pilot install"
	}
	if next != "" {
		next = strings.ReplaceAll(next, "nav-pilot pakke access", "nav-pilot pakke access"+scopeArgs(scope))
		next = strings.ReplaceAll(next, "nav-pilot install", "nav-pilot install"+scopeArgs(scope))
		fmt.Printf("\nNext: %s\n", bold(next))
	}
	return nil
}

func cmdPakkeAccess(scope *InstallScope, scopeProvided bool) error {
	scope, state, err := pakkeScope(scope, scopeProvided)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("no agentpakke is installed here. Run %s first", installCommandFor(nil, nil))
	}
	src, err := installedSource(scope, state)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	access, answer, err := proposalState(scope, src)
	if err != nil {
		return err
	}
	if access == "" {
		fmt.Println("The agentpakke asks for no extra access.")
		return nil
	}
	if accessHeld(answer) {
		fmt.Println("The agentpakke already has the access it asks for.")
		return nil
	}
	if reason, ok := strings.CutPrefix(answer, accessGranted+", not applied: "); ok {
		return fmt.Errorf("the access is granted, but a launch does not apply it: %s", reason)
	}
	if !cpltInstalled() {
		return fmt.Errorf("cplt is not installed, and the access only applies inside its sandbox. Install it: %s", cpltInstallHint)
	}
	if !isInteractive() {
		return fmt.Errorf("nav-pilot pakke access needs a terminal to ask in. It would ask: Let the agent %s?", access)
	}
	return putProposal(scope, src, false, false, true)
}

// scopeArgs is the flag that sends a suggested command to the scope this one
// read: --user, --target for a repository other than the one the user stands
// in, and nothing for that one.
func scopeArgs(scope *InstallScope) string {
	if scope.IsUser() {
		return " --user"
	}
	if wd, err := os.Getwd(); err == nil {
		if root := findGitRoot(wd); root != "" && ScopeRepo(root).RootDir == scope.RootDir {
			return ""
		}
	}
	return " --target " + shellQuote(scope.RootDir)
}
