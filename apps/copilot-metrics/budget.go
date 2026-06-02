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

// UserBudgetData is the resolved effective budget and consumption for a single user.
type UserBudgetData struct {
	GitHubLogin    string
	BudgetAmount   float64
	ConsumedAmount float64
	IsOverride     bool
}

type userBudgetAPIResponse struct {
	Budgets []struct {
		BudgetScope string `json:"budget_scope"`
	} `json:"budgets"`
	EffectiveBudget *struct {
		BudgetAmount   float64 `json:"budget_amount"`
		ConsumedAmount float64 `json:"consumed_amount"`
	} `json:"effective_budget"`
}

// FetchUserEffectiveBudget returns the effective budget and current-month consumption
// for a single GitHub user. Uses the ?user= query param which returns consumed_amount
// for ALL users — both those with override budgets and those on the enterprise default.
// Returns nil, nil if the user has no budget data.
func (c *BudgetClient) FetchUserEffectiveBudget(ctx context.Context, login string) (*UserBudgetData, error) {
	url := fmt.Sprintf(
		"https://api.github.com/enterprises/%s/settings/billing/budgets?user=%s",
		c.enterprise, login,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", login, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch budget for %s: %w", login, err)
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read budget response for %s: %w", login, readErr)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("budget API returned %d for %s: %s", resp.StatusCode, login, truncate(string(body), 200))
	}

	var result userBudgetAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode budget for %s: %w", login, err)
	}

	if result.EffectiveBudget == nil {
		return nil, nil
	}

	isOverride := false
	for _, b := range result.Budgets {
		if b.BudgetScope == "user" {
			isOverride = true
			break
		}
	}

	return &UserBudgetData{
		GitHubLogin:    login,
		BudgetAmount:   result.EffectiveBudget.BudgetAmount,
		ConsumedAmount: result.EffectiveBudget.ConsumedAmount,
		IsOverride:     isOverride,
	}, nil
}

// FetchAllUserBudgets fetches effective budget and consumption for every login in the list.
// Uses 10 concurrent goroutines to stay within GitHub API rate limits.
// Errors for individual users are logged as warnings and skipped — partial data is acceptable.
func (c *BudgetClient) FetchAllUserBudgets(ctx context.Context, logins []string) ([]UserBudgetData, error) {
	const concurrency = 10

	type result struct {
		data *UserBudgetData
		err  error
	}

	results := make(chan result, len(logins))
	sem := make(chan struct{}, concurrency)

	for _, login := range logins {
		login := login
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sem <- struct{}{}:
		}

		go func() {
			defer func() { <-sem }()
			data, err := c.FetchUserEffectiveBudget(ctx, login)
			results <- result{data: data, err: err}
		}()
	}

	var all []UserBudgetData
	var errCount int
	for range logins {
		r := <-results
		if r.err != nil {
			slog.Warn("Failed to fetch user budget (skipping)", "error", r.err)
			errCount++
		} else if r.data != nil {
			all = append(all, *r.data)
		}
	}

	if errCount > 0 {
		slog.Warn("Some user budget fetches failed", "errors", errCount, "successful", len(all))
	}
	slog.Info("Fetched all user budgets", "users_with_data", len(all), "total_users", len(logins))
	return all, nil
}

// truncate from billing.go is available package-wide — no need to redeclare.
