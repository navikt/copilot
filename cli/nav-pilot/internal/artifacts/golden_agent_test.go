package artifacts

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// Golden materialization test (C3).
//
// transformAgent's output is the exact bytes that land in
// ~/.config/opencode/agents/<name>.md on every opencode launch, so this pins
// them byte for byte across the primary/subagent split. The data-driven
// persona refactor (C1/C2) must keep these identical; if a case here has to
// change, a real materialized file changed and that is a regression.

func TestGoldenTransformAgent(t *testing.T) {
	const input = `---
name: %NAME%
description: Plan and build Nav applications
tools:
  - execute
  - read
---

You are %NAME%.
`

	tests := []struct {
		name  string
		agent string
		want  string
	}{
		{
			name:  "nav-pilot is primary",
			agent: "nav-pilot",
			want: `---
description: Plan and build Nav applications
mode: primary
---


You are nav-pilot.
`,
		},
		{
			name:  "nav-pilot-opus is primary",
			agent: "nav-pilot-opus",
			want: `---
description: Plan and build Nav applications
mode: primary
---


You are nav-pilot-opus.
`,
		},
		{
			name:  "every other agent is a subagent",
			agent: "aksel-ekspert",
			want: `---
description: Plan and build Nav applications
mode: subagent
---


You are aksel-ekspert.
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(strings.ReplaceAll(input, "%NAME%", tt.agent))
			got := string(transformAgent(src, tt.agent))
			if got != tt.want {
				t.Errorf("transformAgent(%q)\n got: %q\nwant: %q", tt.agent, got, tt.want)
			}
		})
	}
}

// TestGoldenTransformAgentModel pins the model line materialization writes.
//
// The cases above carry no model: key and their bytes are unchanged by this,
// which is the proof that agents without a declared model still materialize
// exactly as they did. These cases pin the new half: a Nav display name becomes
// a provider-qualified opencode id, and a name nobody recognises becomes no
// model line at all rather than an invented id.
func TestGoldenTransformAgentModel(t *testing.T) {
	const input = `---
name: %NAME%
description: Plan and build Nav applications
model: %MODEL%
tools:
  - execute
  - read
---

You are %NAME%.
`

	tests := []struct {
		name  string
		agent string
		model string
		want  string
	}{
		{
			name:  "display name maps to the provider-qualified id",
			agent: "aksel",
			model: "Claude Sonnet 4.6",
			want: `---
description: Plan and build Nav applications
mode: subagent
model: github-copilot/claude-sonnet-4.6
---


You are aksel.
`,
		},
		{
			name:  "a hyphenated display name maps too",
			agent: "code-review",
			model: "GPT-5.3-Codex",
			want: `---
description: Plan and build Nav applications
mode: subagent
model: github-copilot/gpt-5.3-codex
---


You are code-review.
`,
		},
		{
			name:  "a primary agent keeps its model",
			agent: "nav-pilot-opus",
			model: "Claude Opus 4.6",
			want: `---
description: Plan and build Nav applications
mode: primary
model: github-copilot/claude-opus-4.6
---


You are nav-pilot-opus.
`,
		},
		{
			name:  "an unknown name writes no model line",
			agent: "aksel",
			model: "Claude Sonnet 9000",
			want: `---
description: Plan and build Nav applications
mode: subagent
---


You are aksel.
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := strings.ReplaceAll(input, "%NAME%", tt.agent)
			src = strings.ReplaceAll(src, "%MODEL%", tt.model)
			var got string
			stderr := captureStderr(t, func() {
				got = string(transformAgent([]byte(src), tt.agent))
			})
			if got != tt.want {
				t.Errorf("transformAgent(%q, %q)\n got: %q\nwant: %q", tt.agent, tt.model, got, tt.want)
			}
			// An unmappable name must not pass in silence: the only place a
			// maintainer can notice is the sync that materialized it.
			warned := strings.Contains(stderr, tt.model)
			wantWarn := !strings.Contains(tt.want, "model:")
			if warned != wantWarn {
				t.Errorf("stderr warning = %v, want %v (stderr: %q)", warned, wantWarn, stderr)
			}
		})
	}
}

// captureStderr redirects os.Stderr to a pipe for the duration of fn, then
// returns everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}
