package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
)

const billingUsageTable = "billing_usage"

// BillingUsageRow represents a row in the billing_usage BigQuery table.
type BillingUsageRow struct {
	Day           string    `bigquery:"day"`
	Year          int       `bigquery:"year"`
	Month         int       `bigquery:"month"`
	ScopeID       string    `bigquery:"scope_id"`
	Product       string    `bigquery:"product"`
	SKU           string    `bigquery:"sku"`
	Model         string    `bigquery:"model"`
	UnitType      string    `bigquery:"unit_type"`
	PricePerUnit  float64   `bigquery:"price_per_unit"`
	GrossQuantity float64   `bigquery:"gross_quantity"`
	GrossAmount   float64   `bigquery:"gross_amount"`
	NetQuantity   float64   `bigquery:"net_quantity"`
	NetAmount     float64   `bigquery:"net_amount"`
	RawRecord     string    `bigquery:"raw_record"`
	LoadedAt      time.Time `bigquery:"loaded_at"`
}

// EnsureBillingTableExists creates the billing_usage table if it doesn't exist.
func (c *BigQueryClient) EnsureBillingTableExists(ctx context.Context) error {
	dataset := c.client.Dataset(c.dataset)
	table := dataset.Table(billingUsageTable)

	_, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("Billing table already exists", "table", billingUsageTable)
		return nil
	}

	slog.Info("Creating billing_usage table", "dataset", c.dataset)

	schema := bigquery.Schema{
		{Name: "day", Type: bigquery.DateFieldType, Required: true, Description: "Calendar day of the billing data (first of month for monthly aggregates)"},
		{Name: "year", Type: bigquery.IntegerFieldType, Required: true, Description: "Year of the billing period"},
		{Name: "month", Type: bigquery.IntegerFieldType, Required: true, Description: "Month of the billing period"},
		{Name: "scope_id", Type: bigquery.StringFieldType, Required: true, Description: "Enterprise slug"},
		{Name: "product", Type: bigquery.StringFieldType, Required: true, Description: "Product name (e.g. Copilot)"},
		{Name: "sku", Type: bigquery.StringFieldType, Required: true, Description: "SKU name (e.g. Copilot Premium Request)"},
		{Name: "model", Type: bigquery.StringFieldType, Required: true, Description: "Model name (e.g. Claude Opus 4.7)"},
		{Name: "unit_type", Type: bigquery.StringFieldType, Required: true, Description: "Unit type (e.g. requests)"},
		{Name: "price_per_unit", Type: bigquery.FloatFieldType, Required: true, Description: "Price per unit in USD"},
		{Name: "gross_quantity", Type: bigquery.FloatFieldType, Required: true, Description: "Total quantity before discounts"},
		{Name: "gross_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Total amount in USD before discounts"},
		{Name: "net_quantity", Type: bigquery.FloatFieldType, Required: true, Description: "Quantity after discounts (billed)"},
		{Name: "net_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Amount in USD after discounts (billed)"},
		{Name: "raw_record", Type: bigquery.JSONFieldType, Required: true, Description: "Full JSON record from the API"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was inserted"},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.MonthPartitioningType,
			Field: "day",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"scope_id", "model"},
		},
		Description: "GitHub Copilot premium request billing usage per model from the Enhanced Billing API",
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create billing_usage table: %w", err)
	}

	slog.Info("billing_usage table created successfully")
	return nil
}

// InsertBillingUsage stores billing usage items in BigQuery.
func (c *BigQueryClient) InsertBillingUsage(ctx context.Context, year, month int, scopeID string, items []BillingUsageItem) error {
	if len(items) == 0 {
		slog.Warn("No billing items to insert")
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(billingUsageTable)
	inserter := table.Inserter()

	dayStr := fmt.Sprintf("%04d-%02d-01", year, month)
	loadedAt := time.Now().UTC()

	var rows []*BillingUsageRow
	for _, item := range items {
		raw, _ := json.Marshal(item)
		rows = append(rows, &BillingUsageRow{
			Day:           dayStr,
			Year:          year,
			Month:         month,
			ScopeID:       scopeID,
			Product:       item.Product,
			SKU:           item.SKU,
			Model:         item.Model,
			UnitType:      item.UnitType,
			PricePerUnit:  item.PricePerUnit,
			GrossQuantity: item.GrossQuantity,
			GrossAmount:   item.GrossAmount,
			NetQuantity:   item.NetQuantity,
			NetAmount:     item.NetAmount,
			RawRecord:     string(raw),
			LoadedAt:      loadedAt,
		})
	}

	if err := inserter.Put(ctx, rows); err != nil {
		return fmt.Errorf("failed to insert billing rows: %w", err)
	}

	slog.Info("Inserted billing usage", "year", year, "month", month, "rows", len(rows))
	return nil
}

// BillingMonthExists checks if we already have billing data for a given month.
func (c *BigQueryClient) BillingMonthExists(ctx context.Context, year, month int, scopeID string) (bool, error) {
	dayStr := fmt.Sprintf("%04d-%02d-01", year, month)

	query := c.client.Query(`
		SELECT COUNT(*) as cnt
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + billingUsageTable + "`" + `
		WHERE day = @day AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: dayStr},
		{Name: "scopeID", Value: scopeID},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check billing month: %w", err)
	}

	var row struct {
		Cnt int64 `bigquery:"cnt"`
	}
	if err := it.Next(&row); err != nil {
		return false, fmt.Errorf("failed to read billing check result: %w", err)
	}

	return row.Cnt > 0, nil
}

// DeleteBillingMonth removes existing billing data for a month (for re-ingestion).
func (c *BigQueryClient) DeleteBillingMonth(ctx context.Context, year, month int, scopeID string) error {
	dayStr := fmt.Sprintf("%04d-%02d-01", year, month)

	query := c.client.Query(`
		DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + billingUsageTable + "`" + `
		WHERE day = @day AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: dayStr},
		{Name: "scopeID", Value: scopeID},
	}

	if err := c.runDeleteQuery(ctx, query, billingUsageTable); err != nil {
		return fmt.Errorf("delete billing month: %w", err)
	}
	slog.Debug("Deleted existing billing data", "year", year, "month", month, "scope_id", scopeID)
	return nil
}
