package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTexasClientToken(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"m2m-token","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	client := newTexasClient(srv.URL, "api://cluster.copilot.copilot-api/.default")

	token, err := client.token(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "m2m-token" {
		t.Fatalf("expected m2m-token, got %s", token)
	}

	// Second call should be served from cache, not hit the server again.
	if _, err := client.token(t.Context()); err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call to texas sidecar, got %d", calls)
	}
}

func TestTexasClientMissingEndpoint(t *testing.T) {
	client := newTexasClient("", "api://cluster.copilot.copilot-api/.default")
	if _, err := client.token(t.Context()); err == nil {
		t.Fatal("expected error when NAIS_TOKEN_ENDPOINT is not configured")
	}
}
