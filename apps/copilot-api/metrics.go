package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// MetricsCollector manages Prometheus metrics
type MetricsCollector struct {
	mu                      sync.RWMutex
	lastCollectionTimestamp int64
	githubSeatsTotal        int64
	githubSeatsActive       int64
	githubSeatsInactive     int64
	githubSeatsPending      int64
	githubSeatsCancelling   int64
}

var metricsCollector = &MetricsCollector{}

// metricsHandler returns Prometheus metrics
func metricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metricsCollector.mu.RLock()
		defer metricsCollector.mu.RUnlock()

		// Return metrics in Prometheus text format
		metrics := fmt.Sprintf(`# HELP github_metrics_last_success_timestamp Unix timestamp of last successful GitHub metrics collection
# TYPE github_metrics_last_success_timestamp gauge
github_metrics_last_success_timestamp %d

# HELP copilot_seats_total Total number of Copilot seats
# TYPE copilot_seats_total gauge
copilot_seats_total %d

# HELP copilot_seats_active Number of active Copilot seats this cycle
# TYPE copilot_seats_active gauge
copilot_seats_active %d

# HELP copilot_seats_inactive Number of inactive Copilot seats this cycle
# TYPE copilot_seats_inactive gauge
copilot_seats_inactive %d

# HELP copilot_seats_pending Number of Copilot seats pending invitation
# TYPE copilot_seats_pending gauge
copilot_seats_pending %d

# HELP copilot_seats_cancelling Number of Copilot seats pending cancellation
# TYPE copilot_seats_cancelling gauge
copilot_seats_cancelling %d
`,
			metricsCollector.lastCollectionTimestamp,
			metricsCollector.githubSeatsTotal,
			metricsCollector.githubSeatsActive,
			metricsCollector.githubSeatsInactive,
			metricsCollector.githubSeatsPending,
			metricsCollector.githubSeatsCancelling,
		)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metrics))
	})
}

// startMetricsCollector starts the background metrics collection
func startMetricsCollector(config *Config, client *GitHubClient) {
	// Collect immediately on startup (if configured)
	if client != nil {
		slog.Info("Running initial GitHub metrics collection")
		collectGitHubMetrics(client)
	}

	// Start background collection every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if client != nil {
				collectGitHubMetrics(client)
			}
		}
	}()

	slog.Info("GitHub metrics collector started", "interval", "5m")
}
