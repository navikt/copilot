package types

import (
	"time"

	"cloud.google.com/go/civil"
)

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
