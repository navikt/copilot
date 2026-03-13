package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlackNotifier_NilIsNoOp(t *testing.T) {
	var s *SlackNotifier
	// Should not panic
	s.NotifyScanResult(context.Background(), 100, 0, 100)
	s.NotifyError(context.Background(), "something broke")
}

func TestSlackNotifier_SkipsWhenNoErrors(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSlackNotifier(server.URL)
	s.NotifyScanResult(context.Background(), 100, 100, 0)

	if called {
		t.Error("expected no Slack call when there are no errors")
	}
}

func TestSlackNotifier_SendsOnPartialFailure(t *testing.T) {
	var received slackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSlackNotifier(server.URL)
	s.NotifyScanResult(context.Background(), 100, 90, 10)

	if received.Text == "" {
		t.Fatal("expected Slack message, got empty")
	}
	if !strings.Contains(received.Text, "Partial Failure") {
		t.Errorf("expected 'Partial Failure' in message, got: %s", received.Text)
	}
	if !strings.Contains(received.Text, "#copilot-support") {
		t.Errorf("expected contact channel in message, got: %s", received.Text)
	}
}

func TestSlackNotifier_CompleteFailure(t *testing.T) {
	var received slackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSlackNotifier(server.URL)
	s.NotifyScanResult(context.Background(), 100, 0, 100)

	if !strings.Contains(received.Text, "Failed") {
		t.Errorf("expected 'Failed' in message, got: %s", received.Text)
	}
	if !strings.Contains(received.Text, "Impact") {
		t.Errorf("expected impact section in message, got: %s", received.Text)
	}
}

func TestSlackNotifier_NotifyError(t *testing.T) {
	var received slackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSlackNotifier(server.URL)
	s.NotifyError(context.Background(), "scan failed completely")

	if !strings.Contains(received.Text, "scan failed completely") {
		t.Errorf("expected error message in Slack text, got: %s", received.Text)
	}
	if !strings.Contains(received.Text, "Copilot Adoption") {
		t.Errorf("expected app name in Slack text, got: %s", received.Text)
	}
}

func TestNewSlackNotifier_EmptyURL(t *testing.T) {
	s := NewSlackNotifier("")
	if s != nil {
		t.Error("expected nil notifier for empty URL")
	}
}
