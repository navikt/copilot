package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

const billingUsageReportsTable = "billing_usage_reports"

// BillingUsageReportRow represents one row in the billing_usage_reports table.
type BillingUsageReportRow struct {
	ReportDay      civil.Date          `bigquery:"report_day"`
	Organization   string              `bigquery:"organization"`
	RepositoryName bigquery.NullString `bigquery:"repository_name"`
	Product        string              `bigquery:"product"`
	SKU            string              `bigquery:"sku"`
	Quantity       float64             `bigquery:"quantity"`
	UnitType       string              `bigquery:"unit_type"`
	PricePerUnit   float64             `bigquery:"price_per_unit"`
	GrossAmount    float64             `bigquery:"gross_amount"`
	DiscountAmount float64             `bigquery:"discount_amount"`
	NetAmount      float64             `bigquery:"net_amount"`
	RawRecord      string              `bigquery:"raw_record"`
	LoadedAt       time.Time           `bigquery:"loaded_at"`
}

// EnsureBillingUsageReportsTableExists creates the billing_usage_reports table if needed.
func (c *BigQueryClient) EnsureBillingUsageReportsTableExists(ctx context.Context) error {
	dataset := c.client.Dataset(c.dataset)
	table := dataset.Table(billingUsageReportsTable)

	_, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("Billing usage reports table already exists", "table", billingUsageReportsTable)
		return nil
	}

	slog.Info("Creating billing_usage_reports table", "dataset", c.dataset)

	schema := bigquery.Schema{
		{Name: "report_day", Type: bigquery.DateFieldType, Required: true, Description: "Day covered by the billing usage report"},
		{Name: "organization", Type: bigquery.StringFieldType, Required: true, Description: "Organization login"},
		{Name: "repository_name", Type: bigquery.StringFieldType, Required: false, Description: "Repository name when usage is repository scoped"},
		{Name: "product", Type: bigquery.StringFieldType, Required: true, Description: "Product name"},
		{Name: "sku", Type: bigquery.StringFieldType, Required: true, Description: "SKU name"},
		{Name: "quantity", Type: bigquery.FloatFieldType, Required: true, Description: "Quantity in the report line item"},
		{Name: "unit_type", Type: bigquery.StringFieldType, Required: true, Description: "Unit type for quantity"},
		{Name: "price_per_unit", Type: bigquery.FloatFieldType, Required: true, Description: "Price per unit in USD"},
		{Name: "gross_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Gross amount in USD"},
		{Name: "discount_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Discount amount in USD"},
		{Name: "net_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Net amount in USD"},
		{Name: "raw_record", Type: bigquery.JSONFieldType, Required: true, Description: "Original usage item JSON"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "Row ingestion timestamp"},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.MonthPartitioningType,
			Field: "report_day",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"organization", "product", "sku"},
		},
		Description: "Daily organization billing usage report rows from GitHub billing usage API",
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create billing_usage_reports table: %w", err)
	}

	slog.Info("billing_usage_reports table created successfully")
	return nil
}

// DeleteBillingUsageReportDay removes existing rows for day+org so re-runs are idempotent.
func (c *BigQueryClient) DeleteBillingUsageReportDay(ctx context.Context, day time.Time, org string) error {
	query := c.client.Query(`
		DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + billingUsageReportsTable + "`" + `
		WHERE report_day = @day AND organization = @org
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: civil.DateOf(day)},
		{Name: "org", Value: org},
	}

	if err := c.runDeleteQuery(ctx, query, billingUsageReportsTable); err != nil {
		return fmt.Errorf("delete billing usage report: %w", err)
	}
	slog.Debug("Deleted existing billing usage report rows", "day", day.Format("2006-01-02"), "org", org)
	return nil
}

// InsertBillingUsageReportDay inserts one day's usage report rows.
func (c *BigQueryClient) InsertBillingUsageReportDay(ctx context.Context, day time.Time, org string, items []OrganizationBillingUsageItem) error {
	if len(items) == 0 {
		slog.Info("No billing usage report items to insert", "day", day.Format("2006-01-02"), "org", org)
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(billingUsageReportsTable)
	inserter := table.Inserter()
	reportDay := civil.DateOf(day)
	loadedAt := time.Now().UTC()

	var rows []*BillingUsageReportRow
	for _, item := range items {
		raw, _ := json.Marshal(item)
		rows = append(rows, &BillingUsageReportRow{
			ReportDay:      reportDay,
			Organization:   org,
			RepositoryName: nullableRepositoryName(item.RepositoryName),
			Product:        item.Product,
			SKU:            item.SKU,
			Quantity:       item.Quantity,
			UnitType:       item.UnitType,
			PricePerUnit:   item.PricePerUnit,
			GrossAmount:    item.GrossAmount,
			DiscountAmount: item.DiscountAmount,
			NetAmount:      item.NetAmount,
			RawRecord:      string(raw),
			LoadedAt:       loadedAt,
		})
	}

	if err := inserter.Put(ctx, rows); err != nil {
		return fmt.Errorf("insert billing usage report rows: %w", err)
	}

	slog.Info("Inserted billing usage report rows", "day", day.Format("2006-01-02"), "org", org, "rows", len(rows))
	return nil
}

func nullableRepositoryName(name string) bigquery.NullString {
	if name == "" {
		return bigquery.NullString{}
	}
	return bigquery.NullString{StringVal: name, Valid: true}
}

// GetLatestBillingUsageReportDay returns latest ingested day for the org.
func (c *BigQueryClient) GetLatestBillingUsageReportDay(ctx context.Context, org string) (time.Time, error) {
	query := c.client.Query(`
		SELECT MAX(report_day) as latest_day
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + billingUsageReportsTable + "`" + `
		WHERE organization = @org
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "org", Value: org},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("query latest billing usage report day: %w", err)
	}

	var row struct {
		LatestDay bigquery.NullDate `bigquery:"latest_day"`
	}
	if err := it.Next(&row); err != nil {
		return time.Time{}, fmt.Errorf("read latest billing usage report day: %w", err)
	}

	if !row.LatestDay.Valid {
		return time.Time{}, nil
	}

	return time.Date(
		row.LatestDay.Date.Year,
		row.LatestDay.Date.Month,
		row.LatestDay.Date.Day,
		0, 0, 0, 0, time.UTC,
	), nil
}
