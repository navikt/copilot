package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type SlackNotifier struct {
	webhookURL string
	client     *http.Client
	notified   bool
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

// NotifyScanResult sends a Slack alert when a scan completes with errors.
// Does nothing if the scan succeeded.
func (s *SlackNotifier) NotifyScanResult(ctx context.Context, totalRepos, scannedRepos, errorCount int) {
	if s == nil || errorCount == 0 {
		return
	}

	emoji := "⚠️"
	status := "Partial Failure"
	if scannedRepos == 0 {
		emoji = "🔴"
		status = "Failed"
	}

	text := fmt.Sprintf("%s *Copilot Adoption Scan – %s*\n\n"+
		"Scanned *%d of %d* repositories for AI tool configurations.\n"+
		"• Errors: %d repositories could not be scanned\n\n"+
		"📊 *Impact*: Adoption metrics may be incomplete for some teams.\n"+
		"👤 *Contact*: #copilot-support",
		emoji, status, scannedRepos, totalRepos, errorCount)

	s.send(ctx, text)
}

// NotifyError sends a Slack alert for a fatal error.
func (s *SlackNotifier) NotifyError(ctx context.Context, message string) {
	if s == nil {
		return
	}
	text := fmt.Sprintf("🔴 *Copilot Adoption Scan – Failed*\n\n%s\n\n📊 *Impact*: Adoption tracking will not update until resolved.\n👤 *Contact*: #copilot-support", message)
	s.send(ctx, text)
}

func (s *SlackNotifier) send(ctx context.Context, text string) {
	if s.notified {
		slog.Debug("Slack notification already sent, skipping duplicate")
		return
	}

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
	s.notified = true
}
