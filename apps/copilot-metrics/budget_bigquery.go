package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

const budgetSnapshotsTable = "budget_snapshots"

// BudgetSnapshotRow represents a row in the budget_snapshots BigQuery table.
type BudgetSnapshotRow struct {
	SnapshotDate   civil.Date           `bigquery:"snapshot_date"`
	ScopeID        string               `bigquery:"scope_id"`
	BudgetScope    string               `bigquery:"budget_scope"`
	EntityName     string               `bigquery:"entity_name"`
	BudgetAmount   float64              `bigquery:"budget_amount"`
	ConsumedAmount bigquery.NullFloat64 `bigquery:"consumed_amount"`
	IsOverride     bool                 `bigquery:"is_override"`
	LoadedAt       time.Time            `bigquery:"loaded_at"`
}

// EnsureBudgetSnapshotsTableExists creates the budget_snapshots table if it doesn't exist.
func (c *BigQueryClient) EnsureBudgetSnapshotsTableExists(ctx context.Context) error {
	dataset := c.client.Dataset(c.dataset)
	table := dataset.Table(budgetSnapshotsTable)

	_, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("Budget snapshots table already exists", "table", budgetSnapshotsTable)
		return nil
	}

	slog.Info("Creating budget_snapshots table", "dataset", c.dataset)

	schema := bigquery.Schema{
		{Name: "snapshot_date", Type: bigquery.DateFieldType, Required: true, Description: "Date the snapshot was taken"},
		{Name: "scope_id", Type: bigquery.StringFieldType, Required: true, Description: "Enterprise slug"},
		{Name: "budget_scope", Type: bigquery.StringFieldType, Required: true, Description: "Budget scope: multi_user_customer or user"},
		{Name: "entity_name", Type: bigquery.StringFieldType, Required: true, Description: "Entity name: enterprise slug for defaults, GitHub username for user overrides"},
		{Name: "budget_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Budget limit in USD"},
		{Name: "consumed_amount", Type: bigquery.FloatFieldType, Required: false, Description: "Consumed amount in USD this month (null if not tracked individually)"},
		{Name: "is_override", Type: bigquery.BooleanFieldType, Required: true, Description: "True if this is a user-specific override of the enterprise default"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was inserted"},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.MonthPartitioningType,
			Field: "snapshot_date",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"scope_id", "budget_scope", "entity_name"},
		},
		Description: "Daily snapshots of GitHub Copilot AI credit budgets per user and enterprise default",
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create budget_snapshots table: %w", err)
	}

	slog.Info("budget_snapshots table created successfully")
	return nil
}

// BudgetSnapshotExists checks if we already have a snapshot for the given date.
func (c *BigQueryClient) BudgetSnapshotExists(ctx context.Context, date time.Time, scopeID string) (bool, error) {
	query := c.client.Query(`
		SELECT COUNT(*) as cnt
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + budgetSnapshotsTable + "`" + `
		WHERE snapshot_date = @date AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "date", Value: civil.DateOf(date)},
		{Name: "scopeID", Value: scopeID},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return false, fmt.Errorf("check budget snapshot: %w", err)
	}

	var row struct {
		Cnt int64 `bigquery:"cnt"`
	}
	if err := it.Next(&row); err != nil {
		return false, fmt.Errorf("read budget snapshot check: %w", err)
	}

	return row.Cnt > 0, nil
}

// DeleteBudgetSnapshot removes existing snapshots for a date (for re-ingestion).
func (c *BigQueryClient) DeleteBudgetSnapshot(ctx context.Context, date time.Time, scopeID string) error {
	query := c.client.Query(`
		DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + budgetSnapshotsTable + "`" + `
		WHERE snapshot_date = @date AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "date", Value: civil.DateOf(date)},
		{Name: "scopeID", Value: scopeID},
	}

	if err := c.runDeleteQuery(ctx, query, budgetSnapshotsTable); err != nil {
		return fmt.Errorf("delete budget snapshot: %w", err)
	}
	slog.Debug("Deleted existing budget snapshot", "date", date.Format("2006-01-02"), "scope_id", scopeID)
	return nil
}

// InsertBudgetSnapshots stores budget entries as a daily snapshot in BigQuery.
func (c *BigQueryClient) InsertBudgetSnapshots(ctx context.Context, date time.Time, scopeID string, entries []BudgetEntry) error {
	if len(entries) == 0 {
		slog.Warn("No budget entries to insert")
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(budgetSnapshotsTable)
	inserter := table.Inserter()

	snapshotDate := civil.DateOf(date)
	loadedAt := time.Now().UTC()

	var rows []*BudgetSnapshotRow
	for _, e := range entries {
		isOverride := e.BudgetScope == "user"
		entityName := e.BudgetEntityName
		if entityName == "" {
			entityName = scopeID
		}
		var consumed bigquery.NullFloat64
		if e.ConsumedAmount != nil {
			consumed = bigquery.NullFloat64{Float64: *e.ConsumedAmount, Valid: true}
		}
		rows = append(rows, &BudgetSnapshotRow{
			SnapshotDate:   snapshotDate,
			ScopeID:        scopeID,
			BudgetScope:    e.BudgetScope,
			EntityName:     entityName,
			BudgetAmount:   e.BudgetAmount,
			ConsumedAmount: consumed,
			IsOverride:     isOverride,
			LoadedAt:       loadedAt,
		})
	}

	if err := inserter.Put(ctx, rows); err != nil {
		return fmt.Errorf("insert budget snapshot rows: %w", err)
	}

	slog.Info("Inserted budget snapshot", "date", date.Format("2006-01-02"), "rows", len(rows))
	return nil
}

const userBudgetSnapshotsTable = "user_budget_snapshots"

// UserBudgetSnapshotRow represents a row in the user_budget_snapshots BigQuery table.
type UserBudgetSnapshotRow struct {
	SnapshotDate   civil.Date `bigquery:"snapshot_date"`
	ScopeID        string     `bigquery:"scope_id"`
	GitHubLogin    string     `bigquery:"github_login"`
	BudgetAmount   float64    `bigquery:"budget_amount"`
	ConsumedAmount float64    `bigquery:"consumed_amount"`
	IsOverride     bool       `bigquery:"is_override"`
	LoadedAt       time.Time  `bigquery:"loaded_at"`
}

// EnsureUserBudgetSnapshotsTableExists creates the user_budget_snapshots table if needed.
func (c *BigQueryClient) EnsureUserBudgetSnapshotsTableExists(ctx context.Context) error {
	dataset := c.client.Dataset(c.dataset)
	table := dataset.Table(userBudgetSnapshotsTable)

	_, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("User budget snapshots table already exists", "table", userBudgetSnapshotsTable)
		return nil
	}

	slog.Info("Creating user_budget_snapshots table", "dataset", c.dataset)

	schema := bigquery.Schema{
		{Name: "snapshot_date", Type: bigquery.DateFieldType, Required: true, Description: "Date the snapshot was taken"},
		{Name: "scope_id", Type: bigquery.StringFieldType, Required: true, Description: "Enterprise slug"},
		{Name: "github_login", Type: bigquery.StringFieldType, Required: true, Description: "GitHub username of the Copilot seat holder"},
		{Name: "budget_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Effective budget limit in USD for this user this month"},
		{Name: "consumed_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Consumed AI credits in USD so far this month"},
		{Name: "is_override", Type: bigquery.BooleanFieldType, Required: true, Description: "True if user has an individual override budget (default: false = enterprise default)"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was inserted"},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.MonthPartitioningType,
			Field: "snapshot_date",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"scope_id", "github_login"},
		},
		Description: "Daily snapshots of AI credit budget consumption per Copilot user. One row per user per day.",
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create user_budget_snapshots table: %w", err)
	}

	slog.Info("user_budget_snapshots table created successfully")
	return nil
}

// UserBudgetSnapshotExists checks if we already have a snapshot for the given date.
func (c *BigQueryClient) UserBudgetSnapshotExists(ctx context.Context, date time.Time, scopeID string) (bool, error) {
	query := c.client.Query(`
SELECT COUNT(*) as cnt
FROM ` + "`" + c.projectID + "." + c.dataset + "." + userBudgetSnapshotsTable + "`" + `
WHERE snapshot_date = @date AND scope_id = @scopeID
`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "date", Value: civil.DateOf(date)},
		{Name: "scopeID", Value: scopeID},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return false, fmt.Errorf("check user budget snapshot: %w", err)
	}

	var row struct {
		Cnt int64 `bigquery:"cnt"`
	}
	if err := it.Next(&row); err != nil {
		return false, fmt.Errorf("read user budget snapshot check: %w", err)
	}

	return row.Cnt > 0, nil
}

// DeleteUserBudgetSnapshot removes the snapshot for a given date (for re-ingestion).
func (c *BigQueryClient) DeleteUserBudgetSnapshot(ctx context.Context, date time.Time, scopeID string) error {
	query := c.client.Query(`
DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + userBudgetSnapshotsTable + "`" + `
WHERE snapshot_date = @date AND scope_id = @scopeID
`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "date", Value: civil.DateOf(date)},
		{Name: "scopeID", Value: scopeID},
	}

	if err := c.runDeleteQuery(ctx, query, userBudgetSnapshotsTable); err != nil {
		return fmt.Errorf("delete user budget snapshot: %w", err)
	}
	slog.Debug("Deleted user budget snapshot", "date", date.Format("2006-01-02"), "scope_id", scopeID)
	return nil
}

// InsertUserBudgetSnapshots stores per-user budget consumption as a daily snapshot.
func (c *BigQueryClient) InsertUserBudgetSnapshots(ctx context.Context, date time.Time, scopeID string, entries []UserBudgetData) error {
	if len(entries) == 0 {
		slog.Warn("No user budget entries to insert")
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(userBudgetSnapshotsTable)
	inserter := table.Inserter()

	snapshotDate := civil.DateOf(date)
	loadedAt := time.Now().UTC()

	var rows []*UserBudgetSnapshotRow
	for _, e := range entries {
		rows = append(rows, &UserBudgetSnapshotRow{
			SnapshotDate:   snapshotDate,
			ScopeID:        scopeID,
			GitHubLogin:    e.GitHubLogin,
			BudgetAmount:   e.BudgetAmount,
			ConsumedAmount: e.ConsumedAmount,
			IsOverride:     e.IsOverride,
			LoadedAt:       loadedAt,
		})
	}

	if err := inserter.Put(ctx, rows); err != nil {
		return fmt.Errorf("insert user budget snapshot rows: %w", err)
	}

	slog.Info("Inserted user budget snapshots", "date", date.Format("2006-01-02"), "rows", len(rows))
	return nil
}
