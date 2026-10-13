package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackNotifier_NilIsNoOp(t *testing.T) {
	var s *SlackNotifier
	// Should not panic
	s.NotifyIngestionResult(context.Background(), 0, 3, []string{"2026-03-10", "2026-03-11", "2026-03-12"})
	s.NotifyError(context.Background(), "something broke")
}

func TestSlackNotifier_SkipsWhenNoErrors(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	s := &SlackNotifier{
		webhookURL: "http://test/webhook",
		client:     mockClient(handler),
	}
	s.NotifyIngestionResult(context.Background(), 5, 0, nil)

	if called {
		t.Error("expected no Slack call when there are no errors")
	}
}

func TestSlackNotifier_SendsOnFailure(t *testing.T) {
	var received slackMessage
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	s := &SlackNotifier{
		webhookURL: "http://test/webhook",
		client:     mockClient(handler),
	}
	s.NotifyIngestionResult(context.Background(), 2, 1, []string{"2026-03-11"})

	if received.Text == "" {
		t.Fatal("expected Slack message, got empty")
	}
	if len(received.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(received.Blocks))
	}
	text := received.Blocks[0].Text.Text
	if !contains(text, "Partial Failure") {
		t.Errorf("expected 'Partial Failure' in message, got: %s", text)
	}
	if !contains(text, "2026-03-11") {
		t.Errorf("expected failed day in message, got: %s", text)
	}
	if contains(text, "Contact") {
		t.Errorf("expected no contact line, got: %s", text)
	}
}

func TestSlackNotifier_CompleteFailure(t *testing.T) {
	var received slackMessage
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	})

	s := &SlackNotifier{
		webhookURL: "http://test/webhook",
		client:     mockClient(handler),
	}
	s.NotifyIngestionResult(context.Background(), 0, 3, []string{"2026-03-10", "2026-03-11", "2026-03-12"})

	text := received.Blocks[0].Text.Text
	if !contains(text, "Failed") {
		t.Errorf("expected 'Failed' in message, got: %s", text)
	}
	if !contains(text, "Impact") {
		t.Errorf("expected 'Impact' section in message, got: %s", text)
	}
}

func TestSlackNotifier_NotifyError(t *testing.T) {
	var received slackMessage
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	})

	s := &SlackNotifier{
		webhookURL: "http://test/webhook",
		client:     mockClient(handler),
	}
	s.NotifyError(context.Background(), "database connection failed")

	if !contains(received.Text, "database connection failed") {
		t.Errorf("expected error message in text, got: %s", received.Text)
	}
}

func TestNewSlackNotifier_EmptyURL(t *testing.T) {
	s := NewSlackNotifier("")
	if s != nil {
		t.Error("expected nil notifier for empty URL")
	}
}

func TestSlackNotifier_Channel(t *testing.T) {
	for _, tc := range []struct{ env, want string }{
		{"", "#github-copilot-alerts"},
		{"#other", "#other"},
	} {
		t.Setenv("SLACK_CHANNEL", tc.env)
		var received slackMessage
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
		}))
		NewSlackNotifier(server.URL).NotifyError(context.Background(), "boom")
		server.Close()
		if received.Channel != tc.want {
			t.Errorf("SLACK_CHANNEL=%q: channel = %q, want %q", tc.env, received.Channel, tc.want)
		}
	}
}
