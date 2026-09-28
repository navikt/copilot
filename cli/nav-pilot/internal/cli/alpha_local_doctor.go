package cli

// `nav-pilot alpha local doctor`: what a server nav-pilot did not start can
// actually do. A 4k context or a template that mangles tool calls looks
// exactly like a weak model, so each check says pass, warn or fail and, when
// it is not a pass, the exact fix.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

type checkLevel string

const (
	levelPass checkLevel = "PASS"
	levelWarn checkLevel = "WARN"
	levelFail checkLevel = "FAIL"
	levelSkip checkLevel = "SKIP"
)

type doctorCheck struct {
	Name   string
	Level  checkLevel
	Detail string
	Fix    string
}

// doctorContextTokens is roughly what the context probe sends: more than the
// 21.7k tokens of a Copilot session's static context, which is what a 4k
// window silently cuts. doctorContextMin is the least usage.prompt_tokens a
// server that kept all of it reports: the filler is about 10 tokens a sentence
// in the tokenizers we know, and the margin is for one that merges a word or
// two, not for a window that cuts a few thousand tokens off.
const (
	doctorFillerSentence = "The quick brown fox jumps over the lazy dog. "
	doctorFillerRepeats  = 3000 // about 30k tokens
	doctorContextMin     = 26000
	doctorSlowTTFT       = 60 * time.Second
)

const (
	fixStartServer = "Start your server (ollama serve, or llama-server --jinja -c 65536 -m <model.gguf>), check local_endpoint with nav-pilot config get local_endpoint, then run nav-pilot alpha local doctor again"
	fixTools       = "Ollama: use a library model such as qwen3.6:35b (ollama pull qwen3.6:35b); a hf.co/... pull has no tool-call parser. llama-server: start it with --jinja"
	fixLogprobs    = "Ollama returns logprobs from v0.12.11 (ollama --version); llama-server and vLLM return them; LM Studio's chat endpoint does not. Without them alpha decide refuses this server; the local worker still works"
	fixContext     = "Ollama: start it with OLLAMA_CONTEXT_LENGTH=65536 ollama serve, or make a model with a Modelfile holding FROM <model> and PARAMETER num_ctx 65536 (ollama create <name> -f Modelfile) and set local_endpoint_model to <name>. llama-server: -c 65536. LM Studio: raise the context length when you load the model. On a GPU with 8 GB or less, 65536 may not fit: " + smallMemoryURL
	fixSmallMemory = "Use a context that fits in memory, such as 16384 on a GPU with 8 GB or less, and keep part of the model in RAM. Ollama: OLLAMA_CONTEXT_LENGTH=16384 ollama serve; Ollama splits the layers between GPU and RAM on its own. llama-server: -c 16384, with --n-cpu-moe 999 for a MoE model such as Qwen3.6-35B-A3B, or -ngl <n> to put only n layers on the GPU. A context under 30k tokens fails this check; nav-pilot alpha local setup offers to save it anyway. More: " + smallMemoryURL
	fixServerGone  = "Start the server again; on Linux, dmesg or the cgroup's memory.events shows whether the OOM killer stopped it. " + fixSmallMemory
	smallMemoryURL = "https://ki-utvikling.nav.no/nav-pilot/lokal/egen-server#lite-minne"
	fixSlow        = "A Copilot session's first turn is about 22k tokens. A GPU with the whole model in memory, or llama-server with --n-cpu-moe on a small GPU, prefills faster; opencode's local worker and alpha decide send far less"
)

// doctorTimeout bounds each probe. The context probe gets minutes: on a
// CPU-only laptop a 30k-token prefill is exactly that, and cutting it short
// would report a failure that is only slowness.
var doctorTimeout = map[string]time.Duration{
	"models":   5 * time.Second,
	"tools":    2 * time.Minute,
	"logprobs": time.Minute,
	"context":  10 * time.Minute,
}

func cmdLocalDoctor() error {
	base, model, set, err := configuredEndpoint()
	if !set {
		return fmt.Errorf("doctor checks your own server, and local_endpoint is not set.\n\n  Set it:\n\n    %s\n    %s",
			bold("nav-pilot config set local_endpoint http://127.0.0.1:11434/v1"),
			bold("nav-pilot config set local_endpoint_model qwen3.6:35b"))
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local doctor"), dim("(alpha, unsupported, unmeasured)"))
	checks := runDoctor(context.Background(), base, model)
	if slices.ContainsFunc(checks, func(c doctorCheck) bool { return c.Level == levelFail }) {
		return &exitCode{code: 1}
	}
	return nil
}

// runDoctor runs every check against base, printing each result as it comes:
// the context probe can take minutes, and a screen that stays blank that long
// reads as a hang.
func runDoctor(ctx context.Context, base, model string) []doctorCheck {
	fmt.Printf("  Endpoint  %s\n  Model     %s %s\n\n", base+"/v1", bold(model), dim("(unsupported, unmeasured)"))
	var checks []doctorCheck
	report := func(c doctorCheck) {
		checks = append(checks, c)
		fmt.Printf("  %s  %-10s %s\n", levelColour(c.Level), c.Name, wrapIndent(c.Detail, "                   ", 60))
		if c.Fix != "" {
			fmt.Printf("        %s %s\n", dim("Fix:"), wrapIndent(c.Fix, "             ", 72))
		}
	}
	reachable := checkModels(ctx, base, model)
	report(reachable)
	if reachable.Level == levelFail {
		for _, name := range []string{"tool calls", "logprobs", "context", "TTFT"} {
			report(doctorCheck{Name: name, Level: levelSkip, Detail: "the server did not answer"})
		}
		fmt.Println()
		return checks
	}
	report(checkTools(ctx, base, model))
	report(checkLogprobs(ctx, base, model))
	fmt.Fprintf(os.Stderr, "  %s\n", dim("→ Sending about 30k tokens to measure the context window and the time to first token. On a CPU this takes minutes."))
	ctxCheck, ttft := checkContext(ctx, base, model)
	report(ctxCheck)
	report(ttft)
	fmt.Println()
	return checks
}

func levelColour(l checkLevel) string {
	switch l {
	case levelPass:
		return green(string(l))
	case levelWarn:
		return yellow(string(l))
	case levelFail:
		return red(string(l))
	}
	return dim(string(l))
}

// sameModel is whether a listed id names the model. Ollama lists name:latest
// for a model pulled or created without a tag, and answers to either name.
// Only that direction: another server that lists x may not answer to x:latest.
func sameModel(listed, model string) bool {
	return listed == model || listed == model+":latest"
}

func checkModels(ctx context.Context, base, model string) doctorCheck {
	c := doctorCheck{Name: "server"}
	ctx, cancel := context.WithTimeout(ctx, doctorTimeout["models"])
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		c.Level, c.Detail = levelFail, err.Error()
		return c
	}
	resp, err := local.ServerClient.Do(req)
	if err != nil {
		c.Level, c.Detail, c.Fix = levelFail, "no answer: "+err.Error(), fixStartServer
		return c
	}
	defer resp.Body.Close()
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &list) != nil {
		c.Level, c.Detail, c.Fix = levelFail, fmt.Sprintf("GET /v1/models answered %s, not an OpenAI model list", resp.Status),
			"local_endpoint must be the server's OpenAI-compatible base URL, such as http://127.0.0.1:11434/v1"
		return c
	}
	var ids []string
	for _, m := range list.Data {
		ids = append(ids, m.ID)
	}
	if !slices.ContainsFunc(ids, func(id string) bool { return sameModel(id, model) }) {
		c.Level, c.Detail = levelWarn, fmt.Sprintf("answers, but does not list %s (it lists: %s)", model, strings.Join(ids, ", "))
		c.Fix = "Set one it lists: nav-pilot config set local_endpoint_model <id>. On Ollama you can also pull it: ollama pull " + model +
			". llama-server answers with the model it loaded whatever the name, so there this is harmless"
		return c
	}
	c.Level, c.Detail = levelPass, "answers and lists "+model
	return c
}

// chat posts one non-streamed chat completion with thinking off, the way
// decide does, and decodes the parts the checks read.
type chatAnswer struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Logprobs *struct {
			Content []struct {
				TopLogprobs []json.RawMessage `json:"top_logprobs"`
			} `json:"content"`
		} `json:"logprobs"`
	} `json:"choices"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
	} `json:"usage"`
}

func chat(ctx context.Context, base, model, probe string, body map[string]any) (chatAnswer, error) {
	ctx, cancel := context.WithTimeout(ctx, doctorTimeout[probe])
	defer cancel()
	body["model"] = model
	body["stream"] = false
	body["temperature"] = 0
	// Thinking off both ways: llama-server and mlx-lm read the template
	// switch, Ollama maps reasoning_effort "none" to think=false.
	body["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	body["reasoning_effort"] = "none"
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return chatAnswer{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := local.ServerClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return chatAnswer{}, fmt.Errorf("no answer within %s", doctorTimeout[probe])
		}
		return chatAnswer{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return chatAnswer{}, fmt.Errorf("the server answered %s: %s", resp.Status, strings.TrimSpace(firstLineOf(raw)))
	}
	var a chatAnswer
	if err := json.Unmarshal(raw, &a); err != nil || len(a.Choices) == 0 {
		return chatAnswer{}, fmt.Errorf("the answer is not an OpenAI chat completion")
	}
	return a, nil
}

func firstLineOf(b []byte) string {
	s, _, _ := strings.Cut(string(b), "\n")
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func checkTools(ctx context.Context, base, model string) doctorCheck {
	c := doctorCheck{Name: "tool calls"}
	a, err := chat(ctx, base, model, "tools", map[string]any{
		"max_tokens": 256,
		"messages": []map[string]string{{"role": "user",
			"content": `Call the record_answer tool with answer set to "ok". Do not reply in text.`}},
		"tools": []any{map[string]any{"type": "function", "function": map[string]any{
			"name":        "record_answer",
			"description": "Record an answer.",
			"parameters": map[string]any{"type": "object",
				"properties": map[string]any{"answer": map[string]any{"type": "string"}},
				"required":   []string{"answer"}},
		}}},
	})
	switch {
	case err != nil:
		c.Level, c.Detail, c.Fix = levelFail, err.Error(), fixTools
	case len(a.Choices[0].Message.ToolCalls) > 0 && a.Choices[0].Message.ToolCalls[0].Function.Name == "record_answer":
		c.Level, c.Detail = levelPass, "a parsed tool_calls entry came back"
	case len(a.Choices[0].Message.ToolCalls) > 0:
		c.Level, c.Detail, c.Fix = levelFail, "a tool call came back, but not to the one tool offered", fixTools
	case strings.Contains(a.Choices[0].Message.Content, "record_answer"):
		c.Level, c.Detail, c.Fix = levelFail, "the call came back as text, not as a parsed tool_calls entry", fixTools
	default:
		c.Level, c.Detail, c.Fix = levelFail, "no tool call came back", fixTools
	}
	return c
}

func checkLogprobs(ctx context.Context, base, model string) doctorCheck {
	c := doctorCheck{Name: "logprobs"}
	a, err := chat(ctx, base, model, "logprobs", map[string]any{
		"max_tokens":   1,
		"logprobs":     true,
		"top_logprobs": decideTopLogprobs,
		"messages":     []map[string]string{{"role": "user", "content": "Answer with the single letter A or B."}},
	})
	n := 0
	if err == nil && a.Choices[0].Logprobs != nil && len(a.Choices[0].Logprobs.Content) > 0 {
		n = len(a.Choices[0].Logprobs.Content[0].TopLogprobs)
	}
	switch {
	case err != nil:
		c.Level, c.Detail, c.Fix = levelWarn, "alpha decide is unavailable: "+err.Error(), fixLogprobs
	case n == 0:
		c.Level, c.Detail, c.Fix = levelWarn, "none came back, so alpha decide is unavailable", fixLogprobs
	case n < decideTopLogprobs:
		c.Level, c.Detail = levelWarn, fmt.Sprintf("%d top_logprobs came back, not %d: alpha decide sees fewer alternatives", n, decideTopLogprobs)
	default:
		c.Level, c.Detail = levelPass, fmt.Sprintf("%d top_logprobs came back, enough for alpha decide", n)
	}
	return c
}

// checkContext sends about 30k tokens and reads usage.prompt_tokens back: a
// server with a small window cuts the prompt and reports what it kept.
// Ollama's default is as low as 4k and /v1 cannot raise it. The same request
// measures the time to first token, since with max_tokens 1 the wait is the
// prefill. A nonce first, so a warm prefix cache cannot answer for it.
func checkContext(ctx context.Context, base, model string) (doctorCheck, doctorCheck) {
	c := doctorCheck{Name: "context"}
	t := doctorCheck{Name: "TTFT"}
	prompt := fmt.Sprintf("Run %d. ", time.Now().UnixNano()) + strings.Repeat(doctorFillerSentence, doctorFillerRepeats) +
		"\nAnswer with the single word ok."
	started := time.Now()
	a, err := chat(ctx, base, model, "context", map[string]any{
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	})
	took := time.Since(started)
	switch {
	case err != nil && checkModels(ctx, base, model).Fix == fixStartServer:
		// It answered the checks before this one: a server that stops
		// answering while it loads a long context most likely ran out of
		// memory, and a larger context would only make that worse.
		c.Level, c.Detail, c.Fix = levelFail, "the server stopped answering during the 30k-token prompt, most likely out of memory: "+err.Error(), fixServerGone
		t.Level, t.Detail = levelSkip, "the context probe failed"
		return c, t
	case err != nil:
		c.Level, c.Detail, c.Fix = levelFail, "a 30k-token prompt failed: "+err.Error(), fixContext
		// Ollama: "model requires more system memory"; llama.cpp: "failed
		// to allocate", "out of memory". A larger context is not the fix.
		if low := strings.ToLower(err.Error()); strings.Contains(low, "memory") || strings.Contains(low, "alloc") {
			c.Fix = fixSmallMemory
		}
		t.Level, t.Detail = levelSkip, "the context probe failed"
		return c, t
	case a.Usage.PromptTokens == 0:
		c.Level, c.Detail = levelWarn, "the server reported no prompt token count, so a cut prompt cannot be ruled out"
		c.Fix = fixContext
	case a.Usage.PromptTokens < doctorContextMin:
		c.Level, c.Detail = levelFail, fmt.Sprintf("about 30k tokens went in and %d were kept: the server cuts prompts to its context window, which breaks a Copilot session without saying so", a.Usage.PromptTokens)
		c.Fix = fixContext
	default:
		c.Level, c.Detail = levelPass, fmt.Sprintf("%d prompt tokens kept, nothing cut", a.Usage.PromptTokens)
	}
	t.Detail = fmt.Sprintf("%.1f s for about 30k tokens", took.Seconds())
	t.Level = levelPass
	if took > doctorSlowTTFT {
		t.Level, t.Fix = levelWarn, fixSlow
	}
	return c, t
}

// ─── init and status for local_endpoint ─────────────────────────────────────

// endpointInit is init when local_endpoint is set: nothing to download or
// start, so it checks the server and turns dispatch on when nothing failed.
// Nothing about the machine is checked either: the memory, the platform and
// the weights are the server's business.
func endpointInit() error {
	base, model, _, err := configuredEndpoint()
	if err != nil {
		return err
	}
	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local init"), dim("(alpha, unsupported, unmeasured)"))
	fmt.Printf("  %s\n\n", dim("local_endpoint is set: nav-pilot downloads nothing and starts nothing, and checks your server instead."))
	checks := runDoctor(context.Background(), base, model)
	if slices.ContainsFunc(checks, func(c doctorCheck) bool { return c.Level == levelFail }) {
		return &exitCode{code: 1, err: fmt.Errorf("local dispatch was not turned on. Fix the FAIL lines above, then run %s again", bold("nav-pilot alpha local init"))}
	}
	if _, err := writeConfigKey("local_enabled", "true"); err != nil {
		return err
	}
	fmt.Printf("%s Local dispatch is on, to %s.\n", green("✓"), bold(model))
	fmt.Printf("  %s nav-pilot --client opencode hands scoped tasks to it as local-worker\n", green("✓"))
	decideOK := !slices.ContainsFunc(checks, func(c doctorCheck) bool { return c.Name == "logprobs" && c.Level != levelPass })
	if decideOK {
		fmt.Printf("  %s nav-pilot alpha decide answers from it\n", green("✓"))
	} else {
		fmt.Printf("  %s nav-pilot alpha decide does not: see logprobs above\n", yellow("⚠"))
	}
	fmt.Printf("  %s\n\n", dim("Unmeasured: nav-pilot has benchmarked no model on this server, so routing treats it as unknown."))
	return nil
}

// endpointStatus is status when local_endpoint is set: what is configured and
// whether it answers. One cheap probe; doctor is the full check.
func endpointStatus() error {
	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local status"), dim("(alpha, unsupported, unmeasured)"))
	base, model, _, err := configuredEndpoint()
	if err != nil {
		fmt.Printf("  Endpoint     %s %v\n\n", red("unusable:"), err)
		return nil
	}
	cfg, _ := readConfig()
	enabled := cfg != nil && cfg.LocalEnabled != nil && *cfg.LocalEnabled
	fmt.Printf("  Endpoint     %s %s\n", base+"/v1", dim("(local_endpoint, your own server)"))
	fmt.Printf("  Model        %s %s\n", bold(model), dim("(unsupported, unmeasured)"))
	fmt.Printf("  Dispatch     %s\n", enabledLabel(enabled))
	if c := checkModels(context.Background(), base, model); c.Level == levelFail {
		fmt.Printf("  Server       %s %s\n", red("not answering"), dim(c.Detail))
	} else {
		fmt.Printf("  Server       %s\n", green("answering"))
	}
	if st, ok, _ := local.LoadState(); ok && local.Attach(st).Status().Health != local.HealthCrashed {
		fmt.Printf("  Managed      %s the mlx server nav-pilot started is still running (pid %d). Stop it: %s\n",
			yellow("⚠"), st.PID, bold("nav-pilot alpha local stop"))
	}
	fmt.Printf("\n  Full check: %s\n\n", bold("nav-pilot alpha local doctor"))
	return nil
}
