package cli

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strings"
	"time"
)

func telemetryMode() string {
	if isInteractive() {
		return "interactive"
	}
	return "non_interactive"
}

func runWithCommandTelemetry(command, mode, scope string, fn func() error) error {
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			telemetry.RecordCommand(command, mode, scope, "error", "panic", time.Since(start))

			// Flush telemetry before we crash
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			defer cancel()
			_ = telemetry.Shutdown(ctx)

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
