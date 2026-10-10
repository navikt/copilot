package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BillingClient fetches premium request usage from the GitHub billing API.
// This endpoint requires a classic PAT with admin:enterprise scope —
// GitHub App tokens cannot access billing endpoints.
type BillingClient struct {
	httpClient   *http.Client
	enterprise   string
	token        string
	requests     int
	remaining    int
	budgeted     bool
	requestLimit int
}

var errBillingBudget = fmt.Errorf("billing request budget exhausted")

type billingHTTPError struct {
	Status  int
	RetryAt time.Time
}

func (e *billingHTTPError) Error() string { return fmt.Sprintf("billing API status %d", e.Status) }

func (c *BillingClient) request(ctx context.Context, endpoint string, result any) error {
	for attempt := 0; attempt < 3; attempt++ {
		limit := c.requestLimit
		if limit == 0 {
			limit = 2000
		}
		if c.budgeted && (c.requests >= limit || c.remaining <= 500) {
			return errBillingBudget
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
		c.requests++
		resp, err := c.httpClient.Do(req)
		if err != nil {
			err = safeTransportError(err)
		}
		if err == nil {
			if n, parseErr := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining")); parseErr == nil {
				c.remaining = n
			}
			if resp.StatusCode == http.StatusOK {
				err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(result)
				_ = resp.Body.Close()
				return err
			}
			status := &billingHTTPError{Status: resp.StatusCode}
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil {
				status.RetryAt = time.Now().Add(time.Duration(seconds) * time.Second)
			} else if retryAt, parseErr := http.ParseTime(resp.Header.Get("Retry-After")); parseErr == nil {
				status.RetryAt = retryAt
			} else if c.remaining == 0 {
				if reset, parseErr := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); parseErr == nil {
					status.RetryAt = time.Unix(reset, 0)
				}
			}
			_ = resp.Body.Close()
			if resp.StatusCode < 500 || resp.StatusCode > 599 {
				return status
			}
			err = status
		}
		if attempt == 2 {
			return err
		}
		delay := time.Duration(1<<attempt) * time.Second
		var status *billingHTTPError
		if errors.As(err, &status) && time.Until(status.RetryAt) > delay {
			delay = time.Until(status.RetryAt)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil
}

// AccountDeleted reports whether no GitHub account has this numeric id.
func (c *BillingClient) AccountDeleted(ctx context.Context, id string) (bool, error) {
	var user struct {
		ID int64 `json:"id"`
	}
	err := c.request(ctx, "https://api.github.com/user/"+url.PathEscape(id), &user)
	var status *billingHTTPError
	if errors.As(err, &status) && status.Status == http.StatusNotFound {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if strconv.FormatInt(user.ID, 10) != id {
		return false, fmt.Errorf("account lookup returned another id")
	}
	return false, nil
}

func (c *BillingClient) FetchUserID(ctx context.Context, login string) (string, error) {
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := c.request(ctx, "https://api.github.com/users/"+url.PathEscape(login), &user); err != nil {
		return "", err
	}
	if user.ID <= 0 || !strings.EqualFold(user.Login, login) {
		return "", fmt.Errorf("GitHub identity response does not match requested login")
	}
	return strconv.FormatInt(user.ID, 10), nil
}

func (c *BillingClient) FetchEnterpriseAICreditUsage(ctx context.Context, month time.Time) (*BillingUsageResponse, error) {
	var result BillingUsageResponse
	endpoint := fmt.Sprintf("https://api.github.com/enterprises/%s/settings/billing/ai_credit/usage?year=%d&month=%d", c.enterprise, month.Year(), month.Month())
	if err := c.request(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	if err := validateUserBilling(&result, "", c.enterprise, month); err != nil {
		return nil, err
	}
	return &result, nil
}

// BillingUsageResponse is the response from the premium request usage endpoint.
type BillingUsageResponse struct {
	TimePeriod struct {
		Year  int `json:"year"`
		Month int `json:"month,omitempty"`
		Day   int `json:"day,omitempty"`
	} `json:"timePeriod"`
	Enterprise string             `json:"enterprise"`
	User       string             `json:"user,omitempty"`
	UsageItems []BillingUsageItem `json:"usageItems"`
}

// OrganizationBillingUsageResponse is the response from the organization billing usage report endpoint.
type OrganizationBillingUsageResponse struct {
	UsageItems []OrganizationBillingUsageItem `json:"usageItems"`
}

// OrganizationBillingUsageItem represents a single organization billing usage line item.
type OrganizationBillingUsageItem struct {
	Date             string  `json:"date"`
	Product          string  `json:"product"`
	SKU              string  `json:"sku"`
	Quantity         float64 `json:"quantity"`
	UnitType         string  `json:"unitType"`
	PricePerUnit     float64 `json:"pricePerUnit"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetAmount        float64 `json:"netAmount"`
	OrganizationName string  `json:"organizationName"`
	RepositoryName   string  `json:"repositoryName,omitempty"`
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

// FetchUserAICreditUsage returns the billed gross and net by SKU for one user and month.
func (c *BillingClient) FetchUserAICreditUsage(ctx context.Context, login string, month time.Time) (*BillingUsageResponse, error) {
	endpoint := fmt.Sprintf("https://api.github.com/enterprises/%s/settings/billing/ai_credit/usage?year=%d&month=%d&user=%s",
		c.enterprise, month.Year(), month.Month(), url.QueryEscape(login))
	var result BillingUsageResponse
	if err := c.request(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	if err := validateUserBilling(&result, login, c.enterprise, month); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateUserBilling(response *BillingUsageResponse, login, enterprise string, month time.Time) error {
	if response == nil || !strings.EqualFold(response.User, login) ||
		!strings.EqualFold(response.Enterprise, enterprise) || response.TimePeriod.Year != month.Year() ||
		response.TimePeriod.Month != int(month.Month()) || response.TimePeriod.Day != 0 || response.UsageItems == nil {
		return fmt.Errorf("user billing response does not match requested account and month")
	}
	return nil
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

// FetchEnterpriseUsage fetches billing usage report data for an enterprise for a specific day.
func (c *BillingClient) FetchEnterpriseUsage(ctx context.Context, day time.Time) (*OrganizationBillingUsageResponse, error) {
	url := fmt.Sprintf(
		"https://api.github.com/enterprises/%s/settings/billing/usage?year=%d&month=%d&day=%d",
		c.enterprise, day.Year(), int(day.Month()), day.Day(),
	)

	slog.Debug("Fetching enterprise billing usage report", "enterprise", c.enterprise, "day", day.Format("2006-01-02"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create billing usage request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billing usage request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read billing usage response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("billing usage API returned status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var result OrganizationBillingUsageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode billing usage response: %w", err)
	}

	return &result, nil
}
