package provider

import (
	"strings"
	"testing"
)

// A local session's prompt path is a 127.0.0.1 hop to the loop guard, which
// cplt blocks by default. The port has to reach cplt as `--allow-localhost`, or
// local mode works only for users who turned on the machine-wide
// allow_localhost_any toggle — which cplt's strict preset then supersedes,
// making strict and `nav-pilot local` mutually exclusive.
func TestLocalLaunchNamesTheGuardPortToCplt(t *testing.T) {
	args := withCpltAllowLocalhost([]string{"--agent", "opencode"}, 51234)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--allow-localhost 51234") {
		t.Fatalf("the guard port never reaches cplt: %v", args)
	}
	// It is a cplt-level flag, so it must precede the agent selection the way
	// --yes does, never land in the agent's own argument list.
	if args[0] != "--allow-localhost" {
		t.Errorf("flag is not in cplt's own argument position: %v", args)
	}
}

// One named port, never the machine-wide relaxation: allow_localhost_any is
// what disables cplt's kernel net-connect restriction and what forced-proxy
// egress supersedes. Reaching for it would undo the preset being recommended.
func TestLocalLaunchDoesNotRelaxAllLocalhost(t *testing.T) {
	args := withCpltAllowLocalhost([]string{"--agent", "copilot"}, 51234)
	if strings.Contains(strings.Join(args, " "), "allow-localhost-any") {
		t.Errorf("used the machine-wide toggle instead of one port: %v", args)
	}
}

// No guard, no flag: a cloud session must launch byte-identically to before.
func TestNonLocalLaunchIsUnchanged(t *testing.T) {
	base := []string{"--agent", "copilot", "--", "--model", "x"}
	if got := withCpltAllowLocalhost(base, 0); len(got) != len(base) {
		t.Errorf("a cloud launch grew an argument: %v", got)
	}
}

// The flag has to survive into the vector cplt is actually handed, in the
// cplt-flag slot between the agent selection and the `--` separator.
func TestGuardPortReachesTheLaunchVector(t *testing.T) {
	argv := cpltArgv(cpltLaunch{
		agent:     "opencode",
		cpltArgs:  withCpltAllowLocalhost(nil, 51234),
		agentArgs: []string{"run"},
	})
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--agent opencode --allow-localhost 51234 --") {
		t.Errorf("flag is not in the cplt-flag slot: %v", argv)
	}
}
