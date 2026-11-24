package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var notifyFailures = promauto.NewCounter(prometheus.CounterOpts{
	Name: "fleet_control_notify_failures_total",
	Help: "Slack notices about MCPServer changes that could not be sent.",
})

// slackNotifier posts one line to team copilot's channel when an MCPServer is
// published, revoked or rolled back (ROS FG-14). A nil notifier does nothing.
type slackNotifier struct {
	url    string
	client *http.Client
	// done, when set, is called after each send; tests wait on it.
	done func()
}

func newSlackNotifier(url string) *slackNotifier {
	if url = strings.TrimSpace(url); url == "" {
		slog.Info("FLEET_SLACK_WEBHOOK_URL not set; Slack notices are off")
		return nil
	}
	return &slackNotifier{url: url, client: &http.Client{Timeout: 5 * time.Second}}
}

var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "*", "∗", "_", "＿", "~", "∼", "`", "'")

// serverNotice is the Slack line. by is "owner" or "platform". It holds no
// NAVident and no upstream URL: both are user data.
func serverNotice(action, ns, name string, gen int, by string) string {
	verb := map[string]string{"approve": "publisert", "revoke": "trukket tilbake", "rollback": "rullet tilbake"}[action]
	who := "eierteamet"
	if by == "platform" {
		who = "plattformteamet"
	}
	return fmt.Sprintf("MCPServer %s: %s/%s, generasjon %d, team %s (utført av %s)",
		verb, slackEscaper.Replace(ns), slackEscaper.Replace(name), gen, slackEscaper.Replace(ns), who)
}

// notify sends text in the background. Call it only after the commit; a
// failure is logged and counted, never returned.
func (n *slackNotifier) notify(text string) {
	if n == nil {
		return
	}
	go func() {
		if n.done != nil {
			defer n.done()
		}
		if err := n.send(text); err != nil {
			notifyFailures.Inc()
			slog.Warn("slack notice failed", "error", err)
		}
	}()
}

func (n *slackNotifier) send(text string) error {
	body, _ := json.Marshal(map[string]string{"text": text})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req) //nolint:gosec // URL is the operator-set webhook secret, not user input
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack answered %d", resp.StatusCode)
	}
	return nil
}

// notifyServer reads the committed generation and sends the notice.
func (s *server) notifyServer(ctx context.Context, db *DB, action, ns, name, by string) {
	if s.notifier == nil {
		return
	}
	obj, err := db.serverObject(ctx, ns, name)
	if err != nil {
		notifyFailures.Inc()
		slog.Warn("slack notice: read server", "error", err)
		return
	}
	s.notifier.notify(serverNotice(action, ns, name, obj.Status.PublishedGeneration, by))
}
