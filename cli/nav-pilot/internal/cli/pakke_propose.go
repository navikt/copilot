package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
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

// cpltPrivateDomainCommand is the command a user runs to configure the waiver
// in cplt themselves, for when they decline or have no terminal to be asked in.
func cpltPrivateDomainCommand(hosts []string) string {
	return "cplt config set proxy.allow_private_domains " + strings.Join(hosts, ",")
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
	name := src.Pakke.Name

	// Invariant 7, the reporting half: a key this binary does not implement is
	// named out loud, so "ignored" never means "unnoticed". Printed before the
	// question, and printed even when there is no question to ask.
	if inert := proposal.InertKeys(); len(inert) > 0 && !jsonOutput {
		fmt.Fprintf(os.Stderr, "%s agentpakke %s proposes cplt settings this nav-pilot does not implement: %s. They are ignored.\n",
			yellow("⚠"), bold(name), strings.Join(inert, ", "))
	}

	hosts := proposal.AllowPrivateDomains()
	if len(hosts) == 0 {
		return
	}

	hash := proposal.Hash()
	prior := artifacts.ReadProposalConsent(scope, name)
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
		fmt.Fprintf(os.Stderr, "%s agentpakke %s proposes a cplt sandbox waiver, which only an interactive install can approve.\n  %s\n",
			dim("ℹ"), bold(name), dim(cpltPrivateDomainCommand(hosts)))
		return
	}

	approve := false
	if err := askProposalConsent(proposalTitle(name, prior), proposalDescription(proposal, hosts, prior), &approve); err != nil {
		// Ctrl-C is not an answer: nothing is recorded and the question comes
		// back at the next install or sync.
		return
	}

	record := artifacts.ProposalConsent{
		Scope:    scope.Name,
		Root:     scope.RootDir,
		Pakke:    name,
		Hash:     hash,
		Approved: approve,
		At:       time.Now(),
	}
	if approve {
		record.Hosts = hosts
	}
	if err := artifacts.WriteProposalConsent(record); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not record the answer for %s: %v — you will be asked again.\n", yellow("⚠"), name, err)
		return
	}
	if approve {
		fmt.Printf("%s %s may reach %s from inside the sandbox.\n", green("✓"), bold(name), strings.Join(hosts, ", "))
		return
	}
	fmt.Printf("%s %s is installed without the waiver, so this will fail: %s\n  Allow it yourself:  %s\n",
		yellow("⚠"), bold(name), proposal.Reason, bold(cpltPrivateDomainCommand(hosts)))
}

// proposalTitle is the question. A revision the user has answered before says
// so, because "approve this" and "approve this again, it changed" are different
// questions.
func proposalTitle(name string, prior *artifacts.ProposalConsent) string {
	if prior == nil {
		return fmt.Sprintf("agentpakke %s asks to reach private hosts from inside the sandbox. Allow it?", name)
	}
	return fmt.Sprintf("agentpakke %s changed what it asks to reach from inside the sandbox. Allow it?", name)
}

// proposalDescription is what the person decides on: the pakke's own reason,
// the exact hosts, what the waiver is not, and — for a changed block — what
// moved since the answer it voids.
func proposalDescription(proposal *agentpakke.CpltProposal, hosts []string, prior *artifacts.ProposalConsent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", proposal.Reason)
	if prior != nil {
		for _, line := range proposalDiff(prior.Hosts, hosts) {
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Hosts: %s\n", strings.Join(hosts, ", "))
	b.WriteString("This lifts cplt's DNS-rebinding guard for those names only. " +
		"The allowlist and blocklist still apply, no port is opened, no path is granted, nothing is executed.")
	return b.String()
}

// proposalDiff lists what changed between the hosts an earlier answer covered
// and the ones now asked for.
func proposalDiff(before, after []string) []string {
	var lines []string
	for _, host := range after {
		if !slices.Contains(before, host) {
			lines = append(lines, "  + "+host)
		}
	}
	for _, host := range before {
		if !slices.Contains(after, host) {
			lines = append(lines, "  - "+host)
		}
	}
	if len(lines) == 0 {
		// The hosts are the same, so something else in the block moved — a key
		// nav-pilot does not implement, or the reason. The hash changed, so the
		// old answer is void either way, and saying "nothing changed" would be
		// the one thing that is not true.
		lines = append(lines, "  (the hosts are unchanged; something else in the proposal moved)")
	}
	return lines
}

// forgetProposalConsent drops a scope's answer when the pakke leaves it, so no
// waiver from that pakke outlives the install (invariant 6).
func forgetProposalConsent(scope *InstallScope, pakke string) {
	if err := artifacts.RemoveProposalConsent(scope, pakke); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not remove the sandbox consent record for %s: %v\n", yellow("⚠"), pakke, err)
	}
}
