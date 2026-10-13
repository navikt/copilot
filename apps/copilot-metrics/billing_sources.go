package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/iterator"
)

const billingSourceReceipts = "billing_source_days"

func (c *BigQueryClient) ensureBillingSourceReceipts(ctx context.Context) error {
	table := c.client.Dataset(c.dataset).Table(billingSourceReceipts)
	if _, err := table.Metadata(ctx); err == nil {
		return nil
	} else if !billingTableNotFound(err) {
		return err
	}
	return table.Create(ctx, &bigquery.TableMetadata{Schema: bigquery.Schema{
		{Name: "day", Type: bigquery.DateFieldType},
		{Name: "report", Type: bigquery.StringFieldType},
		{Name: "scope_id", Type: bigquery.StringFieldType},
		{Name: "records", Type: bigquery.IntegerFieldType},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType},
	}})
}

func (c *BigQueryClient) ReplaceUserTeams(ctx context.Context, day time.Time, result *FetchResult) error {
	return c.replaceAllocationReport(ctx, c.userTeamsTable, day, result)
}

func (c *BigQueryClient) ReplaceUserMetrics(ctx context.Context, day time.Time, result *FetchResult) error {
	return c.replaceAllocationReport(ctx, c.userMetricsTable, day, result)
}

func validateAllocationReport(day time.Time, result *FetchResult, teams bool) error {
	seen := map[string]bool{}
	for _, raw := range result.Records {
		var row struct {
			Day     string      `json:"day"`
			UserID  json.Number `json:"user_id"`
			Login   string      `json:"user_login"`
			TeamID  json.Number `json:"team_id"`
			Slug    string      `json:"slug"`
			Credits *float64    `json:"ai_credits_used"`
		}
		if err := json.Unmarshal(raw, &row); err != nil || row.UserID == "" || row.Login == "" || row.Day != day.Format("2006-01-02") {
			return fmt.Errorf("allocation report has an invalid user or day")
		}
		if id, err := row.UserID.Int64(); err != nil || id <= 0 {
			return fmt.Errorf("allocation report has an invalid user ID")
		}
		key := row.UserID.String()
		if !teams && (row.Credits == nil || math.IsNaN(*row.Credits) || math.IsInf(*row.Credits, 0) || *row.Credits < 0) {
			return fmt.Errorf("allocation report has an invalid credit amount")
		}
		if teams {
			if row.TeamID == "" || row.Slug == "" {
				return fmt.Errorf("allocation report has an invalid team")
			}
			if id, err := row.TeamID.Int64(); err != nil || id <= 0 {
				return fmt.Errorf("allocation report has an invalid team ID")
			}
			key += ":" + row.TeamID.String()
		}
		if seen[key] {
			return fmt.Errorf("allocation report has duplicate records")
		}
		seen[key] = true
	}
	return nil
}

func (c *BigQueryClient) replaceAllocationReport(ctx context.Context, table string, day time.Time, result *FetchResult) error {
	if result == nil || (result.Scope == "enterprise" && !result.Complete) {
		return fmt.Errorf("allocation report is incomplete")
	}
	if err := validateAllocationReport(day, result, table == c.userTeamsTable); err != nil {
		return err
	}
	if err := c.ensureBillingSourceReceipts(ctx); err != nil {
		return err
	}
	records := make([]string, 0, len(result.Records))
	for _, raw := range result.Records {
		records = append(records, string(raw))
	}
	receipts := "`" + c.projectID + "." + c.dataset + "." + billingSourceReceipts + "`"
	q := c.client.Query(allocationReplacementSQL("`"+c.projectID+"."+c.dataset+"."+table+"`", receipts))
	q.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: civil.DateOf(day)}, {Name: "scope", Value: result.Scope},
		{Name: "scopeID", Value: result.ScopeID}, {Name: "report", Value: table},
		{Name: "records", Value: records}, {Name: "loaded", Value: time.Now().UTC()},
		{Name: "complete", Value: result.Complete && result.Scope == "enterprise"},
	}
	_, err := q.Read(ctx)
	return err
}

func allocationReplacementSQL(table, receipts string) string {
	return `BEGIN TRANSACTION;
DELETE FROM ` + table + ` WHERE day=@day AND scope_id=@scopeID;
INSERT INTO ` + table + ` (day,scope,scope_id,raw_record,loaded_at)
SELECT @day,@scope,@scopeID,PARSE_JSON(raw),@loaded FROM UNNEST(@records) raw;
DELETE FROM ` + receipts + ` WHERE day=@day AND scope_id=@scopeID AND report=@report;
IF @complete THEN
INSERT INTO ` + receipts + ` (day,report,scope_id,records,loaded_at) VALUES (@day,@report,@scopeID,COALESCE(ARRAY_LENGTH(@records),0),@loaded);
END IF;
COMMIT TRANSACTION;`
}

func (c *BigQueryClient) BillingSourceDays(ctx context.Context, month time.Time, scope string) (map[string]bool, error) {
	if err := c.ensureBillingSourceReceipts(ctx); err != nil {
		return nil, err
	}
	q := c.client.Query(`SELECT CAST(day AS STRING) day,report FROM ` + "`" + c.projectID + "." + c.dataset + "." + billingSourceReceipts + "`" + ` WHERE day>=@start AND day<@end AND scope_id=@scope`)
	q.Parameters = []bigquery.QueryParameter{{Name: "start", Value: civil.DateOf(month)}, {Name: "end", Value: civil.DateOf(month.AddDate(0, 1, 0))}, {Name: "scope", Value: scope}}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}
	days := map[string]bool{}
	for {
		var row struct {
			Day    string
			Report string
		}
		if err := it.Next(&row); err != nil {
			if err == iterator.Done {
				return days, nil
			}
			return nil, err
		}
		days[row.Day+":"+row.Report] = true
	}
}
