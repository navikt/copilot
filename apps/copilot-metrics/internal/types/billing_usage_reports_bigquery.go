package types

import (
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

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
