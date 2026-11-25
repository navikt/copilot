package fetchers

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

	"github.com/navikt/copilot/apps/copilot-metrics/internal/config"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// BillingClient fetches premium request usage from the GitHub billing API.
// It authenticates as the enterprise GitHub App (Enterprise billing: read);
// see billingAuth for the transition-only PAT path.
type BillingClient struct {
	HTTPClient   *http.Client
	enterprise   string
	token        string
	Requests     int
	Remaining    int
	Budgeted     bool
	RequestLimit int
}

var ErrBillingBudget = fmt.Errorf("billing request budget exhausted")

type BillingHTTPError struct {
	Status  int
	RetryAt time.Time
}

func (e *BillingHTTPError) Error() string { return fmt.Sprintf("billing API status %d", e.Status) }

func (c *BillingClient) request(ctx context.Context, endpoint string, result any) error {
	for attempt := 0; attempt < 3; attempt++ {
		limit := c.RequestLimit
		if limit == 0 {
			limit = 2000
		}
		if c.Budgeted && (c.Requests >= limit || c.Remaining <= 500) {
			return ErrBillingBudget
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
		c.Requests++
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			err = SafeTransportError(err)
		}
		if err == nil {
			if n, parseErr := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining")); parseErr == nil {
				c.Remaining = n
			}
			if resp.StatusCode == http.StatusOK {
				err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(result)
				_ = resp.Body.Close()
				return err
			}
			status := &BillingHTTPError{Status: resp.StatusCode}
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil {
				status.RetryAt = time.Now().Add(time.Duration(seconds) * time.Second)
			} else if retryAt, parseErr := http.ParseTime(resp.Header.Get("Retry-After")); parseErr == nil {
				status.RetryAt = retryAt
			} else if c.Remaining == 0 {
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
		var status *BillingHTTPError
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

// AccountLogin returns the current login for a numeric account id, or "" when
// no account has that id.
func (c *BillingClient) AccountLogin(ctx context.Context, id string) (string, error) {
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	err := c.request(ctx, "https://api.github.com/user/"+url.PathEscape(id), &user)
	var status *BillingHTTPError
	if errors.As(err, &status) && status.Status == http.StatusNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if strconv.FormatInt(user.ID, 10) != id || user.Login == "" {
		return "", fmt.Errorf("account lookup returned another account")
	}
	return user.Login, nil
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

// FetchEnterpriseAICreditUsage returns the enterprise total by SKU for one
// month, from the same product endpoint as the per-user rows it is compared with.
func (c *BillingClient) FetchEnterpriseAICreditUsage(ctx context.Context, month time.Time) (*types.BillingUsageResponse, error) {
	var result types.BillingUsageResponse
	endpoint := fmt.Sprintf("https://api.github.com/enterprises/%s/settings/billing/%s/usage?year=%d&month=%d", c.enterprise, billingProduct(month), month.Year(), month.Month())
	if err := c.request(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	if err := ValidateUserBilling(&result, "", c.enterprise, month); err != nil {
		return nil, err
	}
	return &result, nil
}

// NewBillingClient returns nil when httpClient is nil. token is empty for the
// GitHub App, whose transport sets Authorization itself.
func NewBillingClient(httpClient *http.Client, token, enterprise string) *BillingClient {
	if httpClient == nil {
		return nil
	}
	return &BillingClient{
		HTTPClient: httpClient,
		enterprise: enterprise,
		token:      token,
	}
}

// aiCreditStart is the first month GitHub billed Copilot as AI credits.
// Earlier months were billed as premium requests on a separate endpoint.
var aiCreditStart = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

// billingProduct is the usage endpoint product for month.
func billingProduct(month time.Time) string {
	if month.Before(aiCreditStart) {
		return "premium_request"
	}
	return "ai_credit"
}

// FetchUserBillingUsage returns the billed gross and net by SKU for one user and month.
func (c *BillingClient) FetchUserBillingUsage(ctx context.Context, login string, month time.Time) (*types.BillingUsageResponse, error) {
	endpoint := fmt.Sprintf("https://api.github.com/enterprises/%s/settings/billing/%s/usage?year=%d&month=%d&user=%s",
		c.enterprise, billingProduct(month), month.Year(), month.Month(), url.QueryEscape(login))
	var result types.BillingUsageResponse
	if err := c.request(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	if err := ValidateUserBilling(&result, login, c.enterprise, month); err != nil {
		return nil, err
	}
	return &result, nil
}

func ValidateUserBilling(response *types.BillingUsageResponse, login, enterprise string, month time.Time) error {
	if response == nil || !strings.EqualFold(response.User, login) ||
		!strings.EqualFold(response.Enterprise, enterprise) || response.TimePeriod.Year != month.Year() ||
		response.TimePeriod.Month != int(month.Month()) || response.TimePeriod.Day != 0 || response.UsageItems == nil {
		return fmt.Errorf("user billing response does not match requested account and month")
	}
	return nil
}

// FetchMonthlyUsage fetches the premium request billing data for a given month.
func (c *BillingClient) FetchMonthlyUsage(ctx context.Context, year, month int) (*types.BillingUsageResponse, error) {
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := c.HTTPClient.Do(req)
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

	var result types.BillingUsageResponse
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
func (c *BillingClient) FetchDailyUsage(ctx context.Context, day time.Time) (*types.BillingUsageResponse, error) {
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := c.HTTPClient.Do(req)
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

	var result types.BillingUsageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode billing response: %w", err)
	}

	return &result, nil
}

// FetchEnterpriseUsage fetches billing usage report data for an enterprise for a specific day.
func (c *BillingClient) FetchEnterpriseUsage(ctx context.Context, day time.Time) (*types.OrganizationBillingUsageResponse, error) {
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := c.HTTPClient.Do(req)
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

	var result types.OrganizationBillingUsageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode billing usage response: %w", err)
	}

	return &result, nil
}

// billingAuth picks the billing credentials: the enterprise app client, or the
// classic PAT only when BILLING_PAT_FALLBACK=true. The PAT is never a silent
// fallback; with the flag off and no app, billing is unavailable (nil client).
func BillingAuth(cfg *config.Config, app *http.Client) (*http.Client, string) {
	if cfg.BillingPATFallback && cfg.GitHubBillingToken != "" {
		slog.Warn("Billing uses GITHUB_BILLING_TOKEN because BILLING_PAT_FALLBACK=true; remove the flag once the GitHub App works")
		return &http.Client{Timeout: 30 * time.Second}, cfg.GitHubBillingToken
	}
	return app, ""
}

// billingConfigured reports whether billing has credentials: the app, or the
// PAT only when BILLING_PAT_FALLBACK=true.
func BillingConfigured(cfg *config.Config) bool {
	app := cfg.GitHubAppID != 0 && cfg.GitHubAppInstallationID != 0 && cfg.GitHubAppPrivateKey != ""
	return app || (cfg.BillingPATFallback && cfg.GitHubBillingToken != "")
}

// billingClientFor builds a BillingClient for the one-shot billing commands.
func BillingClientFor(cfg *config.Config) *BillingClient {
	var app *http.Client
	if cfg.GitHubAppID != 0 && cfg.GitHubAppInstallationID != 0 && cfg.GitHubAppPrivateKey != "" {
		if gh, err := NewGitHubClient(cfg); err == nil {
			app = gh.HTTPClient
		} else {
			slog.Error("Failed to create GitHub App client for billing")
		}
	}
	client, token := BillingAuth(cfg, app)
	return NewBillingClient(client, token, cfg.EnterpriseSlug)
}
