package cli

import (
	"context"
	"errors"
	"net"
	"os"
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

// The longest exit waits for the last export (#1101). A cold export (DNS,
// TLS, one round trip) takes 150-400 ms, so most commands get a second.
// alpha decide and alpha local ask run in hooks, scripts and loops, where an
// unreachable host made every call wait seconds: they get 300 ms, and a
// dropped sample is cheaper than a slow hook.
const (
	telemetryFlushBudget      = time.Second
	telemetryQuickFlushBudget = 300 * time.Millisecond
)

// flushBudget is the budget for a command line (os.Args[1:]).
func flushBudget(args []string) time.Duration {
	if len(args) > 0 && args[0] == "alpha" {
		if c := alphaCommand(args[1:]); c == "alpha decide" || c == "alpha local ask" {
			return telemetryQuickFlushBudget
		}
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
			flushTelemetry(telemetry, flushBudget(os.Args[1:]))

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
