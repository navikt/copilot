package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// This file wires the committed declaration ([agentpakke.Declaration]) into the
// CLI's existing source resolution. It adds one rung to a precedence ladder
// that already had three, and one write on each of the two commands that own a
// revision.
//
// Precedence for which agentpakke a repo scope gets, highest first:
//
//	1. --source / --ref on the command line
//	2. the scope's own state (sync only — selection is per scope, B4)
//	3. .nav-pilot/agentpakke.lock.json in the repo
//	4. the config file's `source` key
//	5. navikt/copilot
//
// The declaration sits above the config key and below the command line because
// of who wrote each: the config key is one developer's machine-wide default,
// the declaration is the repo's reviewed choice, and the flag is the developer
// saying "no, this one, now". It sits below the scope state on sync for the
// same reason it did before this file existed — syncing one scope must never
// drag another agentpakke's content into it (B4), and switching a repo to a new
// agentpakke is an install, not a sync.
//
// The B3 cross-source guard reads the declaration through the same ladder (see
// crossSourceCheck), so a declaration naming one agentpakke while the scope
// state records another is refused rather than silently mixed — the same
// refusal that already covered the config key.
//
// User scope has no declaration and never will: there is no repository to
// commit it to, and the whole point of the file is being reviewable in a pull
// request.

// scopeDeclaration reads the declaration governing a scope, or nil when there
// is none. A malformed one is an error the command reports: a pin nav-pilot
// cannot read must not silently degrade to the config key.
func scopeDeclaration(scope *InstallScope) (*agentpakke.Declaration, error) {
	if scope == nil || scope.IsUser() {
		return nil, nil
	}
	d, err := agentpakke.LoadDeclaration(scope.RootDir)
	if errors.Is(err, agentpakke.ErrNoDeclaration) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := validateSourceValue(d.Source); err != nil {
		return nil, fmt.Errorf("%s names an invalid source: %w", agentpakke.DeclarationPath, err)
	}
	return d, nil
}

// declaredSourceRepo is the source a scope's declaration names, or "" when it
// has none. Errors are swallowed: this is the guards' view of the ladder, and
// a broken declaration is the running command's error to report, not a reason
// for a guard to fail a run it was only advising on.
func declaredSourceRepo(scope *InstallScope) string {
	d, err := scopeDeclaration(scope)
	if err != nil || d == nil {
		return ""
	}
	return d.Source
}

// declaredPin returns the source and ref a repo's declaration pins, for a
// command that was given neither on the command line.
//
// An explicit --source means the developer is deliberately reaching past the
// repo's declaration, so the pinned revision goes with it: installing another
// agentpakke at this agentpakke's SHA is meaningless. An explicit --ref alone
// keeps the declared source and overrides only the revision.
func declaredPin(scope *InstallScope, flagSource, flagRef string) (repo, ref string) {
	if flagSource != "" {
		return "", ""
	}
	d, err := scopeDeclaration(scope)
	if err != nil || d == nil {
		return "", ""
	}
	if flagRef != "" {
		return d.Source, ""
	}
	return d.Source, d.SHA
}

// applyDeclaredItems narrows a content manifest to the items a declaration
// names, so a team can take four agents out of a platform pakke's twelve
// without forking it.
//
// Selection is a Tier 1 concept. Tier 1 content is a layout of individually
// addressable files, which is what makes "these four" expressible at all. A
// Tier 2 agentpakke ships digest-bound payload trees whose unit of installation
// is the revision, not the file — a payload's agents are staged together and
// verified against the digest as one — so naming items there is refused rather
// than quietly ignored (guardDeclaredItems).
//
// An item the pakke does not ship is an error, not a skip. A typo that silently
// installs three of four agents is precisely the failure a committed, reviewed
// declaration exists to prevent.
func applyDeclaredItems(manifest *Manifest, items map[string]string) (*Manifest, error) {
	if len(items) == 0 {
		return manifest, nil
	}
	available := map[string]map[string]bool{
		KindAgent.Name:       nameSet(manifest.Agents),
		KindSkill.Name:       nameSet(manifest.Skills),
		KindInstruction.Name: nameSet(manifest.Instructions),
		KindPrompt.Name:      nameSet(manifest.Prompts),
	}

	var unknown []string
	selected := map[string][]string{}
	for _, name := range sortedKeys(items) {
		itemType := items[name]
		if !available[itemType][name] {
			unknown = append(unknown, fmt.Sprintf("%s %q", itemType, name))
			continue
		}
		selected[itemType] = append(selected[itemType], name)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("%s names %d item(s) the agentpakke %q does not ship:\n  - %s\n\nNothing was installed. Fix the item list, or run %s to see what it ships",
			agentpakke.DeclarationPath, len(unknown), manifest.Name, strings.Join(unknown, "\n  - "),
			bold("nav-pilot list --items"))
	}

	narrowed := *manifest
	narrowed.Agents = selected[KindAgent.Name]
	narrowed.Skills = selected[KindSkill.Name]
	narrowed.Instructions = selected[KindInstruction.Name]
	narrowed.Prompts = selected[KindPrompt.Name]
	return &narrowed, nil
}

// guardDeclaredItems refuses a per-item selection against a Tier 2 agentpakke,
// whose install unit is a pinned revision of a digest-bound payload tree rather
// than a set of files. Ignoring the list instead would install everything while
// the repo's own committed file says otherwise.
func guardDeclaredItems(src *Source, items map[string]string) error {
	if len(items) == 0 || !payloadOnly(src) {
		return nil
	}
	return fmt.Errorf(
		"%s names individual items, but agentpakke %q ships pre-built payloads (Tier 2).\n"+
			"A payload tree is staged and digest-verified as a whole, so there is no per-item selection to make there.\n\n"+
			"Nothing was installed. Remove the %s block from %s, or ask %s for a payload context that carries only what you need",
		agentpakke.DeclarationPath, src.Pakke.Name,
		bold("items"), agentpakke.DeclarationPath, sourceLabelFor(src))
}

// recordDeclaration writes the repo's declaration after a scope-defining
// install, so the source and the exact revision this install used are in the
// diff of the very commit that adds the content.
//
// It preserves any hand-written item list: the developer wrote it, the install
// honoured it, and rewriting it from what happened to be installed would turn
// every upstream addition into a merge conflict for every consumer.
//
// It never fails the install — the content is already on disk, correct — and it
// never touches user scope, which has no repository to commit to.
func recordDeclaration(scope *InstallScope, src *Source) {
	if scope == nil || scope.IsUser() {
		return
	}
	existing, err := scopeDeclaration(scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not read %s: %v\n", yellow("⚠"), agentpakke.DeclarationPath, err)
		return
	}
	d := &agentpakke.Declaration{ContractVersion: agentpakke.SupportedContractMajors[0]}
	if existing != nil {
		d = existing
	}
	d.Source = sourceLabelFor(src)
	d.SHA = src.SHA
	d.MinNavPilotVersion = ""
	if src.Pakke != nil {
		d.MinNavPilotVersion = src.Pakke.MinNavPilotVersion
	}
	if err := agentpakke.WriteDeclaration(scope.RootDir, d); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write %s: %v\n", yellow("⚠"), agentpakke.DeclarationPath, err)
		return
	}
	fmt.Printf("%s Pinned %s@%s in %s — commit it so the whole team installs this revision.\n",
		green("✓"), bold(d.Source), shortSHA(d.SHA), bold(agentpakke.DeclarationPath))
}

// bumpDeclarationSHA moves the pinned revision forward after a sync that
// actually landed on a new one. That is the whole reason the SHA lives in a
// committed file: an agentpakke update becomes a reviewable one-line diff in a
// pull request instead of a change nobody outside the syncing machine can see.
//
// It only ever bumps a declaration that already names the source being synced.
// Sync does not create the file (that is install's job, which is where the
// source is actually chosen) and does not repoint one at a different
// agentpakke, which is an install too.
func bumpDeclarationSHA(scope *InstallScope, src *Source) {
	d, err := scopeDeclaration(scope)
	if err != nil || d == nil || src.SHA == "" || d.SHA == src.SHA {
		return
	}
	if !sameSourceRepo(d.Source, sourceLabelFor(src)) {
		return
	}
	previous := d.SHA
	d.SHA = src.SHA
	if err := agentpakke.WriteDeclaration(scope.RootDir, d); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not bump the pinned revision in %s: %v\n",
			yellow("⚠"), agentpakke.DeclarationPath, err)
		return
	}
	fmt.Printf("%s Bumped %s: %s → %s. Commit it to share the update.\n",
		green("✓"), bold(agentpakke.DeclarationPath), shortSHA(previous), shortSHA(src.SHA))
}

func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// resolveDeclaredSource resolves the content source for a command targeting a
// scope, honouring that scope's committed declaration when the command line
// named neither a source nor a ref. It is the single place install and add
// reach the declaration from, so the two can never disagree about which
// agentpakke a repo uses.
func resolveDeclaredSource(scope *InstallScope, ref, sourceRepo string) (*Source, error) {
	declRepo, declRef := declaredPin(scope, sourceRepo, ref)
	if ref == "" {
		ref = declRef
	}
	if sourceRepo == "" {
		sourceRepo = declRepo
	}
	return resolveSource(ref, sourceRepo)
}
