package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
// Returns nil, nil if the user has no budget data. Errors never name the login.
func (c *BudgetClient) FetchUserEffectiveBudget(ctx context.Context, login string) (*UserBudgetData, error) {
	endpoint := fmt.Sprintf(
		"https://api.github.com/enterprises/%s/settings/billing/budgets?user=%s",
		c.enterprise, url.QueryEscape(login),
	)

	var body []byte
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create user budget request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch user budget: %w", safeTransportError(err))
		}
		body, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read user budget response: %w", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		delay, retry := budgetRetryDelay(resp, body, attempt)
		if !retry || attempt >= budgetMaxRetries {
			return nil, fmt.Errorf("user budget API returned %d", resp.StatusCode)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	var result userBudgetAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode user budget: %w", err)
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

const budgetMaxRetries = 4

// budgetBaseDelay is the first backoff step; tests shrink it.
var budgetBaseDelay = 2 * time.Second

// budgetRetryDelay decides whether a failed response is a rate limit or a
// transient server error worth retrying, and how long to wait. Retry-After and
// X-RateLimit-Reset win over exponential backoff, capped at one minute.
func budgetRetryDelay(resp *http.Response, body []byte, attempt int) (time.Duration, bool) {
	rateLimited := resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && (resp.Header.Get("Retry-After") != "" ||
			resp.Header.Get("X-RateLimit-Remaining") == "0" ||
			strings.Contains(strings.ToLower(string(body)), "rate limit")))
	if !rateLimited && resp.StatusCode < 500 {
		return 0, false
	}
	delay := budgetBaseDelay << attempt
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
		delay = time.Duration(s) * time.Second
	} else if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			delay = time.Until(time.Unix(reset, 0))
		}
	}
	return min(max(delay, budgetBaseDelay), time.Minute), true
}

// FetchAllUserBudgets fetches effective budget and consumption for every login in
// the list. It keeps every user that succeeded; failed is the number of users
// whose budget could not be fetched after retries. Logs carry counts, never logins.
func (c *BudgetClient) FetchAllUserBudgets(ctx context.Context, logins []string) (all []UserBudgetData, failed int, err error) {
	// Low concurrency keeps ~700 users clear of GitHub's secondary rate limits.
	const concurrency = 4

	type result struct {
		data *UserBudgetData
		err  error
	}

	results := make(chan result, len(logins))
	sem := make(chan struct{}, concurrency)
	started := 0

	for _, login := range logins {
		select {
		case <-ctx.Done():
		case sem <- struct{}{}:
			started++
			go func() {
				defer func() { <-sem }()
				data, err := c.FetchUserEffectiveBudget(ctx, login)
				results <- result{data: data, err: err}
			}()
			continue
		}
		break
	}

	var lastErr error
	for range started {
		r := <-results
		if r.err != nil {
			failed++
			lastErr = r.err
		} else if r.data != nil {
			all = append(all, *r.data)
		}
	}
	failed += len(logins) - started

	slog.Info("Fetched user budgets", "users_with_data", len(all), "failed", failed, "total_users", len(logins))
	if failed > 0 {
		if lastErr == nil {
			lastErr = ctx.Err()
		}
		slog.Warn("User budget census incomplete", "failed", failed, "last_error", lastErr)
	}
	if ctx.Err() != nil {
		return all, failed, ctx.Err()
	}
	return all, failed, nil
}

// truncate from billing.go is available package-wide — no need to redeclare.
