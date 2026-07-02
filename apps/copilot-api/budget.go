package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const budgetCacheTTL = 30 * time.Minute

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

// effectiveBudget is returned by the budgets API when filtered by ?user={username}.
type effectiveBudget struct {
	ID             string  `json:"id"`
	BudgetAmount   float64 `json:"budget_amount"`
	ConsumedAmount float64 `json:"consumed_amount"`
}

type userBudgetResponse struct {
	Budgets         []BudgetEntry    `json:"budgets"`
	EffectiveBudget *effectiveBudget `json:"effective_budget"`
}

// UserBudget is the resolved budget for a specific user.
type UserBudget struct {
	BudgetAmount   float64  `json:"budgetAmount"`
	ConsumedAmount *float64 `json:"consumedAmount"`
	IsOverride     bool     `json:"isOverride"`
	DefaultBudget  float64  `json:"defaultBudget"`
}

// BudgetClient fetches enterprise AI credit budgets using a classic PAT.
// The GitHub App token used elsewhere does NOT work for enterprise billing endpoints.
type BudgetClient struct {
	httpClient   *http.Client
	billingToken string
	enterprise   string

	mu         sync.RWMutex
	cachedAt   time.Time
	cachedData []BudgetEntry
}

func newBudgetClient(billingToken, enterprise string) *BudgetClient {
	return &BudgetClient{
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		billingToken: billingToken,
		enterprise:   enterprise,
	}
}

// getEnterpriseBudgets returns all budget entries, using a 30-minute in-memory cache.
func (c *BudgetClient) getEnterpriseBudgets(ctx context.Context) ([]BudgetEntry, error) {
	c.mu.RLock()
	if !c.cachedAt.IsZero() && time.Since(c.cachedAt) < budgetCacheTTL {
		data := c.cachedData
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	entries, err := c.fetchAllBudgetPages(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cachedData = entries
	c.cachedAt = time.Now()
	c.mu.Unlock()

	slog.Debug("Enterprise budgets cached", "count", len(entries))
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		for _, e := range entries {
			slog.Debug("Budget entry", "scope", e.BudgetScope, "entity", e.BudgetEntityName, "amount", e.BudgetAmount)
		}
	}
	return entries, nil
}

// fetchAllBudgetPages paginates through the enterprise billing budgets endpoint.
func (c *BudgetClient) fetchAllBudgetPages(ctx context.Context) ([]BudgetEntry, error) {
	var all []BudgetEntry
	page := 1

	for {
		url := fmt.Sprintf(
			"https://api.github.com/enterprises/%s/settings/billing/budgets?per_page=100&page=%d",
			c.enterprise, page,
		)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create budget request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.billingToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch budgets page %d: %w", page, err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("budgets API returned %d on page %d", resp.StatusCode, page)
		}

		var result budgetListResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode budgets page %d: %w", page, decodeErr)
		}

		all = append(all, result.Budgets...)

		if !result.HasNextPage {
			break
		}
		page++
	}

	return all, nil
}

var errBudgetNotFound = errors.New("budget not found for user")

// globalBudgetGetter abstracts the enterprise-wide credit budget lookup.
// Used by usage-distribution histograms to scale credit buckets to the
// actual per-user $ budget rather than a hardcoded ceiling.
type globalBudgetGetter interface {
	getGlobalBudget(ctx context.Context) (*GlobalBudget, error)
}

// usdPerAICredit is GitHub Copilot's billing conversion rate: 1 AI credit = $0.01 USD.
// Source: apps/my-copilot/src/lib/model-pricing.ts (auto-generated from GitHub docs).
const usdPerAICredit = 0.01

// defaultPerUserBudgetCredits is used as a fallback when the budget client is
// unavailable or errors, so the usage-distribution endpoint degrades gracefully
// instead of failing outright. As of 2026-07 this matches the $400/user/month
// enterprise default (400 / 0.01 = 40,000 credits).
const defaultPerUserBudgetCredits = 40000

// GlobalBudget aggregates AI credit consumption across all users this month.
type GlobalBudget struct {
	TotalConsumed float64 `json:"totalConsumed"`
	PerUserBudget float64 `json:"perUserBudget"`
	ActiveUsers   int     `json:"activeUsers"`
}

// getGlobalBudget aggregates AI credit consumption across all budget entries.
// Returns total consumed, per-user default budget, and count of active users.
func (c *BudgetClient) getGlobalBudget(ctx context.Context) (*GlobalBudget, error) {
	entries, err := c.getEnterpriseBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("get enterprise budgets: %w", err)
	}

	var perUserBudget float64
	var totalConsumed float64
	activeUsers := 0

	for _, e := range entries {
		if e.BudgetScope == "multi_user_customer" {
			perUserBudget = e.BudgetAmount
		}
		if e.BudgetScope == "user" && e.ConsumedAmount != nil {
			totalConsumed += *e.ConsumedAmount
			if *e.ConsumedAmount > 0 {
				activeUsers++
			}
		}
	}

	if perUserBudget == 0 && totalConsumed == 0 {
		return nil, errBudgetNotFound
	}

	return &GlobalBudget{
		TotalConsumed: totalConsumed,
		PerUserBudget: perUserBudget,
		ActiveUsers:   activeUsers,
	}, nil
}

// getUserBudget resolves the effective budget and consumption for a given GitHub username.
// Uses the ?user={username} query parameter which returns per-user consumed_amount for all
// users — both override users and those on the default multi_user_customer budget.
func (c *BudgetClient) getUserBudget(ctx context.Context, username string) (*UserBudget, error) {
	url := fmt.Sprintf(
		"https://api.github.com/enterprises/%s/settings/billing/budgets?user=%s",
		c.enterprise, username,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create user budget request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.billingToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch user budget: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errBudgetNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user budget API returned %d", resp.StatusCode)
	}

	var result userBudgetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode user budget: %w", err)
	}

	if result.EffectiveBudget == nil {
		return nil, errBudgetNotFound
	}

	// Determine if this is an override budget by checking if the returned scope is "user"
	isOverride := false
	for _, b := range result.Budgets {
		if b.BudgetScope == "user" {
			isOverride = true
			break
		}
	}

	// defaultBudget is the per-user standard amount from the enterprise default
	defaultBudget := result.EffectiveBudget.BudgetAmount
	if isOverride {
		// For override users, fetch the enterprise default from the cached list
		entries, err := c.getEnterpriseBudgets(ctx)
		if err == nil {
			for _, e := range entries {
				if e.BudgetScope == "multi_user_customer" {
					defaultBudget = e.BudgetAmount
					break
				}
			}
		}
	}

	consumed := result.EffectiveBudget.ConsumedAmount
	return &UserBudget{
		BudgetAmount:   result.EffectiveBudget.BudgetAmount,
		ConsumedAmount: &consumed,
		IsOverride:     isOverride,
		DefaultBudget:  defaultBudget,
	}, nil
}
