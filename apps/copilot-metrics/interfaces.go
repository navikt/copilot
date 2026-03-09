package main

import (
	"context"
	"encoding/json"
	"time"
)

// MetricsFetcher defines the interface for fetching Copilot usage metrics.
// This abstraction enables testing with mock implementations.
type MetricsFetcher interface {
	FetchDailyMetrics(ctx context.Context, day time.Time) (*FetchResult, error)
	FetchLatest28DayReport(ctx context.Context) (*FetchResult, error)
}

// MetricsStore defines the interface for storing Copilot usage metrics.
// This abstraction enables testing with mock implementations.
type MetricsStore interface {
	EnsureTableExists(ctx context.Context) error
	InsertMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error
	DayExists(ctx context.Context, day time.Time, scopeID string) (bool, error)
	DeleteDay(ctx context.Context, day time.Time, scopeID string) error
	GetLatestDay(ctx context.Context, scopeID string) (time.Time, error)
	Close() error
}

// Verify that our implementations satisfy the interfaces at compile time.
var (
	_ MetricsFetcher = (*GitHubClient)(nil)
	_ MetricsStore   = (*BigQueryClient)(nil)
)
