package provider

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

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
	// agentArgs are forwarded to the agent process after the "--" separator.
	agentArgs []string
	// env is the process environment. nil inherits the parent environment.
	env []string
	// displayName is the user-facing client name for launch/log messages.
	displayName string
	// messageSuffix is appended to the "Launching …" line (e.g. nav-context summary).
	messageSuffix string
}

// launchViaCplt runs the given client agent inside the cplt sandbox, wiring
// stdio to the current process. cplt is required: if it is not found on PATH the
// launch fails with guidance instead of falling back to an unsandboxed binary.
func launchViaCplt(spec cpltLaunch) error {
	cliPath, cliName := FindCopilotCLI()
	if cliPath == "" || cliName != "cplt" {
		return fmt.Errorf("cplt not found in PATH — nav-pilot launches clients inside the cplt sandbox; install cplt to launch %s", spec.displayName)
	}

	args := append([]string{"--agent", spec.agent, "--"}, spec.agentArgs...)

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
		return err
	}
	return nil
}
