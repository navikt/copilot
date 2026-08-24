package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// scopedStore models how BigQuery actually stores these tables: rows live under
// a (day, scope_id) key, which is both what insertRecords writes and what the
// DELETE predicate matches. The mocks elsewhere in this package ignore scope_id,
// so a write keyed on the wrong scope looks idempotent to them — this store
// makes the extra copies visible.
type scopedStore struct {
	rows         map[string][]json.RawMessage
	existsErr    error
	deleteErr    error
	latestErr    error
	existsCalls  []string
	latestScopes []string
}

func newScopedStore() *scopedStore {
	return &scopedStore{rows: map[string][]json.RawMessage{}}
}

func scopedKey(table string, day time.Time, scopeID string) string {
	return table + "|" + day.Format("2006-01-02") + "|" + scopeID
}

// seed puts n rows in place as if a previous run had ingested them.
func (s *scopedStore) seed(table string, day time.Time, scopeID string, n int) {
	s.rows[scopedKey(table, day, scopeID)] = testRecords(n)
}

// count returns how many rows are stored for one (table, day, scope_id) key.
func (s *scopedStore) count(table string, day time.Time, scopeID string) int {
	return len(s.rows[scopedKey(table, day, scopeID)])
}

// countAllScopes returns how many rows are stored for a day across every scope.
func (s *scopedStore) countAllScopes(table string, day time.Time) int {
	var total int
	prefix := table + "|" + day.Format("2006-01-02") + "|"
	for key, rows := range s.rows {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			total += len(rows)
		}
	}
	return total
}

func (s *scopedStore) exists(table string, day time.Time, scopeID string) (bool, error) {
	s.existsCalls = append(s.existsCalls, scopedKey(table, day, scopeID))
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return len(s.rows[scopedKey(table, day, scopeID)]) > 0, nil
}

func (s *scopedStore) delete(table string, day time.Time, scopeID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.rows, scopedKey(table, day, scopeID))
	return nil
}

// insert appends, exactly like a BigQuery streaming insert — it never replaces.
func (s *scopedStore) insert(table string, day time.Time, scopeID string, records []json.RawMessage) error {
	key := scopedKey(table, day, scopeID)
	s.rows[key] = append(s.rows[key], records...)
	return nil
}

func (s *scopedStore) EnsureTableExists(context.Context) error            { return nil }
func (s *scopedStore) EnsureUserTeamsTableExists(context.Context) error   { return nil }
func (s *scopedStore) EnsureUserMetricsTableExists(context.Context) error { return nil }
func (s *scopedStore) EnsureRepoMetricsTableExists(context.Context) error { return nil }
func (s *scopedStore) Close() error                                       { return nil }

// GetLatestDay mirrors the real query: MAX(day) filtered to a single scope_id.
// Days stored under any other scope_id are invisible to it, which is exactly the
// blind spot the cross-scope high-water mark exists to cover.
func (s *scopedStore) GetLatestDay(_ context.Context, scopeID string) (time.Time, error) {
	s.latestScopes = append(s.latestScopes, scopeID)
	if s.latestErr != nil {
		return time.Time{}, s.latestErr
	}

	var latest time.Time
	prefix := tableUsageMetrics + "|"
	suffix := "|" + scopeID
	for key, rows := range s.rows {
		if len(rows) == 0 || !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
			continue
		}
		day, err := time.Parse("2006-01-02", key[len(prefix):len(key)-len(suffix)])
		if err != nil {
			continue
		}
		if day.After(latest) {
			latest = day
		}
	}
	return latest, nil
}

func (s *scopedStore) InsertMetrics(_ context.Context, day time.Time, _, scopeID string, records []json.RawMessage) error {
	return s.insert(tableUsageMetrics, day, scopeID, records)
}

func (s *scopedStore) InsertUserTeams(_ context.Context, day time.Time, _, scopeID string, records []json.RawMessage) error {
	return s.insert(tableUserTeams, day, scopeID, records)
}

func (s *scopedStore) InsertUserMetrics(_ context.Context, day time.Time, _, scopeID string, records []json.RawMessage) error {
	return s.insert(tableUserMetrics, day, scopeID, records)
}

func (s *scopedStore) InsertRepoMetrics(_ context.Context, day time.Time, _, scopeID string, records []json.RawMessage) error {
	return s.insert(tableRepoMetrics, day, scopeID, records)
}

func (s *scopedStore) DayExists(_ context.Context, day time.Time, scopeID string) (bool, error) {
	return s.exists(tableUsageMetrics, day, scopeID)
}

func (s *scopedStore) UserTeamsDayExists(_ context.Context, day time.Time, scopeID string) (bool, error) {
	return s.exists(tableUserTeams, day, scopeID)
}

func (s *scopedStore) UserMetricsDayExists(_ context.Context, day time.Time, scopeID string) (bool, error) {
	return s.exists(tableUserMetrics, day, scopeID)
}

func (s *scopedStore) RepoMetricsDayExists(_ context.Context, day time.Time, scopeID string) (bool, error) {
	return s.exists(tableRepoMetrics, day, scopeID)
}

func (s *scopedStore) DeleteDay(_ context.Context, day time.Time, scopeID string) error {
	return s.delete(tableUsageMetrics, day, scopeID)
}

func (s *scopedStore) DeleteUserTeamsDay(_ context.Context, day time.Time, scopeID string) error {
	return s.delete(tableUserTeams, day, scopeID)
}

func (s *scopedStore) DeleteUserMetricsDay(_ context.Context, day time.Time, scopeID string) error {
	return s.delete(tableUserMetrics, day, scopeID)
}

func (s *scopedStore) DeleteRepoMetricsDay(_ context.Context, day time.Time, scopeID string) error {
	return s.delete(tableRepoMetrics, day, scopeID)
}

// Table names used as scopedStore keys — they only need to be distinct.
const (
	tableUsageMetrics = "usage_metrics"
	tableUserTeams    = "user_teams"
	tableUserMetrics  = "user_metrics"
	tableRepoMetrics  = "repository_metrics"
)

func testRecords(n int) []json.RawMessage {
	records := make([]json.RawMessage, n)
	for i := range records {
		records[i] = json.RawMessage(`{"user_id":"u1"}`)
	}
	return records
}

// supplementaryFetcher returns the same supplementary reports for every day,
// under a configurable scope, and counts how often each report was fetched.
type supplementaryFetcher struct {
	scope   string
	scopeID string
	records int

	teamsFetches  int
	usersFetches  int
	reposFetches  int
	entityFetches int
}

func (f *supplementaryFetcher) result() *FetchResult {
	return &FetchResult{Records: testRecords(f.records), Scope: f.scope, ScopeID: f.scopeID}
}

func (f *supplementaryFetcher) FetchDailyMetrics(context.Context, time.Time) (*FetchResult, error) {
	f.entityFetches++
	return f.result(), nil
}

func (f *supplementaryFetcher) FetchLatest28DayReport(context.Context) (*FetchResult, error) {
	return f.result(), nil
}

func (f *supplementaryFetcher) FetchDailyUserTeams(context.Context, time.Time) (*FetchResult, error) {
	f.teamsFetches++
	return f.result(), nil
}

func (f *supplementaryFetcher) FetchDailyUserMetrics(context.Context, time.Time) (*FetchResult, error) {
	f.usersFetches++
	return f.result(), nil
}

func (f *supplementaryFetcher) FetchDailyRepoMetrics(context.Context, time.Time) (*FetchResult, error) {
	f.reposFetches++
	return f.result(), nil
}

func supplementaryConfig() *Config {
	return &Config{EnterpriseSlug: "nav", OrganizationSlug: "navikt"}
}

// TestIngestMissingSupplementary_RepeatedRunsDoNotDuplicate is the regression
// test for the org-scope gap-fill: the day is fetched under the org scope while
// the gap-fill's existence probe only knew the enterprise slug, so every nightly
// run considered the day missing and appended another copy of the same rows.
func TestIngestMissingSupplementary_RepeatedRunsDoNotDuplicate(t *testing.T) {
	tests := []struct {
		name         string
		entityScope  string
		reportScope  string
		reportScopeI string
	}{
		{
			name:         "entity and reports both enterprise-scoped",
			entityScope:  "nav",
			reportScope:  "enterprise",
			reportScopeI: "nav",
		},
		{
			// The duplicating combination: entity metrics came from the
			// enterprise endpoint, the supplementary reports fell back to the
			// org endpoint, so they were stored under a scope_id the gap-fill
			// never looked at.
			name:         "enterprise entity, org-scoped reports",
			entityScope:  "nav",
			reportScope:  "organization",
			reportScopeI: "navikt",
		},
		{
			name:         "entity and reports both org-scoped",
			entityScope:  "navikt",
			reportScope:  "organization",
			reportScopeI: "navikt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			day := daysAgo(1)
			cfg := supplementaryConfig()

			store := newScopedStore()
			// Entity metrics exist for the day — that is the gate the gap-fill
			// uses to decide the day is worth topping up.
			store.seed(tableUsageMetrics, day, tt.entityScope, 1)

			fetcher := &supplementaryFetcher{scope: tt.reportScope, scopeID: tt.reportScopeI, records: 3}

			// Three consecutive nightly runs.
			for range 3 {
				ingestMissingSupplementary(ctx, fetcher, store, cfg)
			}

			for _, table := range []string{tableUserTeams, tableUserMetrics, tableRepoMetrics} {
				if got := store.countAllScopes(table, day); got != 3 {
					t.Errorf("%s: expected 3 rows after 3 runs, got %d (rows duplicated per run)", table, got)
				}
			}
			if fetcher.teamsFetches != 1 {
				t.Errorf("expected the user-teams report to be fetched once, got %d", fetcher.teamsFetches)
			}
		})
	}
}

// TestIngestMissingSupplementary_SkipsDayStoredUnderOtherScope covers the same
// day being present under one scope while the gap-fill runs with the other
// configured: the day must not be ingested a second time under the other scope.
func TestIngestMissingSupplementary_SkipsDayStoredUnderOtherScope(t *testing.T) {
	tests := []struct {
		name         string
		storedScope  string
		fetchedScope string
		fetchedID    string
	}{
		{name: "stored enterprise, fetch resolves to org", storedScope: "nav", fetchedScope: "organization", fetchedID: "navikt"},
		{name: "stored org, fetch resolves to enterprise", storedScope: "navikt", fetchedScope: "enterprise", fetchedID: "nav"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			day := daysAgo(1)
			cfg := supplementaryConfig()

			store := newScopedStore()
			store.seed(tableUsageMetrics, day, tt.storedScope, 1)
			for _, table := range []string{tableUserTeams, tableUserMetrics, tableRepoMetrics} {
				store.seed(table, day, tt.storedScope, 4)
			}

			fetcher := &supplementaryFetcher{scope: tt.fetchedScope, scopeID: tt.fetchedID, records: 4}

			ingestMissingSupplementary(ctx, fetcher, store, cfg)

			for _, table := range []string{tableUserTeams, tableUserMetrics, tableRepoMetrics} {
				if got := store.countAllScopes(table, day); got != 4 {
					t.Errorf("%s: expected the day to stay at 4 rows, got %d across scopes", table, got)
				}
				if got := store.count(table, day, tt.fetchedID); got != 0 && tt.fetchedID != tt.storedScope {
					t.Errorf("%s: expected no second copy under scope_id %q, got %d rows", table, tt.fetchedID, got)
				}
			}
			if fetcher.teamsFetches != 0 {
				t.Errorf("expected no fetch for a day already stored, got %d", fetcher.teamsFetches)
			}
		})
	}
}

// TestIngestMissingSupplementary_ExistsErrorDoesNotInsert guards the other way a
// duplicate is born: treating an unreadable existence probe as "missing".
func TestIngestMissingSupplementary_ExistsErrorDoesNotInsert(t *testing.T) {
	ctx := context.Background()
	store := newScopedStore()
	store.existsErr = errTest

	fetcher := &supplementaryFetcher{scope: "organization", scopeID: "navikt", records: 2}

	ingestMissingSupplementary(ctx, fetcher, store, supplementaryConfig())

	if fetcher.teamsFetches != 0 || fetcher.usersFetches != 0 || fetcher.reposFetches != 0 {
		t.Errorf("expected no fetches when the entity probe fails, got teams=%d users=%d repos=%d",
			fetcher.teamsFetches, fetcher.usersFetches, fetcher.reposFetches)
	}
}

// TestUpsertReport_KeysOnFetchedScope pins the invariant the whole fix rests on:
// the exists/delete pair must use the scope_id the insert writes, so a re-import
// replaces its own rows and leaves the other scope's rows alone.
func TestUpsertReport_KeysOnFetchedScope(t *testing.T) {
	tests := []struct {
		name          string
		seedScopeID   string
		seedRows      int
		resultScope   string
		resultScopeID string
		wantByScope   map[string]int
	}{
		{
			name:          "re-import under the same scope replaces its rows",
			seedScopeID:   "navikt",
			seedRows:      5,
			resultScope:   "organization",
			resultScopeID: "navikt",
			wantByScope:   map[string]int{"navikt": 2},
		},
		{
			name:          "import under a new scope leaves the other scope intact",
			seedScopeID:   "nav",
			seedRows:      5,
			resultScope:   "organization",
			resultScopeID: "navikt",
			wantByScope:   map[string]int{"nav": 5, "navikt": 2},
		},
		{
			name:          "first import writes exactly one copy",
			seedScopeID:   "",
			resultScope:   "enterprise",
			resultScopeID: "nav",
			wantByScope:   map[string]int{"nav": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			day := fixedDay()

			store := newScopedStore()
			if tt.seedScopeID != "" {
				store.seed(tableUserMetrics, day, tt.seedScopeID, tt.seedRows)
			}

			result := &FetchResult{Records: testRecords(2), Scope: tt.resultScope, ScopeID: tt.resultScopeID}
			if err := upsertReport(ctx, store.UserMetricsDayExists, store.DeleteUserMetricsDay, store.InsertUserMetrics,
				day, result); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			for scopeID, want := range tt.wantByScope {
				if got := store.count(tableUserMetrics, day, scopeID); got != want {
					t.Errorf("scope_id %q: expected %d rows, got %d", scopeID, want, got)
				}
			}
		})
	}
}

func TestUpsertReport_ExistsErrorSkipsInsert(t *testing.T) {
	ctx := context.Background()
	day := fixedDay()

	store := newScopedStore()
	store.existsErr = errTest

	result := &FetchResult{Records: testRecords(2), Scope: "enterprise", ScopeID: "nav"}
	err := upsertReport(ctx, store.UserMetricsDayExists, store.DeleteUserMetricsDay, store.InsertUserMetrics, day, result)
	if err == nil {
		t.Fatal("expected the existence-check error to surface")
	}
	if got := store.countAllScopes(tableUserMetrics, day); got != 0 {
		t.Errorf("expected no rows written when the existence check fails, got %d", got)
	}
}

func TestUpsertReport_DeleteErrorSkipsInsert(t *testing.T) {
	ctx := context.Background()
	day := fixedDay()

	store := newScopedStore()
	store.seed(tableUserMetrics, day, "nav", 3)
	store.deleteErr = ErrStreamingBuffer

	result := &FetchResult{Records: testRecords(2), Scope: "enterprise", ScopeID: "nav"}
	err := upsertReport(ctx, store.UserMetricsDayExists, store.DeleteUserMetricsDay, store.InsertUserMetrics, day, result)
	if !errors.Is(err, ErrStreamingBuffer) {
		t.Fatalf("expected ErrStreamingBuffer to be preserved for callers, got %v", err)
	}
	// A failed delete must not be followed by an insert, or the day ends up with
	// both the old and the new rows.
	if got := store.count(tableUserMetrics, day, "nav"); got != 3 {
		t.Errorf("expected the 3 existing rows to be left untouched, got %d", got)
	}
}

func TestReportScopeIDs(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want []string
	}{
		{
			name: "distinct enterprise and org slugs",
			cfg:  &Config{EnterpriseSlug: "nav", OrganizationSlug: "navikt"},
			want: []string{"nav", "navikt"},
		},
		{
			name: "identical slugs are not probed twice",
			cfg:  &Config{EnterpriseSlug: "nav", OrganizationSlug: "nav"},
			want: []string{"nav"},
		},
		{
			name: "unset org slug",
			cfg:  &Config{EnterpriseSlug: "nav"},
			want: []string{"nav"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reportScopeIDs(tt.cfg)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestDayExistsInAnyScope(t *testing.T) {
	day := fixedDay()

	tests := []struct {
		name       string
		seedScopes []string
		scopeIDs   []string
		existsErr  error
		want       bool
		wantErr    bool
		wantProbes int
	}{
		{
			name:       "found under the first scope stops probing",
			seedScopes: []string{"nav"},
			scopeIDs:   []string{"nav", "navikt"},
			want:       true,
			wantProbes: 1,
		},
		{
			name:       "found under the fallback scope",
			seedScopes: []string{"navikt"},
			scopeIDs:   []string{"nav", "navikt"},
			want:       true,
			wantProbes: 2,
		},
		{
			name:       "absent from every scope",
			scopeIDs:   []string{"nav", "navikt"},
			want:       false,
			wantProbes: 2,
		},
		{
			name:       "probe error is reported, not read as absent",
			scopeIDs:   []string{"nav", "navikt"},
			existsErr:  errTest,
			wantErr:    true,
			wantProbes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newScopedStore()
			for _, scopeID := range tt.seedScopes {
				store.seed(tableUserTeams, day, scopeID, 1)
			}
			store.existsErr = tt.existsErr

			got, err := dayExistsInAnyScope(context.Background(), store.UserTeamsDayExists, day, tt.scopeIDs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
			if len(store.existsCalls) != tt.wantProbes {
				t.Errorf("expected %d probes, got %d (%v)", tt.wantProbes, len(store.existsCalls), store.existsCalls)
			}
		})
	}
}

// TestIngestMissing_HighWaterMarkSpansScopes is the regression test for the
// cross-scope duplication in usage_metrics that issue #400 describes.
//
// The gap-fill loop resumes from MAX(day), and that query filters on a single
// scope_id. When the newest entity metrics landed under the org scope (the
// enterprise endpoint was failing), an enterprise-only high-water mark points at
// an older day, so the loop walks forward over days that already exist under the
// org scope. By then the enterprise endpoint has recovered, ingestDay's
// DayExists probe — correctly keyed on the scope the fetch resolved to — sees
// nothing, and the day ends up stored under both scope_ids at once.
func TestIngestMissing_HighWaterMarkSpansScopes(t *testing.T) {
	ctx := context.Background()
	cfg := supplementaryConfig()

	store := newScopedStore()
	// The last enterprise-scoped day is three days ago; the two days since then
	// fell back to the org endpoint and are stored under the org scope_id.
	store.seed(tableUsageMetrics, daysAgo(3), "nav", 1)
	store.seed(tableUsageMetrics, daysAgo(2), "navikt", 1)
	store.seed(tableUsageMetrics, daysAgo(1), "navikt", 1)

	// The enterprise endpoint has recovered, so every fetch now resolves to it.
	fetcher := &supplementaryFetcher{scope: "enterprise", scopeID: "nav", records: 1}

	if err := ingestMissing(ctx, fetcher, store, cfg, nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fetcher.entityFetches != 0 {
		t.Errorf("expected no re-ingestion of days already stored under the org scope, got %d entity fetches",
			fetcher.entityFetches)
	}
	for _, day := range []int{1, 2, 3} {
		if got := store.countAllScopes(tableUsageMetrics, daysAgo(day)); got != 1 {
			t.Errorf("day -%d: expected 1 row across scopes, got %d (day duplicated into a second scope)", day, got)
		}
	}
	if got := store.count(tableUsageMetrics, daysAgo(1), "nav"); got != 0 {
		t.Errorf("expected no enterprise-scoped copy of the org-stored day, got %d rows", got)
	}
}

// TestIngestMissing_ResumesFromOrgScopedDay covers the other half of the same
// blind spot: when every stored day is org-scoped, an enterprise-only high-water
// mark reads as "no existing data" and the job silently degrades to ingesting
// yesterday alone, leaving the interior days unfilled forever.
func TestIngestMissing_ResumesFromOrgScopedDay(t *testing.T) {
	ctx := context.Background()
	cfg := supplementaryConfig()

	store := newScopedStore()
	store.seed(tableUsageMetrics, daysAgo(3), "navikt", 1)

	fetcher := &supplementaryFetcher{scope: "enterprise", scopeID: "nav", records: 1}

	if err := ingestMissing(ctx, fetcher, store, cfg, nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fetcher.entityFetches != 2 {
		t.Errorf("expected the two days after the org-scoped high-water mark to be ingested, got %d entity fetches",
			fetcher.entityFetches)
	}
	if got := store.countAllScopes(tableUsageMetrics, daysAgo(3)); got != 1 {
		t.Errorf("expected the org-scoped day to be left alone, got %d rows across scopes", got)
	}
	for _, day := range []int{1, 2} {
		if got := store.count(tableUsageMetrics, daysAgo(day), "nav"); got != 1 {
			t.Errorf("day -%d: expected 1 enterprise-scoped row, got %d", day, got)
		}
	}
}

func TestLatestDayAcrossScopes(t *testing.T) {
	tests := []struct {
		name      string
		seed      map[string]int // scope_id -> days ago
		latestErr error
		scopeIDs  []string
		want      time.Time
		wantErr   bool
	}{
		{
			name:     "no data under any scope",
			scopeIDs: []string{"nav", "navikt"},
			want:     time.Time{},
		},
		{
			name:     "the fallback scope holds the newest day",
			seed:     map[string]int{"nav": 4, "navikt": 2},
			scopeIDs: []string{"nav", "navikt"},
			want:     daysAgo(2),
		},
		{
			name:     "the first scope holds the newest day",
			seed:     map[string]int{"nav": 1, "navikt": 5},
			scopeIDs: []string{"nav", "navikt"},
			want:     daysAgo(1),
		},
		{
			name:     "only the fallback scope has data",
			seed:     map[string]int{"navikt": 3},
			scopeIDs: []string{"nav", "navikt"},
			want:     daysAgo(3),
		},
		{
			// A resume point that cannot be read must not be guessed at — an
			// invented "no data" answer restarts the loop from yesterday.
			name:      "a query error is reported, not read as no data",
			latestErr: errTest,
			scopeIDs:  []string{"nav", "navikt"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newScopedStore()
			for scopeID, day := range tt.seed {
				store.seed(tableUsageMetrics, daysAgo(day), scopeID, 1)
			}
			store.latestErr = tt.latestErr

			got, err := latestDayAcrossScopes(context.Background(), store.GetLatestDay, tt.scopeIDs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// captureLogs redirects the default slog logger into a buffer for the duration
// of one test and returns the records that were emitted.
func captureLogs(t *testing.T) func() []slog.Record {
	t.Helper()

	var mu sync.Mutex
	var records []slog.Record

	previous := slog.Default()
	slog.SetDefault(slog.New(&recordingHandler{
		fn: func(r slog.Record) {
			mu.Lock()
			defer mu.Unlock()
			records = append(records, r)
		},
	}))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return func() []slog.Record {
		mu.Lock()
		defer mu.Unlock()
		return append([]slog.Record(nil), records...)
	}
}

type recordingHandler struct {
	fn func(slog.Record)
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler            { return h }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.fn(r)
	return nil
}

// maxLevelMentioning returns the highest level of any log record whose message
// contains substr, and whether any record matched at all.
func maxLevelMentioning(records []slog.Record, substr string) (slog.Level, bool) {
	var max slog.Level
	var found bool
	for _, r := range records {
		if !strings.Contains(r.Message, substr) {
			continue
		}
		if !found || r.Level > max {
			max, found = r.Level, true
		}
	}
	return max, found
}

// TestIngestMissingSupplementary_StreamingBufferLogsAtInfo pins the gap-fill to
// the convention ingestSupplementary and runRepoMetricsBackfill already follow:
// a DELETE rejected because rows are still in the ~90-minute streaming buffer is
// an expected, self-healing condition, not something to warn the nightly job's
// watchers about.
//
// The branch is reached by giving the probe a narrower scope list than the fetch
// resolves to — enterprise and org slug set to the same value, so the day's rows
// under the org scope_id are invisible to the existence probe but found by
// upsertReport, which then has to delete them.
func TestIngestMissingSupplementary_StreamingBufferLogsAtInfo(t *testing.T) {
	logs := captureLogs(t)

	ctx := context.Background()
	day := daysAgo(1)
	cfg := &Config{EnterpriseSlug: "nav", OrganizationSlug: "nav"}

	store := newScopedStore()
	store.seed(tableUsageMetrics, day, "nav", 1)
	for _, table := range []string{tableUserTeams, tableUserMetrics, tableRepoMetrics} {
		store.seed(table, day, "navikt", 2)
	}
	store.deleteErr = ErrStreamingBuffer

	fetcher := &supplementaryFetcher{scope: "organization", scopeID: "navikt", records: 2}

	ingestMissingSupplementary(ctx, fetcher, store, cfg)

	records := logs()
	for _, report := range []string{"user-teams", "user-metrics", "repo-metrics"} {
		level, found := maxLevelMentioning(records, "Skipping "+report+" re-import")
		if !found {
			t.Errorf("%s: expected a streaming-buffer skip to be logged, found none", report)
		} else if level != slog.LevelInfo {
			t.Errorf("%s: expected the streaming-buffer skip to be logged at Info, got %v", report, level)
		}

		if level, found := maxLevelMentioning(records, "Failed to insert missing "+report); found {
			t.Errorf("%s: expected no failure log for a streaming-buffer skip, got one at %v", report, level)
		}
	}

	// A rejected delete must leave the existing rows alone rather than stacking
	// a second copy on top of them.
	for _, table := range []string{tableUserTeams, tableUserMetrics, tableRepoMetrics} {
		if got := store.countAllScopes(table, day); got != 2 {
			t.Errorf("%s: expected the 2 existing rows to be untouched, got %d", table, got)
		}
	}
}
