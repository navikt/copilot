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

// composedResolverFor builds the resolver every path that installs or lists
// content reads through: this source's own layout, with the pakke it reuses
// behind it.
//
// It exists because four entry points built a bare [resolverFor] and delivered
// only the top pakke's content — `install --all`, the interactive picker, a
// single artifact by name, and `list` (#844). Composition was wired into the
// two paths that noticed it and nowhere else, and `list` showing the same
// incomplete set is what made it hard to see: what the user saw matched what
// they got, and both were half a pakke. One place to ask means the next entry
// point does not have to remember.
//
// Composition is Tier 1 only, and the guard is here rather than in each caller:
// a payload-only pakke installs a pinned revision of a digest-bound tree, not
// files at paths, so there is nothing for a base to contribute to. The callers
// that route Tier 2 to [installPakkePin] do it after this call, and `list`
// never routes it anywhere.
func composedResolverFor(src *Source, collection string) (*SourceResolver, composedBases, error) {
	resolver := resolverFor(src.Dir, pakkeFor(src, collection))
	if payloadOnly(src) {
		return resolver, nil, nil
	}
	return composeResolver(resolver, src)
}

// composedBases is every source a composition resolved, nearest first.
//
// Resolving a base clones it into a temp directory, and only the caller knows
// when the install, the sync or the listing has stopped reading from it. So the
// whole chain goes back, and the caller defers cleanup of the set next to the
// `defer src.Cleanup()` it already has for its own source.
//
// Returning only the nearest base left every link behind it without an owner:
// the recursive call dropped its result, so a pakke that reuses a pakke that
// reuses a pakke leaked a checkout per link, on every run of every command that
// composes — `list` included (#867).
type composedBases []*Source

// cleanup removes the temp checkouts the chain created. A nil chain and a
// source that was a local path are both no-ops.
func (b composedBases) cleanup() {
	for _, src := range b {
		src.Cleanup()
	}
}

// nearest is the base this source reuses directly: the one "Reuses:" names and
// the one whose retired record sync merges in.
func (b composedBases) nearest() *Source {
	if len(b) == 0 {
		return nil
	}
	return b[0]
}

// composedContentsFor is what an install path asks for: the composed resolver,
// the pakke it reuses so the caller can report it, and the manifest of what to
// install — narrowed to the items the scope's declaration selects.
//
// The narrowing lives here for the reason the composition does. It hung on
// `install <navn>` alone, so a repo that had committed a three-item selection
// got every artifact from the base through `install --all` and through the
// picker: the committed file said one thing and the install did another (#869).
// One place to ask means the next entry point does not have to remember.
//
// The Tier 2 half of the same question is not here, because a payload-only
// source returns before any of this: [installPakkePin] is where every path
// routes one, and where the selection it cannot honour is refused.
//
// `list` is deliberately left off this path. The unknown-item refusal sends the
// reader to `nav-pilot list --items` to find the name they mistyped, and a
// listing narrowed by that same list could not contain it.
func composedContentsFor(scope *InstallScope, src *Source, collection string) (*SourceResolver, composedBases, *Manifest, error) {
	resolver, bases, err := composedResolverFor(src, collection)
	if err != nil {
		return nil, nil, nil, err
	}
	// Every refusal from here on is one the caller never sees a chain for, so
	// the checkouts are cleaned here instead (#867).
	var manifest *Manifest
	switch {
	case collection == CollectionAll:
		// Not a name a pakke or a collection can have — CollectionAll is
		// "(all)" — so this branch cannot swallow a real collection.
		manifest, err = collectAllItemsWith(resolver)
	case src.Pakke != nil:
		manifest, err = pakkeContents(resolver, src)
	default:
		manifest, err = loadManifest(src.Dir, collection)
	}
	if err != nil {
		bases.cleanup()
		return nil, nil, nil, err
	}
	items, err := declaredItemsFor(scope)
	if err != nil {
		bases.cleanup()
		return nil, nil, nil, err
	}
	if manifest, err = applyDeclaredItems(manifest, items); err != nil {
		bases.cleanup()
		return nil, nil, nil, err
	}
	return resolver, bases, manifest, nil
}

// composeResolver returns the resolver to install from. When this source
// reuses another agentpakke, the returned resolver falls back to it, and the
// second return value is the chain of sources that were resolved: the caller
// reports the nearest one and cleans up the whole set.
//
// A source that reuses nothing, which is nearly all of them, gets its own
// resolver back unchanged and an empty chain.
func composeResolver(resolver *SourceResolver, src *Source) (*SourceResolver, composedBases, error) {
	// Only a manifest-bearing source composes. A collection-era source has no
	// agentpakke identity to reuse from, and one that happens to carry a
	// declaration is a consumer repo that also serves content: composing it
	// would fetch a second repo mid-install for a source that never asked for
	// it.
	if src.Pakke == nil {
		return resolver, nil, nil
	}
	return composeResolverSeen(resolver, src, []string{sourceLabelFor(src)})
}

// composeResolverSeen carries the labels already on the chain. A pakke that
// reuses one that reuses it back would otherwise fetch the two forever; the
// cycle is reported rather than silently cut, because either half of it is a
// mistake someone has to fix in a committed file.
//
// Membership is sameSourceRepo, not string equality: "Navikt/A" and "navikt/a"
// are one repo, and two spellings of the same path are one directory. With
// plain equality a pakke could name itself in a different case and be chained
// to itself, and a cycle went one fetch further before it was noticed.
func composeResolverSeen(resolver *SourceResolver, src *Source, seen []string) (*SourceResolver, composedBases, error) {
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
	if sameSourceRepo(decl.Source, sourceLabelFor(src)) {
		return resolver, nil, nil
	}

	for _, s := range seen {
		if sameSourceRepo(s, decl.Source) {
			return nil, nil, fmt.Errorf("agentpakke %s reuses %q, which already reuses it back.\nA reuse cycle has no order to resolve in, so nothing was installed.\nBreak the cycle in %s in one of the two repos",
				sourceLabelFor(src), decl.Source, agentpakke.DeclarationPath)
		}
	}
	seen = append(seen, decl.Source)

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
	if err := attachPakkeOrCleanup(baseSrc); err != nil {
		return nil, nil, err
	}
	// A payload-only base has nothing to inherit from. Its unit of delivery is
	// a digest-bound revision, not files at paths, so composing it announced
	// "Reuses:" and then contributed nothing. Refused rather than ignored, for
	// the same reason guardDeclaredItems refuses `items` against a Tier 2
	// pakke: a declaration that cannot do what it says should say so.
	if payloadOnly(baseSrc) {
		baseSrc.Cleanup()
		return nil, nil, fmt.Errorf(
			"agentpakke %s reuses %q, which ships pre-built payloads (Tier 2).\n"+
				"A payload tree is staged and digest-verified as a whole, so there are no files to inherit from it.\n\n"+
				"Nothing was installed. Remove the reuse from %s, or ask %s for a layout-shaped pakke",
			sourceLabelFor(src), decl.Source, agentpakke.DeclarationPath, decl.Source)
	}
	// A reused pakke may itself reuse one. The chain is built depth-first, so
	// by the time this source's resolver is wrapped, everything behind it is
	// already in place.
	baseResolver, deeper, err := composeResolverSeen(resolverFor(baseSrc.Dir, baseSrc.Pakke), baseSrc, seen)
	if err != nil {
		// The frame below cleaned up whatever it resolved; this checkout is
		// the one no one else can reach.
		baseSrc.Cleanup()
		return nil, nil, err
	}
	return resolver.WithBase(baseResolver), append(composedBases{baseSrc}, deeper...), nil
}
