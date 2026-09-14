package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The launch half of #858 step 2: an approved sandbox proposal becomes
// --allow-private-domain on the cplt launch line, and nothing else anywhere.
//
// Not written into cplt's configuration, on purpose (invariant 4).
// allow_private_domains is a plain string array with no room for provenance, so
// marking which entries nav-pilot put there would take a sidecar record
// regardless — and then the configuration is duplicated state that can drift
// from the record that justifies it. Deriving the flag from the record each
// launch cannot drift, and a revoked or superseded approval stops taking effect
// the moment it stops matching.
//
// Two seams carry it, not one: the staged and opencode/pi launches go through
// [cpltArgv], and the legacy Copilot launch assembles its own argv in
// [BuildCopilotArgs]. TestI5NoUnaccountedCpltLaunchPath fails if a third
// appears.

// minCpltStampProtectingNavPilotState is the cplt release that write-denies
// nav-pilot's state directory from inside a sandbox session.
//
// Invariant 3 says the consent record must not be writable from a cplt session,
// or an agent approves its own waiver for the next launch. cplt protects
// ~/.config/cplt and names nothing under ~/.nav-pilot/, so today it does not
// hold. That is a change in cplt, in flight as a separate piece of work, and
// until a user's cplt has it an approved waiver could have been written by the
// agent that benefits from it.
//
// So the gate refuses rather than trusts: below this stamp no waiver is
// applied, whatever the record says. The placeholder is a stamp no release can
// meet, which is the fail-closed direction and means the mechanism applies
// nothing at all until the cplt change ships.
//
// TODO(#858): replace with the real cplt release stamp once the write-deny for
// ~/.nav-pilot/ has shipped, and drop this paragraph.
const minCpltStampProtectingNavPilotState = "9999.12.31-235959"

// cpltProposalFlags returns the cplt flags this launch carries for the active
// agentpakke's approved proposal. A package-level var so a test can pin a
// launch vector without a consent file or a cplt binary.
var cpltProposalFlags = defaultCpltProposalFlags

// defaultCpltProposalFlags reads the consent record for the pakke this launch
// is about to run and turns it into flags.
//
// Ordered so the common case costs nothing: a pakke with no proposal, or with
// no approval, returns before the version probe runs. Only a launch that has
// something to apply pays for `cplt --version`.
func defaultCpltProposalFlags() []string {
	pakke := source.ActivePakke()
	proposal := pakke.CpltProposal()
	if proposal == nil {
		return nil
	}
	hosts := artifacts.ApprovedPrivateDomains(pakke.Name, proposal.Hash())
	if len(hosts) == 0 {
		return nil
	}
	if !cpltProtectsNavPilotState() {
		fmt.Fprintf(os.Stderr, "%s %s: the approved private-domain waiver is not applied — this cplt does not write-protect nav-pilot's state directory, so the approval cannot be trusted.\n",
			domain.Yellow("⚠"), pakke.Name)
		return nil
	}
	// Invariant 5: what a launch carries is said out loud at launch.
	fmt.Fprintf(os.Stderr, "%s %s: cplt may resolve %s to private addresses, approved for this scope.\n",
		domain.Dim("ℹ"), pakke.Name, strings.Join(hosts, ", "))
	return privateDomainFlags(hosts)
}

// cpltProtectsNavPilotState reports whether the installed cplt is at or past
// the release that write-denies nav-pilot's state directory. A version that
// cannot be read is "no": this gate is the thing standing between an agent and
// its own waiver, and one that goes green on "could not tell" is not a gate.
func cpltProtectsNavPilotState() bool {
	out, err := probeCpltVersion()
	if err != nil {
		return false
	}
	stamp := cpltStamp(out)
	return stamp != "" && stamp >= minCpltStampProtectingNavPilotState
}

// privateDomainFlags renders one --allow-private-domain per host. One flag per
// name, never a wildcard and never cplt's machine-wide relaxation: the waiver
// is the DNS-rebinding guard being lifted for named hosts, and widening it is
// the one thing that would turn it into something else.
func privateDomainFlags(hosts []string) []string {
	args := make([]string, 0, 2*len(hosts))
	for _, host := range hosts {
		args = append(args, "--allow-private-domain", host)
	}
	return args
}
