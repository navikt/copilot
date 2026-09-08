package cli

import (
	"errors"
	"fmt"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// Composition: an agentpakke that reuses another one.
//
// There is no `extends` field. A pakke that reuses another is a repo that ships
// a manifest *and* commits the same declaration a consumer repo commits. That
// declaration already says which pakke, at which revision, and optionally which
// items. [agentpakke.Declaration]'s own doc says a repo can hold both, and this
// is the case it was talking about.
//
// The collision rule is that the nearest wins, and it wins by being asked
// first: composeResolver hangs the reused pakke *behind* this source, so a
// pakke that ships its own agent named "grillmester" shadows the reused one.
// That mirrors `overrides` in .github/copilot-sync.json one level up: what a
// team keeps for itself, they keep.
//
// Binding time follows the manifest's shape, not a separate mechanism: a layout
// pakke resolves its declaration here, on every install and sync, while a
// payload pakke resolved it at build time and ships the result. That is why
// this is wired into the Tier 1 path only.

// composeResolver returns the resolver to install from. When this source
// reuses another agentpakke, the returned resolver falls back to it, and the
// second return value is the reused source so callers can report it.
//
// A source that reuses nothing, which is nearly all of them, gets its own
// resolver back unchanged and a nil base.
func composeResolver(resolver *SourceResolver, src *Source) (*SourceResolver, *Source, error) {
	return composeResolverSeen(resolver, src, map[string]bool{sourceLabelFor(src): true})
}

// composeResolverSeen carries the labels already on the chain. A pakke that
// reuses one that reuses it back would otherwise fetch the two forever; the
// cycle is reported rather than silently cut, because either half of it is a
// mistake someone has to fix in a committed file.
func composeResolverSeen(resolver *SourceResolver, src *Source, seen map[string]bool) (*SourceResolver, *Source, error) {
	decl, err := agentpakke.LoadDeclaration(src.Dir)
	if err != nil {
		if errors.Is(err, agentpakke.ErrNoDeclaration) {
			return resolver, nil, nil
		}
		return nil, nil, fmt.Errorf("reading the reuse declaration of %s: %w", sourceLabelFor(src), err)
	}
	if decl.Source == "" {
		return resolver, nil, nil
	}
	// A pakke whose declaration names itself is a consumer repo that happens to
	// ship a manifest, not a composition. Chaining it to itself would resolve
	// every artifact twice and, at a different revision, shadow the checkout
	// being installed with an older copy of itself.
	if decl.Source == sourceLabelFor(src) {
		return resolver, nil, nil
	}

	if seen[decl.Source] {
		return nil, nil, fmt.Errorf("agentpakke %s reuses %q, which already reuses it back.\nA reuse cycle has no order to resolve in, so nothing was installed.\nBreak the cycle in %s in one of the two repos",
			sourceLabelFor(src), decl.Source, agentpakke.DeclarationPath)
	}
	seen[decl.Source] = true

	// A repo-shaped reuse without a pin would clone whatever main happens to
	// hold, so two installs a week apart would compose different content while
	// both reported the same declaration. A local path has no revision to pin
	// and is a working tree by definition, so only the repo form is refused.
	if pinnable(decl.Source) && decl.SHA == "" {
		return nil, nil, fmt.Errorf("agentpakke %s reuses %q without a revision.\nReuse resolves at install and sync, so an unpinned base would compose whatever that repo's main branch holds at the time.\nNothing was installed. Add the 40-character sha of the revision to reuse to %s and commit it",
			sourceLabelFor(src), decl.Source, agentpakke.DeclarationPath)
	}

	baseSrc, err := source.ResolveSource(decl.SHA, decl.Source, "")
	if err != nil {
		return nil, nil, fmt.Errorf("%s reuses agentpakke %q, which could not be resolved at %s: %w\n\nNothing was installed",
			sourceLabelFor(src), decl.Source, shortSHA(decl.SHA), err)
	}
	if err := attachPakke(baseSrc); err != nil {
		return nil, nil, err
	}
	// A reused pakke may itself reuse one. The chain is built depth-first, so
	// by the time this source's resolver is wrapped, everything behind it is
	// already in place.
	baseResolver, _, err := composeResolverSeen(resolverFor(baseSrc.Dir, baseSrc.Pakke), baseSrc, seen)
	if err != nil {
		return nil, nil, err
	}
	return resolver.WithBase(baseResolver), baseSrc, nil
}
