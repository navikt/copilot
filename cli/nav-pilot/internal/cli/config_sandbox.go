package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// findCplt locates the cplt binary. The `copilot` binary is only accepted when
// it really is cplt — plain GitHub Copilot has no `config` subcommand.
func findCplt() (string, error) {
	cliPath, cliName := providerpkg.FindCopilotCLI()
	if cliPath == "" || cliName != "cplt" {
		return "", fmt.Errorf("cplt (Copilot Sandbox) is not available on your PATH. This command requires cplt")
	}
	return cliPath, nil
}

// cpltConfigSet writes one cplt config key.
func cpltConfigSet(cliPath, key, val string) error {
	out, err := exec.Command(cliPath, "config", "set", key, val).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set %s: %v\n%s", key, err, string(out))
	}
	return nil
}

// cmdConfigSandbox runs an interactive wizard to configure the cplt sandbox profile.
func cmdConfigSandbox() error {
	cliPath, err := findCplt()
	if err != nil {
		return err
	}

	var choices []string
	err = huh.NewMultiSelect[string]().
		Title("Configure cplt sandbox relaxations").
		Description("Select which restrictions to lift for agents running under cplt.").
		Options(
			huh.NewOption("Allow Docker (Colima/OrbStack)", "sandbox.allow_docker"),
			huh.NewOption("Allow any localhost port", "sandbox.allow_localhost_any"),
			huh.NewOption("Allow browser access", "sandbox.allow_browser"),
			huh.NewOption("Allow executing /tmp binaries", "sandbox.allow_tmp_exec"),
		).
		Value(&choices).
		WithTheme(navTheme()).
		Run()

	if err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}

	keys := []string{"sandbox.allow_docker", "sandbox.allow_localhost_any", "sandbox.allow_browser", "sandbox.allow_tmp_exec"}
	for _, key := range keys {
		val := "false"
		for _, c := range choices {
			if c == key {
				val = "true"
				break
			}
		}
		if err := cpltConfigSet(cliPath, key, val); err != nil {
			return err
		}
	}

	fmt.Printf("%s Successfully updated cplt sandbox configuration\n", domain.Green("✓"))
	return nil
}

// ─── security posture (sandbox.preset) ───────────────────────────────────────

// cpltRecommendedPreset is the sandbox preset nav-pilot recommends: it turns on
// gh_guard, git_guard and proxy.forced in one key. Individually set keys still
// override it, so it is safe to recommend to users who have tuned cplt already.
const cpltRecommendedPreset = "strict"

// cpltPresets are the values cplt accepts for sandbox.preset. Anything else is
// treated as unknown rather than guessed at.
var cpltPresets = []string{"strict", "standard", "permissive", "full-trust"}

// cpltPresetFromConfigGet extracts the preset from `cplt config get
// sandbox.preset` output: the value is on the first line, and a following
// annotation line such as "[cplt] (default — not set in config file)" is
// ignored. An empty or unrecognised value yields "" — unknown, never guessed at.
func cpltPresetFromConfigGet(out string) string {
	first, _, _ := strings.Cut(out, "\n")
	val := strings.TrimSpace(first)
	if !containsStr(cpltPresets, val) {
		return ""
	}
	return val
}

// cpltSandboxPreset returns the effective cplt sandbox.preset, or "" when it
// cannot be determined — no cplt, a failed command, or a value this build does
// not know. Callers skip their recommendation on "" rather than guessing.
// A var so tests can stub the process spawn.
var cpltSandboxPreset = func() string {
	cliPath, err := findCplt()
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, cliPath, "config", "get", "sandbox.preset").Output()
	if err != nil {
		return ""
	}
	return cpltPresetFromConfigGet(string(out))
}

// cpltRecommendStrict reports whether nav-pilot should nudge the user towards
// the strict preset. An unknown preset is left alone.
func cpltRecommendStrict(preset string) bool {
	return preset != "" && preset != cpltRecommendedPreset
}

// cmdConfigStrictPreset asks for confirmation and sets sandbox.preset = strict.
// cplt config is personal: nav-pilot never sets it silently.
func cmdConfigStrictPreset() error {
	cliPath, err := findCplt()
	if err != nil {
		return err
	}

	var ok bool
	if err := huh.NewConfirm().
		Title("Set cplt sandbox.preset = strict?").
		Description("Turns on gh_guard, git_guard and forced proxy in one key. Keys you set yourself still win.").
		Value(&ok).
		WithTheme(navTheme()).
		Run(); err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}
	if !ok {
		return nil
	}

	if err := cpltConfigSet(cliPath, "sandbox.preset", cpltRecommendedPreset); err != nil {
		return err
	}
	fmt.Printf("%s cplt sandbox.preset = %s\n", domain.Green("✓"), cpltRecommendedPreset)
	return nil
}

// Sandbox enforcement, verified rather than inferred from configuration.
//
// doctor used to check cplt's *setup* only: a version, a preset, and a grep of
// `cplt config show`. That answers "is it configured", never "is it enforcing",
// and the failure mode is on record — an earlier grep here tested a config key
// cplt has never had, so the check reported a problem that could not exist and
// pointed users at a `cplt config set` cplt rejects (#406).
//
// `cplt check` (navikt/cplt#145) runs probes inside the real resolved sandbox —
// the same policy an agent would get — and reports, per probe, whether cplt
// allows or blocks it, why, and the exact fix. `--json` makes it machine-
// readable and the battery exits non-zero when it cannot confirm enforcement.

// cpltCheckReport is the part of a `cplt check --json` battery report doctor
// reads. cplt's Report carries per-probe items with reasons and fixes too;
// doctor prints the verdict and leaves the detail to `cplt check` itself.
type cpltCheckReport struct {
	// Enforcing is true only when every graded expectation held AND at least
	// one protection was actually verified — cplt's own definition.
	Enforcing bool `json:"enforcing"`
	// Verified counts the expected-blocked probes that really were blocked.
	Verified int `json:"verified"`
	// Battery marks the full enforcement battery. A targeted query
	// (`cplt check path …`) is not graded and must never be read as a verdict.
	Battery bool `json:"battery"`
}

// parseCpltCheckReport decodes a battery report, or returns nil for "unknown".
//
// Unknown covers every way this can fail to be an answer: a cplt old enough to
// have no `check` subcommand (clap writes a usage error to stderr and leaves
// stdout empty), no cplt at all, a killed process, or a report that is not the
// graded battery. Unknown is never rendered as enforcing and never as failing —
// a security check that goes green, or red, on missing data is the same bug in
// two directions.
func parseCpltCheckReport(out []byte) *cpltCheckReport {
	var r cpltCheckReport
	if err := json.Unmarshal(out, &r); err != nil || !r.Battery {
		return nil
	}
	return &r
}

// cpltCheckTimeout bounds the enforcement battery. It is longer than
// cpltCommandTimeout because `cplt check` builds the resolved sandbox and spawns
// probes inside it rather than reading a config file — measured at well under a
// second warm, but a cold run after a cplt upgrade does more work.
const cpltCheckTimeout = 15 * time.Second

// cpltEnforcement runs `cplt check --json` in the current directory and returns
// the battery verdict, or nil when it could not be established.
//
// The non-zero exit cplt uses for "not enforcing" is the answer, not a failure:
// the report is on stdout either way, so the error is deliberately ignored and
// the decision left to the decode.
//
// A var so tests can exercise doctor without spawning a sandbox.
var cpltEnforcement = func() *cpltCheckReport {
	cliPath, err := findCplt()
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), cpltCheckTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx, cliPath, "check", "--json").Output()
	return parseCpltCheckReport(out)
}
