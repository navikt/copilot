package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// The consent half of #858 step 2: an agentpakke proposes sandbox
// configuration, and the person installing it answers.
//
// Asked at install and at any sync where the block changed, once per scope,
// pakke and content hash. A revision that adds to or widens an entry is a new
// hash, so the previous answer stops matching and the question comes back with
// the difference on screen. Nothing here writes cplt configuration; an approval
// is recorded, and the launch derives its flags from the record.
//
// A decline installs anyway. Someone may want the pakke for its other skills,
// and losing the whole install over one waiver would be a worse answer than the
// one the user gave. What they get instead is the workflow that will fail and
// the one line that fixes it by hand.
//
// # Everything a pakke wrote is untrusted text
//
// The prompt shows the pakke's own `reason` and the names of keys nav-pilot
// does not implement. A pakke that can put a newline in either can write a line
// that looks like nav-pilot's own — "nothing else in the sandbox changes" — and
// an escape sequence can erase the lines above it (#861 review). Two rules,
// both of them rather than either:
//
//   - the manifest schema bounds the length and refuses control, format and
//     line separators, which is where an agentpakke author is told;
//   - [domain.SafeText] collapses every whitespace run to a space and replaces
//     control runes at the point of printing, which is what holds for a record
//     written by an older binary or a manifest validated under a looser rule.
//
// So a pakke cannot produce a line break on the screen at all: the prompt owns
// every one of them.

const (
	// proposalReasonWidth bounds a pakke's reason on screen. The schema already
	// caps it at 400; this is the second bound, applied where it is printed.
	proposalReasonWidth = 400
	// proposalValueWidth bounds one rendered value in the change list, so a
	// large inert key cannot push the hosts out of view.
	proposalValueWidth = 120
	// proposalPathWidth bounds a proposed read grant. The schema caps it at
	// 256, and this matches rather than undercuts it: the same string is
	// printed as a command the user can run by hand, and a truncated path is a
	// command that silently grants something else.
	proposalPathWidth = 256
)

// cpltSetupCommands are the commands a user runs to configure the proposal in
// cplt themselves, for when they decline or have no terminal to be asked in.
//
// The read grants keep their "~/" form: `cplt config set` stores the string and
// expand_tilde resolves it at load, so this is the same path the launch would
// have passed, spelled the way the manifest spells it.
func cpltSetupCommands(hosts, reads []string) []string {
	var out []string
	if len(hosts) > 0 {
		out = append(out, "cplt config set proxy.allow_private_domains "+strings.Join(hosts, ","))
	}
	for _, path := range reads {
		out = append(out, fmt.Sprintf("cplt config set allow.read %q", safe(path, proposalPathWidth)))
	}
	return out
}

// askProposalConsent puts the question. A var so tests answer it, like every
// other prompt the install path owns.
var askProposalConsent = func(title, description string, approve *bool) error {
	return huh.NewConfirm().
		Title(title).
		Description(description).
		Affirmative("Allow").
		Negative("Decline").
		Value(approve).
		WithTheme(navTheme()).
		Run()
}

// noteProposalConsent asks about the sandbox configuration a source's
// agentpakke proposes, and records the answer for this scope.
//
// Best effort by design: it is a question about a launch flag, and neither a
// declined proposal nor an unwritable record is a reason to fail the install
// that is otherwise fine. Every failure path ends with no approval, which is
// the direction that applies nothing.
func noteProposalConsent(scope *InstallScope, src *Source, dryRun, jsonOutput bool) {
	if scope == nil || src == nil || src.Pakke == nil || dryRun {
		return
	}
	proposal := src.Pakke.CpltProposal()
	if proposal == nil {
		return
	}
	name := safe(src.Pakke.Name, 64)

	// Invariant 7, the reporting half: a key this binary does not implement is
	// named out loud, so "ignored" never means "unnoticed". Printed before the
	// question, and printed even when there is no question to ask. The names
	// are the pakke's, so they are sanitised like its prose.
	if inert := proposal.InertKeys(); len(inert) > 0 && !jsonOutput {
		fmt.Fprintf(os.Stderr, "%s agentpakke %s proposes cplt settings this nav-pilot does not implement: %s. They are ignored.\n",
			yellow("⚠"), bold(name), safeList(inert))
	}

	hosts := proposal.AllowPrivateDomains()
	reads := proposal.AllowRead()
	if len(hosts) == 0 && len(reads) == 0 {
		return
	}
	commands := cpltSetupCommands(hosts, reads)

	hash := proposal.Hash()
	prior, err := artifacts.ReadProposalConsent(scope, src.Pakke.Name)
	if err != nil {
		// Unreadable is not "no answer". Asking again over a file this process
		// cannot read would write an answer on top of one it never saw.
		fmt.Fprintf(os.Stderr, "%s agentpakke %s: could not read the sandbox consent record, so nothing was asked or recorded: %v\n",
			yellow("⚠"), bold(name), err)
		return
	}
	if prior != nil && prior.Hash == hash {
		// Answered already, for exactly this block. An approval needs no
		// second look, and a decline must not be asked again — that is what
		// recording it is for.
		return
	}

	// Invariant 8. A scripted install cannot consent on a person's behalf, and
	// nothing is recorded either: an unanswered question is not a "no", and
	// recording one would stop the terminal install from ever asking.
	if !isInteractive() || jsonOutput {
		fmt.Fprintf(os.Stderr, "%s agentpakke %s proposes cplt sandbox settings, which only an interactive install can approve.\n%s\n",
			dim("ℹ"), bold(name), indented(commands))
		return
	}

	approve := false
	if err := askProposalConsent(proposalTitle(name, prior), proposalDescription(proposal, hosts, reads, prior), &approve); err != nil {
		// Ctrl-C is not an answer: nothing is recorded and the question comes
		// back at the next install or sync.
		return
	}

	record := artifacts.ProposalConsent{
		Scope: scope.Name,
		Root:  scope.RootDir,
		Pakke: src.Pakke.Name,
		Hash:  hash,
		Block: proposal.CanonicalJSON(),
		// The cplt in force right now, so a launch can tell whether the answer
		// was given while the record was protected (#861 review).
		CpltStamp: providerpkg.CpltStamp(),
		Approved:  approve,
		At:        time.Now(),
	}
	if approve {
		record.Hosts = hosts
		record.Reads = reads
	}
	if err := artifacts.WriteProposalConsent(record); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not record the answer for %s: %v — you will be asked again.\n", yellow("⚠"), name, err)
		return
	}
	if !approve {
		fmt.Printf("%s %s is installed without it, so this will fail: %s\n  Allow it yourself:\n%s\n",
			yellow("⚠"), bold(name), safe(proposal.Reason, proposalReasonWidth), indented(commands))
		return
	}
	fmt.Printf("%s %s may %s from inside the sandbox.\n", green("✓"), bold(name), proposal.Summary())
	if !providerpkg.CpltProtectsNavPilotState() {
		// Recorded, and deliberately not usable yet. Saying so here is the
		// difference between a waiver that has not taken effect and one that
		// looks like it was never given.
		fmt.Printf("  %s\n", dim("It takes effect once cplt is new enough to deny writes to ~/.nav-pilot/."))
	}
}

// indented renders the commands a user can run by hand, two spaces in, one per
// line. A block that carries both a private-domain waiver and a read grant has
// more than one, and the prompt owns every line break on the screen.
func indented(commands []string) string {
	lines := make([]string, 0, len(commands))
	for _, command := range commands {
		lines = append(lines, "  "+bold(command))
	}
	return strings.Join(lines, "\n")
}

// proposalTitle is the question. A revision the user has answered before says
// so, because "approve this" and "approve this again, it changed" are different
// questions.
//
// One question for the whole block, however many kinds of setting it carries:
// the consent record is keyed on the block's hash, so a second prompt would be
// a second answer to a question that has only one record to live in.
func proposalTitle(name string, prior *artifacts.ProposalConsent) string {
	if prior == nil {
		return fmt.Sprintf("agentpakke %s asks for more than the sandbox gives it. Allow it?", name)
	}
	return fmt.Sprintf("agentpakke %s changed what it asks of the sandbox. Allow it?", name)
}

// proposalDescription is what the person decides on: the pakke's own reason,
// the exact hosts and files, what each of them is not, and — for a changed
// block — what moved since the answer it voids.
//
// Both kinds in one description, because both are one block, one hash and one
// answer. A block carrying both lists both and is accepted or declined whole.
func proposalDescription(proposal *agentpakke.CpltProposal, hosts, reads []string, prior *artifacts.ProposalConsent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", safe(proposal.Reason, proposalReasonWidth))
	if prior != nil {
		b.WriteString("Changed since you last answered:\n")
		for _, line := range proposalChanges(prior.Block, proposal.CanonicalJSON()) {
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n")
	}
	if len(hosts) > 0 {
		fmt.Fprintf(&b, "Hosts: %s\n", strings.Join(hosts, ", "))
		b.WriteString("This lifts cplt's DNS-rebinding guard for those names only. " +
			"The allowlist and blocklist still apply, no port is opened, no path is granted, nothing is executed.\n")
	}
	if len(reads) > 0 {
		if len(hosts) > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Files: %s\n", safePathList(reads))
		b.WriteString("Read-only, one file each, under your home directory. " +
			"Nothing is written, no directory is opened, and a file that turns out to be a directory is skipped at launch.")
	}
	return strings.TrimRight(b.String(), "\n")
}

// proposalChanges lists what moved between the block an earlier answer covered
// and the block now proposed.
//
// Field-level, not host-level. The hash covers the whole block, so the change
// that voided the approval can be the reason or a key nav-pilot does not
// implement — and a diff that could only compare hosts said "the hosts are
// unchanged" and showed nothing for exactly those cases (#861 review). Hosts
// still get their own +/- lines, because that is the list a person reads first.
func proposalChanges(before, now string) []string {
	prev, prevOK := decodeBlock(before)
	next, nextOK := decodeBlock(now)
	if !prevOK || !nextOK {
		// A record from a binary that did not keep the block, or one that will
		// not decode. Saying so beats rendering a difference against nothing.
		return []string{
			"  (the proposal you answered before was not kept, so this is the whole of the new one)",
			"  " + safe(now, proposalValueWidth),
		}
	}

	seen := map[string]bool{}
	var ordered []string
	for _, block := range []map[string]any{prev, next} {
		for key := range block {
			if !seen[key] {
				seen[key] = true
				ordered = append(ordered, key)
			}
		}
	}
	sort.Strings(ordered)

	var lines []string
	for _, key := range ordered {
		was, is := render(prev[key]), render(next[key])
		if was == is {
			continue
		}
		if key == "proxy" {
			lines = append(lines, hostChanges(prev[key], next[key])...)
			continue
		}
		lines = append(lines, changeLine(key, was, is))
	}
	if len(lines) == 0 {
		// Unreachable while the hash is taken over this very JSON: different
		// hashes mean different bytes mean at least one key differs. Kept so
		// that if that ever stops holding, the prompt says nothing false.
		lines = append(lines, "  (the proposal changed in a way nav-pilot cannot render)")
	}
	return lines
}

// changeLine renders one field that moved, in whichever of the three shapes is
// true: added, removed, or replaced.
func changeLine(key, was, now string) string {
	key = safe(key, 64)
	switch {
	case was == "":
		return fmt.Sprintf("  + %s: %s", key, safe(now, proposalValueWidth))
	case now == "":
		return fmt.Sprintf("  - %s: %s", key, safe(was, proposalValueWidth))
	default:
		return fmt.Sprintf("  ~ %s: %s → %s", key, safe(was, proposalValueWidth), safe(now, proposalValueWidth))
	}
}

// hostChanges renders the proxy section as the +/- host list a person reads,
// and falls back to a whole-value line for anything else that moved under it.
func hostChanges(before, now any) []string {
	was, wasOK := blockHosts(before)
	is, isOK := blockHosts(now)
	if !wasOK || !isOK {
		return []string{changeLine("proxy", render(before), render(now))}
	}
	var lines []string
	for _, host := range is {
		if !slices.Contains(was, host) {
			lines = append(lines, "  + "+safe(host, 253))
		}
	}
	for _, host := range was {
		if !slices.Contains(is, host) {
			lines = append(lines, "  - "+safe(host, 253))
		}
	}
	if len(lines) == 0 {
		return []string{changeLine("proxy", render(before), render(now))}
	}
	return lines
}

// blockHosts pulls proxy.allow_private_domains out of a decoded block.
func blockHosts(section any) ([]string, bool) {
	if section == nil {
		return nil, true
	}
	obj, ok := section.(map[string]any)
	if !ok {
		return nil, false
	}
	raw, ok := obj["allow_private_domains"].([]any)
	if !ok {
		return nil, len(obj) == 0
	}
	hosts := make([]string, 0, len(raw))
	for _, item := range raw {
		host, ok := item.(string)
		if !ok {
			return nil, false
		}
		hosts = append(hosts, host)
	}
	return hosts, true
}

func decodeBlock(block string) (map[string]any, bool) {
	if block == "" {
		return nil, false
	}
	var out map[string]any
	if json.Unmarshal([]byte(block), &out) != nil {
		return nil, false
	}
	return out, true
}

// render is one field's value as compact JSON, "" when the field is absent.
func render(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "?"
	}
	return string(data)
}

// safe renders text an agentpakke wrote for this terminal. Every pakke-supplied
// string on this file's output paths goes through it — see the file comment
// above for why the schema check is not enough on its own.
func safe(s string, max int) string { return domain.SafeText(s, max) }

// safeList renders a list of pakke-supplied names.
func safeList(items []string) string { return safeJoin(items, 64) }

// safePathList renders proposed read grants, which are longer than a name and
// have to stay whole: the person answering is reading the exact path.
func safePathList(items []string) string { return safeJoin(items, proposalPathWidth) }

func safeJoin(items []string, max int) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, safe(item, max))
	}
	return strings.Join(out, ", ")
}

// forgetProposalConsent drops every answer a scope holds, so no waiver from
// anything installed there outlives the uninstall (invariant 6).
//
// It returns its error, and [cmdUninstall] calls it before it removes anything
// else and fails on it. Printing a warning and reporting success was the hole:
// a rename that failed left an approved record behind, the retry found no state
// left to uninstall, and a later reinstall of the same revision picked the old
// approval straight back up (#861 review).
func forgetProposalConsent(scope *InstallScope) (int, error) {
	return artifacts.RemoveProposalConsentsIn(scope)
}
