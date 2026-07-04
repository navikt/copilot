package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestBudgetHandlersIdentityShadowMode verifies that BudgetHandlers.handleGetBudget's
// shadow-mode identity comparison logs disagreements without changing the
// legacy SAML-resolved username actually used to fetch budget data.
func TestBudgetHandlersIdentityShadowMode(t *testing.T) {
	newRequestWithUser := func(user *User) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/budget", nil)
		if user != nil {
			req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		}
		return req
	}

	t.Run("matching chain result logs no mismatch", func(t *testing.T) {
		h := newBudgetHandlers(nil, &mockGitHubClient{samlUsername: "hans"})
		h.setIdentityChain(NewIdentityResolverChain(NewSAMLIdentityResolver(&mockGitHubClient{samlUsername: "hans"})))

		req := newRequestWithUser(&User{Email: "hans@nav.no"})
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		h.logIdentityShadowComparison(req, &User{Email: "hans@nav.no"}, "hans")
		slog.SetDefault(prev)

		if strings.Contains(buf.String(), "mismatch") {
			t.Errorf("expected no mismatch log, got: %s", buf.String())
		}
	})

	t.Run("disagreeing chain result logs a mismatch", func(t *testing.T) {
		h := newBudgetHandlers(nil, &mockGitHubClient{samlUsername: "hans"})
		h.setIdentityChain(NewIdentityResolverChain(NewSAMLIdentityResolver(&mockGitHubClient{samlUsername: "someone-else"})))

		req := newRequestWithUser(&User{Email: "hans@nav.no"})
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		h.logIdentityShadowComparison(req, &User{Email: "hans@nav.no"}, "hans")
		slog.SetDefault(prev)

		if !strings.Contains(buf.String(), "Identity resolution shadow-mode mismatch (budget)") {
			t.Errorf("expected a mismatch log, got: %s", buf.String())
		}
	})

	t.Run("nil identity chain is a no-op", func(t *testing.T) {
		h := newBudgetHandlers(nil, &mockGitHubClient{samlUsername: "hans"})

		req := newRequestWithUser(&User{Email: "hans@nav.no"})
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		h.logIdentityShadowComparison(req, &User{Email: "hans@nav.no"}, "hans")
		slog.SetDefault(prev)

		if strings.Contains(buf.String(), "shadow-mode") {
			t.Errorf("expected no shadow-mode logging when identityChain is nil, got: %s", buf.String())
		}
	})
}
