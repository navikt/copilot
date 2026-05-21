package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// BillingClient fetches premium request usage from the GitHub billing API.
// This endpoint requires a classic PAT with admin:enterprise scope —
// GitHub App tokens cannot access billing endpoints.
type BillingClient struct {
	httpClient *http.Client
	enterprise string
	token      string
}

// BillingUsageResponse is the response from the premium request usage endpoint.
type BillingUsageResponse struct {
	TimePeriod struct {
		Year  int `json:"year"`
		Month int `json:"month,omitempty"`
		Day   int `json:"day,omitempty"`
	} `json:"timePeriod"`
	Enterprise string             `json:"enterprise"`
	UsageItems []BillingUsageItem `json:"usageItems"`
}

// BillingUsageItem represents a single line item from the billing API.
type BillingUsageItem struct {
	Product          string  `json:"product"`
	SKU              string  `json:"sku"`
	Model            string  `json:"model"`
	UnitType         string  `json:"unitType"`
	PricePerUnit     float64 `json:"pricePerUnit"`
	GrossQuantity    float64 `json:"grossQuantity"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountQuantity float64 `json:"discountQuantity"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetQuantity      float64 `json:"netQuantity"`
	NetAmount        float64 `json:"netAmount"`
}

func NewBillingClient(token, enterprise string) *BillingClient {
	if token == "" {
		return nil
	}
	return &BillingClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		enterprise: enterprise,
		token:      token,
	}
}

// FetchMonthlyUsage fetches the premium request billing data for a given month.
func (c *BillingClient) FetchMonthlyUsage(ctx context.Context, year, month int) (*BillingUsageResponse, error) {
	url := fmt.Sprintf(
		"https://api.github.com/enterprises/%s/settings/billing/premium_request/usage?year=%d&month=%d",
		c.enterprise, year, month,
	)

	slog.Info("Fetching billing premium request usage", "year", year, "month", month)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create billing request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billing request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read billing response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("billing API returned status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var result BillingUsageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode billing response: %w", err)
	}

	slog.Info("Fetched billing usage",
		"year", year,
		"month", month,
		"items", len(result.UsageItems),
	)

	return &result, nil
}

// FetchDailyUsage fetches the premium request billing data for a specific day.
func (c *BillingClient) FetchDailyUsage(ctx context.Context, day time.Time) (*BillingUsageResponse, error) {
	url := fmt.Sprintf(
		"https://api.github.com/enterprises/%s/settings/billing/premium_request/usage?year=%d&month=%d&day=%d",
		c.enterprise, day.Year(), int(day.Month()), day.Day(),
	)

	slog.Debug("Fetching daily billing usage", "day", day.Format("2006-01-02"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create billing request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billing request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read billing response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("billing API returned status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var result BillingUsageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode billing response: %w", err)
	}

	return &result, nil
}
