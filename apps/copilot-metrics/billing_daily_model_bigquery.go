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

const billingUsageDailyModelTable = "billing_usage_daily_model"

type BillingUsageDailyModelRow struct {
	Day              civil.Date `bigquery:"day"`
	ScopeID          string     `bigquery:"scope_id"`
	Product          string     `bigquery:"product"`
	SKU              string     `bigquery:"sku"`
	Model            string     `bigquery:"model"`
	UnitType         string     `bigquery:"unit_type"`
	PricePerUnit     float64    `bigquery:"price_per_unit"`
	GrossQuantity    float64    `bigquery:"gross_quantity"`
	DiscountQuantity float64    `bigquery:"discount_quantity"`
	NetQuantity      float64    `bigquery:"net_quantity"`
	GrossAmount      float64    `bigquery:"gross_amount"`
	DiscountAmount   float64    `bigquery:"discount_amount"`
	NetAmount        float64    `bigquery:"net_amount"`
	RawRecord        string     `bigquery:"raw_record"`
	LoadedAt         time.Time  `bigquery:"loaded_at"`
}

func (c *BigQueryClient) EnsureBillingUsageDailyModelTableExists(ctx context.Context) error {
	dataset := c.client.Dataset(c.dataset)
	table := dataset.Table(billingUsageDailyModelTable)

	_, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("Billing daily model table already exists", "table", billingUsageDailyModelTable)
		return nil
	}

	slog.Info("Creating billing_usage_daily_model table", "dataset", c.dataset)

	schema := bigquery.Schema{
		{Name: "day", Type: bigquery.DateFieldType, Required: true, Description: "Calendar day of billing usage"},
		{Name: "scope_id", Type: bigquery.StringFieldType, Required: true, Description: "Enterprise slug"},
		{Name: "product", Type: bigquery.StringFieldType, Required: true, Description: "Product name"},
		{Name: "sku", Type: bigquery.StringFieldType, Required: true, Description: "SKU name"},
		{Name: "model", Type: bigquery.StringFieldType, Required: true, Description: "Model name"},
		{Name: "unit_type", Type: bigquery.StringFieldType, Required: true, Description: "Unit type"},
		{Name: "price_per_unit", Type: bigquery.FloatFieldType, Required: true, Description: "Price per unit in USD"},
		{Name: "gross_quantity", Type: bigquery.FloatFieldType, Required: true, Description: "Quantity before discounts"},
		{Name: "discount_quantity", Type: bigquery.FloatFieldType, Required: true, Description: "Discounted quantity"},
		{Name: "net_quantity", Type: bigquery.FloatFieldType, Required: true, Description: "Billed quantity"},
		{Name: "gross_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Gross amount in USD"},
		{Name: "discount_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Discount amount in USD"},
		{Name: "net_amount", Type: bigquery.FloatFieldType, Required: true, Description: "Net amount in USD"},
		{Name: "raw_record", Type: bigquery.JSONFieldType, Required: true, Description: "Full API row as JSON"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "Insert timestamp"},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.DayPartitioningType,
			Field: "day",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"scope_id", "model"},
		},
		Description: "Daily GitHub Copilot premium request usage by model",
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create billing_usage_daily_model table: %w", err)
	}

	slog.Info("billing_usage_daily_model table created successfully")
	return nil
}

func (c *BigQueryClient) DeleteBillingUsageDailyModelDay(ctx context.Context, day time.Time, scopeID string) error {
	query := c.client.Query(`
		DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + billingUsageDailyModelTable + "`" + `
		WHERE day = @day AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: civil.DateOf(day)},
		{Name: "scopeID", Value: scopeID},
	}

	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("run billing daily model delete: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("billing daily model delete job failed: %w", err)
	}
	if status.Err() != nil {
		return fmt.Errorf("billing daily model delete query failed: %w", status.Err())
	}
	return nil
}

func (c *BigQueryClient) InsertBillingUsageDailyModelDay(ctx context.Context, day time.Time, scopeID string, items []BillingUsageItem) error {
	if len(items) == 0 {
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(billingUsageDailyModelTable)
	inserter := table.Inserter()
	loadedAt := time.Now().UTC()
	dayCivil := civil.DateOf(day)

	rows := make([]*BillingUsageDailyModelRow, 0, len(items))
	for _, item := range items {
		raw, _ := json.Marshal(item)
		rows = append(rows, &BillingUsageDailyModelRow{
			Day:              dayCivil,
			ScopeID:          scopeID,
			Product:          item.Product,
			SKU:              item.SKU,
			Model:            item.Model,
			UnitType:         item.UnitType,
			PricePerUnit:     item.PricePerUnit,
			GrossQuantity:    item.GrossQuantity,
			DiscountQuantity: item.DiscountQuantity,
			NetQuantity:      item.NetQuantity,
			GrossAmount:      item.GrossAmount,
			DiscountAmount:   item.DiscountAmount,
			NetAmount:        item.NetAmount,
			RawRecord:        string(raw),
			LoadedAt:         loadedAt,
		})
	}

	if err := inserter.Put(ctx, rows); err != nil {
		return fmt.Errorf("insert billing daily model rows: %w", err)
	}
	return nil
}

func (c *BigQueryClient) GetLatestBillingUsageDailyModelDay(ctx context.Context, scopeID string) (time.Time, error) {
	query := c.client.Query(`
		SELECT MAX(day) as latest_day
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + billingUsageDailyModelTable + "`" + `
		WHERE scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "scopeID", Value: scopeID},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("query latest billing daily model day: %w", err)
	}

	var row struct {
		LatestDay bigquery.NullDate `bigquery:"latest_day"`
	}
	if err := it.Next(&row); err != nil {
		return time.Time{}, fmt.Errorf("read latest billing daily model day: %w", err)
	}
	if !row.LatestDay.Valid {
		return time.Time{}, nil
	}

	return time.Date(row.LatestDay.Date.Year, row.LatestDay.Date.Month, row.LatestDay.Date.Day, 0, 0, 0, 0, time.UTC), nil
}
