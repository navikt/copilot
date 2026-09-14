package provider

import (
	"fmt"
	"os"
	"path/filepath"
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

// minCpltStampProtectingNavPilotState is the cplt release that denies
// ~/.nav-pilot/ by name, so no user configuration can reopen it.
//
// Invariant 3 says the consent record must not be writable from a cplt session,
// or an agent approves its own waiver for the next launch. The precise claim
// matters here, because the weaker one is the true one: ~/.nav-pilot/ was never
// writable in a *default* session — macOS runs (deny default) and Linux runs
// grant-only Landlock, so an unnamed path is already out of reach. What was
// missing is that a user's own allow.write = ["~"] reopened it, and nothing
// told them it had.
//
// navikt/cplt#508 closed that by naming ~/.nav-pilot/ in DENIED_DOTFILES, the
// same list that holds .config/cplt: denied read and write, after every user
// allow. Grants *inside* it keep working on purpose — a staged Tier 2 launch
// passes ~/.nav-pilot/pakker/<owner>-<repo>/<sha>/<client>/<context> as
// --allow-read — and only a grant on the directory itself is refused.
//
// So the gate refuses rather than trusts: below this stamp no waiver is
// applied, whatever the record says.
//
// The stamp is cplt 2026.09.14-105131-446dfbb, the release cut from 446dfbb —
// the commit that added the entry. Only the date-time part is comparable across
// builds, which is what cpltStamp returns.
const minCpltStampProtectingNavPilotState = "2026.09.14-105131"

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
	name := domain.SafeText(pakke.Name, 64)
	record, err := artifacts.ApprovedProposal(pakke.Name, proposal.Hash())
	if err != nil {
		// Unreadable is not approved. Said out loud rather than swallowed: the
		// user's answer is in that file, and silently launching without it is
		// how a waiver looks like it was never given.
		fmt.Fprintf(os.Stderr, "%s %s: no private-domain waiver is applied — %v\n", domain.Yellow("⚠"), name, err)
		return nil
	}
	if record == nil || len(record.Hosts) == 0 {
		return nil
	}
	if reason := untrustworthyRecord(record); reason != "" {
		fmt.Fprintf(os.Stderr, "%s %s: the approved private-domain waiver is not applied — %s.\n",
			domain.Yellow("⚠"), name, reason)
		return nil
	}
	hosts := record.Hosts
	// Invariant 5: what a launch carries is said out loud at launch.
	fmt.Fprintf(os.Stderr, "%s %s: cplt may resolve %s to private addresses, approved for this scope.\n",
		domain.Dim("ℹ"), name, strings.Join(hosts, ", "))
	return privateDomainFlags(hosts)
}

// untrustworthyRecord names the reason an approval must not be acted on, or ""
// when there is none. Three questions, all of which have to answer yes:
//
//  1. Does the cplt running now deny ~/.nav-pilot/? Without it, a user's own
//     allow.write = ["~"] reopens the record for the session about to start.
//  2. Did the cplt running when the answer was recorded deny it? Upgrading cplt
//     afterwards does not make an answer written by an agent trustworthy, and
//     the launch-time check alone cannot see that far back (#861 review).
//  3. Is the record actually inside the tree cplt denies? NAV_PILOT_CONFIG
//     relocates nav-pilot's state directory, which is supported and which moves
//     the record outside the deny rule while the version check still says yes
//     (#861 review).
func untrustworthyRecord(record *artifacts.ProposalConsent) string {
	if !cpltProtectsNavPilotState() {
		return "this cplt does not deny writes to nav-pilot's state directory, so the approval cannot be trusted"
	}
	if record.CpltStamp < minCpltStampProtectingNavPilotState {
		return fmt.Sprintf(
			"it was recorded under cplt %s, before %s denied writes to nav-pilot's state directory. Run the install or sync again to answer once more",
			recordedCpltLabel(record), minCpltStampProtectingNavPilotState)
	}
	if !consentRecordIsProtected() {
		return fmt.Sprintf(
			"the consent record is at %s, outside the ~/.nav-pilot/ tree cplt denies, so a session could write it",
			artifacts.ProposalConsentPath())
	}
	return ""
}

// recordedCpltLabel names the cplt a record was made under, for the one line
// that refuses it. An empty stamp means the probe could not read a version then.
func recordedCpltLabel(record *artifacts.ProposalConsent) string {
	if record.CpltStamp == "" {
		return "a version nav-pilot could not read"
	}
	return domain.SafeText(record.CpltStamp, 32)
}

// consentRecordIsProtected reports whether the record actually sits inside the
// directory cplt denies. cplt's rule names ~/.nav-pilot/, while
// NAV_PILOT_CONFIG can put nav-pilot's state anywhere — a supported relocation
// that would otherwise leave the record writable from a session with the
// version gate still green (#861 review).
func consentRecordIsProtected() bool {
	path := artifacts.ProposalConsentPath()
	home, err := os.UserHomeDir()
	if path == "" || err != nil {
		return false
	}
	return domain.PathWithinRoot(filepath.Join(home, ".nav-pilot"), path)
}

// CpltStamp is the installed cplt release stamp, or "" when there is no cplt or
// its version cannot be read. It is what a consent record keeps so a later
// launch can tell which cplt the answer was given under.
func CpltStamp() string {
	out, err := probeCpltVersion()
	if err != nil {
		return ""
	}
	return cpltStamp(out)
}

// CpltProtectsNavPilotState reports whether a consent recorded now would be
// usable at launch: the installed cplt denies writes to nav-pilot's state
// directory, and that directory is the one cplt's rule names. Exported so the
// consent prompt can say up front that an answer will not take effect yet.
func CpltProtectsNavPilotState() bool {
	return cpltProtectsNavPilotState() && consentRecordIsProtected()
}

// cpltProtectsNavPilotState reports whether the installed cplt is at or past
// the release that denies writes to nav-pilot's state directory. A version that
// cannot be read is "no": this gate is the thing standing between an agent and
// its own waiver, and one that goes green on "could not tell" is not a gate.
func cpltProtectsNavPilotState() bool {
	stamp := CpltStamp()
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
