package artifacts

import (
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
