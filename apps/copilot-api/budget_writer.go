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
	budgetType string // BUDGET_GITHUB_BUDGET_TYPE; P1 verifies the write
	// owns reports whether an ID is in budget_github. PATCH refuses any ID it
	// does not confirm (PRD chapter 7), whoever the caller is. nil refuses all.
	owns func(ctx context.Context, id string) (bool, error)
}

// ghBudget is one entry in the enterprise budgets list. Only the list and
// create responses carry expires_at.
type ghBudget struct {
	ID          string  `json:"id"`
	BudgetScope string  `json:"budget_scope"`
	BudgetType  string  `json:"budget_type,omitempty"`
	SKU         string  `json:"budget_product_sku,omitempty"`
	Amount      float64 `json:"budget_amount"`
	ExpiresAt   string  `json:"expires_at,omitempty"`
}

type ghUserBudgets struct {
	Budgets     []ghBudget       `json:"budgets"`
	Effective   *effectiveBudget `json:"effective_budget"`
	HasNextPage bool             `json:"has_next_page"`
}

var errNotOwnBudget = errors.New("refusing to write a budget ID not in budget_github")

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
	// Budget create/update docs name 2026-03-10; the list keeps 2022-11-28 (verify in P1).
	version := "2022-11-28"
	if method != http.MethodGet {
		version = "2026-03-10"
	}
	req.Header.Set("X-GitHub-Api-Version", version)
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
	status, err := b.do(ctx, http.MethodGet, b.path("?per_page=100&user="+url.QueryEscape(login)), nil, &out)
	if err == nil && out.HasNextPage {
		// shortcut: one page of 100 for a single user; more means something
		// we do not understand, so stop rather than miss a foreign budget.
		err = errors.New("budget list for one user has more than one page")
	}
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
		"budget_type":           b.budgetType,
		"budget_product_sku":    "ai_credits",
		"expires_at":            expires.Format(time.DateOnly),
	}
	// The enterprise endpoint answers {"message": ..., "budget": {"id": ...}}
	// (docs, and seen in dev 2026-11-23). A flat {"id": ...} is accepted too.
	var out struct {
		Budget ghBudget `json:"budget"`
		ID     string   `json:"id"`
	}
	status, err := b.do(ctx, http.MethodPost, b.path(""), body, &out)
	id := out.Budget.ID
	if id == "" {
		id = out.ID
	}
	if err == nil && id == "" {
		err = errors.New("budget create returned no id") // unknown outcome: reconciled by listing
	}
	return id, status, err
}

// patchOwnBudget changes amount and expiry on one of our budgets.
func (b *budgetWriter) patchOwnBudget(ctx context.Context, id string, amount int, expires time.Time) (int, error) {
	if b == nil || b.owns == nil {
		return 0, errNotOwnBudget
	}
	if ok, err := b.owns(ctx, id); err != nil || !ok {
		return 0, errors.Join(errNotOwnBudget, err)
	}
	body := map[string]any{
		"budget_amount":         amount,
		"prevent_further_usage": true,
		"expires_at":            expires.Format(time.DateOnly),
	}
	return b.do(ctx, http.MethodPatch, b.path("/"+url.PathEscape(id)), body, nil)
}
