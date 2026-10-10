package provider

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// openCodeCommand runs opencode for its output. A variable so the test can
// answer for it.
var openCodeCommand = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = dir
	// These reads are not a session: no spans from them.
	cmd.Env = append(os.Environ(), "OTEL_SDK_DISABLED=true")
	return cmd.Output()
}

// readOpenCodeUsage sums the tokens, model calls and tool calls of the
// sessions in dir that changed since the launch, counting only messages from
// since on, so a resumed session is not counted twice.
//
// opencode sends traces but no metrics, so this is where its usage becomes a
// metric. It reads opencode's own store through its CLI after the session has
// ended, and --sanitize keeps the transcript out of what nav-pilot reads.
//
// ponytail: a second opencode in the same directory at the same time is
// counted by both launches; tag sessions with the launch if that shows up.
func readOpenCodeUsage(dir string, since time.Time) telemetry.ClientUsage {
	u := telemetry.ClientUsage{Client: "opencode", Models: map[telemetry.ModelKey]telemetry.ModelUsage{}, Tools: map[string]int64{}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := openCodeCommand(ctx, dir, "session", "list", "--format", "json")
	if err != nil {
		return u
	}
	var sessions []struct {
		ID        string `json:"id"`
		Updated   int64  `json:"updated"`
		Directory string `json:"directory"`
	}
	if json.Unmarshal(out, &sessions) != nil {
		return u
	}
	sinceMS := since.UnixMilli()
	for _, s := range sessions {
		if s.Updated < sinceMS || filepath.Clean(s.Directory) != filepath.Clean(dir) {
			continue
		}
		out, err := openCodeCommand(ctx, dir, "export", "--sanitize", s.ID)
		if err != nil {
			continue
		}
		addOpenCodeExport(&u, out, sinceMS)
	}
	return u
}

func addOpenCodeExport(u *telemetry.ClientUsage, export []byte, sinceMS int64) {
	var e struct {
		Messages []struct {
			Info struct {
				Role       string `json:"role"`
				ModelID    string `json:"modelID"`
				ProviderID string `json:"providerID"`
				Time       struct {
					Created int64 `json:"created"`
				} `json:"time"`
				Tokens *struct {
					Input, Output, Reasoning int64
					Cache                    struct{ Read, Write int64 }
				} `json:"tokens"`
			} `json:"info"`
			Parts []struct {
				Type string `json:"type"`
				Tool string `json:"tool"`
			} `json:"parts"`
		} `json:"messages"`
	}
	if json.Unmarshal(export, &e) != nil {
		return
	}
	for _, m := range e.Messages {
		if m.Info.Role != "assistant" || m.Info.Time.Created < sinceMS {
			continue
		}
		k := telemetry.ModelKey{Provider: m.Info.ProviderID, Model: openCodeTelemetryModel(m.Info.ProviderID, m.Info.ModelID)}
		mu := u.Models[k]
		mu.Calls++
		if t := m.Info.Tokens; t != nil {
			mu.Input += t.Input
			mu.Output += t.Output
			mu.Reasoning += t.Reasoning
			mu.CacheRead += t.Cache.Read
			mu.CacheWrite += t.Cache.Write
		}
		u.Models[k] = mu
		for _, p := range m.Parts {
			if p.Type == "tool" && p.Tool != "" {
				u.Tools[p.Tool]++
			}
		}
	}
}

// openCodeTelemetryModel keeps model ids bounded. Copilot's are a short public
// list and nav-pilot's local provider goes through [local.TelemetryModel]; a
// provider the developer added has ids they typed, so those are "custom".
func openCodeTelemetryModel(provider, model string) string {
	switch provider {
	case "github-copilot":
		return model
	case LocalProviderID:
		return local.TelemetryModel(model)
	}
	return "custom"
}
