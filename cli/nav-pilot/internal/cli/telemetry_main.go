package cli

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"
)

func telemetryMode() string {
	if isInteractive() {
		return "interactive"
	}
	return "non_interactive"
}

// telemetrySendCommand is the hidden command the detached sender runs.
const telemetrySendCommand = "__telemetry-send"

// flushTelemetry ends telemetry. It writes the last export to a spool file
// and never waits for the network: a detached child sends the file right
// after exit, and the next nav-pilot sends whatever the child did not (see
// telemetry/spool.go).
func flushTelemetry(t telemetryRecorder) {
	_ = t.Shutdown(context.Background())
	if s, ok := t.(interface{ Spooled() bool }); ok && s.Spooled() && telemetrySenderAllowed() {
		spawnTelemetrySender()
	}
}

// telemetrySenderAllowed reports whether exit may start the detached sender.
// Where it may not, the spool waits for the next run.
//   - Inside cplt's sandbox: cplt's audit flags a process that escapes the
//     session with setsid. Other agent sandboxes are not detected; there the
//     child at worst fails to reach the network and the file waits.
//   - In the e2e build, unless a test asks for it: the budget test and the
//     journeys would leave senders running into their temp homes.
//   - Opted out: nothing was spooled then anyway (flushTelemetry checks), so
//     this is the belt to that.
//
// Native Windows is not here because nav-pilot is not built for it (the
// setsid below, like provider's syscall.Kill, does not compile there); WSL is
// Linux.
func telemetrySenderAllowed() bool {
	if insideCpltSandbox() || !telemetryEnabled() {
		return false
	}
	return e2eSeams != "1" || os.Getenv("NAV_PILOT_E2E_TELEMETRY_CHILD") == "1"
}

// spawnTelemetrySender starts `nav-pilot __telemetry-send` in a session of its
// own and does not wait for it. Its stdin, stdout and stderr are the null
// device (os/exec's default for nil): a child holding the parent's pipes
// would make `$(nav-pilot ...)`, and every test reading the output, wait for
// it. It inherits the environment, so the endpoint, proxies and opt-out are
// the parent's.
func spawnTelemetrySender() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, telemetrySendCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
}

func runWithCommandTelemetry(command, mode, scope string, fn func() error) error {
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			telemetry.RecordCommand(command, mode, scope, "error", "panic", time.Since(start))

			// Flush telemetry before we crash
			flushTelemetry(telemetry)

			panic(r)
		}
	}()

	err := fn()
	telemetry.RecordCommand(command, mode, scope, telemetryResult(err), classifyError(err), time.Since(start))
	return err
}

func telemetryResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, errUpdatesAvailable):
		return "updates_available"
	default:
		return "error"
	}
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	var netErr net.Error
	var exitErr *exec.ExitError
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return "client_not_found"
	case errors.As(err, &exitErr):
		return "launch_failed"
	case errors.As(err, &netErr) && netErr.Timeout():
		return "network_error"
	case strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403"):
		return "auth_error"
	case errors.Is(err, errSyncFailed):
		return "sync_failed"
	case errors.Is(err, errUpdatesAvailable):
		return "" // Not an error
	default:
		return "unknown"
	}
}

// configModelLabel collapses an arbitrary model id to a low-cardinality label:
// a model id known to any registered provider, "custom" for anything else, or
// "unset" when blank. Known model lists are owned by the provider implementations
// in provider.go; cardinality is bounded by the curated list sizes.
func configModelLabel(model string) string {
	if strings.TrimSpace(model) == "" {
		return "unset"
	}
	for _, p := range allProviders() {
		for _, m := range p.KnownModels() {
			if strings.EqualFold(m.ID, model) {
				return m.ID
			}
		}
	}
	return "custom"
}

// alphaCommand is the telemetry command name for an alpha invocation: "alpha
// decide", "alpha decide eval", "alpha local <sub>", or plain "alpha" for help
// and anything unknown. Only these fixed names; never an argument.
func alphaCommand(args []string) string {
	if len(args) == 0 {
		return "alpha"
	}
	switch args[0] {
	case "decide":
		for _, a := range args[1:] {
			if a == "--eval" || a == "-eval" || strings.HasPrefix(a, "--eval=") || strings.HasPrefix(a, "-eval=") {
				return "alpha decide eval"
			}
		}
		return "alpha decide"
	case "local":
		if len(args) > 1 {
			if slices.Contains(localCommands, args[1]) {
				return "alpha local " + args[1]
			}
		}
	}
	return "alpha"
}
