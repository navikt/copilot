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

// BudgetClient fetches enterprise AI credit budgets from the GitHub billing API.
// Requires a classic PAT with admin:enterprise scope — same token as BillingClient.
type BudgetClient struct {
	httpClient *http.Client
	enterprise string
	token      string
}

// BudgetEntry represents a single entry from the enterprise billing budgets API.
type BudgetEntry struct {
	BudgetScope      string   `json:"budget_scope"`
	BudgetEntityName string   `json:"budget_entity_name"`
	BudgetAmount     float64  `json:"budget_amount"`
	ConsumedAmount   *float64 `json:"consumed_amount"`
}

type budgetListResponse struct {
	Budgets     []BudgetEntry `json:"budgets"`
	HasNextPage bool          `json:"has_next_page"`
}

func NewBudgetClient(token, enterprise string) *BudgetClient {
	if token == "" {
		return nil
	}
	return &BudgetClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		enterprise: enterprise,
		token:      token,
	}
}

// FetchAllBudgets paginates through the enterprise billing budgets endpoint
// and returns all entries.
func (c *BudgetClient) FetchAllBudgets(ctx context.Context) ([]BudgetEntry, error) {
	var all []BudgetEntry
	page := 1

	for {
		url := fmt.Sprintf(
			"https://api.github.com/enterprises/%s/settings/billing/budgets?per_page=100&page=%d",
			c.enterprise, page,
		)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create budget request page %d: %w", page, err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch budgets page %d: %w", page, err)
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read budgets page %d: %w", page, readErr)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("budgets API returned %d on page %d: %s", resp.StatusCode, page, truncate(string(body), 500))
		}

		var result budgetListResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("decode budgets page %d: %w", page, err)
		}

		all = append(all, result.Budgets...)
		slog.Debug("Fetched budget page", "page", page, "entries", len(result.Budgets), "has_next", result.HasNextPage)

		if !result.HasNextPage {
			break
		}
		page++
	}

	slog.Info("Fetched all budget entries", "total", len(all), "pages", page)
	return all, nil
}
