package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlackNotifier_NilIsNoOp(t *testing.T) {
	var s *SlackNotifier
	// Should not panic
	s.NotifyError(context.Background(), "something broke")
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

func TestSlackNotifier_TransportErrorHidesWebhookURL(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(prev)

	server := httptest.NewServer(http.NotFoundHandler())
	server.Close() // connection refused: a transport error
	s := NewSlackNotifier(server.URL + "/services/T000/B000/secret-token")
	s.NotifyError(context.Background(), "scan failed")

	if !strings.Contains(logs.String(), "Failed to send Slack notification") {
		t.Fatalf("no transport error logged (control): %s", logs.String())
	}
	if strings.Contains(logs.String(), "secret-token") {
		t.Errorf("log leaks the webhook URL: %s", logs.String())
	}
}
