package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr/funcr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

var otelDiagnosticsOnce sync.Once

// configureOTelDiagnostics installs global OTel error handler and logger so
// that export failures are routed through DebugLog (visible only when DEBUG is
// set) and never written directly to stderr. Safe to call multiple times.
func configureOTelDiagnostics() {
	otelDiagnosticsOnce.Do(func() {
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			DebugLog("otel telemetry error: %v", err)
		}))
		otel.SetLogger(funcr.New(func(prefix, args string) {
			DebugLog("otel %s %s", prefix, args)
		}, funcr.Options{Verbosity: 0}))
	})
}

const defaultTelemetryEndpoint = "https://collector-internet.nav.cloud.nais.io/v1/metrics"

type Recorder interface {
	RecordCommand(command, mode, scope, result string, duration time.Duration)
	RecordInstallItems(scope, mode string, count int64)
	RecordSyncUpdates(scope, mode string, count int64)
	RecordSyncConflicts(scope, mode string, count int64)
	RecordInstallPresent(scope, collection string, present bool)
	RecordInstalledItems(scope, itemType, status string, count int64)
	RecordStalenessCheck(component, scope, result string)
	RecordUpToDate(component, scope string, upToDate bool)
	RecordVersionSkewDays(component, scope string, days int64)
	RecordConfig(client, configMode, model, reasoningEffort, contextTier, otelLogLevel string, allowAllTools, askUser bool)
	RecordClientAvailable(client string, available bool)
	RecordLaunchError(client, errorType string)
	RecordRtkSetup(client, choice, result string)
	Shutdown(ctx context.Context) error
}

type NoopRecorder struct{}

func (NoopRecorder) RecordCommand(string, string, string, string, time.Duration) {}
func (NoopRecorder) RecordInstallItems(string, string, int64)                    {}
func (NoopRecorder) RecordSyncUpdates(string, string, int64)                     {}
func (NoopRecorder) RecordSyncConflicts(string, string, int64)                   {}
func (NoopRecorder) RecordInstallPresent(string, string, bool)                   {}
func (NoopRecorder) RecordInstalledItems(string, string, string, int64)          {}
func (NoopRecorder) RecordStalenessCheck(string, string, string)                 {}
func (NoopRecorder) RecordUpToDate(string, string, bool)                         {}
func (NoopRecorder) RecordVersionSkewDays(string, string, int64)                 {}
func (NoopRecorder) RecordConfig(string, string, string, string, string, string, bool, bool) {
}
func (NoopRecorder) RecordClientAvailable(string, bool)    {}
func (NoopRecorder) RecordLaunchError(string, string)      {}
func (NoopRecorder) RecordRtkSetup(string, string, string) {}
func (NoopRecorder) Shutdown(context.Context) error        { return nil }

type otelTelemetry struct {
	provider *sdkmetric.MeterProvider

	commandTotal       metric.Int64Counter
	commandDurationMS  metric.Int64Histogram
	commandErrorTotal  metric.Int64Counter
	launchErrorTotal   metric.Int64Counter
	installItemsTotal  metric.Int64Counter
	syncUpdatesTotal   metric.Int64Counter
	syncConflictsTotal metric.Int64Counter
	infoGauge          metric.Int64Gauge
	installPresent     metric.Int64Gauge
	installedItems     metric.Int64Gauge
	configInfo         metric.Int64Gauge
	clientAvailable    metric.Int64Gauge
	stalenessCheck     metric.Int64Counter
	upToDate           metric.Int64Gauge
	versionSkewDays    metric.Int64Histogram
	rtkSetupTotal      metric.Int64Counter

	version          string
	device           string
	executionContext string
	os               string
	arch             string
	rtkInstalled     string
}

func InitTelemetry(ctx context.Context, cliVersion string, rtkInstalled string) (Recorder, error) {
	configureOTelDiagnostics()

	if !TelemetryEnabled() {
		return NoopRecorder{}, nil
	}

	deviceID, err := GetOrCreateDeviceID()
	if err != nil {
		DebugLog("failed to get device ID: %v; continuing without it", err)
		deviceID = "unknown"
	}
	executionContext := detectExecutionContext()
	version := normalizeTelemetryDimension(cliVersion, "dev")
	device := normalizeTelemetryDimension(deviceID, "unknown")
	osName := normalizeTelemetryDimension(runtime.GOOS, "unknown")
	arch := normalizeTelemetryDimension(runtime.GOARCH, "unknown")
	execCtx := normalizeTelemetryDimension(executionContext, "unknown")

	endpoint := strings.TrimSpace(os.Getenv("NAV_PILOT_TELEMETRY_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	if endpoint == "" {
		endpoint = defaultTelemetryEndpoint
	}

	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithTemporalitySelector(func(kind sdkmetric.InstrumentKind) metricdata.Temporality {
			switch kind {
			case sdkmetric.InstrumentKindCounter,
				sdkmetric.InstrumentKindUpDownCounter,
				sdkmetric.InstrumentKindObservableCounter,
				sdkmetric.InstrumentKindObservableUpDownCounter:
				return metricdata.DeltaTemporality
			default:
				return metricdata.CumulativeTemporality
			}
		}),
	}
	if endpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpointURL(endpoint))
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create OTLP metrics exporter: %w", err)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", "nav-pilot"),
		attribute.String("service.version", version),
		attribute.String("os", osName),
		attribute.String("arch", arch),
		attribute.String("device_id", device),
		attribute.String("execution_context", execCtx),
	))
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create telemetry resource: %w", err)
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
		return NoopRecorder{}, fmt.Errorf("create command total counter: %w", err)
	}
	commandDurationMS, err := meter.Int64Histogram("nav_pilot_command_duration_ms")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create command duration histogram: %w", err)
	}
	commandErrorTotal, err := meter.Int64Counter("nav_pilot_command_error_total")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create command error counter: %w", err)
	}
	launchErrorTotal, err := meter.Int64Counter("nav_pilot_launch_error_total",
		metric.WithDescription("Counts client launch failures by client and error type."))
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create launch error counter: %w", err)
	}
	installItemsTotal, err := meter.Int64Counter("nav_pilot_install_items_total")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create install items counter: %w", err)
	}
	syncUpdatesTotal, err := meter.Int64Counter("nav_pilot_sync_updates_total")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create sync updates counter: %w", err)
	}
	syncConflictsTotal, err := meter.Int64Counter("nav_pilot_sync_conflicts_total")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create sync conflicts counter: %w", err)
	}
	infoGauge, err := meter.Int64Gauge("nav_pilot_info")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create info gauge: %w", err)
	}
	installPresent, err := meter.Int64Gauge("nav_pilot_install_present")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create install present gauge: %w", err)
	}
	installedItems, err := meter.Int64Gauge("nav_pilot_installed_items")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create installed items gauge: %w", err)
	}
	configInfo, err := meter.Int64Gauge("nav_pilot_config_info")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create config info gauge: %w", err)
	}
	clientAvailable, err := meter.Int64Gauge("nav_pilot_client_available")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create client available gauge: %w", err)
	}
	stalenessCheck, err := meter.Int64Counter("nav_pilot_staleness_check_total")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create staleness check counter: %w", err)
	}
	upToDate, err := meter.Int64Gauge("nav_pilot_up_to_date")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create up to date gauge: %w", err)
	}
	versionSkewDays, err := meter.Int64Histogram("nav_pilot_version_skew_days")
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create version skew days histogram: %w", err)
	}
	rtkSetupTotal, err := meter.Int64Counter("nav_pilot_rtk_setup_total",
		metric.WithDescription("Counts the result of the interactive RTK setup prompt."))
	if err != nil {
		return NoopRecorder{}, fmt.Errorf("create rtk setup counter: %w", err)
	}

	tel := &otelTelemetry{
		provider:           provider,
		commandTotal:       commandTotal,
		commandDurationMS:  commandDurationMS,
		commandErrorTotal:  commandErrorTotal,
		launchErrorTotal:   launchErrorTotal,
		installItemsTotal:  installItemsTotal,
		syncUpdatesTotal:   syncUpdatesTotal,
		syncConflictsTotal: syncConflictsTotal,
		infoGauge:          infoGauge,
		installPresent:     installPresent,
		installedItems:     installedItems,
		configInfo:         configInfo,
		clientAvailable:    clientAvailable,
		stalenessCheck:     stalenessCheck,
		upToDate:           upToDate,
		versionSkewDays:    versionSkewDays,
		rtkSetupTotal:      rtkSetupTotal,
		version:            version,
		device:             device,
		executionContext:   execCtx,
		os:                 osName,
		arch:               arch,
		rtkInstalled:       rtkInstalled,
	}
	tel.recordInfo()

	return tel, nil
}

func TelemetryEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NAV_PILOT_TELEMETRY_ENABLED"))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func detectExecutionContext() string {
	if override := normalizeExecutionContextOverride(os.Getenv("NAV_PILOT_EXECUTION_CONTEXT")); override != "" {
		return override
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")), "true") {
		return "ci_github_actions"
	}
	if isGenericCI() {
		return "ci_other"
	}
	return "organic"
}

func normalizeExecutionContextOverride(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "organic", "ci_github_actions", "ci_other", "unknown":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func isGenericCI() bool {
	if envTruthy("CI") {
		return true
	}
	for _, key := range []string{"GITLAB_CI", "JENKINS_URL", "BUILDKITE", "CIRCLECI", "TF_BUILD", "BUILD_ID"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (t *otelTelemetry) recordInfo() {
	t.infoGauge.Record(context.Background(), 1, metric.WithAttributes(
		attribute.String("version", t.version),
		attribute.String("device_id", t.device),
		attribute.String("execution_context", t.executionContext),
		attribute.String("os", t.os),
		attribute.String("arch", t.arch),
		attribute.String("rtk_installed", t.rtkInstalled),
	))
}

// RecordConfig emits the resolved per-launch configuration preferences as a
// gauge so we can see which clients, modes and models users actually run with.
// All labels are bounded enums (validated upstream), a clamped model id, or
// booleans, so cardinality stays low and no PII is recorded.
func (t *otelTelemetry) RecordConfig(client, configMode, model, reasoningEffort, contextTier, otelLogLevel string, allowAllTools, askUser bool) {
	t.configInfo.Record(context.Background(), 1, metric.WithAttributes(
		attribute.String("client", orUnset(client)),
		attribute.String("config_mode", orUnset(configMode)),
		attribute.String("model", orUnset(model)),
		attribute.String("reasoning_effort", orUnset(reasoningEffort)),
		attribute.String("context_tier", orUnset(contextTier)),
		attribute.String("otel_log_level", orUnset(otelLogLevel)),
		attribute.String("allow_all_tools", strconv.FormatBool(allowAllTools)),
		attribute.String("ask_user", strconv.FormatBool(askUser)),
		attribute.String("version", t.version),
		attribute.String("device_id", t.device),
		attribute.String("execution_context", t.executionContext),
	))
}

// RecordClientAvailable emits whether a coding-agent client is on PATH.
func (t *otelTelemetry) RecordClientAvailable(client string, available bool) {
	v := int64(0)
	if available {
		v = 1
	}
	t.clientAvailable.Record(context.Background(), v, metric.WithAttributes(
		attribute.String("client", client),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

func (t *otelTelemetry) RecordCommand(command, mode, scope, result string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("command", normalizeTelemetryDimension(command, "unknown")),
		attribute.String("mode", normalizeTelemetryDimension(mode, "non_interactive")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("result", normalizeTelemetryDimension(result, "error")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	}

	t.commandTotal.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	t.commandDurationMS.Record(context.Background(), maxInt64(0, duration.Milliseconds()), metric.WithAttributes(attrs...))

	if result == "error" {
		t.commandErrorTotal.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("command", normalizeTelemetryDimension(command, "unknown")),
			attribute.String("mode", normalizeTelemetryDimension(mode, "non_interactive")),
			attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
			attribute.String("version", t.version),
			attribute.String("execution_context", t.executionContext),
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
		attribute.String("execution_context", t.executionContext),
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
		attribute.String("execution_context", t.executionContext),
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
		attribute.String("execution_context", t.executionContext),
	))
}

func (t *otelTelemetry) RecordInstallPresent(scope, collection string, present bool) {
	value := int64(0)
	if present {
		value = 1
	}
	t.installPresent.Record(context.Background(), value, metric.WithAttributes(
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("collection", normalizeTelemetryDimension(collection, "other")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

func (t *otelTelemetry) RecordInstalledItems(scope, itemType, status string, count int64) {
	if count < 0 {
		return
	}
	t.installedItems.Record(context.Background(), count, metric.WithAttributes(
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("type", normalizeTelemetryDimension(itemType, "unknown")),
		attribute.String("status", normalizeTelemetryDimension(status, "active")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

func (t *otelTelemetry) RecordStalenessCheck(component, scope, result string) {
	t.stalenessCheck.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("component", normalizeTelemetryDimension(component, "unknown")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("result", normalizeTelemetryDimension(result, "error")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

func (t *otelTelemetry) RecordUpToDate(component, scope string, upToDate bool) {
	value := int64(0)
	if upToDate {
		value = 1
	}
	t.upToDate.Record(context.Background(), value, metric.WithAttributes(
		attribute.String("component", normalizeTelemetryDimension(component, "unknown")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

func (t *otelTelemetry) RecordVersionSkewDays(component, scope string, days int64) {
	if days < 0 {
		days = 0
	}
	t.versionSkewDays.Record(context.Background(), days, metric.WithAttributes(
		attribute.String("component", normalizeTelemetryDimension(component, "unknown")),
		attribute.String("scope", normalizeTelemetryDimension(scope, "unknown")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

func (t *otelTelemetry) Shutdown(ctx context.Context) error {
	return t.provider.Shutdown(ctx)
}

// RecordLaunchError records a client launch failure with a normalized error type.
// client: "copilot", "opencode", "pi"
// errorType: "client_not_found", "launch_failed", "unknown"
func (t *otelTelemetry) RecordLaunchError(client, errorType string) {
	t.launchErrorTotal.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("client", normalizeTelemetryDimension(client, "unknown")),
		attribute.String("error_type", normalizeTelemetryDimension(errorType, "unknown")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

// RecordRtkSetup records the interactive result for RTK token optimizer setup.
func (t *otelTelemetry) RecordRtkSetup(client, choice, result string) {
	t.rtkSetupTotal.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("client", normalizeTelemetryDimension(client, "unknown")),
		attribute.String("choice", normalizeTelemetryDimension(choice, "unknown")),
		attribute.String("result", normalizeTelemetryDimension(result, "unknown")),
		attribute.String("version", t.version),
		attribute.String("execution_context", t.executionContext),
	))
}

func normalizeTelemetryDimension(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	switch v {
	case "install", "sync", "upgrade", "list", "startup", "launch",
		"interactive", "non_interactive",
		"repo", "user", "auto", "none", "unknown",
		"success", "error", "updates_available", "dev",
		"all", "other",
		"fullstack", "kotlin-backend", "frontend", "nextjs-frontend", "platform",
		"agent", "skill", "instruction", "prompt",
		"active", "ignored", "conflict",
		"collection", "cli",
		"up_to_date", "stale", "lookup_failed", "cooldown", "no_install", "corrupted",
		"organic", "ci_github_actions", "ci_other",
		"darwin", "linux", "windows",
		"amd64", "arm64", "arm", "386",
		"copilot", "opencode", "pi",
		"client_not_found", "launch_failed",
		"yes", "no", "aborted", "brew_failed", "curl_failed", "init_failed", "already_installed":
		return v
	default:
		if strings.Count(v, ".") >= 1 || strings.Count(v, "-") >= 1 {
			return v
		}
		return fallback
	}
}

func orUnset(v string) string {
	if strings.TrimSpace(v) == "" {
		return "unset"
	}
	return v
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
