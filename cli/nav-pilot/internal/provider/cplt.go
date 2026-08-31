package provider

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// cpltLaunch describes how to launch a coding-agent client inside the cplt
// kernel-level sandbox via `cplt --agent <agent> -- <agentArgs>`.
//
// All clients (copilot, opencode, pi, …) are launched through cplt so the agent
// runs sandboxed and consistently — it can read/write project files but cannot
// reach SSH keys, cloud credentials, or other secrets.
type cpltLaunch struct {
	// agent is the cplt --agent value selecting which agent to sandbox
	// (e.g. "copilot", "opencode", "pi").
	agent string
	// noAudit emits cplt's --no-audit ahead of "--agent", as the reference
	// launcher does (grillmester.py line 663). Set by staged Tier 2 launches
	// only; false for every legacy launch, which keeps their argument vectors
	// byte-identical (golden_launch_test.go, cplt_test.go).
	noAudit bool
	// cpltArgs are cplt-level flags placed between "--agent <agent>" and the
	// "--" separator (e.g. --allow-read, --pass-env). Empty for every legacy
	// launch, which keeps their argument vectors byte-identical
	// (golden_launch_test.go, cplt_test.go).
	cpltArgs []string
	// agentArgs are forwarded to the agent process after the "--" separator.
	agentArgs []string
	// env is the process environment. nil inherits the parent environment.
	env []string
	// displayName is the user-facing client name for launch/log messages.
	displayName string
	// messageSuffix is appended to the "Launching …" line (e.g. nav-context summary).
	messageSuffix string
}

// cpltArgv is the argument vector launchViaCplt passes to cplt:
// `[--no-audit] --agent <agent> [cpltArgs...] -- [agentArgs...]`. Pure, so the
// vector is testable without launching anything. With noAudit false and no
// cpltArgs it is byte-identical to what every legacy launch produced before
// Tier 2 staging existed.
func cpltArgv(spec cpltLaunch) []string {
	var args []string
	if spec.noAudit {
		args = append(args, "--no-audit")
	}
	args = append(args, "--agent", spec.agent)
	args = append(args, spec.cpltArgs...)
	args = append(args, "--")
	return append(args, spec.agentArgs...)
}

// withCpltConfirmation prefixes cplt's --yes when no terminal can answer the
// launch confirmation cplt asks before it starts an agent. Without it cplt
// stops with "No TTY available for confirmation. Use --yes for non-interactive
// runs." and exits 1, so every launch from a script, a pipe or a dispatched
// task died before the agent ran. Detected rather than flagged: nav-pilot
// already decides interactivity by looking at stdin everywhere it matters, and
// a flag would make the supported path opt-in to something the process can see
// for itself — while everyone who did not pass it kept the hard error.
//
// --yes and nothing else. It skips the prompt and changes nothing else: cplt
// still prints the sandbox configuration summary, in its own words, "for
// auditability", and every restriction — the gh guard, the git guard, the
// filesystem and network policy — is untouched. The reference launcher splices
// --yes and --quiet together (grillmester.py line 879), and nav-pilot's own
// version probe copies that pair, but --quiet is wrong here: it suppresses the
// post-session change audit (`cplt --help` under --no-audit: "The report is
// also suppressed under --quiet") along with the configuration summary, and a
// launch nobody is watching is the one whose audit is worth the most.
//
// Pure, and takes the terminal state rather than reading it, so a test can pin
// both vectors — and so the interactive one stays exactly what it is today.
func withCpltConfirmation(args []string, tty bool) []string {
	if tty {
		return args
	}
	return append([]string{"--yes"}, args...)
}

// launchViaCplt runs the given client agent inside the cplt sandbox, wiring
// stdio to the current process. cplt is required: if it is not found on PATH the
// launch fails with guidance instead of falling back to an unsandboxed binary.
func launchViaCplt(spec cpltLaunch) error {
	cliPath, cliName := FindCopilotCLI()
	if cliPath == "" || cliName != "cplt" {
		telemetryRecorder.RecordLaunchError(spec.agent, "client_not_found")
		return fmt.Errorf("cplt not found in PATH — nav-pilot launches clients inside the cplt sandbox; install cplt to launch %s", spec.displayName)
	}

	args := withCpltConfirmation(cpltArgv(spec), IsTerminal(os.Stdin))

	fmt.Printf("Launching %s via %s%s...\n\n",
		domain.Bold(spec.displayName), domain.Bold("cplt sandbox"), spec.messageSuffix)

	cmd := exec.Command(cliPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = spec.env

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "%s Could not launch %s via cplt: %v\n", domain.Yellow("⚠"), spec.displayName, err)
		}
		if kind := classifyLaunchError(err); kind != "" {
			telemetryRecorder.RecordLaunchError(spec.agent, kind)
		}
		return err
	}
	return nil
}

// classifyLaunchError maps a launch error to a normalized error_type label for
// nav_pilot_launch_error_total. "" means do not record this at all.
//
// The distinction that matters: did the client start? cmd.Run returns an
// *exec.ExitError whenever the launched client exits non-zero, and that
// includes a developer pressing Ctrl-C. Classifying every ExitError as
// launch_failed made a counter described as "client launch failures" mostly a
// count of normal session endings, so the panel measured how much people used
// the tool and called it a failure rate.
//
// A signalled exit is the developer quitting, which is not an event worth a
// data point at all. Any other non-zero exit is the client failing after it
// started, which is worth knowing and is not a launch failure either — the real
// launch failure is the one where nothing ever ran.
func classifyLaunchError(err error) string {
	if err == nil {
		return ""
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return ""
		}
		// 128+n is how a shell reports a signalled child, and some clients pass
		// their own child's status through that way rather than being signalled
		// themselves. 130 is SIGINT, 143 is SIGTERM.
		if code := exitErr.ExitCode(); code == 130 || code == 143 {
			return ""
		}
		return "client_exit"
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "client_not_found"
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return "client_not_found"
	}
	return "unknown"
}
