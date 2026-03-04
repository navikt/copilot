package main

import (
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
		clientRegistrations: map[string]*ClientRegistration{
			"client1": {ClientID: "client1"},
			"client2": {ClientID: "client2"},
			"client3": {ClientID: "client3"},
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

	clientRegs := testutil.ToFloat64(tokenStoreSize.WithLabelValues("client_registrations"))
	if clientRegs != 3 {
		t.Errorf("expected client_registrations=3, got %v", clientRegs)
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
