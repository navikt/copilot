package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// fakeEndpoint answers /v1/models and chat completions the way a working
// Ollama does. keep caps the prompt tokens it reports, 0 for no cap.
func fakeEndpoint(t *testing.T, keep int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"data":[{"id":"qwen3.6:35b"}]}`))
			return
		}
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
			Tools    []any                      `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		prompt := len(req.Messages[0].Content) / 4
		if keep > 0 && prompt > keep {
			prompt = keep
		}
		msg := map[string]any{"content": "A"}
		if len(req.Tools) > 0 {
			msg["tool_calls"] = []any{map[string]any{"function": map[string]any{"name": "record_answer"}}}
		}
		top := make([]map[string]any, 11)
		for i := range top {
			top[i] = map[string]any{"token": string(rune('A' + i)), "logprob": -1.0 - float64(i)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": msg, "logprobs": map[string]any{"content": []any{map[string]any{"top_logprobs": top}}}}},
			"usage":   map[string]any{"prompt_tokens": prompt},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setEndpointConfig(t *testing.T, url string) {
	t.Helper()
	for k, v := range map[string]string{"local_endpoint": url + "/v1", "local_endpoint_model": "qwen3.6:35b"} {
		if _, err := writeConfigKey(k, v); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { local.SetEndpoint("", ""); local.SetActive(nil); local.SetSelectedModel("") })
}

// The endpoint path must not reach the macOS gates: the wired-memory check
// runs /usr/sbin/sysctl, which Linux does not have, and init would fail there
// on a machine that needs none of it.
func TestEndpointInitSkipsTheMacOSChecks(t *testing.T) {
	localTestHome(t)
	orig := checkWiredLimit
	checkWiredLimit = func(local.Model) (local.WiredLimit, error) {
		t.Fatal("init checked the wired-memory limit for an endpoint")
		return local.WiredLimit{}, nil
	}
	t.Cleanup(func() { checkWiredLimit = orig })
	setEndpointConfig(t, fakeEndpoint(t, 0).URL)

	out := captureStdout(func() {
		if err := cmdLocalInit([]string{"--yes"}); err != nil {
			t.Errorf("init: %v", err)
		}
	})
	if !strings.Contains(out, "Local dispatch is on") {
		t.Errorf("init did not turn dispatch on:\n%s", out)
	}
	if local.Installed() {
		t.Error("init provisioned an environment for an endpoint")
	}

	applyLocalConfig()
	base, model := local.Endpoint()
	if !local.Enabled() || !local.IsLocal("qwen3.6:35b") || model != "qwen3.6:35b" || !strings.HasPrefix(base, "http://127.0.0.1:") {
		t.Errorf("after init: enabled=%v IsLocal=%v endpoint=%s %s", local.Enabled(), local.IsLocal("qwen3.6:35b"), base, model)
	}
	if e, _ := local.Lookup("qwen3.6:35b"); e.Capabilities != nil {
		t.Error("an endpoint model has capability verdicts")
	}
}

// Ollama's default num_ctx cuts a 30k prompt to 4k and reports what it kept:
// doctor fails it, and init leaves dispatch off.
func TestEndpointInitRefusesATruncatingServer(t *testing.T) {
	localTestHome(t)
	setEndpointConfig(t, fakeEndpoint(t, 4096).URL)
	out := captureStdout(func() {
		if err := cmdLocalInit(nil); err == nil {
			t.Error("init turned dispatch on over a truncating server")
		}
	})
	if !strings.Contains(out, "FAIL  context") || !strings.Contains(out, "OLLAMA_CONTEXT_LENGTH") {
		t.Errorf("doctor output:\n%s", out)
	}
	if cfg, _ := readConfig(); cfg.LocalEnabled != nil && *cfg.LocalEnabled {
		t.Error("local_enabled was written")
	}
}

// A config edited by hand past `config set` must leave dispatch off.
func TestApplyEndpointConfigFailsClosed(t *testing.T) {
	localTestHome(t)
	if err := updateConfigKey("local_endpoint", `"http://8.8.8.8:11434/v1"`); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"local_endpoint_model": "qwen3.6:35b", "local_enabled": "true"} {
		if _, err := writeConfigKey(k, v); err != nil {
			t.Fatal(err)
		}
	}
	errOut := captureStderr(applyLocalConfig)
	if base, _ := local.Endpoint(); local.Enabled() || base != "" {
		t.Errorf("a public endpoint armed dispatch: enabled=%v base=%q", local.Enabled(), base)
	}
	if !strings.Contains(errOut, "Local dispatch is off this run") {
		t.Errorf("stderr = %q, want it said", errOut)
	}
	if _, err := writeConfigKey("local_endpoint", "http://8.8.8.8/v1"); err == nil {
		t.Error("config set accepted a public endpoint")
	}
}

// With local_endpoint set but dispatch off, decide must not fall back to a
// managed server that is still recorded: it refuses before asking anything.
func TestDecideDoesNotFallBackFromAnEndpoint(t *testing.T) {
	localTestHome(t)
	setEndpointConfig(t, "http://127.0.0.1:1")
	applyLocalConfig()
	orig := decideServer
	decideServer = func(context.Context) (string, string, func(), error) {
		t.Fatal("decide asked a server while local_endpoint was set but off")
		return "", "", nil, nil
	}
	t.Cleanup(func() { decideServer = orig })
	_, err := decide(context.Background(), "q", []string{"yes", "no"}, "", false)
	if err == nil || !strings.Contains(err.Error(), "local dispatch is off") {
		t.Fatalf("decide = %v", err)
	}
}
