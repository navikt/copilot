package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

type slackMessage struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks,omitempty"`
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

// NotifyIngestionResult sends a Slack alert when ingestion has failures.
// Does nothing if all days succeeded.
func (s *SlackNotifier) NotifyIngestionResult(ctx context.Context, successCount, errorCount int, failedDays []string) {
	if s == nil || errorCount == 0 {
		return
	}

	totalDays := successCount + errorCount
	emoji := "⚠️"
	status := "partial failure"
	if successCount == 0 {
		emoji = "🔴"
		status = "complete failure"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s *copilot-metrics ingestion %s*\n", emoji, status)
	fmt.Fprintf(&sb, "• Success: %d/%d days\n", successCount, totalDays)
	fmt.Fprintf(&sb, "• Failed: %d/%d days\n", errorCount, totalDays)

	if len(failedDays) > 0 {
		displayed := failedDays
		if len(displayed) > 10 {
			displayed = displayed[:10]
		}
		fmt.Fprintf(&sb, "• Failed days: `%s`", strings.Join(displayed, "`, `"))
		if len(failedDays) > 10 {
			fmt.Fprintf(&sb, " … and %d more", len(failedDays)-10)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Check logs in NAIS for details.")

	s.send(ctx, sb.String())
}

// NotifyError sends a Slack alert for a fatal error.
func (s *SlackNotifier) NotifyError(ctx context.Context, message string) {
	if s == nil {
		return
	}
	s.send(ctx, fmt.Sprintf("🔴 *copilot-metrics*: %s", message))
}

func (s *SlackNotifier) send(ctx context.Context, text string) {
	msg := slackMessage{
		Text: text,
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
		slog.Error("Failed to create Slack request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Error("Failed to send Slack notification", "error", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("Slack webhook returned non-OK status", "status", resp.StatusCode, "body", string(respBody))
		return
	}

	slog.Info("Slack notification sent")
}
