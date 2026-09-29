package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The autonomy presets of the setup wizard (#1348). Each maps to keys that
// already exist: nav-pilot's autonomy, cplt's sandbox.preset and cplt's
// git_guard.protect_default_branch_only. Merge, push to main and force push
// stay blocked by cplt in every one of them.
const (
	presetSandbox = "sandbox" // autonomous in the sandbox
	presetAsk     = "ask"     // ask before each command
	presetLocked  = "locked"  // ask, no pushes, allowlist
	presetCustom  = "custom"
)

// autonomyChoice is what a preset, or the custom questions, settle on.
type autonomyChoice struct {
	Autonomy string // nav-pilot autonomy; "" for a client that does not read it
	Preset   string // cplt sandbox.preset
	Push     bool   // the agent may push branches (protect_default_branch_only)
}

// presetChoices is the fixed part of the mapping. Custom is built from answers.
var presetChoices = map[string]autonomyChoice{
	presetSandbox: {Autonomy: "sandbox", Preset: "standard", Push: true},
	presetAsk:     {Autonomy: "conservative", Preset: "standard", Push: true},
	presetLocked:  {Autonomy: "conservative", Preset: cpltStrictPreset, Push: false},
}

// cpltGitState is the part of the cplt config the presets touch. Preset is ""
// when cplt could not say. Protect is the value in the config file, nil when
// the file does not set it and the preset's baseline applies.
type cpltGitState struct {
	Preset  string
	Protect *bool
}

// readCpltGitState asks cplt for the two keys. `cplt config get` prints the
// default, not the preset's baseline, for a key the file does not set, and
// marks it with a "(default" line on stderr; that is how an unset key is told
// apart.
func readCpltGitState(cliPath string) cpltGitState {
	var s cpltGitState
	if out, _, err := cpltConfigGetDefault(cliPath, "sandbox.preset"); err == nil {
		s.Preset = cpltPresetFromConfigGet(out)
	}
	if out, isDefault, err := cpltConfigGetDefault(cliPath, "git_guard.protect_default_branch_only"); err == nil && !isDefault {
		if v, perr := strconv.ParseBool(out); perr == nil {
			s.Protect = &v
		}
	}
	return s
}

// cpltConfigGetDefault reads one key: the value is the first line of stdout,
// and cplt says "(default, not set in config file)" on stderr when the file
// does not set it. Both streams are searched for the marker, stdout only for
// the value, so a warning on stderr can never pose as the value.
func cpltConfigGetDefault(cliPath, key string) (val string, isDefault bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), cpltCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cliPath, "config", "get", key)
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", false, err
	}
	first, _, _ := strings.Cut(string(out), "\n")
	isDefault = bytes.Contains(out, []byte("(default")) || bytes.Contains(stderr.Bytes(), []byte("(default"))
	return strings.TrimSpace(first), isDefault, nil
}

// effectivePush is whether cplt lets the agent push a feature branch: the
// file's key if set, else the preset's baseline (only standard allows it).
// An unknown preset reads as cplt's default, standard.
func (s cpltGitState) effectivePush() bool {
	if s.Protect != nil {
		return *s.Protect
	}
	return s.Preset == "" || s.Preset == "standard"
}

// currentPreset is the preset matching today's settings, or custom. It is the
// wizard's default, so rerunning setup and pressing Enter changes nothing.
func currentPreset(autonomy string, copilot bool, s cpltGitState) string {
	preset := s.Preset
	if preset == "" {
		preset = "standard"
	}
	if !copilot {
		autonomy = "" // not read, so it cannot tell presets apart
	}
	for _, name := range []string{presetSandbox, presetAsk, presetLocked} {
		c := presetChoices[name]
		if !copilot && name == presetAsk {
			continue
		}
		if c.Preset == preset && c.Push == s.effectivePush() && (autonomy == "" || autonomy == c.Autonomy) {
			return name
		}
	}
	return presetCustom
}

// guardedPreset is true for the cplt presets whose git and gh guards are on.
func guardedPreset(p string) bool { return p == "standard" || p == cpltStrictPreset }

// cpltChange is one `cplt config set` the wizard makes.
type cpltChange struct{ Key, From, To string }

// cpltChanges is what it takes to get from s to c, and nothing more: a key
// already at the wanted value is not written.
func cpltChanges(s cpltGitState, c autonomyChoice) []cpltChange {
	var out []cpltChange
	if s.Preset != c.Preset {
		out = append(out, cpltChange{"sandbox.preset", s.Preset, c.Preset})
	}
	// The guard only has a meaning under the two presets that turn it on.
	if !guardedPreset(c.Preset) {
		return out
	}
	after := cpltGitState{Preset: c.Preset, Protect: s.Protect}
	if after.effectivePush() != c.Push {
		from := ""
		if s.Protect != nil {
			from = strconv.FormatBool(*s.Protect)
		}
		out = append(out, cpltChange{"git_guard.protect_default_branch_only", from, strconv.FormatBool(c.Push)})
	}
	return out
}

// askAutonomy is the wizard's autonomy question, and the custom questions
// behind its last option. def is the preselected preset.
func askAutonomy(client string, def string, cur autonomyChoice) (autonomyChoice, error) {
	copilot := client == "copilot"
	choice := def
	opts := []huh.Option[string]{
		huh.NewOption("Autonomous in the sandbox (recommended): commits, pushes branches, opens PRs", presetSandbox),
	}
	locked := "Locked down: no pushes at all, network limited to an allowlist"
	desc := "cplt blocks merging and pushing to main under each preset here. The agent asks you when unsure."
	if !guardedPreset(cur.Preset) {
		desc = "cplt blocks merging and pushing to main under each preset here. Custom can keep your cplt preset " +
			cur.Preset + ", which blocks neither. The agent asks you when unsure."
	}
	if copilot {
		opts = append(opts, huh.NewOption("Ask before each command: same git rules, but Copilot asks first", presetAsk))
		locked = "Locked down: asks first, no pushes at all, network limited to an allowlist"
	} else {
		desc += " " + clientLabel[client] + " keeps its own permission settings"
		if client == "opencode" {
			desc += " (the permission key in opencode.json)"
		}
		desc += "."
	}
	opts = append(opts,
		huh.NewOption(locked, presetLocked),
		huh.NewOption("Custom: choose each setting", presetCustom),
	)
	if err := huh.NewSelect[string]().
		Title("How much should the agent do on its own?").
		Description(desc).
		Options(opts...).
		Value(&choice).
		WithTheme(navTheme()).
		Run(); err != nil {
		return autonomyChoice{}, err
	}
	if choice != presetCustom {
		c := presetChoices[choice]
		if !copilot {
			c.Autonomy = ""
		}
		return c, nil
	}

	c := cur
	if !copilot {
		c.Autonomy = ""
	} else if err := huh.NewSelect[string]().
		Title("Should Copilot ask before each command?").
		Options(
			huh.NewOption("No: it works on its own inside the sandbox and asks when unsure", "sandbox"),
			huh.NewOption("Yes: ask before each command", "conservative"),
		).
		Value(&c.Autonomy).
		WithTheme(navTheme()).
		Run(); err != nil {
		return autonomyChoice{}, err
	}
	netOpts := []huh.Option[string]{
		huh.NewOption("Standard: GitHub, Nav and package hosts reachable", "standard"),
		huh.NewOption("Allowlist only (cplt strict)", cpltStrictPreset),
	}
	if !guardedPreset(c.Preset) {
		netOpts = append(netOpts, huh.NewOption("Keep cplt preset "+c.Preset, c.Preset))
	}
	if err := huh.NewSelect[string]().
		Title("Network").
		Options(netOpts...).
		Value(&c.Preset).
		WithTheme(navTheme()).
		Run(); err != nil {
		return autonomyChoice{}, err
	}
	// Under permissive and full-trust cplt's git guard is off: there is
	// nothing for the git answer to set.
	if !guardedPreset(c.Preset) {
		return c, nil
	}
	if !guardedPreset(cur.Preset) {
		c.Push = true // coming from no guard at all: start from standard's baseline
	}
	if err := huh.NewSelect[bool]().
		Title("What may the agent do with git?").
		Options(
			huh.NewOption("Commit, push branches and open PRs", true),
			huh.NewOption("Commit only; you push", false),
		).
		Value(&c.Push).
		WithTheme(navTheme()).
		Run(); err != nil {
		return autonomyChoice{}, err
	}
	return c, nil
}

// autonomySummary is the one screen shown before anything is written.
// allowlist is the proxy.allowed_domains that stays set under a preset other
// than strict, or "".
func autonomySummary(c autonomyChoice, changes []cpltChange, cpltFound bool, allowlist string) string {
	var b strings.Builder
	if c.Autonomy != "" {
		fmt.Fprintf(&b, "nav-pilot autonomy = %s\n", c.Autonomy)
	}
	if !cpltFound {
		b.WriteString("cplt is not installed: its git and network rules are left for later.\n")
	}
	for _, ch := range changes {
		from, to := ch.From, ch.To
		if from == "" {
			from = "unset"
		}
		if to == "" {
			to = "unset"
		}
		fmt.Fprintf(&b, "cplt %s: %s → %s\n", ch.Key, from, to)
		if ch.Key == "sandbox.preset" && ch.To == cpltStrictPreset {
			b.WriteString("  also points proxy.allowed_domains at nav-pilot's list of Nav hosts\n")
		}
	}
	if cpltFound && len(changes) == 0 {
		b.WriteString("cplt: no changes\n")
	}
	switch {
	case cpltFound && !guardedPreset(c.Preset):
		fmt.Fprintf(&b, "cplt preset %s: its git and gh guards are off, so nothing is blocked, not even pushing to main or merging.\n", c.Preset)
	case !c.Push:
		b.WriteString("The agent cannot push at all: you push.\n")
	}
	if allowlist != "" {
		fmt.Fprintf(&b, "cplt proxy.allowed_domains stays set: only the hosts in %s are reachable.\n", allowlist)
		if allowlist == navAllowedDomainsPath() {
			b.WriteString("  That is nav-pilot's list from strict. To reach every host, run `cplt config set --global --unset proxy.allowed_domains`.\n")
		}
	}
	fmt.Fprintf(&b, "Your other answers go to %s.", configPath())
	return b.String()
}

// leavingStrictAllowlist is the change that drops nav-pilot's own allowlist
// when the user leaves strict, or nil. A list the user chose is theirs: it
// only gets the summary note.
func leavingStrictAllowlist(from, to, allowlist string) *cpltChange {
	if from != cpltStrictPreset || to == cpltStrictPreset || allowlist != navAllowedDomainsPath() {
		return nil
	}
	return &cpltChange{"proxy.allowed_domains", allowlist, ""}
}

// cpltConfigUnset removes one key from the global cplt config. Setting it to
// "" instead leaves an empty allowlist that cplt refuses to launch with.
func cpltConfigUnset(cliPath, key string) error {
	out, err := exec.Command(cliPath, "config", "set", key, "--unset", "--global").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to unset %s: %v\n%s", key, err, string(out))
	}
	return nil
}

// applyCpltChanges writes the changes. A change to "" removes the key. The guard key goes first, so the
// strict preset is never armed with a guard that still allows a push it
// should not; strict itself goes through applyStrictPreset, which seeds the
// Nav hosts before it sets the preset.
func applyCpltChanges(cliPath string, changes []cpltChange, allowlistPath, host string) error {
	for _, ch := range changes {
		if ch.Key == "sandbox.preset" {
			continue
		}
		if ch.To == "" {
			if err := cpltConfigUnset(cliPath, ch.Key); err != nil {
				return err
			}
			fmt.Printf("%s cplt %s removed\n", domain.Green("✓"), ch.Key)
			continue
		}
		if err := cpltConfigSet(cliPath, ch.Key, ch.To); err != nil {
			return err
		}
		fmt.Printf("%s cplt %s = %s\n", domain.Green("✓"), ch.Key, ch.To)
	}
	for _, ch := range changes {
		if ch.Key != "sandbox.preset" {
			continue
		}
		if ch.To == cpltStrictPreset {
			return applyStrictPreset(cliPath, allowlistPath, host)
		}
		if err := cpltConfigSet(cliPath, ch.Key, ch.To); err != nil {
			return err
		}
		fmt.Printf("%s cplt %s = %s\n", domain.Green("✓"), ch.Key, ch.To)
	}
	return nil
}

// ─── GitHub sign-in ──────────────────────────────────────────────────────────

type ghAuth int

const (
	ghAuthUnknown ghAuth = iota // gh ran but did not answer
	ghAuthMissing               // no gh on PATH
	ghAuthNone                  // not signed in, or the token no longer works
	ghAuthOK                    // signed in, token in a file or the environment
	ghAuthKeyring               // signed in, token in the system keyring
)

// ghAuthTimeout bounds `gh auth status`, which asks GitHub about the token.
const ghAuthTimeout = 5 * time.Second

// ghAuthProbe runs `gh auth status`. Setup and doctor only, never a launch.
// A var so tests need no gh.
var ghAuthProbe = func() (ghAuth, string) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return ghAuthMissing, ""
	}
	out, err := runBoundedTimeout(ghAuthTimeout, true, gh, "auth", "status", "--hostname", "github.com")
	return ghAuthFrom(out, err)
}

// ghAuthFrom reads `gh auth status`: exit 0 is signed in, exit 1 is not (or
// a token GitHub rejects), anything else (a kill at the deadline) is unknown.
func ghAuthFrom(out []byte, err error) (ghAuth, string) {
	var ee *exec.ExitError
	switch {
	case err == nil && bytes.Contains(out, []byte("(keyring)")):
		return ghAuthKeyring, ""
	case err == nil:
		return ghAuthOK, ""
	case errors.As(err, &ee) && ee.ExitCode() == 1:
		return ghAuthNone, ""
	default:
		return ghAuthUnknown, err.Error()
	}
}

// keychainBlocked is true where cplt keeps the client from the macOS
// Keychain, which is where gh keeps its token by default (#1348).
func keychainBlocked(client string) bool {
	return runtime.GOOS == "darwin" && (client == "opencode" || client == "pi")
}

// reportGHAuth prints the sign-in result and, when push or PRs would fail,
// the exact command that fixes it.
func reportGHAuth(w io.Writer, indent, client string, st ghAuth, detail string) {
	login := "gh auth login"
	if keychainBlocked(client) {
		login = "gh auth login --insecure-storage"
	}
	switch {
	case st == ghAuthMissing:
		fmt.Fprintf(w, "%s%s gh is not installed; the agent needs it to open PRs. Install it with %s, then run %s\n", indent, yellow("⚠"),
			bold(domain.PkgForInstall().Pick("brew install gh", "sudo apt install gh")), bold(login))
	case st == ghAuthNone:
		fmt.Fprintf(w, "%s%s gh is not signed in to github.com (or its token no longer works), so the agent cannot push or open PRs. Run %s\n", indent, yellow("⚠"), bold(login))
	case st == ghAuthKeyring && keychainBlocked(client):
		fmt.Fprintf(w, "%s%s gh keeps its token in the macOS Keychain, which cplt does not let %s read: push and PRs fail in the sandbox. Run %s\n", indent, yellow("⚠"), clientLabel[client], bold(login))
	case (st == ghAuthOK || st == ghAuthKeyring) && originUsesSSH():
		fmt.Fprintf(w, "%s%s gh is signed in to github.com: the agent can open PRs. This repo's origin uses SSH, which cplt blocks, so pushes fail. Run %s\n", indent, yellow("⚠"),
			bold(`git config --global url."https://github.com/".insteadOf "git@github.com:"`))
	case st == ghAuthOK || st == ghAuthKeyring:
		fmt.Fprintf(w, "%s%s gh is signed in to github.com: the agent can push branches and open PRs\n", indent, green("✓"))
	default:
		fmt.Fprintf(w, "%s%s Could not check the gh sign-in (%s). Run %s to see it.\n", indent, dim("-"), detail, bold("gh auth status"))
	}
}

// originUsesSSH is whether the current repo's origin is an SSH remote. cplt
// denies ~/.ssh and SSH_AUTH_SOCK, so a push over it fails in the sandbox
// whatever gh says. Setup and doctor only. A var so tests need no repo.
var originUsesSSH = func() bool {
	out, err := runBounded("git", "remote", "get-url", "origin")
	if err != nil {
		return false
	}
	u := strings.TrimSpace(string(out))
	return strings.Contains(u, "ssh://") || (!strings.Contains(u, "://") && strings.Contains(u, "@"))
}

// ─── the repo's suggested sandbox rules ──────────────────────────────────────

// offerCpltInit offers `cplt init --write` when the current repository has no
// .cplt.toml and cplt finds tooling to suggest rules for. It never approves
// them: trust is the user's own step.
func offerCpltInit(cliPath string) {
	root := source.FindGitRoot(".")
	if root == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(root, ".cplt.toml")); err == nil {
		return
	}
	preview := exec.Command(cliPath, "init", "-q")
	preview.Dir = root
	out, err := preview.Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return
	}
	detected := ""
	for _, line := range strings.Split(string(out), "\n") {
		if d, ok := strings.CutPrefix(line, "# Detected: "); ok {
			detected = "Detected: " + d + ". "
		}
	}
	yes := true
	if err := huh.NewConfirm().
		Title("Apply the suggested sandbox rules for this repo?").
		Description(detected + "Runs cplt init --write, which writes .cplt.toml. Nothing is approved until you run cplt trust accept.").
		Value(&yes).
		WithTheme(navTheme()).
		Run(); err != nil || !yes {
		return
	}
	write := exec.Command(cliPath, "init", "--write")
	write.Dir = root
	write.Stdout, write.Stderr = os.Stdout, os.Stderr
	if err := write.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s cplt init --write: %v\n", yellow("⚠"), err)
		return
	}
	fmt.Printf("  Review .cplt.toml, then %s and %s to approve its rules.\n", bold("git add .cplt.toml"), bold("cplt trust accept"))
}
