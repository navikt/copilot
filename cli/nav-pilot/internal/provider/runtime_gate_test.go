package provider

import (
	"errors"
	"os"
	"strings"
	"testing"
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

func TestParseVersionRange(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  []comparator
		isErr bool
	}{
		{name: "contract example", in: ">=1.18.20,<2", want: []comparator{
			{op: ">=", v: semver3{1, 18, 20}},
			{op: "<", v: semver3{2, 0, 0}},
		}},
		{name: "spaces are tolerated", in: " >= 1.0.79 , < 2 ", want: []comparator{
			{op: ">=", v: semver3{1, 0, 79}},
			{op: "<", v: semver3{2, 0, 0}},
		}},
		{name: "exact", in: "=1.2.3", want: []comparator{{op: "=", v: semver3{1, 2, 3}}}},
		{name: "two-part operand zero-fills", in: "<=1.18", want: []comparator{{op: "<=", v: semver3{1, 18, 0}}}},
		{name: "greater than", in: ">1.0.0", want: []comparator{{op: ">", v: semver3{1, 0, 0}}}},

		{name: "empty", in: "", isErr: true},
		{name: "no operator", in: "1.2.3", isErr: true},
		{name: "operator without operand", in: ">=", isErr: true},
		{name: "trailing comma", in: ">=1.0.0,", isErr: true},
		{name: "leading comma", in: ",<2", isErr: true},
		{name: "non-numeric part", in: ">=1.x.3", isErr: true},
		{name: "four parts", in: ">=1.2.3.4", isErr: true},
		{name: "prerelease operand", in: ">=1.2.3-beta", isErr: true},
		{name: "leading zero", in: ">=01.2.3", isErr: true},
		{name: "caret is not an operator", in: "^1.2.3", isErr: true},
		{name: "double equals", in: "==1.2.3", isErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVersionRange(tc.in)
			if tc.isErr {
				if err == nil {
					t.Fatalf("parseVersionRange(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVersionRange(%q) errored: %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseVersionRange(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseVersionRange(%q)[%d] = %v, want %v", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestVersionRangeContains(t *testing.T) {
	tests := []struct {
		rng     string
		version semver3
		want    bool
	}{
		{">=1.18.20,<2", semver3{1, 18, 20}, true},  // lower bound is inclusive
		{">=1.18.20,<2", semver3{1, 18, 19}, false}, // one patch below
		{">=1.18.20,<2", semver3{1, 19, 0}, true},
		{">=1.18.20,<2", semver3{2, 0, 0}, false}, // upper bound is exclusive
		{">=1.18.20,<2", semver3{1, 99, 99}, true},
		{">=1.18.20,<2", semver3{0, 99, 99}, false},
		{">1.0.0", semver3{1, 0, 0}, false},
		{">1.0.0", semver3{1, 0, 1}, true},
		{"<=1.0.0", semver3{1, 0, 0}, true},
		{"=1.2.3", semver3{1, 2, 3}, true},
		{"=1.2.3", semver3{1, 2, 4}, false},
		{"=1.2", semver3{1, 2, 0}, true},
	}
	for _, tc := range tests {
		rng, err := parseVersionRange(tc.rng)
		if err != nil {
			t.Fatalf("parseVersionRange(%q): %v", tc.rng, err)
		}
		if got := rng.contains(tc.version); got != tc.want {
			t.Errorf("%q contains %v = %v, want %v", tc.rng, tc.version, got, tc.want)
		}
	}
}

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
