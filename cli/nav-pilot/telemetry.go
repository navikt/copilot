package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type telemetryRecorder interface {
	RecordCommand(command, mode, scope, result string, duration time.Duration)
	RecordInstallItems(scope, mode string, count int64)
	RecordSyncUpdates(scope, mode string, count int64)
	RecordSyncConflicts(scope, mode string, count int64)
	Shutdown(ctx context.Context) error
}

type noopTelemetry struct{}

func (noopTelemetry) RecordCommand(string, string, string, string, time.Duration) {}
func (noopTelemetry) RecordInstallItems(string, string, int64)                    {}
func (noopTelemetry) RecordSyncUpdates(string, string, int64)                     {}
func (noopTelemetry) RecordSyncConflicts(string, string, int64)                   {}
func (noopTelemetry) Shutdown(context.Context) error                              { return nil }

type otelTelemetry struct {
	provider *sdkmetric.MeterProvider

	commandTotal       metric.Int64Counter
	commandDurationMS  metric.Int64Histogram
	commandErrorTotal  metric.Int64Counter
	installItemsTotal  metric.Int64Counter
	syncUpdatesTotal   metric.Int64Counter
	syncConflictsTotal metric.Int64Counter

	version string
}

func initTelemetry(ctx context.Context, cliVersion string) (telemetryRecorder, error) {
	if !telemetryEnabled() {
		return noopTelemetry{}, nil
	}

	// Load retention policy from environment
	retention := loadRetentionConfig()
	debugLog("telemetry retention policy: %s", retention)

	// Load or generate stable device ID
	deviceID, err := getOrCreateDeviceID()
	if err != nil {
		debugLog("failed to get device ID: %v; continuing without it", err)
		deviceID = "unknown"
	}

	endpoint := strings.TrimSpace(os.Getenv("NAV_PILOT_TELEMETRY_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}

	if endpoint == "" {
		return noopTelemetry{}, fmt.Errorf("telemetry enabled, but no OTLP endpoint configured (set NAV_PILOT_TELEMETRY_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT)")
	}

	opts := []otlpmetrichttp.Option{}
	if endpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpointURL(endpoint))
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create OTLP metrics exporter: %w", err)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", "nav-pilot"),
		attribute.String("service.version", normalizeTelemetryDimension(cliVersion, "dev")),
		attribute.String("os", runtime.GOOS),
		attribute.String("arch", runtime.GOARCH),
		attribute.String("device_id", deviceID),
		attribute.Int("telemetry_retention_days", retention.DaysRetained),
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create telemetry resource: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(exporter,
		sdkmetric.WithInterval(10*time.Second),
		sdkmetric.WithTimeout(2*time.Second),
	)
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)

	meter := provider.Meter("github.com/navikt/copilot/cli/nav-pilot")
	commandTotal, err := meter.Int64Counter("nav_pilot_command_total")
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create command total counter: %w", err)
	}
	commandDurationMS, err := meter.Int64Histogram("nav_pilot_command_duration_ms")
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create command duration histogram: %w", err)
	}
	commandErrorTotal, err := meter.Int64Counter("nav_pilot_command_error_total")
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create command error counter: %w", err)
	}
	installItemsTotal, err := meter.Int64Counter("nav_pilot_install_items_total")
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create install items counter: %w", err)
	}
	syncUpdatesTotal, err := meter.Int64Counter("nav_pilot_sync_updates_total")
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create sync updates counter: %w", err)
	}
	syncConflictsTotal, err := meter.Int64Counter("nav_pilot_sync_conflicts_total")
	if err != nil {
		return noopTelemetry{}, fmt.Errorf("create sync conflicts counter: %w", err)
	}

	return &otelTelemetry{
		provider:           provider,
		commandTotal:       commandTotal,
		commandDurationMS:  commandDurationMS,
		commandErrorTotal:  commandErrorTotal,
		installItemsTotal:  installItemsTotal,
		syncUpdatesTotal:   syncUpdatesTotal,
		syncConflictsTotal: syncConflictsTotal,
		version:            normalizeTelemetryDimension(cliVersion, "dev"),
	}, nil
}

func telemetryEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NAV_PILOT_TELEMETRY_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func telemetryMode() string {
	if isInteractive() {
		return "interactive"
	}
	return "non_interactive"
}

func runWithCommandTelemetry(command, mode, scope string, fn func() error) error {
	start := time.Now()
	err := fn()
	telemetry.RecordCommand(command, mode, scope, telemetryResult(err), time.Since(start))
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

func (t *otelTelemetry) RecordCommand(command, mode, scope, result string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("command", normalizeTelemetryDimension(command, "unknown")),
		attribute.String("mode", normalizeTelemetryDimension(mode, "non_interactive")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("result", normalizeTelemetryDimension(result, "error")),
		attribute.String("version", t.version),
	}

	t.commandTotal.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	t.commandDurationMS.Record(context.Background(), maxInt64(0, duration.Milliseconds()), metric.WithAttributes(attrs...))

	if result == "error" {
		t.commandErrorTotal.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("command", normalizeTelemetryDimension(command, "unknown")),
			attribute.String("mode", normalizeTelemetryDimension(mode, "non_interactive")),
			attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
			attribute.String("version", t.version),
		))
	}
}

func (t *otelTelemetry) RecordInstallItems(scope, mode string, count int64) {
	if count <= 0 {
		return
	}
	t.installItemsTotal.Add(context.Background(), count, metric.WithAttributes(
		attribute.String("command", "install"),
		attribute.String("mode", normalizeTelemetryDimension(mode, "non_interactive")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("version", t.version),
	))
}

func (t *otelTelemetry) RecordSyncUpdates(scope, mode string, count int64) {
	if count <= 0 {
		return
	}
	t.syncUpdatesTotal.Add(context.Background(), count, metric.WithAttributes(
		attribute.String("command", "sync"),
		attribute.String("mode", normalizeTelemetryDimension(mode, "non_interactive")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("version", t.version),
	))
}

func (t *otelTelemetry) RecordSyncConflicts(scope, mode string, count int64) {
	if count <= 0 {
		return
	}
	t.syncConflictsTotal.Add(context.Background(), count, metric.WithAttributes(
		attribute.String("command", "sync"),
		attribute.String("mode", normalizeTelemetryDimension(mode, "non_interactive")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("version", t.version),
	))
}

func (t *otelTelemetry) Shutdown(ctx context.Context) error {
	return t.provider.Shutdown(ctx)
}

func normalizeTelemetryDimension(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	switch v {
	case "install", "sync", "upgrade", "list",
		"interactive", "non_interactive",
		"repo", "user", "auto", "none", "unknown",
		"success", "error", "updates_available", "dev":
		return v
	default:
		if strings.Count(v, ".") >= 1 || strings.Count(v, "-") >= 1 {
			return v
		}
		return fallback
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
