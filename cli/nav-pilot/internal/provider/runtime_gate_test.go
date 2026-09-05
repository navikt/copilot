package provider

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// stubProbes replaces both staged version probes for the duration of a test, so
// no real binary is ever spawned.
func stubProbes(t *testing.T, cplt string, cpltErr error, client string, clientErr error) {
	t.Helper()
	origCplt, origClient := probeCpltVersion, probeClientVersion
	probeCpltVersion = func() (string, error) { return cplt, cpltErr }
	probeClientVersion = func(string) (string, error) { return client, clientErr }
	t.Cleanup(func() { probeCpltVersion, probeClientVersion = origCplt, origClient })
}

// okCplt is a cplt release comfortably past minStagedCpltStamp.
const okCplt = "cplt 2026.08.24-153138-0d1d66d\n"

func TestParseClientVersion(t *testing.T) {
	tests := []struct {
		name   string
		client string
		out    string
		want   semver3
		isErr  bool
	}{
		{name: "bare opencode semver", client: "opencode", out: "1.18.20\n", want: semver3{1, 18, 20}},
		{name: "opencode prefix", client: "opencode", out: "opencode 1.18.20\n", want: semver3{1, 18, 20}},
		{name: "opencode version prefix", client: "opencode", out: "OpenCode version 1.18.20", want: semver3{1, 18, 20}},
		{name: "bare copilot semver", client: "copilot", out: "1.0.79\n", want: semver3{1, 0, 79}},
		{name: "copilot prefix and trailing dot", client: "copilot", out: "GitHub Copilot CLI 1.0.79.\n", want: semver3{1, 0, 79}},
		{name: "copilot update hint is tolerated", client: "copilot",
			out:  "1.0.79\n" + copilotUpdateHint + "\n",
			want: semver3{1, 0, 79}},

		{name: "empty output", client: "opencode", out: "", isErr: true},
		{name: "two version lines", client: "opencode", out: "1.2.3\n1.2.4\n", isErr: true},
		{name: "prerelease", client: "opencode", out: "1.2.3-beta.1\n", isErr: true},
		{name: "not a version", client: "opencode", out: "command not found\n", isErr: true},
		{name: "copilot noise beyond the hint", client: "copilot", out: "1.0.79\nsomething else\n", isErr: true},
		{name: "opencode does not get the copilot hint", client: "opencode",
			out: "1.18.20\n" + copilotUpdateHint + "\n", isErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseClientVersion(tc.client, tc.out)
			if tc.isErr {
				if err == nil {
					t.Fatalf("parseClientVersion(%q, %q) = %v, want error", tc.client, tc.out, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseClientVersion(%q, %q) errored: %v", tc.client, tc.out, err)
			}
			if got != tc.want {
				t.Errorf("parseClientVersion(%q, %q) = %v, want %v", tc.client, tc.out, got, tc.want)
			}
		})
	}
}

func TestCpltStamp(t *testing.T) {
	tests := []struct {
		out  string
		want string
	}{
		{"cplt 2026.08.24-153138-0d1d66d\n", "2026.08.24-153138"},
		{"cplt 2026.08.17-062831-1008a92", "2026.08.17-062831"},
		{"cplt unknown\n", ""},
		{"cplt dev", ""},
		{"", ""},
		{"2026.08.24\n", ""},          // no time component to compare
		{"cplt 2026.8.24-153138", ""}, // not zero-padded, so not lexicographically comparable
	}
	for _, tc := range tests {
		if got := cpltStamp(tc.out); got != tc.want {
			t.Errorf("cpltStamp(%q) = %q, want %q", tc.out, got, tc.want)
		}
	}
}

func TestCheckStagedRuntimeCpltFloor(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		err     error
		wantErr string // substring the message must carry, "" means no error
	}{
		{name: "at the floor", out: "cplt " + minStagedCpltStamp + "-1008a92\n"},
		{name: "past the floor", out: okCplt},
		{name: "below the floor", out: "cplt 2026.08.16-235959-deadbee\n",
			wantErr: "2026.08.16-235959"},
		{name: "unparseable version", out: "cplt unknown\n", wantErr: "could not read a cplt version"},
		{name: "probe failed", err: errors.New("boom"), wantErr: "could not read the cplt version"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubProbes(t, tc.out, tc.err, "1.18.20\n", nil)
			err := checkStagedRuntime("opencode", "")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkStagedRuntime errored: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("checkStagedRuntime succeeded, want a fatal error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not name %q", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), minStagedCpltStamp) {
				t.Errorf("error %q does not name the required floor %q", err, minStagedCpltStamp)
			}
			if !strings.Contains(err.Error(), cpltUpgradeHint) {
				t.Errorf("error %q does not say how to upgrade", err)
			}
		})
	}
}

func TestCheckStagedRuntimeCompatibility(t *testing.T) {
	tests := []struct {
		name          string
		client        string
		compatibility string
		clientOut     string
		clientErr     error
		wantErr       string // substring, "" means the launch may proceed
	}{
		{name: "no compatibility declared means no gate", client: "opencode",
			compatibility: "", clientOut: "", clientErr: errors.New("never called")},
		{name: "in range", client: "opencode", compatibility: ">=1.18.20,<2", clientOut: "1.18.20\n"},
		{name: "in range sandboxed copilot", client: "copilot", compatibility: ">=1.0.79,<2",
			clientOut: "1.0.79\n" + copilotUpdateHint + "\n"},
		{name: "below the range", client: "opencode", compatibility: ">=1.18.20,<2",
			clientOut: "1.18.19\n", wantErr: "1.18.19"},
		{name: "above the range", client: "opencode", compatibility: ">=1.18.20,<2",
			clientOut: "2.0.0\n", wantErr: "2.0.0"},
		// The stub returns an in-range version *and* an error: a swallowed probe
		// error would otherwise be indistinguishable from unparseable output.
		{name: "probe failure is fatal", client: "opencode", compatibility: ">=1.18.20,<2",
			clientOut: "1.18.20\n", clientErr: errors.New("exit status 127"),
			wantErr: "exit status 127"},
		{name: "unparseable output is fatal", client: "opencode", compatibility: ">=1.18.20,<2",
			clientOut: "no version here\n", wantErr: "could not read the opencode version"},
		{name: "malformed range is fatal", client: "opencode", compatibility: "^1.2.3",
			clientOut: "1.2.3\n", wantErr: "unusable compatibility range"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubProbes(t, okCplt, nil, tc.clientOut, tc.clientErr)
			err := checkStagedRuntime(tc.client, tc.compatibility)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkStagedRuntime errored: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("checkStagedRuntime succeeded, want a fatal error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not name %q", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.compatibility) {
				t.Errorf("error %q does not name the required range %q", err, tc.compatibility)
			}
		})
	}
}

// TestPakkeCompatibilityReadsTheManifest pins the accessor the launch paths hand
// to the gate: declared range through, undeclared client silent.
func TestPakkeCompatibilityReadsTheManifest(t *testing.T) {
	m := stagedFixturePakke()
	entry := m.Clients["opencode"]
	entry.Compatibility = ">=1.18.20,<2"
	m.Clients["opencode"] = entry
	SetActivePakke(m)
	t.Cleanup(func() { SetActivePakke(nil) })

	if got := pakkeCompatibility("opencode"); got != ">=1.18.20,<2" {
		t.Errorf("pakkeCompatibility(opencode) = %q, want the declared range", got)
	}
	if got := pakkeCompatibility("copilot"); got != "" {
		t.Errorf("pakkeCompatibility(copilot) = %q, want \"\" when nothing is declared", got)
	}
	if got := pakkeCompatibility("pi"); got != "" {
		t.Errorf("pakkeCompatibility(pi) = %q, want \"\" for an undeclared client", got)
	}
}

// TestStagedLaunchesAreGated pins the wiring itself. Neither Launch*Staged can
// be called in a test — one needs opencode on PATH, both need cplt — so the
// call sites are asserted against the source. Losing the call would silently
// un-enforce eSyfo's requirement, which is exactly the regression worth a
// brittle-looking test.
func TestStagedLaunchesAreGated(t *testing.T) {
	src, err := os.ReadFile("staged_launch.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range []string{"func LaunchOpenCodeStaged", "func LaunchCopilotStaged"} {
		body, _, found := strings.Cut(string(src)[strings.Index(string(src), fn):], "\n}\n")
		if !found {
			t.Fatalf("could not isolate the body of %s", fn)
		}
		if !strings.Contains(body, "checkStagedRuntime(") {
			t.Errorf("%s does not call checkStagedRuntime — the staged runtime gate is not wired", fn)
		}
	}
}

// realCopilotProbeOutput is what `cplt --yes --quiet --no-audit --agent copilot
// --project-dir <empty dir> -- --version` actually printed on stdout, verbatim,
// against cplt 2026.08.26-203934-976b64f. Every stub in this file is a
// hypothesis; this one is a transcript.
const realCopilotProbeOutput = "GitHub Copilot CLI 1.0.81-14.\nRun 'copilot update' to check for updates.\n"

// TestParseClientVersionAcceptsTheRealCopilotOutput pins finding 2 of the #462
// review: the copilot cplt ships prints a numeric build suffix, and refusing it
// the way the reference does makes every copilot compatibility range — starting
// with the contract's own ">=1.0.79,<2" — impossible to satisfy.
func TestParseClientVersionAcceptsTheRealCopilotOutput(t *testing.T) {
	got, err := parseClientVersion("copilot", realCopilotProbeOutput)
	if err != nil {
		t.Fatalf("parseClientVersion rejected what cplt's copilot actually prints: %v", err)
	}
	if want := (semver3{1, 0, 81}); got != want {
		t.Fatalf("parseClientVersion = %v, want %v (the build suffix is dropped, not compared)", got, want)
	}
	rng, err := agentpakke.ParseVersionRange(">=1.0.79,<2")
	if err != nil {
		t.Fatal(err)
	}
	if !rng.Contains(got) {
		t.Errorf("%v is outside the contract's documented copilot range >=1.0.79,<2", got)
	}
}

// TestCopilotBuildSuffixIsNotAPrereleaseAmnesty keeps the divergence narrow:
// numeric build suffixes only, genuine prereleases still fatal.
func TestCopilotBuildSuffixIsNotAPrereleaseAmnesty(t *testing.T) {
	ok := []string{"1.0.81-14\n", "GitHub Copilot CLI 1.0.81-14.\n", "1.0.81-0\n"}
	for _, out := range ok {
		if _, err := parseClientVersion("copilot", out); err != nil {
			t.Errorf("parseClientVersion(copilot, %q) errored: %v", out, err)
		}
	}
	bad := []string{"1.19.0-next.3\n", "1.0.81-beta\n", "1.0.81-rc.1\n", "1.0.81-14.2\n", "1.0.81-\n"}
	for _, out := range bad {
		if v, err := parseClientVersion("copilot", out); err == nil {
			t.Errorf("parseClientVersion(copilot, %q) = %v, want a fatal error", out, v)
		}
	}
	// opencode keeps the reference's rule: no suffix of any kind.
	if v, err := parseClientVersion("opencode", "1.18.20-14\n"); err == nil {
		t.Errorf("parseClientVersion(opencode, \"1.18.20-14\") = %v, want a fatal error", v)
	}
}

// TestParseCpltVersionAnchorsOnTheFirstLine pins finding 6: the parse must not
// trust the last token of whatever cplt printed.
func TestParseCpltVersionAnchorsOnTheFirstLine(t *testing.T) {
	tests := []struct{ out, want string }{
		{"cplt 2026.08.26-203934-976b64f\n", "2026.08.26-203934-976b64f"}, // real output
		{"cplt 2026.08.24-153138-0d1d66d", "2026.08.24-153138-0d1d66d"},
		// A future cplt appending an update hint must not let the hint's
		// version stand in for the installed one.
		{"cplt 2026.01.01-000000-deadbee\nnewest release: 2027.01.01-000000-abcdef0\n", "2026.01.01-000000-deadbee"},
		{"newest release: 2027.01.01-000000-abcdef0\n", ""},
		{"cplt 2026.08.24-153138-0d1d66d extra", ""},
		{"cplt unknown", ""},
		{"cplt dev", ""},
		{"2026.08.24-153138-0d1d66d", ""}, // no "cplt " prefix: not a version line
		{"", ""},
	}
	for _, tc := range tests {
		if got := ParseCpltVersion(tc.out); got != tc.want {
			t.Errorf("ParseCpltVersion(%q) = %q, want %q", tc.out, got, tc.want)
		}
	}
	// And the floor it feeds refuses the appended-hint case rather than
	// promoting the hint's stamp.
	stubProbes(t, "cplt 2026.01.01-000000-deadbee\nnewest release: 2099.01.01-000000-abcdef0\n", nil, "", nil)
	err := checkStagedRuntime("opencode", "")
	if err == nil || !strings.Contains(err.Error(), "2026.01.01-000000") {
		t.Errorf("checkCpltFloor accepted an old cplt on the strength of an appended hint: %v", err)
	}
}

// TestCopilotProbeVector pins the argv itself, without spawning anything, so a
// vector regression fails on a machine that has no cplt installed — the gap
// that let the #462 review find a probe that never worked.
func TestCopilotProbeVector(t *testing.T) {
	fakeCpltOnPath(t)
	var gotName string
	var gotArgs []string
	var gotTimeout time.Duration
	orig := runStagedProbe
	runStagedProbe = func(timeout time.Duration, name string, args ...string) (string, error) {
		gotTimeout, gotName, gotArgs = timeout, name, args
		return "", nil
	}
	t.Cleanup(func() { runStagedProbe = orig })

	if _, err := probeClientVersion("copilot"); err != nil {
		t.Fatalf("probeClientVersion: %v", err)
	}
	if filepath.Base(gotName) != "cplt" {
		t.Errorf("probe ran %q, want cplt", gotName)
	}
	// The budget is asserted against the reference's number, not against the
	// constant: comparing a constant to itself would let any value pass. 30s is
	// _bounded_command_output's default (grillmester.py line 770), and the first
	// probe after a cplt upgrade needs it to unpack the client runtime.
	if gotTimeout != 30*time.Second {
		t.Errorf("client probe timeout = %s, want the reference's 30s", gotTimeout)
	}
	for _, flag := range []string{"--yes", "--quiet", "--no-audit", "--project-dir"} {
		if !slices.Contains(gotArgs, flag) {
			t.Errorf("probe vector %v is missing %s", gotArgs, flag)
		}
	}
	// --yes and --quiet come first, where the reference splices them in.
	if len(gotArgs) < 2 || gotArgs[0] != "--yes" || gotArgs[1] != "--quiet" {
		t.Errorf("probe vector %v does not start with --yes --quiet", gotArgs)
	}
	// --project-dir must point at an empty directory that is not the cwd, and
	// it must be cleaned up once the probe returns.
	i := slices.Index(gotArgs, "--project-dir")
	if i < 0 || i+1 >= len(gotArgs) {
		t.Fatalf("probe vector %v has no --project-dir operand", gotArgs)
	}
	dir := gotArgs[i+1]
	cwd, _ := os.Getwd()
	if dir == cwd || dir == "." || dir == "" {
		t.Errorf("--project-dir %q is the working directory; the probe must be isolated", dir)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("probe directory %q survived the probe, want it removed", dir)
	}
	// The client sees only --version, after the separator.
	if j := slices.Index(gotArgs, "--"); j < 0 || !slices.Equal(gotArgs[j+1:], []string{"--version"}) {
		t.Errorf("probe vector %v does not end in `-- --version`", gotArgs)
	}
}

// fakeCpltOnPath puts an inert executable named cplt at the front of PATH, so
// vector tests resolve a binary without any real cplt being installed.
func fakeCpltOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestRunStagedProbeTimeoutIsReal is the teeth of finding 3, in the shape cplt
// actually has: the process nav-pilot spawned is still alive while a
// grandchild holds the stdout pipe open for 30 seconds. Signalling only the
// direct child leaves Output() blocked on that pipe — the hang the reviewer
// reproduced against a 2-second context.
//
// The assertion is the elapsed time, not just the error: with the process-group
// kill removed this measures 2.3s (WaitDelay closing the pipes, the grandchild
// left running) against 0.3s with it. The whole test is bounded twice over so a
// regression fails in seconds rather than stalling the suite.
func TestRunStagedProbeTimeoutIsReal(t *testing.T) {
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, err := runStagedProbe(300*time.Millisecond, "sh", "-c", "sleep 30 & wait")
		done <- err
	}()
	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("runStagedProbe succeeded, want a timeout error")
		}
		if !strings.Contains(err.Error(), "still running after") {
			t.Errorf("timeout error %q does not say what happened", err)
		}
		if !strings.Contains(err.Error(), "again") {
			t.Errorf("timeout error %q does not tell the user to retry", err)
		}
		if elapsed > 1500*time.Millisecond {
			t.Errorf("runStagedProbe took %s for a 300ms deadline: the deadline is not killing the process group, only closing pipes", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runStagedProbe hung well past its 300ms deadline: the timeout bounds nothing")
	}
}

// TestRealCopilotProbe runs the actual probe against the installed cplt. It is
// the check every stub in this file is blind to: a probe vector that cannot
// work on a real machine. Skipped where cplt is not installed (CI), which is
// why TestCopilotProbeVector pins the argv separately.
func TestRealCopilotProbe(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns the real cplt")
	}
	if _, err := stagedCpltPath(); err != nil {
		t.Skip("cplt is not installed")
	}
	out, err := probeClientVersion("copilot")
	if err != nil {
		t.Fatalf("the real copilot probe failed: %v\nstdout: %q", err, out)
	}
	v, err := parseClientVersion("copilot", out)
	if err != nil {
		t.Fatalf("could not parse the real copilot probe output %q: %v", out, err)
	}
	t.Logf("real copilot probe: %q -> %v", out, v)
}

// TestRealCpltFloor runs the real `cplt --version` through the floor check.
// TestCpltProbeBudget pins the other half of the reference's budgets.
func TestCpltProbeBudget(t *testing.T) {
	fakeCpltOnPath(t)
	var gotTimeout time.Duration
	orig := runStagedProbe
	runStagedProbe = func(timeout time.Duration, name string, args ...string) (string, error) {
		gotTimeout = timeout
		return "", nil
	}
	t.Cleanup(func() { runStagedProbe = orig })
	if _, err := probeCpltVersion(); err != nil {
		t.Fatalf("probeCpltVersion: %v", err)
	}
	// _trusted_cplt_version_output, grillmester.py line 741.
	if gotTimeout != 8*time.Second {
		t.Errorf("cplt probe timeout = %s, want the reference's 8s", gotTimeout)
	}
}

func TestRealCpltFloor(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns the real cplt")
	}
	if _, err := stagedCpltPath(); err != nil {
		t.Skip("cplt is not installed")
	}
	out, err := probeCpltVersion()
	if err != nil {
		t.Fatalf("the real cplt version probe failed: %v", err)
	}
	if stamp := cpltStamp(out); stamp == "" {
		t.Fatalf("could not read a stamp from the real cplt output %q", out)
	}
	t.Logf("real cplt probe: %q", strings.TrimSpace(out))
}

// An out-of-range client version must name a way forward (#504 U6), the way
// the cplt floor right above it names its upgrade command.
func TestOutOfRangeClientVersionNamesACommand(t *testing.T) {
	stubProbes(t, okCplt, nil, "1.18.19\n", nil)
	err := checkStagedRuntime("opencode", ">=1.18.20,<2")
	if err == nil {
		t.Fatal("checkStagedRuntime accepted an out-of-range version")
	}
	if !strings.Contains(err.Error(), "nav-pilot doctor") {
		t.Errorf("out-of-range refusal %q names no command to run", err)
	}
}
