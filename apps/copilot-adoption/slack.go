package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"
)

type SlackNotifier struct {
	webhookURL string
	client     *http.Client
	notified   bool
}

type slackMessage struct {
	// Channel is honoured by legacy incoming webhooks such as the Nais
	// slack-webhook secret; app-scoped webhooks ignore it.
	Channel string       `json:"channel,omitempty"`
	Text    string       `json:"text"`
	Blocks  []slackBlock `json:"blocks,omitempty"`
}

type slackBlock struct {
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	if webhookURL == "" {
		return nil
	}
	return &SlackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// NotifyError sends a Slack alert for a fatal error.
func (s *SlackNotifier) NotifyError(ctx context.Context, message string) {
	if s == nil {
		return
	}
	text := fmt.Sprintf("🔴 *Copilot Adoption Scan – Failed*\n\n%s\n\n📊 *Impact*: Adoption tracking will not update until resolved.", message)
	s.send(ctx, text)
}

func (s *SlackNotifier) send(ctx context.Context, text string) {
	if s.notified {
		slog.Debug("Slack notification already sent, skipping duplicate")
		return
	}

	msg := slackMessage{
		Channel: cmp.Or(os.Getenv("SLACK_CHANNEL"), "#github-copilot-alerts"),
		Text:    text,
		Blocks: []slackBlock{
			{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: text},
			},
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		slog.Error("Failed to marshal Slack message", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		slog.Error("Failed to create Slack request") // err holds the webhook URL, a secret
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Error("Failed to send Slack notification", "error", safeTransportError(err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("Slack webhook returned non-OK status", "status", resp.StatusCode, "body", string(respBody))
		return
	}

	slog.Info("Slack notification sent")
	s.notified = true
}

// safeTransportError drops the request URL (the webhook secret) that
// *url.Error carries in its message.
func safeTransportError(err error) error {
	var transport *url.Error
	if errors.As(err, &transport) {
		return transport.Err
	}
	return err
}
