package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

type GitHubClient struct {
	httpClient     *http.Client // GitHub API client with auth
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

	return &GitHubClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		downloadClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for file downloads
		},
		enterprise: cfg.EnterpriseSlug,
		org:        cfg.OrganizationSlug,
	}, nil
}

// FetchDailyMetrics fetches metrics for a specific day, trying enterprise first then org.
func (c *GitHubClient) FetchDailyMetrics(ctx context.Context, day time.Time) (*FetchResult, error) {
	dayStr := day.Format("2006-01-02")

	// Try enterprise endpoint first
	url := fmt.Sprintf("https://api.github.com/enterprises/%s/copilot/metrics/reports/enterprise-1-day?day=%s",
		c.enterprise, dayStr)

	slog.Debug("Fetching metrics report", "url", url, "day", dayStr)

	records, err := c.fetchMetricsFromURLWithRetry(ctx, url)
	if err == nil {
		return &FetchResult{
			Records: records,
			Scope:   "enterprise",
			ScopeID: c.enterprise,
		}, nil
	}

	slog.Warn("Enterprise endpoint failed, trying organization endpoint", "error", err)

	// Fall back to organization endpoint
	url = fmt.Sprintf("https://api.github.com/orgs/%s/copilot/metrics/reports/organization-1-day?day=%s",
		c.org, dayStr)
	slog.Debug("Fetching metrics report (org fallback)", "url", url, "day", dayStr)

	records, err = c.fetchMetricsFromURLWithRetry(ctx, url)
	if err != nil {
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
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second // 2s, 4s
			slog.Debug("Retrying after backoff", "attempt", attempt+1, "backoff", backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		records, err := c.fetchMetricsFromURL(ctx, url)
		if err == nil {
			return records, nil
		}
		lastErr = err

		// Don't retry on 4xx errors (client errors)
		if isClientError(err) {
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

func (c *GitHubClient) fetchMetricsFromURL(ctx context.Context, url string) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var reportResp MetricsReportResponse
	if err := json.NewDecoder(resp.Body).Decode(&reportResp); err != nil {
		return nil, fmt.Errorf("failed to decode report response: %w", err)
	}

	slog.Info("Got download links", "count", len(reportResp.DownloadLinks), "report_day", reportResp.ReportDay)

	var allRecords []json.RawMessage
	for i, downloadURL := range reportResp.DownloadLinks {
		records, err := c.downloadAndParseNDJSON(ctx, downloadURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download file %d: %w", i, err)
		}
		allRecords = append(allRecords, records...)
	}

	return allRecords, nil
}

func (c *GitHubClient) downloadAndParseNDJSON(ctx context.Context, url string) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// Use downloadClient (no auth) for pre-signed URLs
	resp, err := c.downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	var records []json.RawMessage
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if !json.Valid(line) {
			slog.Warn("Invalid JSON line", "line", lineNum)
			continue
		}

		record := make(json.RawMessage, len(line))
		copy(record, line)
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading NDJSON: %w", err)
	}

	slog.Debug("Parsed NDJSON", "records", len(records))
	return records, nil
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
