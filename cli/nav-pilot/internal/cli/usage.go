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

// usageResponse mirrors the JSON shape returned by copilot-cli's
// GET /api/v1/usage endpoint, which in turn proxies copilot-api. Fields are
// intentionally permissive (all optional) since this endpoint is still under
// active development on the copilot-api side.
type usageResponse struct {
	Username string `json:"username"`
	Period   string `json:"period"`
	Credits  struct {
		Used       int `json:"used"`
		Limit      int `json:"limit"`
		Percentage int `json:"percentage"`
	} `json:"credits"`
	Interactions struct {
		Total          int     `json:"total"`
		Accepted       int     `json:"accepted"`
		AcceptanceRate float64 `json:"acceptance_rate"`
	} `json:"interactions"`
	ActiveDays int `json:"active_days"`
	Forecast   struct {
		ProjectedCredits int `json:"projected_credits"`
	} `json:"forecast"`
	Subscription struct {
		Status string `json:"status"`
		Plan   string `json:"plan"`
	} `json:"subscription"`
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
