package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// Runtime gates on the staged (Tier 2) launch path.
//
// Two checks run before a staged launch builds its cplt invocation: the cplt
// binary must be at or past a reviewed baseline, and — when the agentpakke
// declares one — the client must fall inside its `compatibility` range. Both
// are fatal. Team eSyfo made runtime enforcement a condition for G4 sign-off
// (#437, comment 5437575432): "Runtime client compatibility ranges and a
// reviewed cplt minimum should be enforced, not only validated as manifest
// syntax." The legacy path is untouched.
//
// Everything uncertain is fatal here, not a warning. This is past the tier
// gate, where fail-closed is the rule (agentpakke-beslutninger.md §4), and a
// version check that goes green on "could not tell" is exactly the failure
// mode #452 fixed in nav-pilot's own cplt skew check.

// minStagedCpltStamp is the reviewed cplt baseline a staged launch requires,
// adopted from the reference launcher's SUPPORTED_CPLT_RELEASE
// ("2026.08.17-062831-1008a92", scripts/grillmester.py line 27 at
// 3573b93cc8b7568516117263562d073cae9ee7fc) with the release suffix dropped —
// only the date-time stamp is comparable across builds.
//
// Moving it follows the same joint-decision rule as the reference pin itself
// (README.agentpakke.md, "Referansepinning"): it names a baseline both projects
// have reviewed, so nav-pilot does not raise it alone.
//
// This constant is also the exit ramp for --no-audit. That flag is on the
// staged vectors because cplt's parent-side audit can execute
// repository-controlled Git helpers outside the sandbox at the baseline
// grillmester v0.3.0 is tested against (agentpakke-beslutninger.md §2.5). When
// a reviewed cplt baseline fixes that, the joint change is one reviewable diff:
// raise this stamp, drop the flag, attach the evidence.
const minStagedCpltStamp = "2026.08.17-062831"

// Probe budgets, adopted from the reference: 8 seconds for `cplt --version`
// (_trusted_cplt_version_output, grillmester.py line 741) and 30 for the
// sandboxed client probe (_bounded_command_output, line 770).
//
// The client budget is the loose one on purpose. cplt unpacks the Copilot
// runtime into its cache on the first run after every cplt upgrade; a warm
// probe answers in well under a second, but the first staged launch after
// `brew upgrade cplt` does that extraction inside the probe. A 2-second cap —
// what this gate shipped with — turns that one run into a fatal launch error
// for a perfectly healthy install.
const (
	cpltProbeTimeout   = 8 * time.Second
	clientProbeTimeout = 30 * time.Second
)

// stagedProbeWaitDelay is how long Wait keeps reading the probe's stdout after
// the process group is killed, so a wedged pipe cannot outlive the deadline.
const stagedProbeWaitDelay = 2 * time.Second

// probeCpltVersion runs a bounded `cplt --version`. A package-level variable so
// tests can exercise the gate without spawning a binary.
var probeCpltVersion = func() (string, error) {
	path, err := stagedCpltPath()
	if err != nil {
		return "", err
	}
	return runStagedProbe(cpltProbeTimeout, path, "--version")
}

// probeClientVersion runs a bounded version probe for the client a staged
// launch is about to start.
//
// opencode is probed directly: LaunchOpenCodeStaged already requires the binary
// on PATH. copilot is not — only cplt is — so it is probed sandboxed, the way
// the reference does in _sandboxed_client_version (grillmester.py lines
// 883-905). Two parts of that vector are not decoration:
//
//   - --yes --quiet. The reference splices exactly these two in ahead of
//     everything else (_client_probe, grillmester.py line 879:
//     `command[1:1] = ["--yes", "--quiet"]`). A probe gets /dev/null on stdin
//     and a pipe on stdout, and without --yes cplt stops on its launch
//     confirmation — "No TTY available for confirmation. Use --yes for
//     non-interactive runs." — and exits 1. Not sometimes: every run, on every
//     machine, even in an empty directory with no .cplt.toml. Omitting them
//     made the copilot arm of this gate fail 100% of the time (#462 review,
//     finding 1).
//
//   - --project-dir <empty 0700 temp dir>. The reference probes inside a
//     throwaway directory (_sandboxed_client_version, grillmester.py lines
//     884-886, and _client_probe's project_dir at line 862). Asking a version
//     question from the user's cwd instead engages that repository's .cplt.toml
//     trust flow and hands the client read/write over the user's repo — for a
//     `--version`. This is the first and only place --project-dir appears in
//     nav-pilot, and it is not a reversal of the recorded decision to omit it
//     on the launch (see staged_launch.go): a launch is scoped to the user's
//     project on purpose, a version probe is scoped to nothing on purpose.
var probeClientVersion = func(client string) (string, error) {
	if client == "opencode" {
		return runStagedProbe(clientProbeTimeout, "opencode", "--version")
	}
	path, err := stagedCpltPath()
	if err != nil {
		return "", err
	}
	// os.MkdirTemp already creates the directory 0700, which is the mode the
	// reference chmods to.
	dir, err := os.MkdirTemp("", "nav-pilot-client-probe-")
	if err != nil {
		return "", fmt.Errorf("could not create an isolated probe directory: %w", err)
	}
	defer os.RemoveAll(dir)
	return runStagedProbe(clientProbeTimeout, path,
		"--yes", "--quiet", "--no-audit", "--agent", client,
		"--project-dir", dir, "--", "--version")
}

func stagedCpltPath() (string, error) {
	path, name := FindCopilotCLI()
	if path == "" || name != "cplt" {
		return "", errors.New("cplt not found in PATH")
	}
	return path, nil
}

// runStagedProbe runs one version probe under a deadline that actually bounds
// it, and returns its stdout.
//
// The probe runs in its own process group and the deadline kills the group, not
// just the direct child. exec.CommandContext's default cancel signals the
// process it started and nothing else, while cplt hands its stdout pipe to a
// sandboxed grandchild: SIGKILLing cplt alone leaves Output() blocked on a pipe
// nobody is left to close, and the "timeout" bounds nothing at all (#462
// review, finding 3 — a 30-second hang against a 2-second context). The
// reference has the same shape for the same reason: start_new_session=True
// (grillmester.py line 782) plus _terminate_process_group (lines 753-763),
// which kills the group and falls back to the child.
//
// WaitDelay is the Go-native backstop for the same pipe: even if the group kill
// finds nothing to kill, Wait stops waiting on the pipes shortly after and
// returns. Both are one line each, so this takes both rather than choosing —
// and they cover different cases. Measured against a 300ms deadline with a
// grandchild holding the pipe for 30s: 0.3s with the group kill, 2.3s with only
// WaitDelay. The remaining ceiling is the case where the probe process exits
// before its grandchild: Go has already reaped it by then and never calls
// Cancel, so WaitDelay bounds us but the grandchild is orphaned rather than
// killed. cplt does not exit while its client runs, so that is not this probe.
//
// A package-level variable for the same reason the two probes above are: a test
// can capture the exact argv a probe would run without spawning anything, which
// is what keeps a broken probe vector from passing the suite on a machine with
// no cplt installed.
var runStagedProbe = func(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = stagedProbeWaitDelay
	out, err := cmd.Output()
	if ctx.Err() != nil {
		// This gate is fatal, so the message has to carry both halves: what
		// happened, and that retrying is the right move. cplt unpacks the client
		// runtime on the first run after an upgrade, which is the one legitimate
		// slow path (#462 review, finding 4).
		return "", fmt.Errorf(
			"the %s version probe was still running after %s and was stopped — the first probe after a cplt upgrade unpacks the client runtime and can be slow, so run the same command again; if it times out twice, check %s",
			name, timeout, domain.Bold("cplt doctor"))
	}
	return string(out), err
}

// checkStagedRuntime is the runtime gate a staged launch passes before it
// builds anything: the cplt floor always, the declared compatibility range when
// there is one. An empty compatibility means the manifest declares no range, so
// there is nothing to enforce and nothing to probe.
func checkStagedRuntime(client, compatibility string) error {
	if err := checkCpltFloor(); err != nil {
		return err
	}
	if compatibility == "" {
		return nil
	}
	return checkClientCompatibility(client, compatibility)
}

// pakkeCompatibility returns the client version range the active agentpakke
// declares, or "" when it declares none.
func pakkeCompatibility(client string) string {
	entry, ok := source.ActivePakke().Client(client)
	if !ok {
		return ""
	}
	return entry.Compatibility
}

const cpltUpgradeHint = "brew upgrade cplt"

// checkCpltFloor refuses a staged launch on a cplt older than the reviewed
// baseline, or on one whose version cannot be read at all. Mirrors the
// reference's fatal check_cplt (grillmester.py lines 987-1005).
func checkCpltFloor() error {
	out, err := probeCpltVersion()
	if err != nil {
		return fmt.Errorf(
			"could not read the cplt version, which a staged agentpakke launch requires to be %s or newer: %w\n\n  Upgrade it: %s",
			minStagedCpltStamp, err, domain.Bold(cpltUpgradeHint))
	}
	stamp := cpltStamp(out)
	if stamp == "" {
		return fmt.Errorf(
			"could not read a cplt version from %q — a staged agentpakke launch requires cplt %s or newer.\n\n  Upgrade it: %s",
			strings.TrimSpace(out), minStagedCpltStamp, domain.Bold(cpltUpgradeHint))
	}
	if stamp < minStagedCpltStamp {
		return fmt.Errorf(
			"cplt %s is older than %s, the reviewed baseline a staged agentpakke launch requires.\n\n  Upgrade it: %s",
			stamp, minStagedCpltStamp, domain.Bold(cpltUpgradeHint))
	}
	return nil
}

// cpltStampPattern is the comparable part of a cplt release, the stamp group of
// the reference's CPLT_VERSION_PATTERN (grillmester.py lines 94-96).
var cpltStampPattern = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}-\d{6}$`)

// cpltStamp returns the comparable date-time stamp of a `cplt --version` line,
// or "" when the output carries no version this gate can compare.
func cpltStamp(out string) string {
	stamp := artifacts.VersionTimestamp(ParseCpltVersion(out))
	if !cpltStampPattern.MatchString(stamp) {
		return ""
	}
	return stamp
}

// cpltVersionPattern is the reference's CPLT_VERSION_PATTERN (grillmester.py
// lines 94-96): a whole line, "cplt <stamp>-<commit>", anchored at both ends.
var cpltVersionPattern = regexp.MustCompile(`^cplt (\d{4}\.\d{2}\.\d{2}-\d{6}-[0-9a-f]{7,40})$`)

// ParseCpltVersion pulls the release out of `cplt --version` output, which
// reads "cplt 2026.08.24-153138-0d1d66d". Anything that is not a comparable
// version ("unknown", "dev", a future format) yields "" — unknown, so callers
// cannot mistake it for "up to date".
//
// It matches the anchored pattern against the first line, where the version is.
// It used to take the last whitespace-separated token of the entire output,
// which trusts whatever cplt printed last: a future cplt that appends an update
// hint ("newest release: 2027.01.01-000000-abcdef0") would hand that hint's
// token back as the installed version, and an arbitrarily old binary would
// clear the staged floor (#462 review, finding 6).
//
// Hardened here rather than at the staged call site because all three callers —
// this gate's floor, the update skew check and doctor, both through aliases.go
// — feed it the output of the same `cplt --version`. A strict parse local to
// the gate would fix one of the three and leave the other two reading the last
// token of anything.
//
// It lives here, not in internal/cli where the update/doctor skew checks first
// needed it, because internal/cli imports internal/provider and not the other
// way round: the staged gate could not have borrowed it in the other direction.
// internal/cli keeps calling it under its old unexported name via aliases.go.
func ParseCpltVersion(out string) string {
	first, _, _ := strings.Cut(out, "\n")
	m := cpltVersionPattern.FindStringSubmatch(strings.TrimSpace(first))
	if m == nil {
		return ""
	}
	return m[1]
}

// checkClientCompatibility refuses a staged launch whose client falls outside
// the range the agentpakke declares — and equally one whose version cannot be
// established, since an unenforceable declaration is not an enforced one.
func checkClientCompatibility(client, compatibility string) error {
	pakke := source.ActivePakke().Name
	rng, err := agentpakke.ParseVersionRange(compatibility)
	if err != nil {
		return fmt.Errorf("agentpakke %q declares an unusable compatibility range %q for %s: %w", pakke, compatibility, client, err)
	}
	out, err := probeClientVersion(client)
	if err != nil {
		return fmt.Errorf("could not read the %s version, which agentpakke %q requires to be %s: %w", client, pakke, compatibility, err)
	}
	found, err := parseClientVersion(client, out)
	if err != nil {
		return fmt.Errorf("could not read the %s version, which agentpakke %q requires to be %s: %w", client, pakke, compatibility, err)
	}
	if !rng.Contains(found) {
		// Which way to move the version depends on which bound failed, and
		// how depends on how the client was installed — doctor knows both
		// halves, so it is the command named here (#504 U6), the way the cplt
		// floor above names brew.
		return fmt.Errorf("%s %s is outside %s, the range agentpakke %q declares it supports.\n\n  Diagnose the install:  %s",
			client, found, compatibility, pakke, domain.Bold("nav-pilot doctor"))
	}
	return nil
}

// semver3 and the compatibility-range grammar live in internal/agentpakke,
// the package that owns the contract, so `nav-pilot validate` rejects with
// exactly the grammar this gate enforces (#504 U2). Aliased here because this
// gate and the client version parsing below are their main consumers.
type semver3 = agentpakke.Semver3

// Client version-line patterns, transcribed from the reference's
// OPENCODE_VERSION_PATTERN and COPILOT_VERSION_PATTERN (grillmester.py lines
// 78-92). The reference matches prereleases and then refuses them; these just
// do not match, which lands in the same fatal branch with less code.
const semverCorePattern = `(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)`

// copilotBuildSuffixPattern is a deliberate divergence from the reference.
//
// The copilot that cplt ships today prints "GitHub Copilot CLI 1.0.81-14." —
// 1.0.81, build 14. Semver reads "-14" as a prerelease and the reference
// refuses every prerelease outright (_semantic_version, grillmester.py lines
// 941-947). Transcribed literally that makes the copilot arm of this gate
// unsatisfiable: the contract's own documented example range for copilot,
// ">=1.0.79,<2" (README.agentpakke.md), can never be met by the binary the
// contract is about, so every copilot agentpakke declaring a range is a brick.
//
// So: a numeric-only suffix is accepted and ignored, and the comparison runs on
// the major.minor.patch triple in front of it. A build number carries no
// ordering information the range grammar can use, and dropping it cannot make
// an out-of-range version look in-range. Genuine prereleases keep being refused
// exactly as the reference refuses them — "-next.3", "-beta", "-rc.1" do not
// match, and land in the same fatal branch.
const copilotBuildSuffixPattern = `(?:-\d+)?`

var (
	openCodeVersionPattern = regexp.MustCompile(`(?i)^(?:OpenCode(?: version)? )?` + semverCorePattern + `$`)
	copilotVersionPattern  = regexp.MustCompile(`(?i)^(?:GitHub Copilot CLI(?: version)? )?` + semverCorePattern + copilotBuildSuffixPattern + `\.?$`)
)

// copilotUpdateHint is the one extra line the reference tolerates in copilot's
// version output (grillmester.py line 93, applied at lines 921-928).
const copilotUpdateHint = "Run 'copilot update' to check for updates."

// parseClientVersion reads exactly one strict version line out of a probe's
// stdout, the way the reference's _strict_client_version_output does.
func parseClientVersion(client, out string) (semver3, error) {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || (client == "copilot" && line == copilotUpdateHint) {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) != 1 {
		return semver3{}, fmt.Errorf("expected one version line in the %s version probe output, got %d", client, len(lines))
	}
	pattern := openCodeVersionPattern
	if client == "copilot" {
		pattern = copilotVersionPattern
	}
	m := pattern.FindStringSubmatch(lines[0])
	if m == nil {
		return semver3{}, fmt.Errorf("unrecognised %s version line %q", client, lines[0])
	}
	var v semver3
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return semver3{}, fmt.Errorf("unrecognised %s version line %q: %w", client, lines[0], err)
		}
		v[i] = n
	}
	return v, nil
}
