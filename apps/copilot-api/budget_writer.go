package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// budgetWriter writes individual user budgets on the enterprise budgets
// endpoint. It never decides which budget to touch: the raise flow only hands
// it IDs read back from budget_github (PRD chapter 7).
type budgetWriter struct {
	httpClient *http.Client
	baseURL    string
	enterprise string
	token      func(context.Context) (string, error)
}

// ghBudget is one entry in the enterprise budgets list. Only the list and
// create responses carry expires_at.
type ghBudget struct {
	ID          string  `json:"id"`
	BudgetScope string  `json:"budget_scope"`
	Amount      float64 `json:"budget_amount"`
	ExpiresAt   string  `json:"expires_at"`
}

type ghUserBudgets struct {
	Budgets   []ghBudget       `json:"budgets"`
	Effective *effectiveBudget `json:"effective_budget"`
}

var errNoWriteToken = errors.New("budget write token not configured")

// newBudgetTokenSource uses copilot-metrics' enterprise GitHub App (owner
// decision 2026-11-23; a dedicated app replaces it after New Year). The values
// come from a separate Nais secret with BUDGET_ prefixed names, because
// copilot-api's GITHUB_APP_* already hold the org app. Unset: nil, raises 503.
func newBudgetTokenSource(cfg *Config) (func(context.Context) (string, error), error) {
	if cfg.BudgetAppID == "" && cfg.BudgetAppPrivateKey == "" && cfg.BudgetAppInstallationID == "" {
		return nil, nil
	}
	app, err := newGitHubClient(&Config{GitHubAppID: cfg.BudgetAppID, GitHubAppPrivateKey: cfg.BudgetAppPrivateKey, GitHubInstallationID: cfg.BudgetAppInstallationID})
	if err != nil {
		return nil, fmt.Errorf("budget app: %w", err)
	}
	return app.getInstallationToken, nil
}

func (b *budgetWriter) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	if b == nil || b.token == nil {
		return 0, errNoWriteToken
	}
	tok, err := b.token(ctx)
	if err != nil {
		return 0, fmt.Errorf("budget token: %w", err)
	}
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.baseURL+path, rd)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("budget %s: %T", method, err) // *url.Error names the URL, and so the login
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return resp.StatusCode, fmt.Errorf("budget %s returned %d", method, resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode budget %s: %w", method, err)
		}
	}
	return resp.StatusCode, nil
}

func (b *budgetWriter) path(suffix string) string {
	return "/enterprises/" + url.PathEscape(b.enterprise) + "/settings/billing/budgets" + suffix
}

func (b *budgetWriter) listUser(ctx context.Context, login string) (*ghUserBudgets, int, error) {
	var out ghUserBudgets
	status, err := b.do(ctx, http.MethodGet, b.path("?user="+url.QueryEscape(login)), nil, &out)
	return &out, status, err
}

// createUserBudget only ever creates budget_scope=user for the given login.
func (b *budgetWriter) createUserBudget(ctx context.Context, login string, amount int, expires time.Time) (string, int, error) {
	body := map[string]any{
		"budget_scope":          "user",
		"user":                  login,
		"budget_entity_name":    "", // required even when empty
		"budget_amount":         amount,
		"prevent_further_usage": true,
		"budget_type":           "SkuPricing",
		"budget_product_sku":    "ai_credits",
		"expires_at":            expires.Format(time.DateOnly),
	}
	var out ghBudget
	status, err := b.do(ctx, http.MethodPost, b.path(""), body, &out)
	if err == nil && out.ID == "" {
		err = errors.New("budget create returned no id")
	}
	return out.ID, status, err
}

// patchOwnBudget changes amount and expiry on a budget ID the caller read from
// budget_github. Callers must never pass an ID from the GitHub list.
func (b *budgetWriter) patchOwnBudget(ctx context.Context, id string, amount int, expires time.Time) (int, error) {
	body := map[string]any{
		"budget_amount":         amount,
		"prevent_further_usage": true,
		"expires_at":            expires.Format(time.DateOnly),
	}
	return b.do(ctx, http.MethodPatch, b.path("/"+url.PathEscape(id)), body, nil)
}
