package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

// ErrReportNotAvailable indicates the metrics report has not been generated yet.
// This is a transient condition — the report typically becomes available later in the day.
var ErrReportNotAvailable = errors.New("report not available")

type GitHubClient struct {
	httpClient     *http.Client // GitHub API client with enterprise installation auth
	orgHttpClient  *http.Client // GitHub API client with org installation auth (nil if not configured)
	downloadClient *http.Client // Plain client for pre-signed URLs
	enterprise     string
	org            string
}

type MetricsReportResponse struct {
	DownloadLinks []string `json:"download_links"`
	ReportDay     string   `json:"report_day"`
}

// FetchResult contains the metrics records along with metadata about the fetch.
type FetchResult struct {
	Records []json.RawMessage
	Scope   string // "enterprise" or "organization"
	ScopeID string // enterprise slug or org name
}

func NewGitHubClient(cfg *Config) (*GitHubClient, error) {
	transport, err := ghinstallation.New(
		http.DefaultTransport,
		cfg.GitHubAppID,
		cfg.GitHubAppInstallationID,
		[]byte(cfg.GitHubAppPrivateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	client := &GitHubClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		downloadClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for file downloads
		},
		enterprise: cfg.EnterpriseSlug,
		org:        cfg.OrganizationSlug,
	}

	// Create separate org client if org installation ID is configured
	if cfg.GitHubAppOrgInstallationID != 0 {
		orgTransport, err := ghinstallation.New(
			http.DefaultTransport,
			cfg.GitHubAppID,
			cfg.GitHubAppOrgInstallationID,
			[]byte(cfg.GitHubAppPrivateKey),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub App org transport: %w", err)
		}
		client.orgHttpClient = &http.Client{
			Transport: orgTransport,
			Timeout:   30 * time.Second,
		}
		slog.Info("Org installation configured", "org", cfg.OrganizationSlug, "installation_id", cfg.GitHubAppOrgInstallationID)
	} else {
		slog.Warn("No org installation ID configured — org-level endpoints will use enterprise token (may fail with 403)")
	}

	return client, nil
}

// FetchDailyMetrics fetches metrics for a specific day, trying enterprise first then org.
func (c *GitHubClient) FetchDailyMetrics(ctx context.Context, day time.Time) (*FetchResult, error) {
	dayStr := day.Format("2006-01-02")

	// Try enterprise endpoint first
	enterpriseURL := fmt.Sprintf("https://api.github.com/enterprises/%s/copilot/metrics/reports/enterprise-1-day?day=%s",
		c.enterprise, dayStr)

	slog.Debug("Fetching metrics report", "url", enterpriseURL, "day", dayStr)

	records, err := c.fetchMetricsFromURLWithRetry(ctx, enterpriseURL)
	if err == nil {
		return &FetchResult{
			Records: records,
			Scope:   "enterprise",
			ScopeID: c.enterprise,
		}, nil
	}

	enterpriseErr := err
	slog.Warn("Enterprise endpoint failed, trying organization endpoint", "error", enterpriseErr)

	// Fall back to organization endpoint
	orgURL := fmt.Sprintf("https://api.github.com/orgs/%s/copilot/metrics/reports/organization-1-day?day=%s",
		c.org, dayStr)
	slog.Debug("Fetching metrics report (org fallback)", "url", orgURL, "day", dayStr)

	records, err = c.fetchMetricsFromURLWithRetry(ctx, orgURL)
	if err != nil {
		if isReportNotAvailable(enterpriseErr) {
			return nil, fmt.Errorf("%w for %s: enterprise report not generated yet and org endpoint also failed: %v",
				ErrReportNotAvailable, dayStr, err)
		}
		return nil, fmt.Errorf("both enterprise and org endpoints failed: %w", err)
	}

	return &FetchResult{
		Records: records,
		Scope:   "organization",
		ScopeID: c.org,
	}, nil
}

// fetchMetricsFromURLWithRetry retries transient failures with exponential backoff.
func (c *GitHubClient) fetchMetricsFromURLWithRetry(ctx context.Context, url string) ([]json.RawMessage, error) {
	return c.fetchWithRetry(ctx, url, c.fetchMetricsFromURL)
}

// fetchMetricsFromURLWithRetryOrg uses the org installation token with retries.
func (c *GitHubClient) fetchMetricsFromURLWithRetryOrg(ctx context.Context, url string) ([]json.RawMessage, error) {
	return c.fetchWithRetry(ctx, url, c.fetchMetricsFromURLWithOrg)
}

func (c *GitHubClient) fetchWithRetry(ctx context.Context, url string, fetchFn func(context.Context, string) ([]json.RawMessage, error)) ([]json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second // 2s, 4s
			slog.Debug("Retrying after backoff", "url", url, "attempt", attempt+1, "backoff", backoff, "last_error", lastErr)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		records, err := fetchFn(ctx, url)
		if err == nil {
			if attempt > 0 {
				slog.Info("Request succeeded after retry", "url", url, "attempts", attempt+1)
			}
			return records, nil
		}
		lastErr = err

		// Don't retry on errors that won't resolve with a retry
		if isClientError(err) || isReportNotAvailable(err) || isDecodeError(err) {
			slog.Debug("Non-retryable error", "url", url, "error", err,
				"is_client_error", isClientError(err),
				"is_not_available", isReportNotAvailable(err),
				"is_decode_error", isDecodeError(err))
			return nil, err
		}
	}
	return nil, fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// isClientError checks if the error indicates a 4xx HTTP status.
func isClientError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "status 4")
}

// isReportNotAvailable checks if the error indicates the report hasn't been generated yet.
func isReportNotAvailable(err error) bool {
	return errors.Is(err, ErrReportNotAvailable) || strings.Contains(err.Error(), "No report available")
}

// isDecodeError checks if the error is a non-retryable response decode failure.
func isDecodeError(err error) bool {
	return strings.Contains(err.Error(), "failed to decode report response")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func (c *GitHubClient) fetchMetricsFromURL(ctx context.Context, url string) ([]json.RawMessage, error) {
	return c.fetchMetricsFromURLWith(ctx, url, c.httpClient)
}

// fetchMetricsFromURLWithOrg uses the org installation token for org-level endpoints.
func (c *GitHubClient) fetchMetricsFromURLWithOrg(ctx context.Context, url string) ([]json.RawMessage, error) {
	client := c.httpClient
	if c.orgHttpClient != nil {
		client = c.orgHttpClient
	}
	return c.fetchMetricsFromURLWith(ctx, url, client)
}

func (c *GitHubClient) fetchMetricsFromURLWith(ctx context.Context, url string, client *http.Client) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	slog.Debug("GitHub API request", "method", "GET", "url", url)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("GitHub API response", "url", url, "status", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"),
		"rate_remaining", resp.Header.Get("X-Ratelimit-Remaining"),
		"rate_limit", resp.Header.Get("X-Ratelimit-Limit"))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	// Read body so we can inspect it before decoding. The API sometimes returns
	// a plain JSON string (e.g. "No report available") instead of an object.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Debug("GitHub API response body", "url", url, "body_bytes", len(body))

	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) > 0 && trimmed[0] == '"' {
		// API returned a JSON string — typically means the report isn't ready yet
		return nil, fmt.Errorf("%w: %s", ErrReportNotAvailable, trimmed)
	}

	var reportResp MetricsReportResponse
	if err := json.Unmarshal(body, &reportResp); err != nil {
		return nil, fmt.Errorf("failed to decode report response (body: %s): %w", truncate(trimmed, 200), err)
	}

	slog.Info("Got download links", "count", len(reportResp.DownloadLinks), "report_day", reportResp.ReportDay, "url", url)

	var allRecords []json.RawMessage
	for i, downloadURL := range reportResp.DownloadLinks {
		slog.Debug("Downloading NDJSON file", "file_index", i, "total_files", len(reportResp.DownloadLinks))
		records, err := c.downloadAndParseNDJSON(ctx, downloadURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download file %d: %w", i, err)
		}
		slog.Debug("Downloaded NDJSON file", "file_index", i, "records", len(records))
		allRecords = append(allRecords, records...)
	}

	return allRecords, nil
}

func (c *GitHubClient) downloadAndParseNDJSON(ctx context.Context, url string) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	slog.Debug("Downloading NDJSON", "content_length_hint", req.Header.Get("Content-Length"))

	// Use downloadClient (no auth) for pre-signed URLs
	resp, err := c.downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	slog.Debug("NDJSON download response", "status", resp.StatusCode,
		"content_length", resp.Header.Get("Content-Length"),
		"content_type", resp.Header.Get("Content-Type"))

	var records []json.RawMessage
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	lineNum := 0
	invalidLines := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if !json.Valid(line) {
			invalidLines++
			slog.Warn("Invalid JSON line in NDJSON", "line_number", lineNum)
			continue
		}

		record := make(json.RawMessage, len(line))
		copy(record, line)
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading NDJSON: %w", err)
	}

	slog.Debug("Parsed NDJSON", "total_lines", lineNum, "valid_records", len(records), "invalid_lines", invalidLines)
	return records, nil
}

// FetchDailyUserTeams fetches the user-teams-1-day report for the given day.
// Fetches from both enterprise and org endpoints to ensure all teams are captured.
// Enterprise teams and org teams may differ — the enterprise endpoint returns teams
// visible at enterprise level, while org endpoint returns org-level teams.
func (c *GitHubClient) FetchDailyUserTeams(ctx context.Context, day time.Time) (*FetchResult, error) {
	dayStr := day.Format("2006-01-02")

	// Try enterprise endpoint
	enterpriseURL := fmt.Sprintf("https://api.github.com/enterprises/%s/copilot/metrics/reports/%s?day=%s",
		c.enterprise, "user-teams-1-day", dayStr)
	slog.Info("Fetching user-teams report", "scope", "enterprise", "day", dayStr)
	enterpriseRecords, enterpriseErr := c.fetchMetricsFromURLWithRetry(ctx, enterpriseURL)
	if enterpriseErr != nil {
		slog.Warn("Enterprise user-teams endpoint failed", "day", dayStr, "error", enterpriseErr)
	} else {
		slog.Info("Enterprise user-teams fetched", "day", dayStr, "records", len(enterpriseRecords))
	}

	// Also try org endpoint for org-level teams
	orgURL := fmt.Sprintf("https://api.github.com/orgs/%s/copilot/metrics/reports/%s?day=%s",
		c.org, "user-teams-1-day", dayStr)
	slog.Info("Fetching user-teams report", "scope", "organization", "org", c.org, "day", dayStr)
	orgRecords, orgErr := c.fetchMetricsFromURLWithRetryOrg(ctx, orgURL)
	if orgErr != nil {
		slog.Warn("Org user-teams endpoint failed", "day", dayStr, "org", c.org, "error", orgErr)
	} else {
		slog.Info("Org user-teams fetched", "day", dayStr, "org", c.org, "records", len(orgRecords))
	}

	// Merge results — deduplicate by user_id + team_id
	var allRecords []json.RawMessage
	seen := make(map[string]bool)

	addRecords := func(records []json.RawMessage) {
		for _, r := range records {
			var entry struct {
				UserID string `json:"user_id"`
				TeamID string `json:"team_id"`
			}
			if err := json.Unmarshal(r, &entry); err == nil {
				key := entry.UserID + ":" + entry.TeamID
				if !seen[key] {
					seen[key] = true
					allRecords = append(allRecords, r)
				}
			} else {
				allRecords = append(allRecords, r)
			}
		}
	}

	if enterpriseErr == nil {
		addRecords(enterpriseRecords)
	}
	if orgErr == nil {
		addRecords(orgRecords)
	}

	slog.Info("User-teams merge complete", "day", dayStr,
		"enterprise_ok", enterpriseErr == nil, "org_ok", orgErr == nil,
		"total_records", len(allRecords), "deduplicated_keys", len(seen))

	if len(allRecords) > 0 {
		scope := "enterprise"
		scopeID := c.enterprise
		if enterpriseErr != nil {
			scope = "organization"
			scopeID = c.org
		}
		return &FetchResult{
			Records: allRecords,
			Scope:   scope,
			ScopeID: scopeID,
		}, nil
	}

	// Both failed
	if enterpriseErr != nil && orgErr != nil {
		if isReportNotAvailable(enterpriseErr) {
			return nil, fmt.Errorf("%w for %s (user-teams-1-day): both endpoints failed", ErrReportNotAvailable, dayStr)
		}
		return nil, fmt.Errorf("both enterprise and org user-teams-1-day endpoints failed: enterprise=%v, org=%v", enterpriseErr, orgErr)
	}

	return &FetchResult{Records: allRecords, Scope: "enterprise", ScopeID: c.enterprise}, nil
}

// FetchDailyUserMetrics fetches the users-1-day report for the given day.
// This report contains per-user usage metrics needed for team-level aggregation.
func (c *GitHubClient) FetchDailyUserMetrics(ctx context.Context, day time.Time) (*FetchResult, error) {
	return c.fetchDailyReport(ctx, day, "users-1-day")
}

// fetchDailyReport fetches a daily report by type, trying enterprise first then org.
func (c *GitHubClient) fetchDailyReport(ctx context.Context, day time.Time, reportType string) (*FetchResult, error) {
	dayStr := day.Format("2006-01-02")

	enterpriseURL := fmt.Sprintf("https://api.github.com/enterprises/%s/copilot/metrics/reports/%s?day=%s",
		c.enterprise, reportType, dayStr)

	slog.Debug("Fetching report", "type", reportType, "url", enterpriseURL, "day", dayStr)

	records, err := c.fetchMetricsFromURLWithRetry(ctx, enterpriseURL)
	if err == nil {
		return &FetchResult{
			Records: records,
			Scope:   "enterprise",
			ScopeID: c.enterprise,
		}, nil
	}

	enterpriseErr := err
	slog.Warn("Enterprise endpoint failed, trying organization endpoint", "type", reportType, "error", enterpriseErr)

	orgURL := fmt.Sprintf("https://api.github.com/orgs/%s/copilot/metrics/reports/%s?day=%s",
		c.org, reportType, dayStr)
	slog.Debug("Fetching report (org fallback)", "type", reportType, "url", orgURL, "day", dayStr)

	records, err = c.fetchMetricsFromURLWithRetry(ctx, orgURL)
	if err != nil {
		if isReportNotAvailable(enterpriseErr) {
			return nil, fmt.Errorf("%w for %s (%s): enterprise report not generated yet and org endpoint also failed: %v",
				ErrReportNotAvailable, dayStr, reportType, err)
		}
		return nil, fmt.Errorf("both enterprise and org %s endpoints failed: %w", reportType, err)
	}

	return &FetchResult{
		Records: records,
		Scope:   "organization",
		ScopeID: c.org,
	}, nil
}

// FetchLatest28DayReport fetches the latest 28-day rolling report.
func (c *GitHubClient) FetchLatest28DayReport(ctx context.Context) (*FetchResult, error) {
	url := fmt.Sprintf("https://api.github.com/enterprises/%s/copilot/metrics/reports/enterprise-28-day/latest",
		c.enterprise)

	slog.Debug("Fetching 28-day report", "url", url)

	records, err := c.fetchMetricsFromURLWithRetry(ctx, url)
	if err == nil {
		return &FetchResult{
			Records: records,
			Scope:   "enterprise",
			ScopeID: c.enterprise,
		}, nil
	}

	slog.Warn("Enterprise 28-day endpoint failed, trying organization endpoint", "error", err)

	url = fmt.Sprintf("https://api.github.com/orgs/%s/copilot/metrics/reports/organization-28-day/latest",
		c.org)
	records, err = c.fetchMetricsFromURLWithRetry(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("both enterprise and org 28-day endpoints failed: %w", err)
	}

	return &FetchResult{
		Records: records,
		Scope:   "organization",
		ScopeID: c.org,
	}, nil
}
