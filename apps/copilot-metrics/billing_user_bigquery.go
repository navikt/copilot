package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

const userBillingTable = "billing_user_monthly"

const userBillingRunsTable = "billing_user_monthly_runs"

type UserBillingRun struct {
	Month    civil.Date `bigquery:"month"`
	ScopeID  string     `bigquery:"scope_id"`
	Users    int64      `bigquery:"users"`
	Status   string     `bigquery:"status"`
	LoadedAt time.Time  `bigquery:"loaded_at"`
}

type UserBillingRow struct {
	Month       civil.Date `bigquery:"month"`
	ScopeID     string     `bigquery:"scope_id"`
	UserID      string     `bigquery:"user_id"`
	GitHubLogin string     `bigquery:"github_login"`
	SKU         string     `bigquery:"sku"`
	GrossAmount float64    `bigquery:"gross_amount"`
	NetAmount   float64    `bigquery:"net_amount"`
	LoadedAt    time.Time  `bigquery:"loaded_at"`
}

func (c *BigQueryClient) EnsureUserBillingTableExists(ctx context.Context) error {
	table := c.client.Dataset(c.dataset).Table(userBillingTable)
	if _, err := table.Metadata(ctx); err == nil {
		return nil
	} else if !billingTableNotFound(err) {
		return err
	}
	schema, err := bigquery.InferSchema(UserBillingRow{})
	if err != nil {
		return fmt.Errorf("infer user billing schema: %w", err)
	}
	if err := table.Create(ctx, &bigquery.TableMetadata{
		Schema:           schema,
		TimePartitioning: &bigquery.TimePartitioning{Type: bigquery.MonthPartitioningType, Field: "month"},
		Clustering:       &bigquery.Clustering{Fields: []string{"scope_id", "user_id"}},
	}); err != nil {
		return err
	}
	return nil
}

func (c *BigQueryClient) EnsureUserBillingRunsTableExists(ctx context.Context) error {
	table := c.client.Dataset(c.dataset).Table(userBillingRunsTable)
	if _, err := table.Metadata(ctx); err == nil {
		return nil
	} else if !billingTableNotFound(err) {
		return err
	}
	schema, err := bigquery.InferSchema(UserBillingRun{})
	if err != nil {
		return err
	}
	return table.Create(ctx, &bigquery.TableMetadata{
		Schema:           schema,
		TimePartitioning: &bigquery.TimePartitioning{Type: bigquery.MonthPartitioningType, Field: "month"},
	})
}

func billingTableNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 404
}

func (c *BigQueryClient) CompleteUserBilling(ctx context.Context, month time.Time, scope string, users int) error {
	// The completion marker only follows a successful pass over every source user.
	q := c.client.Query(`MERGE ` + "`" + c.projectID + "." + c.dataset + "." + userBillingRunsTable + "`" + ` t
USING (SELECT @month month, @scope scope_id, @users users, 'complete' status, @loaded loaded_at) s
ON t.month=s.month AND t.scope_id=s.scope_id
WHEN MATCHED THEN UPDATE SET users=s.users, status=s.status, loaded_at=s.loaded_at
WHEN NOT MATCHED THEN INSERT (month,scope_id,users,status,loaded_at) VALUES (s.month,s.scope_id,s.users,s.status,s.loaded_at)`)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "month", Value: civil.DateOf(month)}, {Name: "scope", Value: scope},
		{Name: "users", Value: users}, {Name: "loaded", Value: time.Now().UTC()},
	}
	_, err := q.Read(ctx)
	return err
}

func (c *BigQueryClient) UserBillingDone(ctx context.Context, month time.Time, scope string) (map[string]bool, error) {
	q := c.client.Query(`SELECT DISTINCT user_id FROM ` + "`" + c.projectID + "." + c.dataset + "." + userBillingTable + "`" + `
WHERE month=@month AND scope_id=@scope AND sku='done'`)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "month", Value: civil.DateOf(month)}, {Name: "scope", Value: scope},
	}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}
	done := map[string]bool{}
	for {
		var row struct {
			UserID string `bigquery:"user_id"`
		}
		if err := it.Next(&row); err != nil {
			if err == iterator.Done {
				return done, nil
			}
			return nil, err
		}
		done[row.UserID] = true
	}
}

func (c *BigQueryClient) UserBillingRunComplete(ctx context.Context, month time.Time, scope string) (bool, error) {
	q := c.client.Query(`SELECT COUNT(*) n FROM ` + "`" + c.projectID + "." + c.dataset + "." + userBillingRunsTable + "`" + `
WHERE month=@month AND scope_id=@scope AND status='complete'`)
	q.Parameters = []bigquery.QueryParameter{{Name: "month", Value: civil.DateOf(month)}, {Name: "scope", Value: scope}}
	it, err := q.Read(ctx)
	if err != nil {
		return false, err
	}
	var row struct {
		N int64 `bigquery:"n"`
	}
	if err := it.Next(&row); err != nil {
		return false, err
	}
	return row.N > 0, nil
}

// Replace the user snapshot and its completion marker in one transaction so a
// retry cannot preserve SKUs from a response that never finished storing.
func (c *BigQueryClient) ReplaceUserBilling(ctx context.Context, rows []UserBillingRow) error {
	if len(rows) == 0 {
		return fmt.Errorf("billing snapshot must include a completion marker")
	}
	row := rows[0]
	table := "`" + c.projectID + "." + c.dataset + "." + userBillingTable + "`"
	q := c.client.Query(userBillingReplacementSQL(table))
	q.Parameters = []bigquery.QueryParameter{
		{Name: "month", Value: row.Month}, {Name: "scope", Value: row.ScopeID},
		{Name: "uid", Value: row.UserID}, {Name: "rows", Value: rows},
	}
	_, err := q.Read(ctx)
	return err
}

func userBillingReplacementSQL(table string) string {
	return `BEGIN TRANSACTION;
DELETE FROM ` + table + ` WHERE month=@month AND scope_id=@scope AND user_id=@uid;
INSERT INTO ` + table + ` (month,scope_id,user_id,github_login,sku,gross_amount,net_amount,loaded_at)
SELECT month,scope_id,user_id,github_login,sku,gross_amount,net_amount,loaded_at FROM UNNEST(@rows);
COMMIT TRANSACTION;`
}

func (c *BigQueryClient) GetBillingUsers(ctx context.Context, month time.Time, scope string) (map[string]string, error) {
	q := c.client.Query(`WITH source AS (
  SELECT JSON_VALUE(raw_record, '$.user_id') user_id, 0 priority,
    ARRAY_AGG(JSON_VALUE(raw_record, '$.user_login') ORDER BY day DESC LIMIT 1)[OFFSET(0)] login
  FROM ` + "`" + c.projectID + "." + c.dataset + "." + c.userTeamsTable + "`" + `
  WHERE day >= @start AND day < @end AND scope='enterprise' AND scope_id=@scope
  GROUP BY user_id
  UNION ALL
  SELECT JSON_VALUE(raw_record, '$.user_id') user_id, 1 priority,
    ARRAY_AGG(JSON_VALUE(raw_record, '$.user_login') ORDER BY day DESC LIMIT 1)[OFFSET(0)] login
  FROM ` + "`" + c.projectID + "." + c.dataset + "." + c.userMetricsTable + "`" + `
  WHERE day >= @start AND day < @end AND scope='enterprise' AND scope_id=@scope
  GROUP BY user_id
)
SELECT user_id, ARRAY_AGG(login ORDER BY priority LIMIT 1)[OFFSET(0)] login
FROM source WHERE user_id IS NOT NULL AND login IS NOT NULL AND login != '' GROUP BY user_id`)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "start", Value: civil.DateOf(month)},
		{Name: "end", Value: civil.DateOf(month.AddDate(0, 1, 0))},
		{Name: "scope", Value: scope},
	}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}
	users := map[string]string{}
	for {
		var row struct {
			UserID string `bigquery:"user_id"`
			Login  string `bigquery:"login"`
		}
		if err := it.Next(&row); err != nil {
			if err == iterator.Done {
				return users, nil
			}
			return nil, err
		}
		users[row.UserID] = strings.ToLower(row.Login)
	}
}
