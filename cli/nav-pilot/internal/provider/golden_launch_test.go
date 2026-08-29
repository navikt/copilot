package provider

import (
	"os"
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Golden launch-argument tests (C3).
//
// These pin the exact argument vectors nav-pilot passes to the clients today,
// byte for byte. They exist so the data-driven persona refactor (C1/C2) is
// provably a no-op for every existing user: if any of these vectors changes,
// some real launch changed, and the change is a regression until proven
// otherwise. They must never be "updated to match" a refactor.

func TestGoldenBuildCopilotArgs(t *testing.T) {
	tests := []struct {
		name     string
		cliName  string
		resolved domain.ResolvedConfig
		want     []string
	}{
		{
			name:    "cplt/zero config",
			cliName: "cplt",
			// AskUser is false in a zero ResolvedConfig, hence --no-ask-user.
			resolved: domain.ResolvedConfig{},
			want:     []string{"--agent", "copilot", "--", "--agent", "nav-pilot", "--no-ask-user"},
		},
		{
			name:     "cplt/ask user",
			cliName:  "cplt",
			resolved: domain.ResolvedConfig{AskUser: true},
			want:     []string{"--agent", "copilot", "--", "--agent", "nav-pilot"},
		},
		{
			name:    "cplt/every field set",
			cliName: "cplt",
			resolved: domain.ResolvedConfig{
				Client:          "copilot",
				Model:           "claude-opus-5",
				Mode:            "autopilot",
				ReasoningEffort: "high",
				ContextTier:     "large",
				AllowAllTools:   true,
				AskUser:         true,
				LogLevel:        "debug",
				ExtraArgs:       []string{"-p", "hei"},
			},
			want: []string{
				"--agent", "copilot", "--",
				"--agent", "nav-pilot",
				"--model", "claude-opus-5",
				"--mode", "autopilot",
				"--effort", "high",
				"--context", "large",
				"--allow-all-tools",
				"--log-level", "debug",
				"-p", "hei",
			},
		},
		{
			name:    "cplt/default mode and context tier are omitted",
			cliName: "cplt",
			resolved: domain.ResolvedConfig{
				Mode:        "default",
				ContextTier: "default",
				AskUser:     true,
			},
			want: []string{"--agent", "copilot", "--", "--agent", "nav-pilot"},
		},
		{
			name:     "copilot/zero config",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{},
			want:     []string{"--agent", "nav-pilot", "--no-ask-user"},
		},
		{
			name:    "copilot/every field set",
			cliName: "copilot",
			resolved: domain.ResolvedConfig{
				Model:           "gpt-5.5",
				Mode:            "plan",
				ReasoningEffort: "low",
				ContextTier:     "small",
				AllowAllTools:   true,
				AskUser:         false,
				LogLevel:        "error",
				ExtraArgs:       []string{"--foo"},
			},
			want: []string{
				"--agent", "nav-pilot",
				"--model", "gpt-5.5",
				"--mode", "plan",
				"--effort", "low",
				"--context", "small",
				"--allow-all-tools",
				"--no-ask-user",
				"--log-level", "error",
				"--foo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCopilotArgs(tt.cliName, tt.resolved)
			if !slices.Equal(got, tt.want) {
				t.Errorf("BuildCopilotArgs(%q, %+v)\n got: %q\nwant: %q", tt.cliName, tt.resolved, got, tt.want)
			}
		})
	}
}

func TestGoldenOpenCodeArgs(t *testing.T) {
	tests := []struct {
		name     string
		resolved domain.ResolvedConfig
		want     []string
	}{
		{
			name:     "zero config",
			resolved: domain.ResolvedConfig{},
			want:     []string{"--model", "github-copilot/auto", "--agent", "nav-pilot"},
		},
		{
			name:     "explicit auto model",
			resolved: domain.ResolvedConfig{Model: "auto"},
			want:     []string{"--model", "github-copilot/auto", "--agent", "nav-pilot"},
		},
		{
			name:     "bare copilot model id gains the provider prefix",
			resolved: domain.ResolvedConfig{Model: "claude-sonnet-4.6"},
			want:     []string{"--model", "github-copilot/claude-sonnet-4.6", "--agent", "nav-pilot"},
		},
		{
			name:     "provider-qualified model passes through",
			resolved: domain.ResolvedConfig{Model: "anthropic/claude-opus-5"},
			want:     []string{"--model", "anthropic/claude-opus-5", "--agent", "nav-pilot"},
		},
		{
			name:     "plan mode selects opencode's built-in plan agent",
			resolved: domain.ResolvedConfig{Mode: "plan"},
			want:     []string{"--model", "github-copilot/auto", "--agent", "plan"},
		},
		{
			name: "every field set",
			resolved: domain.ResolvedConfig{
				Model:           "claude-opus-5",
				Mode:            "autopilot",
				ReasoningEffort: "high",
				ContextTier:     "large",
				AllowAllTools:   true,
				AskUser:         true,
				LogLevel:        "debug",
				ExtraArgs:       []string{"--ignored"},
			},
			want: []string{
				"--model", "github-copilot/claude-opus-5",
				"--agent", "nav-pilot",
				"--variant", "high",
				"--dangerously-skip-permissions",
				"--log-level", "DEBUG",
			},
		},
		{
			name:     "warning log level maps to WARN",
			resolved: domain.ResolvedConfig{LogLevel: "warning"},
			want:     []string{"--model", "github-copilot/auto", "--agent", "nav-pilot", "--log-level", "WARN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OpenCodeArgs(tt.resolved)
			if !slices.Equal(got, tt.want) {
				t.Errorf("OpenCodeArgs(%+v)\n got: %q\nwant: %q", tt.resolved, got, tt.want)
			}
		})
	}
}

// TestGoldenOpenCodeLaunchAgent pins the cplt agent opencode is launched under,
// alongside the client args above: together they are the full opencode
// invocation LaunchOpenCode builds.
func TestGoldenOpenCodeLaunchAgent(t *testing.T) {
	if got := (openCodeProvider{}).DefaultModel(); got != "github-copilot/auto" {
		t.Errorf("opencode DefaultModel() = %q, want %q", got, "github-copilot/auto")
	}
	if got := ToOpenCodeModel(""); got != "github-copilot/auto" {
		t.Errorf("ToOpenCodeModel(\"\") = %q, want %q", got, "github-copilot/auto")
	}
}

// ─── the non-interactive launch path ─────────────────────────────────────────
//
// cplt asks for confirmation before it starts an agent and refuses to guess
// when nobody can answer: "No TTY available for confirmation. Use --yes for
// non-interactive runs.", exit 1. Every nav-pilot launch without a terminal on
// stdin died there. The tests below pin both halves of the fix: the vectors
// gain exactly --yes when there is no terminal, and are byte-identical to the
// golden ones above when there is.

// TestGoldenCpltArgvNonInteractive pins the non-interactive vector for every
// launch that goes through cpltArgv — opencode, pi, and both staged clients.
func TestGoldenCpltArgvNonInteractive(t *testing.T) {
	spec := cpltLaunch{agent: "opencode", agentArgs: OpenCodeArgs(domain.ResolvedConfig{})}
	interactive := []string{"--agent", "opencode", "--", "--model", "github-copilot/auto", "--agent", "nav-pilot"}

	if got := withCpltConfirmation(cpltArgv(spec), true); !slices.Equal(got, interactive) {
		t.Errorf("with a terminal the vector must not change\n got: %q\nwant: %q", got, interactive)
	}
	want := append([]string{"--yes"}, interactive...)
	if got := withCpltConfirmation(cpltArgv(spec), false); !slices.Equal(got, want) {
		t.Errorf("without a terminal\n got: %q\nwant: %q", got, want)
	}

	// --yes is a cplt flag, so it belongs ahead of the whole cplt vector — in
	// front of --no-audit, never after the "--" where the client would read it.
	staged := cpltLaunch{
		agent:     "opencode",
		noAudit:   true,
		cpltArgs:  []string{"--allow-read", "/staged/x"},
		agentArgs: []string{"--agent", "grillmester"},
	}
	wantStaged := []string{
		"--yes", "--no-audit",
		"--agent", "opencode",
		"--allow-read", "/staged/x",
		"--", "--agent", "grillmester",
	}
	if got := withCpltConfirmation(cpltArgv(staged), false); !slices.Equal(got, wantStaged) {
		t.Errorf("staged vector without a terminal\n got: %q\nwant: %q", got, wantStaged)
	}
}

// TestNonInteractiveLaunchAddsOnlyYes pins that --yes is the only thing a
// missing terminal adds.
//
// --quiet is the one that matters. The reference launcher splices --yes and
// --quiet together and nav-pilot's own version probe copies the pair, so the
// obvious fix here is to copy it a third time. It must not: --quiet suppresses
// cplt's post-session change audit as well as the startup summary, and the
// launch nobody is sitting in front of is the one whose audit report is worth
// the most. Nothing here may turn a sandbox restriction, guard, or report off.
func TestNonInteractiveLaunchAddsOnlyYes(t *testing.T) {
	for _, spec := range []cpltLaunch{
		{agent: "opencode", agentArgs: OpenCodeArgs(domain.ResolvedConfig{})},
		{agent: "pi"},
		{agent: "copilot", noAudit: true, cpltArgs: []string{"--allow-read", "/staged/x"}},
	} {
		before := cpltArgv(spec)
		after := withCpltConfirmation(before, false)
		if len(after) != len(before)+1 || after[0] != "--yes" {
			t.Fatalf("a missing terminal must add exactly one leading --yes\n got: %q\nwas: %q", after, before)
		}
		if !slices.Equal(after[1:], before) {
			t.Errorf("the rest of the vector must be untouched\n got: %q\nwant: %q", after[1:], before)
		}
	}
}

// TestGoldenCopilotLaunchArgsNonInteractive pins the copilot path, which builds
// its cplt vector itself instead of going through cpltArgv: it needs --yes on
// the same terms, and the plain copilot CLI must never see it.
func TestGoldenCopilotLaunchArgsNonInteractive(t *testing.T) {
	resolved := domain.ResolvedConfig{AskUser: true}

	interactive := []string{"--agent", "copilot", "--", "--agent", "nav-pilot"}
	if got := copilotLaunchArgs("cplt", resolved, true); !slices.Equal(got, interactive) {
		t.Errorf("cplt with a terminal\n got: %q\nwant: %q", got, interactive)
	}
	want := append([]string{"--yes"}, interactive...)
	if got := copilotLaunchArgs("cplt", resolved, false); !slices.Equal(got, want) {
		t.Errorf("cplt without a terminal\n got: %q\nwant: %q", got, want)
	}

	plain := []string{"--agent", "nav-pilot"}
	for _, tty := range []bool{true, false} {
		if got := copilotLaunchArgs("copilot", resolved, tty); !slices.Equal(got, plain) {
			t.Errorf("the plain copilot CLI must never be given cplt's --yes (tty=%v)\n got: %q\nwant: %q", tty, got, plain)
		}
	}
}

// TestIsTerminalRejectsDevNull pins the detection itself against the case that
// makes this feature work at all: a dispatched run gets /dev/null on stdin,
// /dev/null is a character device, and the os.ModeCharDevice check nav-pilot
// uses elsewhere calls it a terminal. If isTerminal ever goes back to that
// check, every non-interactive launch silently loses --yes again.
func TestIsTerminalRejectsDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	defer f.Close()

	if isTerminal(f) {
		t.Errorf("isTerminal(%s) = true, want false", os.DevNull)
	}
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("stat %s: %v", os.DevNull, err)
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		t.Skip("this platform's /dev/null is not a character device, so it is not the trap this pins")
	}
}

// TestGoldenOpenCodeAgentArgs pins how a legacy opencode launch forwards the
// user's pass-through arguments — which it did not do at all until the
// non-interactive path needed them: `nav-pilot -- run "…"` resolved the request
// and then dropped it, and opencode started its TUI instead.
func TestGoldenOpenCodeAgentArgs(t *testing.T) {
	bind := []string{"--model", "github-copilot/auto", "--agent", "nav-pilot"}

	tests := []struct {
		name  string
		extra []string
		want  []string
	}{
		{
			// The byte-identical case: every launch that has ever worked.
			name: "no pass-through arguments",
			want: bind,
		},
		{
			name:  "flags before a subcommand keep their order",
			extra: []string{"--pure", "run", "add a docstring"},
			want:  append(slices.Clone(bind), "--pure", "run", "add a docstring"),
		},
		{
			// opencode wants its subcommand first, so run leads and the bind
			// follows it — the same rule the staged path applies.
			name:  "run leads",
			extra: []string{"run", "add a docstring"},
			want:  append(append([]string{"run"}, bind...), "add a docstring"),
		},
		{
			// Another opencode subcommand is its own invocation; binding a
			// model and an agent onto `auth login` is nonsense.
			name:  "another subcommand is forwarded alone",
			extra: []string{"models"},
			want:  []string{"models"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := openCodeAgentArgs(domain.ResolvedConfig{ExtraArgs: tt.extra})
			if !slices.Equal(got, tt.want) {
				t.Errorf("openCodeAgentArgs(%q)\n got: %q\nwant: %q", tt.extra, got, tt.want)
			}
		})
	}
}
