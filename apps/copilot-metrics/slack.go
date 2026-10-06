package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	status := "Partial Failure"
	if successCount == 0 {
		emoji = "🔴"
		status = "Failed"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s *Copilot Usage Metrics – %s*\n\n", emoji, status)
	fmt.Fprintf(&sb, "Daily usage data failed to sync for *%d of %d* days.\n", errorCount, totalDays)

	if len(failedDays) > 0 {
		displayed := failedDays
		if len(displayed) > 5 {
			displayed = displayed[:5]
		}
		fmt.Fprintf(&sb, "• Missing dates: %s", strings.Join(displayed, ", "))
		if len(failedDays) > 5 {
			fmt.Fprintf(&sb, " (+%d more)", len(failedDays)-5)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n📊 *Impact*: Usage dashboard in my-copilot may show incomplete data.\n")
	sb.WriteString("👤 *Contact*: #copilot-support\n")

	s.send(ctx, sb.String())
}

// NotifyError sends a Slack alert for a fatal error.
func (s *SlackNotifier) NotifyError(ctx context.Context, message string) {
	if s == nil {
		return
	}
	text := fmt.Sprintf("🔴 *Copilot Usage Metrics – Failed*\n\n%s\n\n📊 *Impact*: Usage dashboard will not update until resolved.\n👤 *Contact*: #copilot-support", message)
	s.send(ctx, text)
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
		slog.Error("Failed to create Slack request")
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
}

func safeTransportError(err error) error {
	var transport *url.Error
	if errors.As(err, &transport) {
		return transport.Err
	}
	return err
}
