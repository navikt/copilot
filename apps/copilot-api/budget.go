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
	Budgets []BudgetEntry `json:"budgets"`
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

		// GitHub paginates with Link header; stop when we get fewer than a full page
		if len(result.Budgets) < 100 {
			break
		}
		page++
	}

	return all, nil
}

var errBudgetNotFound = errors.New("budget not found for user")

// GlobalBudget is the enterprise-level AI credit budget (multi_user_customer scope).
type GlobalBudget struct {
	BudgetAmount   float64  `json:"budgetAmount"`
	ConsumedAmount *float64 `json:"consumedAmount"`
}

// getGlobalBudget returns the enterprise-wide default AI credit budget.
func (c *BudgetClient) getGlobalBudget(ctx context.Context) (*GlobalBudget, error) {
	entries, err := c.getEnterpriseBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("get enterprise budgets: %w", err)
	}
	for _, e := range entries {
		if e.BudgetScope == "multi_user_customer" {
			return &GlobalBudget{
				BudgetAmount:   e.BudgetAmount,
				ConsumedAmount: e.ConsumedAmount,
			}, nil
		}
	}
	return nil, errBudgetNotFound
}

// getUserBudget resolves the budget for a given GitHub username.
// Returns the user-specific override if present, otherwise the enterprise default.
func (c *BudgetClient) getUserBudget(ctx context.Context, username string) (*UserBudget, error) {
	entries, err := c.getEnterpriseBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("get enterprise budgets: %w", err)
	}

	var defaultBudget float64
	var userEntry *BudgetEntry

	for i := range entries {
		e := &entries[i]
		if e.BudgetScope == "multi_user_customer" {
			defaultBudget = e.BudgetAmount
		}
		if e.BudgetScope == "user" && e.BudgetEntityName == username {
			userEntry = e
		}
	}

	if defaultBudget == 0 && userEntry == nil {
		return nil, errBudgetNotFound
	}

	if userEntry != nil {
		return &UserBudget{
			BudgetAmount:   userEntry.BudgetAmount,
			ConsumedAmount: userEntry.ConsumedAmount,
			IsOverride:     true,
			DefaultBudget:  defaultBudget,
		}, nil
	}

	return &UserBudget{
		BudgetAmount:  defaultBudget,
		IsOverride:    false,
		DefaultBudget: defaultBudget,
	}, nil
}
