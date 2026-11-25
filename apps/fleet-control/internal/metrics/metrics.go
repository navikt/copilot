// Package metrics holds fleet-control's Prometheus collectors, HTTP
// instrumentation and pgx tracer.
package metrics //nolint:revive // fleet-control never imports runtime/metrics; same name as fleet-gateway

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collectors are exported for the callers in package main and its tests.
var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_control_http_requests_total",
		Help: "HTTP requests by mux pattern and status code.",
	}, []string{"route", "code"})
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "fleet_control_http_request_duration_seconds",
		Help:    "HTTP request duration by mux pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})
	DBErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fleet_control_db_errors_total",
		Help: "Database statements that failed, not counting cancelled requests and constraint violations.",
	})
	AuditWriteFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fleet_control_audit_write_failures_total",
		Help: "Inserts into audit_events that failed.",
	})
	ConsoleLookups = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_control_console_lookups_total",
		Help: "Nais Console team lookups (one per page) by outcome: ok, error or 503.",
	}, []string{"outcome"})
	ConsoleDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "fleet_control_console_lookup_duration_seconds",
		Help:    "Nais Console team lookup duration, one per page.",
		Buckets: prometheus.DefBuckets,
	})
	SnapshotBuildDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "fleet_control_snapshot_build_duration_seconds",
		Help:    "Duration of a successful snapshot build and publish, by kind.",
		Buckets: prometheus.DefBuckets,
	}, []string{"kind"})
	SnapshotBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "fleet_control_snapshot_bytes",
		Help: "Size of the items of the last built snapshot, by kind.",
	}, []string{"kind"})
	SnapshotLastSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "fleet_control_snapshot_last_success_timestamp_seconds",
		Help: "Unix time of the last successful snapshot build, by kind.",
	}, []string{"kind"})
)

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.code == 0 {
		s.code = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.code == 0 {
		s.code = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Instrument counts requests by r.Pattern, never the raw path, so the label
// set stays bounded; unmatched requests share one label.
func Instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		if rec.code == 0 {
			rec.code = http.StatusOK
		}
		HTTPRequests.WithLabelValues(route, strconv.Itoa(rec.code)).Inc()
		HTTPDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

// DBTracer counts failed statements for DBErrors and AuditWriteFailures.
type DBTracer struct{}

type sqlKey struct{}

// TraceQueryStart keeps the statement so TraceQueryEnd can classify it.
func (DBTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, sqlKey{}, d.SQL)
}

// TraceQueryEnd counts the failure, if any.
func (DBTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if d.Err == nil {
		return
	}
	sql, _ := ctx.Value(sqlKey{}).(string)
	if strings.Contains(sql, "INSERT INTO audit_events") {
		AuditWriteFailures.Inc()
	}
	var pgErr *pgconn.PgError
	if errors.Is(d.Err, context.Canceled) || errors.As(d.Err, &pgErr) && strings.HasPrefix(pgErr.Code, "23") {
		return
	}
	DBErrors.Inc()
}
