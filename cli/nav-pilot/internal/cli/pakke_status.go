package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
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
)

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
// revision it is pinned to: the repo's lock file when it has one, else what the
// state recorded.
func installedSource(scope *InstallScope, state *StateFile) (*Source, error) {
	repo, ref, err := declaredPin(scope, "", "")
	if err != nil {
		return nil, err
	}
	if repo == "" {
		repo, ref = state.SourceRepo, state.SourceSHA
	}
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
	prior, err := artifacts.ReadProposalConsent(scope, src.Pakke.Name)
	switch {
	case err != nil:
		return access, "", err
	case prior == nil || prior.Hash != proposal.Hash():
		// A changed block voids the answer: it is asked again at the next
		// install or sync, so for now it has none.
		return access, accessNotAsked, nil
	case prior.Approved:
		return access, accessGranted, nil
	default:
		return access, accessDeclined, nil
	}
}

// installedContents narrows what the source ships to what this scope has on
// disk: an item counts when a tracked, not-ignored file sits at its place in
// the layout. A pin tracks no files, and has everything its revision ships.
func installedContents(m *Manifest, state *StateFile) *Manifest {
	if len(state.Files) == 0 && pinnedState(state) {
		return m
	}
	var paths []string
	for _, f := range state.Files {
		if f.Status != fileStatusIgnored {
			paths = append(paths, "/"+filepath.ToSlash(f.Path))
		}
	}
	keep := func(kind *ArtifactKind, names []string) []string {
		var out []string
		for _, name := range names {
			at := "/" + kind.Dir + "/" + name
			for _, p := range paths {
				if strings.Contains(p, at+"/") || strings.Contains(p, at+".") {
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
	fmt.Fprintln(os.Stderr, dim("Resolving source..."))
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

	next := ""
	if src.Pakke != nil {
		shipped, err := pakkeContents(resolver, src)
		if err != nil {
			return err
		}
		installed := installedContents(shipped, state)
		hooks := installed.Hooks
		installed.Hooks = nil
		printManifestContents(installed)
		switch {
		case len(shipped.Hooks) == 0:
		case len(hooks) == len(shipped.Hooks):
			fmt.Printf("  %-16s installed (%d)\n", "Hooks:", len(hooks))
		default:
			fmt.Printf("  %-16s %d of %d not installed\n", "Hooks:", len(shipped.Hooks)-len(hooks), len(shipped.Hooks))
			next = "nav-pilot install"
		}
	}

	access, answer, err := proposalState(scope, src)
	if err != nil {
		fmt.Printf("  %-16s %s %s: could not read the answer: %v\n", "Access:", yellow("⚠"), access, err)
	} else if access != "" {
		fmt.Printf("  %-16s %s: %s\n", "Access:", access, answer)
		if answer != accessGranted {
			// Access first: it is the one a hook-less install cannot fix.
			next = "nav-pilot pakke access"
		}
	}
	if next != "" {
		fmt.Printf("\nNext: %s\n", bold(next))
	}
	return nil
}

// errNoTerminal is pakke access without a terminal: it printed what it would
// ask, and answering is for a person.
var errNoTerminal = errors.New("nav-pilot pakke access asks in a terminal, and there is none here")

func cmdPakkeAccess(scope *InstallScope, scopeProvided bool) error {
	scope, state, err := pakkeScope(scope, scopeProvided)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("no agentpakke is installed here. Run %s first", installCommandFor(nil, nil))
	}
	fmt.Fprintln(os.Stderr, dim("Resolving source..."))
	src, err := installedSource(scope, state)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	access, answer, err := proposalState(scope, src)
	if err != nil {
		return err
	}
	if access == "" || answer == accessGranted {
		fmt.Println("All access the agentpakke proposes is granted.")
		return nil
	}
	if !cpltInstalled() {
		return fmt.Errorf("the access applies inside the cplt sandbox, and cplt is not installed. Install it: brew install navikt/tap/cplt")
	}
	if !isInteractive() {
		fmt.Printf("Would ask: Let the agent %s? (%s)\n", access, answer)
		return errNoTerminal
	}
	putProposal(scope, src, false, false, true)
	return nil
}
