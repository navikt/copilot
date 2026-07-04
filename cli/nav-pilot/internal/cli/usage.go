package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// defaultCopilotCLIURL is the naisdevice-only ingress for the copilot-cli
// gateway (see apps/copilot-cli). Overridable via the copilot_cli_url config
// key for testing against a different environment (e.g. dev-gcp).
const defaultCopilotCLIURL = "https://copilot-cli.intern.nav.no"

// usageResponse mirrors copilot-api's UserMetricsSummary JSON shape
// (see apps/copilot-api/bigquery_stats.go), which is what copilot-cli's
// GET /api/v1/usage returns verbatim after proxying
// GET /api/v1/copilot/usage/user/{username}. Keep this in sync with that
// struct — copilot-api is the source of truth for the wire format.
type usageResponse struct {
	UserLogin           string       `json:"user_login"`
	TotalAcceptances    int64        `json:"total_acceptances"`
	TotalInteractions   int64        `json:"total_interactions"`
	TotalGenerations    int64        `json:"total_generations"`
	TotalLinesSuggested int64        `json:"total_lines_suggested"`
	TotalLinesAccepted  int64        `json:"total_lines_accepted"`
	TotalLinesDeleted   int64        `json:"total_lines_deleted"`
	ActiveDays          int64        `json:"active_days"`
	DaysInPeriod        int64        `json:"days_in_period"`
	DaysUsedAgent       int64        `json:"days_used_agent"`
	DaysUsedChat        int64        `json:"days_used_chat"`
	DaysUsedCLI         int64        `json:"days_used_cli"`
	DaysUsedCodeReview  int64        `json:"days_used_code_review"`
	ChatAgentRequests   int64        `json:"chat_agent_requests"`
	ChatAskRequests     int64        `json:"chat_ask_requests"`
	ChatEditRequests    int64        `json:"chat_edit_requests"`
	ChatPlanRequests    int64        `json:"chat_plan_requests"`
	ChatCustomRequests  int64        `json:"chat_custom_requests"`
	CLITotalRequests    int64        `json:"cli_total_requests"`
	CLIPrompts          int64        `json:"cli_prompts"`
	CLISessions         int64        `json:"cli_sessions"`
	CLIPromptTokens     int64        `json:"cli_prompt_tokens"`
	CLIOutputTokens     int64        `json:"cli_output_tokens"`
	TopModels           []usageModel `json:"top_models"`
	Teams               []string     `json:"teams"`
}

// usageModel mirrors copilot-api's ModelInteractions.
type usageModel struct {
	Model        string `json:"model"`
	Interactions int64  `json:"interactions"`
}

// acceptanceRate returns the accepted-suggestion rate as a percentage,
// or 0 if there were no generations to accept.
func (u *usageResponse) acceptanceRate() float64 {
	if u.TotalGenerations == 0 {
		return 0
	}
	return float64(u.TotalAcceptances) / float64(u.TotalGenerations) * 100
}

// copilotCLIURL resolves the copilot-cli endpoint: user config takes
// precedence over the built-in default. No CLI flag is exposed for this yet —
// it's intended for internal testing against dev-gcp during Phase 1.
func copilotCLIURL() string {
	if v := os.Getenv("NAV_PILOT_COPILOT_CLI_URL"); v != "" {
		return v
	}
	return defaultCopilotCLIURL
}

// cmdUsage fetches the developer's GitHub Copilot usage summary from
// copilot-cli and renders it to the terminal.
func cmdUsage(jsonOutput bool, tmuxFormat bool) error {
	token, err := loadToken()
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("not logged in — run 'nav-pilot auth login' first")
		}
		return fmt.Errorf("reading stored token: %w", err)
	}
	if token.expired() {
		return fmt.Errorf("token expired on %s — run 'nav-pilot auth login' to re-authenticate", token.ExpiresAt.Format("2006-01-02 15:04"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	usage, err := fetchUsage(ctx, copilotCLIURL(), token.AccessToken)
	if err != nil {
		return fmt.Errorf("fetching usage from copilot-cli: %w", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(usage, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding usage: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if tmuxFormat {
		fmt.Println(formatUsageTmux(usage))
		return nil
	}

	fmt.Println(formatUsageTerminal(usage))
	return nil
}

// fetchUsage calls GET {baseURL}/api/v1/usage with the developer's GitHub
// token as a Bearer token, per the copilot-cli auth contract.
func fetchUsage(ctx context.Context, baseURL, githubToken string) (*usageResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(baseURL, "/")+"/api/v1/usage", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+githubToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling copilot-cli: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot-cli returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var usage usageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("decoding usage response: %w", err)
	}
	return &usage, nil
}
