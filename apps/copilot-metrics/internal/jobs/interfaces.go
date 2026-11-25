package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/storage"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// MetricsFetcher defines the interface for fetching Copilot usage metrics.
// This abstraction enables testing with mock implementations.
type MetricsFetcher interface {
	FetchDailyMetrics(ctx context.Context, day time.Time) (*types.FetchResult, error)
	FetchDailyUserTeams(ctx context.Context, day time.Time) (*types.FetchResult, error)
	FetchDailyUserMetrics(ctx context.Context, day time.Time) (*types.FetchResult, error)
	FetchDailyRepoMetrics(ctx context.Context, day time.Time) (*types.FetchResult, error)
	FetchLatest28DayReport(ctx context.Context) (*types.FetchResult, error)
}

// MetricsStore defines the interface for storing Copilot usage metrics.
// This abstraction enables testing with mock implementations.
type MetricsStore interface {
	ReplaceUserTeams(context.Context, time.Time, *types.FetchResult) error
	ReplaceUserMetrics(context.Context, time.Time, *types.FetchResult) error
	EnsureTableExists(ctx context.Context) error
	EnsureUserTeamsTableExists(ctx context.Context) error
	EnsureUserMetricsTableExists(ctx context.Context) error
	EnsureRepoMetricsTableExists(ctx context.Context) error
	InsertMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error
	InsertUserTeams(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error
	InsertUserMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error
	InsertRepoMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error
	DayExists(ctx context.Context, day time.Time, scopeID string) (bool, error)
	UserTeamsDayExists(ctx context.Context, day time.Time, scopeID string) (bool, error)
	UserMetricsDayExists(ctx context.Context, day time.Time, scopeID string) (bool, error)
	RepoMetricsDayExists(ctx context.Context, day time.Time, scopeID string) (bool, error)
	DeleteDay(ctx context.Context, day time.Time, scopeID string) error
	DeleteUserTeamsDay(ctx context.Context, day time.Time, scopeID string) error
	DeleteUserMetricsDay(ctx context.Context, day time.Time, scopeID string) error
	DeleteRepoMetricsDay(ctx context.Context, day time.Time, scopeID string) error
	GetLatestDay(ctx context.Context, scopeID string) (time.Time, error)
	Close() error
}

// Verify that our implementations satisfy the interfaces at compile time.
var (
	_ MetricsFetcher = (*fetchers.GitHubClient)(nil)
	_ MetricsStore   = (*storage.BigQueryClient)(nil)
)
