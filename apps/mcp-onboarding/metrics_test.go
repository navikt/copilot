package main

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestUpdateTokenStoreGauges(t *testing.T) {
	store := &TokenStore{
		tokens: map[string]*TokenData{
			"tok1": {UserLogin: "user1", ExpiresAt: time.Now().Add(time.Hour)},
			"tok2": {UserLogin: "user2", ExpiresAt: time.Now().Add(time.Hour)},
		},
		refreshTokens: map[string]*RefreshTokenData{
			"ref1": {UserLogin: "user1"},
		},
	}

	tokenStoreSize.Reset()
	updateTokenStoreGauges(store)

	activeTokens := testutil.ToFloat64(tokenStoreSize.WithLabelValues("active_tokens"))
	if activeTokens != 2 {
		t.Errorf("expected active_tokens=2, got %v", activeTokens)
	}

	refreshTokens := testutil.ToFloat64(tokenStoreSize.WithLabelValues("refresh_tokens"))
	if refreshTokens != 1 {
		t.Errorf("expected refresh_tokens=1, got %v", refreshTokens)
	}
}

func TestRecordToolCall(t *testing.T) {
	mcpToolCallsTotal.Reset()

	recordToolCall("hello_world", "success")
	recordToolCall("hello_world", "success")
	recordToolCall("greet", "error")

	successCount := testutil.ToFloat64(mcpToolCallsTotal.WithLabelValues("hello_world", "success"))
	if successCount != 2 {
		t.Errorf("expected 2 success calls, got %v", successCount)
	}

	errorCount := testutil.ToFloat64(mcpToolCallsTotal.WithLabelValues("greet", "error"))
	if errorCount != 1 {
		t.Errorf("expected 1 error call, got %v", errorCount)
	}
}

func TestRecordOAuthFlow(t *testing.T) {
	oauthFlowsTotal.Reset()

	recordOAuthFlow("authorize", "started")
	recordOAuthFlow("callback", "success")
	recordOAuthFlow("callback", "org_denied")

	started := testutil.ToFloat64(oauthFlowsTotal.WithLabelValues("authorize", "started"))
	if started != 1 {
		t.Errorf("expected 1 authorize/started, got %v", started)
	}

	callbackSuccess := testutil.ToFloat64(oauthFlowsTotal.WithLabelValues("callback", "success"))
	if callbackSuccess != 1 {
		t.Errorf("expected 1 callback/success, got %v", callbackSuccess)
	}

	callbackDenied := testutil.ToFloat64(oauthFlowsTotal.WithLabelValues("callback", "org_denied"))
	if callbackDenied != 1 {
		t.Errorf("expected 1 callback/org_denied, got %v", callbackDenied)
	}
}

// TestRecordHTTPMetrics_PathLabelIsBounded covers the unbounded label: the
// request path went straight into http_requests_total{path=...}, so anyone
// hitting the catch-all "/" route with made-up paths could grow the series
// count without limit. Known routes must survive (the shared dashboard groups
// by path); everything else collapses to one bucket.
func TestRecordHTTPMetrics_PathLabelIsBounded(t *testing.T) {
	httpRequestsTotal.Reset()

	known := []string{
		"/",
		"/mcp",
		"/register",
		"/oauth/authorize",
		"/oauth/callback",
		"/oauth/token",
		"/.well-known/oauth-authorization-server",
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
	}
	for _, p := range known {
		recordHTTPMetrics("GET", p, 200, time.Millisecond)
		if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", p, "200")); got != 1 {
			t.Errorf("known route %q: expected it to keep its own label, got %v", p, got)
		}
	}

	for _, p := range []string{"/wp-admin", "/../etc/passwd", "/random/" + strings.Repeat("x", 64), "/MCP"} {
		recordHTTPMetrics("GET", p, 404, time.Millisecond)
		if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", p, "404")); got != 0 {
			t.Errorf("unknown path %q: expected no series of its own, got %v", p, got)
		}
	}
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "other", "404")); got != 4 {
		t.Errorf("expected 4 unknown paths bucketed as \"other\", got %v", got)
	}
}
