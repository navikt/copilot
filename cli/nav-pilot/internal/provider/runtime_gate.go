package provider

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

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

// stagedProbeTimeout bounds every version probe a staged launch spawns. Same
// cap as isCplt: a hanging binary must not hang the launch.
const stagedProbeTimeout = 2 * time.Second

// probeCpltVersion runs a bounded `cplt --version`. A package-level variable so
// tests can exercise the gate without spawning a binary.
var probeCpltVersion = func() (string, error) {
	path, err := stagedCpltPath()
	if err != nil {
		return "", err
	}
	return runStagedProbe(path, "--version")
}

// probeClientVersion runs a bounded version probe for the client a staged
// launch is about to start.
//
// opencode is probed directly: LaunchOpenCodeStaged already requires the binary
// on PATH. copilot is not — only cplt is — so it is probed sandboxed, the way
// the reference does in _sandboxed_client_version (grillmester.py lines
// 883-905): one `cplt --no-audit --agent copilot -- --version`.
var probeClientVersion = func(client string) (string, error) {
	if client == "opencode" {
		return runStagedProbe("opencode", "--version")
	}
	path, err := stagedCpltPath()
	if err != nil {
		return "", err
	}
	return runStagedProbe(path, "--no-audit", "--agent", client, "--", "--version")
}

func stagedCpltPath() (string, error) {
	path, name := FindCopilotCLI()
	if path == "" || name != "cplt" {
		return "", errors.New("cplt not found in PATH")
	}
	return path, nil
}

func runStagedProbe(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stagedProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
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

// ParseCpltVersion pulls the bare version out of `cplt --version` output, which
// reads "cplt 2026.08.24-153138-0d1d66d". Anything that is not a comparable
// version ("unknown", "dev", a future format) yields "" — unknown, so callers
// cannot mistake it for "up to date".
//
// It lives here, not in internal/cli where the update/doctor skew checks first
// needed it, because internal/cli imports internal/provider and not the other
// way round: the staged gate could not have borrowed it in the other direction.
// internal/cli keeps calling it under its old unexported name via aliases.go.
func ParseCpltVersion(out string) string {
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return ""
	}
	v := fields[len(fields)-1]
	if !artifacts.VersionParseable(v) {
		return ""
	}
	return v
}

// checkClientCompatibility refuses a staged launch whose client falls outside
// the range the agentpakke declares — and equally one whose version cannot be
// established, since an unenforceable declaration is not an enforced one.
func checkClientCompatibility(client, compatibility string) error {
	pakke := source.ActivePakke().Name
	rng, err := parseVersionRange(compatibility)
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
	if !rng.contains(found) {
		return fmt.Errorf("%s %s is outside %s, the range agentpakke %q declares it supports", client, found, compatibility, pakke)
	}
	return nil
}

// semver3 is a major.minor.patch triple. The contract's compatibility grammar
// has no ordering over prereleases and the reference refuses them outright, so
// three integers is the whole model.
type semver3 [3]int

func (v semver3) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

func (v semver3) compare(o semver3) int {
	for i := range v {
		switch {
		case v[i] < o[i]:
			return -1
		case v[i] > o[i]:
			return 1
		}
	}
	return 0
}

// comparator is one clause of a compatibility range, e.g. ">=1.18.20".
type comparator struct {
	op string
	v  semver3
}

// versionRange is the conjunction of every comparator in a compatibility
// string: all must hold.
type versionRange []comparator

// comparatorOps are the operators the contract documents, longest first so
// ">=" is not read as ">" followed by a bad operand.
var comparatorOps = []string{">=", "<=", ">", "<", "="}

// parseVersionRange parses the range grammar README.agentpakke.md documents:
// comma-separated comparators over semver, e.g. ">=1.18.20,<2". Operands may be
// partial ("2" means 2.0.0) — that is what makes an upper major bound writable.
func parseVersionRange(s string) (versionRange, error) {
	var rng versionRange
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty comparator in %q", s)
		}
		op := ""
		for _, candidate := range comparatorOps {
			if strings.HasPrefix(part, candidate) {
				op = candidate
				break
			}
		}
		if op == "" {
			return nil, fmt.Errorf("comparator %q has no operator (expected one of >= > <= < =)", part)
		}
		v, err := parseVersionOperand(strings.TrimSpace(part[len(op):]))
		if err != nil {
			return nil, fmt.Errorf("comparator %q: %w", part, err)
		}
		rng = append(rng, comparator{op: op, v: v})
	}
	return rng, nil
}

var versionPartPattern = regexp.MustCompile(`^(0|[1-9]\d*)$`)

// parseVersionOperand parses one to three dot-separated numeric parts, filling
// the missing ones with zero.
func parseVersionOperand(s string) (semver3, error) {
	var v semver3
	if s == "" {
		return v, errors.New("missing version")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return v, fmt.Errorf("version %q has more than three parts", s)
	}
	for i, part := range parts {
		if !versionPartPattern.MatchString(part) {
			return semver3{}, fmt.Errorf("version %q has a non-numeric part %q", s, part)
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return semver3{}, fmt.Errorf("version %q: %w", s, err)
		}
		v[i] = n
	}
	return v, nil
}

func (r versionRange) contains(v semver3) bool {
	for _, c := range r {
		cmp := v.compare(c.v)
		ok := false
		switch c.op {
		case ">=":
			ok = cmp >= 0
		case ">":
			ok = cmp > 0
		case "<=":
			ok = cmp <= 0
		case "<":
			ok = cmp < 0
		case "=":
			ok = cmp == 0
		}
		if !ok {
			return false
		}
	}
	return true
}

// Client version-line patterns, transcribed from the reference's
// OPENCODE_VERSION_PATTERN and COPILOT_VERSION_PATTERN (grillmester.py lines
// 78-92). The reference matches prereleases and then refuses them; these just
// do not match, which lands in the same fatal branch with less code.
const semverCorePattern = `(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)`

var (
	openCodeVersionPattern = regexp.MustCompile(`(?i)^(?:OpenCode(?: version)? )?` + semverCorePattern + `$`)
	copilotVersionPattern  = regexp.MustCompile(`(?i)^(?:GitHub Copilot CLI(?: version)? )?` + semverCorePattern + `\.?$`)
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
