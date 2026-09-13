package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBudgetHandlersHandleGetBudget verifies handleGetBudget relies on
// IdentityMiddleware having already resolved the caller's GitHub username
// (Phase 3 of the auth architecture migration — see identity_middleware.go),
// rather than doing its own SAML lookup.
func TestBudgetHandlersHandleGetBudget(t *testing.T) {
	t.Run("missing resolved identity is rejected", func(t *testing.T) {
		h := newBudgetHandlers(nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/budget", nil)
		rec := httptest.NewRecorder()

		h.handleGetBudget(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("resolved identity is used to fetch the user's budget", func(t *testing.T) {
		budgetClient := newBudgetClient("fake-token", "nav")
		h := newBudgetHandlers(budgetClient)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/budget", nil)
		ctx := context.WithValue(req.Context(), resolvedIdentityContextKey, &ResolvedIdentity{GitHubUsername: "hans", Source: "test"})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		// No real GitHub API available in this unit test, so we just assert
		// that handleGetBudget got past the identity check (it will fail
		// later trying to reach the real budget API — any status other than
		// 401 confirms requireOwnership/GetResolvedIdentity was satisfied).
		h.handleGetBudget(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected identity check to pass with a resolved identity in context, got 401")
		}
	})
}
