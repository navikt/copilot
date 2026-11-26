package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

const (
	budgetCacheTTL = 30 * time.Minute
	maxPages       = 50
)

// BudgetEntry represents a single entry from the enterprise billing budgets API.
type BudgetEntry struct {
	BudgetScope      string   `json:"budget_scope"`
	BudgetProductSKU string   `json:"budget_product_sku"`
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

// BudgetClient reads enterprise AI credit budgets with an installation token
// from the enterprise budget app (BUDGET_GITHUB_APP_*, Enterprise billing).
// copilot-api's own GITHUB_APP_* is an org app and cannot read enterprise billing.
type BudgetClient struct {
	httpClient *http.Client
	token      func(context.Context) (string, error)
	enterprise string

	mu         sync.RWMutex
	cachedAt   time.Time
	cachedData []BudgetEntry
}

func NewBudgetClient(token func(context.Context) (string, error), enterprise string) *BudgetClient {
	return &BudgetClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		token:      token,
		enterprise: enterprise,
	}
}

// staticToken wraps a fixed token as a token source (PAT fallback, tests).
func staticToken(tok string) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return tok, nil }
}

// NewBillingTokenSource returns the token source for enterprise billing reads:
// the budget app, or the classic PAT only when BILLING_PAT_FALLBACK=true. The
// PAT is never a silent fallback; nil means budget endpoints are unavailable.
func NewBillingTokenSource(cfg *config.Config) (func(context.Context) (string, error), error) {
	if cfg.BillingPATFallback && cfg.GitHubBillingToken != "" {
		slog.Warn("Billing reads use GITHUB_BILLING_TOKEN because BILLING_PAT_FALLBACK=true; remove the flag once the budget GitHub App works")
		return staticToken(cfg.GitHubBillingToken), nil
	}
	return newBudgetTokenSource(cfg)
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

	for page := 1; page <= maxPages; page++ {
		reqURL := fmt.Sprintf(
			"https://api.github.com/enterprises/%s/settings/billing/budgets?per_page=100&page=%d",
			url.PathEscape(c.enterprise), page,
		)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create budget request: %w", err)
		}
		tok, err := c.token(ctx)
		if err != nil {
			return nil, fmt.Errorf("budget token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
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
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode budgets page %d: %w", page, decodeErr)
		}

		all = append(all, result.Budgets...)

		if !result.HasNextPage {
			return all, nil
		}
	}

	return nil, fmt.Errorf("budget pagination exceeded %d pages", maxPages)
}

var ErrBudgetNotFound = errors.New("budget not found for user")

// GlobalBudgetGetter abstracts the enterprise-wide credit budget lookup.
// Used by usage-distribution histograms to scale credit buckets to the
// actual per-user $ budget rather than a hardcoded ceiling.
type GlobalBudgetGetter interface {
	GetGlobalBudget(ctx context.Context) (*GlobalBudget, error)
}

// GlobalBudget aggregates AI credit consumption across all users this month.
type GlobalBudget struct {
	TotalConsumed float64 `json:"totalConsumed"`
	PerUserBudget float64 `json:"perUserBudget"`
	ActiveUsers   int     `json:"activeUsers"`
	// EnterpriseBudget is the monthly ai_credits budget for the whole
	// enterprise in USD, 0 when none is set. Internal: kept out of the public
	// budget/global response and served only to admins.
	EnterpriseBudget float64 `json:"-"`
}

// GetGlobalBudget aggregates AI credit consumption across all budget entries.
// Returns total consumed, per-user default budget, and count of active users.
func (c *BudgetClient) GetGlobalBudget(ctx context.Context) (*GlobalBudget, error) {
	entries, err := c.getEnterpriseBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("get enterprise budgets: %w", err)
	}

	var perUserBudget, totalConsumed, enterpriseBudget float64
	activeUsers := 0

	for _, e := range entries {
		if e.BudgetScope == "enterprise" && e.BudgetProductSKU == "ai_credits" {
			enterpriseBudget = e.BudgetAmount
		}
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

	if perUserBudget == 0 && totalConsumed == 0 && enterpriseBudget == 0 {
		return nil, ErrBudgetNotFound
	}

	return &GlobalBudget{
		TotalConsumed:    totalConsumed,
		PerUserBudget:    perUserBudget,
		ActiveUsers:      activeUsers,
		EnterpriseBudget: enterpriseBudget,
	}, nil
}

// getUserBudget resolves the effective budget and consumption for a given GitHub username.
// Uses the ?user={username} query parameter which returns per-user consumed_amount for all
// users — both override users and those on the default multi_user_customer budget.
func (c *BudgetClient) getUserBudget(ctx context.Context, username string) (*UserBudget, error) {
	reqURL := fmt.Sprintf(
		"https://api.github.com/enterprises/%s/settings/billing/budgets?user=%s",
		url.PathEscape(c.enterprise), url.QueryEscape(username),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create user budget request: %w", err)
	}
	tok, err := c.token(ctx)
	if err != nil {
		return nil, fmt.Errorf("budget token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch user budget: %T", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrBudgetNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user budget API returned %d", resp.StatusCode)
	}

	var result userBudgetResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode user budget: %w", err)
	}

	if result.EffectiveBudget == nil {
		return nil, ErrBudgetNotFound
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
