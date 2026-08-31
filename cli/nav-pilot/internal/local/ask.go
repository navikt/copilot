package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// askMaxTokens is room for a real answer plus the thinking that precedes it.
// The server default of 512 is not: this model spends all of it reasoning.
const askMaxTokens = 4096

// Ask puts one question to the running local server and returns the answer
// with the token counts.
//
// It takes the same machine-wide lock the guard takes, for the same reason:
// two concurrent requests wedge mlx-lm, and a developer asking a question
// while a session is running is precisely that case. Waiting behind a session
// is correct here; answering while wedging the server for everyone is not.
//
// No streaming, so the usage block comes back in the response rather than only
// in a final frame the caller has to ask for.
func Ask(ctx context.Context, prompt string) (answer string, in, out int64, err error) {
	st, ok, err := LoadState()
	if err != nil {
		return "", 0, 0, err
	}
	if !ok {
		return "", 0, 0, fmt.Errorf("no local server is running. Start it: nav-pilot alpha local start")
	}

	body, err := json.Marshal(map[string]any{
		"model":    st.Model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"stream":   false,
		// The server's default cap is 512 tokens, and this is a thinking model:
		// at 512 it spends the whole budget reasoning and returns a message with
		// no content at all, finish_reason "length". Asked for five words, it
		// answered nothing. The clients set their own budget, so only this
		// command has to.
		"max_tokens": askMaxTokens,
	})
	if err != nil {
		return "", 0, 0, err
	}

	release, err := lockServer(ctx)
	if err != nil {
		return "", 0, 0, fmt.Errorf("the local server is busy with another session: %w", err)
	}
	defer release()

	// Prove the recorded server is still ours before sending a prompt to it.
	// The guard does this on every completion it forwards; this path bypassed
	// the guard entirely and trusted the port in the state file, so a server
	// that died and left its port to whatever bound next would have been handed
	// the developer's question. Under the lock, so the answer cannot go stale
	// between the check and the request.
	if err := EnsureOwnServer(); err != nil {
		return "", 0, 0, err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", st.ServerPort())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	// No client timeout. A local model at a large context legitimately takes
	// minutes for one answer, and cutting it off at some round number would
	// report a failure that did not happen. The context is the caller's to
	// cancel.
	started := time.Now()
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("the local server did not answer: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", 0, 0, fmt.Errorf("reading the answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, fmt.Errorf("the local server answered %s: %s", resp.Status, firstLine(raw))
	}

	var parsed struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
				// Where a thinking model puts its working. Read only so an
				// empty answer can be explained rather than printed as a blank.
				Reasoning string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, 0, fmt.Errorf("could not read the answer as JSON: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("the local server returned no answer at all")
	}

	// Counted like any other completion, because it is one: it occupied the
	// GPU and generated tokens that would otherwise have been billed.
	RecordCompletion(parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, time.Since(started).Seconds())

	c := parsed.Choices[0]
	answer = c.Message.Content
	if strings.TrimSpace(answer) == "" {
		// Thought until it ran out of room. Saying that is more use than
		// printing nothing, which reads as a broken server.
		switch {
		case c.FinishReason == "length" && strings.TrimSpace(c.Message.Reasoning) != "":
			answer = fmt.Sprintf("(the model used all %d tokens thinking and never got to an answer. "+
				"Ask something narrower, or the same thing more directly.)", parsed.Usage.CompletionTokens)
		case strings.TrimSpace(c.Message.Reasoning) != "":
			answer = strings.TrimSpace(c.Message.Reasoning)
		default:
			answer = "(the model returned an empty answer)"
		}
	}
	return answer, parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, nil
}

func firstLine(b []byte) string {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[:i]
	}
	if len(b) > 200 {
		b = b[:200]
	}
	return string(b)
}
