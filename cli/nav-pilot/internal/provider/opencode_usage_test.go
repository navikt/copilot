package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

func TestReadOpenCodeUsage(t *testing.T) {
	since := time.UnixMilli(1000)
	old := openCodeCommand
	t.Cleanup(func() { openCodeCommand = old })
	openCodeCommand = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "session" {
			return []byte(`[{"id":"new","updated":2000,"directory":"/repo"},{"id":"old","updated":500,"directory":"/repo"},{"id":"other","updated":2000,"directory":"/elsewhere"}]`), nil
		}
		if args[len(args)-1] != "new" || !strings.Contains(strings.Join(args, " "), "--sanitize") {
			t.Fatalf("unexpected export %v", args)
		}
		return []byte(`{"messages":[
{"info":{"role":"user","time":{"created":1500}}},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":900},"tokens":{"input":999,"output":999,"reasoning":0,"cache":{"read":0,"write":0}}},"parts":[{"type":"tool","tool":"bash"}]},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":1500},"tokens":{"input":10,"output":5,"reasoning":2,"cache":{"read":100,"write":1}}},"parts":[{"type":"text"},{"type":"tool","tool":"bash"},{"type":"tool","tool":"read"},{"type":"tool","tool":"my_mcp_secret"}]},
{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":1600},"tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}},"parts":[{"type":"tool","tool":"bash"}]}]}`), nil
	}
	u := readOpenCodeUsage("/repo", since)
	want := telemetry.ModelUsage{Calls: 2, Input: 11, Output: 6, Reasoning: 2, CacheRead: 100, CacheWrite: 1}
	if got := u.Models[telemetry.ModelKey{Provider: "github-copilot", Model: "m"}]; got != want || len(u.Models) != 1 {
		t.Fatalf("models = %+v, want %+v", u.Models, want)
	}
	if p, m := openCodeTelemetryModel("my-own", "secret-name"); p != "custom" || m != "custom" {
		t.Fatalf("own provider = %q/%q, want custom/custom", p, m)
	}
	if u.Tools["bash"] != 2 || u.Tools["read"] != 1 || u.Tools["other"] != 1 || len(u.Tools) != 3 {
		t.Fatalf("tools = %v", u.Tools)
	}
}

func TestReadOpenCodeUsageResolvesSymlinks(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	old := openCodeCommand
	t.Cleanup(func() { openCodeCommand = old })
	openCodeCommand = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[len(args)-1] != "s1" {
			return []byte(`[{"id":"s1","updated":2000,"directory":"` + real + `"}]`), nil
		}
		return []byte(`{"messages":[{"info":{"role":"assistant","modelID":"m","providerID":"github-copilot","time":{"created":1500}}}]}`), nil
	}
	u := readOpenCodeUsage(link, time.UnixMilli(1000))
	if u.Models[telemetry.ModelKey{Provider: "github-copilot", Model: "m"}].Calls != 1 {
		t.Fatalf("symlinked dir not matched: %+v", u.Models)
	}
}

func TestOpenCodeUsageDirResolvesRelative(t *testing.T) {
	wd, _ := os.Getwd()
	if got, want := openCodeUsageDir("sub"), filepath.Join(wd, "sub"); got != want {
		t.Fatalf("openCodeUsageDir(sub) = %q, want %q", got, want)
	}
}
