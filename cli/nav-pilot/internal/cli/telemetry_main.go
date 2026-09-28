package cli

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"slices"
	"strings"
	"time"
)

func telemetryMode() string {
	if isInteractive() {
		return "interactive"
	}
	return "non_interactive"
}

// The exit waits for the last export (#1101), but not for long. A cold
// export (DNS, TLS, one round trip) takes 150-400 ms. It used to get a second,
// and on a network that does not answer, `nav-pilot config get` took a second
// each time it ran from a script or a shell prompt (#1234). Every command now
// gets 300 ms, as alpha decide and alpha local ask already had: a dropped
// sample is cheaper than a slow command.
//
// After a session the budget is shortest: the periodic reader exported every
// ten seconds while the session ran, so what is left is one export over a
// connection already open, and the user is waiting for the shell.
const (
	telemetryFlushBudget        = 300 * time.Millisecond
	telemetrySessionFlushBudget = 150 * time.Millisecond
)

// flushBudget is how long the exit waits for the last export.
func flushBudget() time.Duration {
	if sessionClient != "" {
		return telemetrySessionFlushBudget
	}
	return telemetryFlushBudget
}

// flushTelemetry exports what is left and returns after budget in any case.
// The SDK honours the context on its own, except that Shutdown waits out a
// periodic export already in flight (up to the reader's 2 s timeout), and a
// firewall that holds connect() can keep it past the deadline too.
func flushTelemetry(t telemetryRecorder, budget time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = t.Shutdown(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func runWithCommandTelemetry(command, mode, scope string, fn func() error) error {
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			telemetry.RecordCommand(command, mode, scope, "error", "panic", time.Since(start))

			// Flush telemetry before we crash
			flushTelemetry(telemetry, flushBudget())

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
